//go:build darwin

package coreapp

import (
	"reflect"
	"testing"
)

func TestGUILaunchCommand(t *testing.T) {
	cmd := guiLaunchCommand("/Applications/THRM.app/Contents/MacOS/THRM")
	if want := []string{"/usr/bin/open", "/Applications/THRM.app"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("bundle args = %v, want %v", cmd.Args, want)
	}
	cmd = guiLaunchCommand("/Users/dev/THRM/build/bin/THRM")
	if want := []string{"/Users/dev/THRM/build/bin/THRM"}; !reflect.DeepEqual(cmd.Args, want) {
		t.Fatalf("bare args = %v, want %v", cmd.Args, want)
	}
}
