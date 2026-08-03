//go:build !windows

package config

// Unix package-managed locations are read-only to normal users. Failure to
// resolve or write the per-user configuration directory must not turn into an
// attempted write beside the executable.
func platformInstallConfigWriteDir(string) string {
	return ""
}
