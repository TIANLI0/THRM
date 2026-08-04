//go:build linux

package guiapp

import (
	"os/exec"
	"strings"
	"testing"
)

func TestConfigureCoreCommandDropsGUIExplicitSyncWorkaround(t *testing.T) {
	t.Setenv("__NV_DISABLE_EXPLICIT_SYNC", "1")
	t.Setenv("THRM_CORE_ENV_TEST", "kept")
	cmd := exec.Command("thrm-core")
	configureCoreCommand(cmd)

	preserved := false
	for _, variable := range cmd.Env {
		if strings.HasPrefix(variable, "__NV_DISABLE_EXPLICIT_SYNC=") {
			t.Fatalf("core command inherited GUI-only variable %q", variable)
		}
		preserved = preserved || variable == "THRM_CORE_ENV_TEST=kept"
	}
	if !preserved {
		t.Fatal("core command lost unrelated environment variables")
	}
}
