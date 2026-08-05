//go:build windows

package rtss

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

var (
	ErrRTSSNotInstalled = errors.New("未找到 RTSS 安装目录")
)

func InspectLayout() LayoutStatus {
	installPath := findInstallPath()
	status := LayoutStatus{Supported: true, Installed: installPath != "", InstallPath: installPath, AnchorIndex: -1}
	if installPath == "" {
		return status
	}
	status.ConfigPath = filepath.Join(installPath, "Plugins", "Client", "OverlayEditor.cfg")
	data, err := os.ReadFile(status.ConfigPath)
	if err != nil {
		return status
	}
	layoutName := readSettingsValue(data, "Layout")
	status.LayoutName = layoutName
	if layoutName == "" {
		return status
	}
	overlayDir := filepath.Join(installPath, "Plugins", "Client", "Overlays")
	layoutPath, ok := safeLayoutPath(overlayDir, layoutName)
	if !ok {
		return status
	}
	status.LayoutPath = layoutPath
	layoutData, err := os.ReadFile(status.LayoutPath)
	if err != nil {
		return status
	}
	status.AnchorState, status.AnchorIndex, status.LayerCount = inspectOverlayLayout(layoutData)
	return status
}

func safeLayoutPath(overlayDir, layoutName string) (string, bool) {
	path := filepath.Clean(filepath.Join(overlayDir, layoutName))
	relative, err := filepath.Rel(overlayDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return path, true
}

func CreateAnchor() (LayoutStatus, error) {
	status := InspectLayout()
	if !status.Installed || status.LayoutPath == "" {
		return status, ErrRTSSNotInstalled
	}
	if status.AnchorState == anchorStateConfirmed {
		return status, nil
	}
	data, err := os.ReadFile(status.LayoutPath)
	if err != nil {
		return status, fmt.Errorf("读取 RTSS 布局失败: %w", err)
	}
	updated, err := configureAnchor(data)
	if err != nil {
		return status, err
	}
	backupPath, err := backupLayout(status.LayoutPath, data)
	if err != nil {
		return status, fmt.Errorf("备份 RTSS 布局失败: %w", err)
	}
	if err := writeLayoutAtomically(status.LayoutPath, updated); err != nil {
		return status, fmt.Errorf("写入 RTSS 布局失败（原文件备份于 %s）: %w", backupPath, err)
	}
	result := InspectLayout()
	result.BackupPath = backupPath
	return result, nil
}

func readSettingsValue(data []byte, wanted string) string {
	inSettings := false
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inSettings = strings.EqualFold(line, "[Settings]")
			continue
		}
		if !inSettings {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), wanted) {
			return unquote(strings.TrimSpace(value))
		}
	}
	return ""
}

func backupLayout(path string, data []byte) (string, error) {
	stamp := time.Now().Format("20060102-150405")
	backup := path + ".bak-" + stamp
	for suffix := 1; ; suffix++ {
		candidate := backup
		if suffix > 1 {
			candidate = fmt.Sprintf("%s-%d", backup, suffix)
		}
		file, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return "", err
		}
		_, copyErr := io.Copy(file, strings.NewReader(string(data)))
		closeErr := file.Close()
		if copyErr != nil {
			_ = os.Remove(candidate)
			return "", copyErr
		}
		if closeErr != nil {
			_ = os.Remove(candidate)
			return "", closeErr
		}
		return candidate, nil
	}
}

func writeLayoutAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".thrm-rtss-*.ovl")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	// Windows cannot replace an existing file with os.Rename. The original is
	// already backed up, so replacing it after a successful temp write is safe.
	if err := os.Remove(path); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func findInstallPath() string {
	var candidates []string
	if configured := os.Getenv("RTSS_INSTALL_PATH"); configured != "" {
		candidates = append(candidates, configured)
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if root != "" {
			candidates = append(candidates, filepath.Join(root, "RivaTuner Statistics Server"))
		}
	}
	registryLocations := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\RTSS`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\RivaTuner Statistics Server`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\RTSS`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\RivaTuner Statistics Server`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\RTSS`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\RivaTuner Statistics Server`},
	}
	for _, location := range registryLocations {
		key, err := registry.OpenKey(location.root, location.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		for _, valueName := range []string{"InstallLocation", "DisplayIcon", "UninstallString"} {
			value, _, err := key.GetStringValue(valueName)
			if err == nil && value != "" {
				if valueName != "InstallLocation" {
					value = executableDirectory(value)
				}
				candidates = append(candidates, value)
			}
		}
		_ = key.Close()
	}
	for _, candidate := range candidates {
		candidate = filepath.Clean(candidate)
		if _, err := os.Stat(filepath.Join(candidate, "Plugins", "Client", "OverlayEditor.cfg")); err == nil {
			return candidate
		}
	}
	return ""
}

func executableDirectory(command string) string {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, "\"") {
		if end := strings.Index(command[1:], "\""); end >= 0 {
			command = command[1 : end+1]
		}
	} else if fields := strings.Fields(command); len(fields) > 0 {
		command = fields[0]
	}
	command = strings.TrimSuffix(strings.Trim(command, "\""), ",0")
	return filepath.Dir(command)
}
