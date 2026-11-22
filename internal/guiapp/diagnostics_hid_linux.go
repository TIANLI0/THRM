//go:build linux

package guiapp

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// 诊断包里的 Linux HID 快照（issue #55）：日志只会说"未找到受支持的 HID 设备"，
// 分不清是设备根本没出现在 hidraw 下（蓝牙未配对 / 未连接），还是出现了但权限不足
// （udev 规则没装或没生效）。这里只读 sysfs 与文件元数据，不打开设备。

const flydigiHIDVendorHex = "000037D7"

var (
	diagnosticHIDRawSysDir = "/sys/class/hidraw"
	diagnosticDevDir       = "/dev"
	diagnosticUdevRuleDirs = []string{"/etc/udev/rules.d", "/usr/lib/udev/rules.d", "/lib/udev/rules.d"}
)

const diagnosticUdevRuleName = "99-flydigi-fan.rules"

type diagnosticHIDRaw struct {
	Node      string `json:"node"`
	HIDID     string `json:"hidId"`
	Bus       string `json:"bus"`
	Name      string `json:"name,omitempty"`
	Phys      string `json:"phys,omitempty"`
	Flydigi   bool   `json:"flydigi"`
	Mode      string `json:"mode,omitempty"`
	UID       int    `json:"uid"`
	GID       int    `json:"gid"`
	ReadWrite bool   `json:"readWrite"`
	Error     string `json:"error,omitempty"`
}

type diagnosticHIDSnapshot struct {
	UID       int                `json:"uid"`
	Groups    []int              `json:"groups"`
	UdevRules []string           `json:"udevRules"`
	HIDRaw    []diagnosticHIDRaw `json:"hidraw"`
	ListError string             `json:"listError,omitempty"`
}

// parseHIDUevent 解析 hidraw 父设备的 uevent（HID_ID=0005:000037D7:00001002 等）。
func parseHIDUevent(data string) (hidID, name, phys string) {
	for _, line := range strings.Split(data, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "HID_ID":
			hidID = value
		case "HID_NAME":
			name = value
		case "HID_PHYS":
			phys = value
		}
	}
	return hidID, name, phys
}

func hidBusName(hidID string) string {
	bus, _, _ := strings.Cut(hidID, ":")
	switch strings.ToUpper(bus) {
	case "0003":
		return "usb"
	case "0005":
		return "bluetooth"
	case "0018":
		return "i2c"
	case "":
		return ""
	default:
		return bus
	}
}

func isFlydigiHIDID(hidID string) bool {
	parts := strings.Split(hidID, ":")
	return len(parts) >= 2 && strings.EqualFold(parts[1], flydigiHIDVendorHex)
}

func collectDiagnosticHIDSnapshot() diagnosticHIDSnapshot {
	snapshot := diagnosticHIDSnapshot{UID: os.Getuid(), UdevRules: []string{}, HIDRaw: []diagnosticHIDRaw{}}
	snapshot.Groups, _ = os.Getgroups()

	for _, dir := range diagnosticUdevRuleDirs {
		path := filepath.Join(dir, diagnosticUdevRuleName)
		if _, err := os.Stat(path); err == nil {
			snapshot.UdevRules = append(snapshot.UdevRules, path)
		}
	}

	entries, err := os.ReadDir(diagnosticHIDRawSysDir)
	if err != nil {
		snapshot.ListError = err.Error()
		return snapshot
	}
	for _, entry := range entries {
		node := entry.Name()
		item := diagnosticHIDRaw{Node: node, UID: -1, GID: -1}
		if data, readErr := os.ReadFile(filepath.Join(diagnosticHIDRawSysDir, node, "device", "uevent")); readErr == nil {
			item.HIDID, item.Name, item.Phys = parseHIDUevent(string(data))
		} else {
			item.Error = readErr.Error()
		}
		item.Bus = hidBusName(item.HIDID)
		item.Flydigi = isFlydigiHIDID(item.HIDID)

		devPath := filepath.Join(diagnosticDevDir, node)
		if info, statErr := os.Stat(devPath); statErr == nil {
			item.Mode = fmt.Sprintf("%04o", info.Mode().Perm())
			if st, ok := info.Sys().(*syscall.Stat_t); ok {
				item.UID, item.GID = int(st.Uid), int(st.Gid)
			}
			item.ReadWrite = unix.Access(devPath, unix.R_OK|unix.W_OK) == nil
		} else if item.Error == "" {
			item.Error = statErr.Error()
		}
		snapshot.HIDRaw = append(snapshot.HIDRaw, item)
	}
	sort.SliceStable(snapshot.HIDRaw, func(i, j int) bool {
		return snapshot.HIDRaw[i].Flydigi && !snapshot.HIDRaw[j].Flydigi
	})
	return snapshot
}

func addDiagnosticHIDSnapshot(archive *zip.Writer) error {
	data, err := json.MarshalIndent(collectDiagnosticHIDSnapshot(), "", "  ")
	if err != nil {
		return err
	}
	entry, err := archive.Create("linux-hid.json")
	if err != nil {
		return err
	}
	_, err = entry.Write(data)
	return err
}
