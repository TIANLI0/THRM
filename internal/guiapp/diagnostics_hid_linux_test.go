//go:build linux

package guiapp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseHIDUevent(t *testing.T) {
	hidID, name, phys := parseHIDUevent("DRIVER=hid-generic\nHID_ID=0005:000037D7:00001002\nHID_NAME=BS2PRO\nHID_PHYS=aa:bb\n")
	if hidID != "0005:000037D7:00001002" || name != "BS2PRO" || phys != "aa:bb" {
		t.Fatalf("parseHIDUevent = (%q, %q, %q)", hidID, name, phys)
	}
	if bus := hidBusName(hidID); bus != "bluetooth" {
		t.Fatalf("hidBusName = %q, want bluetooth", bus)
	}
	if !isFlydigiHIDID(hidID) || isFlydigiHIDID("0003:0000046D:0000C52B") {
		t.Fatal("isFlydigiHIDID mismatch")
	}
}

func TestCollectDiagnosticHIDSnapshotListsFlydigiFirst(t *testing.T) {
	root := t.TempDir()
	sysDir := filepath.Join(root, "sys")
	devDir := filepath.Join(root, "dev")
	for node, uevent := range map[string]string{
		"hidraw0": "HID_ID=0003:0000046D:0000C52B\nHID_NAME=Mouse\n",
		"hidraw1": "HID_ID=0003:000037D7:00001002\nHID_NAME=BS2PRO\n",
	} {
		dir := filepath.Join(sysDir, node, "device")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "uevent"), []byte(uevent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(devDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(devDir, "hidraw1"), nil, 0o600); err != nil {
		t.Fatal(err)
	}

	oldSys, oldDev, oldRules := diagnosticHIDRawSysDir, diagnosticDevDir, diagnosticUdevRuleDirs
	diagnosticHIDRawSysDir, diagnosticDevDir, diagnosticUdevRuleDirs = sysDir, devDir, []string{filepath.Join(root, "rules")}
	t.Cleanup(func() { diagnosticHIDRawSysDir, diagnosticDevDir, diagnosticUdevRuleDirs = oldSys, oldDev, oldRules })

	snapshot := collectDiagnosticHIDSnapshot()
	if len(snapshot.HIDRaw) != 2 {
		t.Fatalf("hidraw count = %d, want 2", len(snapshot.HIDRaw))
	}
	first := snapshot.HIDRaw[0]
	if first.Node != "hidraw1" || !first.Flydigi || first.Bus != "usb" || first.Mode != "0600" || !first.ReadWrite {
		t.Fatalf("first entry = %+v", first)
	}
	if snapshot.HIDRaw[1].Error == "" {
		t.Fatal("missing /dev node should be reported")
	}
	if len(snapshot.UdevRules) != 0 {
		t.Fatalf("udevRules = %v, want empty", snapshot.UdevRules)
	}
}
