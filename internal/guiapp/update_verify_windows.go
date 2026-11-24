//go:build windows

package guiapp

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modcrypt32            = windows.NewLazySystemDLL("crypt32.dll")
	procCryptMsgGetParam  = modcrypt32.NewProc("CryptMsgGetParam")
	procCryptMsgClose     = modcrypt32.NewProc("CryptMsgClose")
	errInstallerUnsigned  = errors.New("安装包没有有效的数字签名")
	errInstallerPublisher = errors.New("安装包的签名者与当前程序不一致")
)

const cmsgSignerCertInfoParam = 7 // CMSG_SIGNER_CERT_INFO_PARAM

// updateDirSDDL 只允许 Administrators 与 SYSTEM 访问，且不继承父目录的权限。
const updateDirSDDL = "O:BAG:BAD:P(A;OICI;FA;;;BA)(A;OICI;FA;;;SY)"

// signerIdentity 是 Authenticode 签名证书里用于比对发布者的字段。
type signerIdentity struct {
	CommonName   string
	Organization string
}

func (s signerIdentity) String() string {
	if s.Organization == "" || s.Organization == s.CommonName {
		return s.CommonName
	}
	return s.CommonName + " (" + s.Organization + ")"
}

// verifyAuthenticode 用系统信任链校验文件的内嵌 Authenticode 签名（含时间戳）。
// 不做在线吊销检查：证书吊销列表在部分网络下不可达，会让更新卡住或失败。
func verifyAuthenticode(path string) error {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	fileInfo := &windows.WinTrustFileInfo{
		Size:     uint32(unsafe.Sizeof(windows.WinTrustFileInfo{})),
		FilePath: path16,
	}
	data := &windows.WinTrustData{
		Size:                            uint32(unsafe.Sizeof(windows.WinTrustData{})),
		UIChoice:                        windows.WTD_UI_NONE,
		RevocationChecks:                windows.WTD_REVOKE_NONE,
		UnionChoice:                     windows.WTD_CHOICE_FILE,
		StateAction:                     windows.WTD_STATEACTION_VERIFY,
		FileOrCatalogOrBlobOrSgnrOrCert: unsafe.Pointer(fileInfo),
	}
	verifyErr := windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	data.StateAction = windows.WTD_STATEACTION_CLOSE
	_ = windows.WinVerifyTrustEx(windows.InvalidHWND, &windows.WINTRUST_ACTION_GENERIC_VERIFY_V2, data)
	return verifyErr
}

// authenticodeSigner 读出文件内嵌签名的签名者证书身份。只负责取证书，
// 不校验信任链，调用前应先通过 verifyAuthenticode。
func authenticodeSigner(path string) (signerIdentity, error) {
	path16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return signerIdentity{}, err
	}
	var encoding uint32
	var store, msg windows.Handle
	err = windows.CryptQueryObject(
		windows.CERT_QUERY_OBJECT_FILE,
		unsafe.Pointer(path16),
		windows.CERT_QUERY_CONTENT_FLAG_PKCS7_SIGNED_EMBED,
		windows.CERT_QUERY_FORMAT_FLAG_BINARY,
		0, &encoding, nil, nil, &store, &msg, nil,
	)
	if err != nil {
		return signerIdentity{}, fmt.Errorf("读取签名失败: %w", err)
	}
	defer windows.CertCloseStore(store, 0)
	defer procCryptMsgClose.Call(uintptr(msg))

	var size uint32
	r, _, callErr := procCryptMsgGetParam.Call(uintptr(msg), cmsgSignerCertInfoParam, 0, 0, uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return signerIdentity{}, fmt.Errorf("读取签名者信息失败: %w", callErr)
	}
	// 用 uint64 切片保证 CERT_INFO 所需的指针对齐。
	buf := make([]uint64, (size+7)/8)
	r, _, callErr = procCryptMsgGetParam.Call(uintptr(msg), cmsgSignerCertInfoParam, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 {
		return signerIdentity{}, fmt.Errorf("读取签名者信息失败: %w", callErr)
	}

	certCtx, err := windows.CertFindCertificateInStore(store, encoding, 0, windows.CERT_FIND_SUBJECT_CERT, unsafe.Pointer(&buf[0]), nil)
	if err != nil {
		return signerIdentity{}, fmt.Errorf("找不到签名者证书: %w", err)
	}
	defer windows.CertFreeCertificateContext(certCtx)

	der := unsafe.Slice(certCtx.EncodedCert, certCtx.Length)
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return signerIdentity{}, fmt.Errorf("解析签名者证书失败: %w", err)
	}
	return signerIdentity{
		CommonName:   cert.Subject.CommonName,
		Organization: strings.Join(cert.Subject.Organization, ","),
	}, nil
}

