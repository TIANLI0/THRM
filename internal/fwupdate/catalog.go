// Package fwupdate 提供固件来源（官方 API / THRM 托管 / 本地文件）与下载，
// 配合 internal/fwflash 引擎完成 THRM 内的固件更新。
package fwupdate

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 来源标识。
const (
	SourceOfficial = "official"
	SourceCustom   = "custom" // THRM 自定义固件（托管在 GitHub 仓库）
	SourceLocal    = "local"
)

// CustomFirmwareBaseURL 是 THRM 自定义固件清单的 GitHub 原始文件基址。
// 约定清单在 <base>/<deviceCode>.json，内容为 []FirmwareInfo。
const CustomFirmwareBaseURL = "https://raw.githubusercontent.com/TIANLI0/THRM/main/firmware"

// officialFirmwareDomain 是飞智官方固件下载地址所在的域（实际为 api-web.cdn.flydigi.com，仅提供 http）。
const officialFirmwareDomain = "flydigi.com"

// maxFirmwareDownload 是下载固件的大小上限。CH59x 代码区最大 448KiB，
// Intel HEX 文本约为镜像的 2.8 倍，2MiB 足够且能挡住明显不对的文件。
const maxFirmwareDownload = 2 * 1024 * 1024

// FirmwareInfo 描述一个可刷写的固件条目。
type FirmwareInfo struct {
	Source  string `json:"source"`
	Version string `json:"version"`
	Name    string `json:"name"`
	Info    string `json:"info"`
	Date    string `json:"date"`
	URL     string `json:"url"`
	// SHA256 是下载内容（.hex/.bin 文件原始字节）的十六进制 SHA-256。
	// THRM 自定义固件必填，刷写前校验；官方接口不提供，为空。
	SHA256 string `json:"sha256,omitempty"`
}

// DeviceCode 把 USB ProductID 映射成飞智 API 的 device_code。
func DeviceCode(productID uint16) string {
	switch productID {
	case 0x1002:
		return "bs2pro"
	case 0x1004:
		return "bs3pro"
	case 0x1001:
		return "bs2"
	case 0x1003:
		return "bs3"
	default:
		return ""
	}
}

func httpClient() *http.Client { return &http.Client{Timeout: 20 * time.Second} }

// FetchOfficial 查询飞智官方更新接口，返回该设备当前可下载的最新固件。
// 传一个很旧的 main_chip 以强制服务器返回最新包（见抓包分析）。
func FetchOfficial(ctx context.Context, deviceCode string) (*FirmwareInfo, error) {
	if deviceCode == "" {
		return nil, fmt.Errorf("未知设备，无法查询官方固件")
	}
	body, _ := json.Marshal(map[string]any{
		"device_code": deviceCode,
		"device_id":   0,
		"app_version": "",
		"main_chip":   "0.0.1.0",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.flydigi.com/pc/Update/firmware", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接官方服务器失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("官方服务器返回 HTTP %d", resp.StatusCode)
	}

	var out struct {
		Code int `json:"code"`
		Data struct {
			DeviceCode string `json:"device_code"`
			ChipList   struct {
				MainChip *struct {
					Version string `json:"version"`
					URL     string `json:"url"`
					Info    string `json:"info"`
				} `json:"main_chip"`
			} `json:"chip_list"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("解析官方响应失败: %w", err)
	}
	mc := out.Data.ChipList.MainChip
	if mc == nil || mc.URL == "" {
		return nil, fmt.Errorf("官方暂无该设备的固件")
	}
	if err := ValidateURL(SourceOfficial, mc.URL); err != nil {
		return nil, err
	}
	return &FirmwareInfo{
		Source:  SourceOfficial,
		Version: mc.Version,
		Name:    fmt.Sprintf("%s_%s", strings.ToUpper(deviceCode), mc.Version),
		Info:    mc.Info,
		URL:     mc.URL,
	}, nil
}

// FetchCustom 从 GitHub 仓库拉该设备的 THRM 自定义固件清单。
// 约定: <base>/<deviceCode>.json 返回 []FirmwareInfo。源不可用时返回空列表而非报错。
// 地址不在仓库 firmware/ 目录下或缺少 sha256 的条目无法通过刷写前校验，直接丢弃。
func FetchCustom(ctx context.Context, baseURL, deviceCode string) ([]FirmwareInfo, error) {
	if baseURL == "" {
		baseURL = CustomFirmwareBaseURL
	}
	if deviceCode == "" {
		return nil, nil
	}
	url := strings.TrimRight(baseURL, "/") + "/" + deviceCode + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, nil // 源不可用不算错误
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	var list []FirmwareInfo
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, nil
	}
	valid := list[:0]
	for _, fw := range list {
		fw.Source = SourceCustom
		fw.SHA256 = strings.ToLower(strings.TrimSpace(fw.SHA256))
		if ValidateURL(SourceCustom, fw.URL) != nil || !isSHA256Hex(fw.SHA256) {
			continue
		}
		valid = append(valid, fw)
	}
	return valid, nil
}

// ValidateURL 检查固件下载地址是否属于该来源允许的位置。
//   - 官方：飞智域名下的 http/https 地址（官方 CDN 只提供 http）。
//   - 自定义：必须是本仓库 firmware/ 目录下的 https 原始文件地址。
func ValidateURL(source, rawURL string) error {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("固件下载地址无效")
	}
	host := strings.ToLower(u.Hostname())
	switch source {
	case SourceOfficial:
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("固件下载地址协议不受支持: %s", u.Scheme)
		}
		if host != officialFirmwareDomain && !strings.HasSuffix(host, "."+officialFirmwareDomain) {
			return fmt.Errorf("官方固件下载地址不在飞智域名下: %s", host)
		}
	case SourceCustom:
		if !strings.HasPrefix(u.String(), CustomFirmwareBaseURL+"/") || strings.Contains(u.Path, "..") {
			return fmt.Errorf("自定义固件下载地址不在 THRM 仓库的 firmware 目录下")
		}
	default:
		return fmt.Errorf("未知的固件来源: %q", source)
	}
	return nil
}

func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// VerifySHA256 校验下载内容与清单里的 SHA-256 是否一致。
func VerifySHA256(data []byte, want string) error {
	want = strings.ToLower(strings.TrimSpace(want))
	if !isSHA256Hex(want) {
		return fmt.Errorf("固件清单缺少有效的 sha256，拒绝刷写")
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != want {
		return fmt.Errorf("固件校验失败：sha256 为 %s，清单为 %s", got, want)
	}
	return nil
}

// Download 下载固件内容（.hex/.bin 原始字节）。调用方应先用 ValidateURL 检查地址，
// 下载后对有 sha256 的来源用 VerifySHA256 校验。
func Download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载固件失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载固件失败: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxFirmwareDownload+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("下载到的固件为空")
	}
	if len(data) > maxFirmwareDownload {
		return nil, fmt.Errorf("下载到的固件超过 %d 字节上限", maxFirmwareDownload)
	}
	return data, nil
}
