package guiapp

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	gort "runtime"
	"sort"
	"strings"
	"time"

	"github.com/TIANLI0/THRM/internal/appmeta"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type diagnosticManifest struct {
	CreatedAt string         `json:"createdAt"`
	App       string         `json:"app"`
	OS        string         `json:"os"`
	Arch      string         `json:"arch"`
	Debug     map[string]any `json:"debug"`
	Config    AppConfig      `json:"config"`
}

func (a *App) ExportDiagnosticPackage() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("application is not ready")
	}
	name := fmt.Sprintf("THRM-diagnostics-%s.zip", time.Now().Format("20060102-150405"))
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export THRM diagnostics",
		DefaultFilename: name,
		Filters:         []wailsruntime.FileFilter{{DisplayName: "ZIP archive", Pattern: "*.zip"}},
	})
	if err != nil || strings.TrimSpace(path) == "" {
		return "", err
	}

	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(file)
	closeWithError := func(current error) error {
		if zipErr := zw.Close(); current == nil {
			current = zipErr
		}
		if fileErr := file.Close(); current == nil {
			current = fileErr
		}
		return current
	}

	manifest := diagnosticManifest{
		CreatedAt: time.Now().Format(time.RFC3339), App: appmeta.AppName,
		OS: gort.GOOS, Arch: gort.GOARCH, Debug: a.GetDebugInfo(), Config: a.GetConfig(),
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", closeWithError(err)
	}
	entry, err := zw.Create("diagnostics.json")
	if err != nil {
		return "", closeWithError(err)
	}
	if _, err = entry.Write(data); err != nil {
		return "", closeWithError(err)
	}

	exe, _ := os.Executable()
	for _, dir := range []string{filepath.Join(filepath.Dir(exe), "logs"), filepath.Join(filepath.Dir(exe), "bridge", "logs")} {
		_ = addRecentDiagnosticLogs(zw, dir, 8)
	}
	if err := closeWithError(nil); err != nil {
		return "", err
	}
	return path, nil
}

func addRecentDiagnosticLogs(zw *zip.Writer, dir string, limit int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		ii, _ := entries[i].Info()
		jj, _ := entries[j].Info()
		return ii.ModTime().After(jj.ModTime())
	})
	added := 0
	for _, item := range entries {
		if item.IsDir() || added >= limit || !strings.HasSuffix(strings.ToLower(item.Name()), ".log") {
			continue
		}
		src, openErr := os.Open(filepath.Join(dir, item.Name()))
		if openErr != nil {
			continue
		}
		dst, createErr := zw.Create(filepath.Join("logs", filepath.Base(dir)+"-"+item.Name()))
		if createErr == nil {
			_, createErr = io.Copy(dst, io.LimitReader(src, 2<<20))
		}
		_ = src.Close()
		if createErr == nil {
			added++
		}
	}
	return nil
}