// verifyUpdateInstaller 在提权运行安装包之前确认它确实出自 THRM 的发布流程：
//   - 安装包必须带有被系统信任的有效 Authenticode 签名；
//   - 当前程序自身已签名时（所有正式发布与预览发布都是），安装包的签名者必须与之一致。
//     比对的是证书主体而非指纹，证书续期后仍然成立。
//
// 当前程序未签名（本地构建、PR 构建产物）时无从比对发布者，只要求签名有效。
func verifyUpdateInstaller(installerPath, selfPath string) error {
	if err := verifyAuthenticode(installerPath); err != nil {
		return fmt.Errorf("%w: %v", errInstallerUnsigned, err)
	}
	signer, err := authenticodeSigner(installerPath)
	if err != nil {
		return fmt.Errorf("%w: %v", errInstallerUnsigned, err)
	}

	if selfPath == "" || verifyAuthenticode(selfPath) != nil {
		guiLogger.Warnf("当前程序未签名，仅校验安装包签名有效（签名者: %s）", signer)
		return nil
	}
	self, err := authenticodeSigner(selfPath)
	if err != nil {
		guiLogger.Warnf("读取当前程序签名者失败，仅校验安装包签名有效: %v", err)
		return nil
	}
	if signer != self {
		return fmt.Errorf("%w: 安装包为 %s，当前程序为 %s", errInstallerPublisher, signer, self)
	}
	return nil
}

// createUpdateDir 建一个全新的、随机命名的目录放安装包与更新脚本。
//
// GUI 以管理员权限运行，更新脚本和安装包都会在提权状态下被执行。如果放在用户的
// %TEMP% 里，同一用户下任何未提权的进程都能在"校验通过"到"开始执行"之间把文件换掉。
// 因此提权时放到 %ProgramData% 下，目录 ACL 只给 Administrators 和 SYSTEM：
// 普通进程既改不了目录里的文件，也没有权限把目录本身改名或删除。
func createUpdateDir() (string, error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		// 未提权时没有可被借用的权限，用普通临时目录即可。
		return os.MkdirTemp("", "THRM-update-")
	}

	base, err := windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
	if err != nil {
		return "", fmt.Errorf("定位 ProgramData 失败: %w", err)
	}
	cleanupStaleUpdateDirs(base)

	sd, err := windows.SecurityDescriptorFromString(updateDirSDDL)
	if err != nil {
		return "", err
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	dir := filepath.Join(base, "THRM-update-"+hex.EncodeToString(suffix[:]))
	dir16, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return "", err
	}
	// CreateDirectory 在目录已存在时失败，不会沿用别人预先建好的目录。
	if err := windows.CreateDirectory(dir16, sa); err != nil {
		return "", fmt.Errorf("创建更新目录失败: %w", err)
	}
	return dir, nil
}

// cleanupStaleUpdateDirs 尽力清掉以前更新留下的目录。
func cleanupStaleUpdateDirs(base string) {
	matches, _ := filepath.Glob(filepath.Join(base, "THRM-update-*"))
	for _, m := range matches {
		_ = os.RemoveAll(m)
	}
}
