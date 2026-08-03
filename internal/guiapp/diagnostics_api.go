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
	"github.com/TIANLI0/THRM/internal/types"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type diagnosticManifest struct {
	CreatedAt string             `json:"createdAt"`
	App       string             `json:"app"`
	OS        string             `json:"os"`
	Arch      string             `json:"arch"`
	NumCPU    int                `json:"numCpu"`
	Hardware  diagnosticHardware `json:"hardware"`
	Debug     map[string]any     `json:"debug"`
	Config    AppConfig          `json:"config"`
}

// diagnosticHardware 记录本机 CPU/GPU 的识别结果与读数：排查温控问题先要确认读到的是
// 哪颗 CPU/哪块 GPU、哪些传感器可用、读数是否有效。
type diagnosticHardware struct {
	CPUModel string `json:"cpuModel"`
	GPUModel string `json:"gpuModel"`

	CPUTemp  int     `json:"cpuTemp"`
	GPUTemp  int     `json:"gpuTemp"`
	CPUPower float64 `json:"cpuPower"`
	GPUPower float64 `json:"gpuPower"`
	// 笔记本内置风扇转速，0 表示本机读不到。
	CPUFanRPM int `json:"cpuFanRpm"`
	GPUFanRPM int `json:"gpuFanRpm"`

	ControlTemp   int    `json:"controlTemp"`
	ControlSource string `json:"controlSource"`

	SelectedGPUDevice string                       `json:"selectedGpuDevice"`
	GPUDevices        []types.TemperatureGPUDevice `json:"gpuDevices"`
	CPUSensors        []types.TemperatureSensor    `json:"cpuSensors"`
	GPUSensors        []types.TemperatureSensor    `json:"gpuSensors"`
	CPUPowerSensors   []types.PowerSensor          `json:"cpuPowerSensors"`
	GPUPowerSensors   []types.PowerSensor          `json:"gpuPowerSensors"`

	UpdateTime    int64          `json:"updateTime"`
	BridgeOK      bool           `json:"bridgeOk"`
	BridgeMessage string         `json:"bridgeMessage"`
	CPUTempError  string         `json:"cpuTempError"`
	Bridge        map[string]any `json:"bridge"`
}

// collectDiagnosticHardware 汇总 CPU/GPU 识别结果。核心不可用时照原样写入零值快照，
// "读不到"本身就是诊断包需要保留的信息。
func (a *App) collectDiagnosticHardware() diagnosticHardware {
	temp := a.GetTemperature()
	return diagnosticHardware{
		CPUModel:          temp.CpuModel,
		GPUModel:          temp.GpuModel,
		CPUTemp:           temp.CPUTemp,
		GPUTemp:           temp.GPUTemp,
		CPUPower:          temp.CPUPower,
		GPUPower:          temp.GPUPower,
		CPUFanRPM:         temp.CPUFanRPM,
		GPUFanRPM:         temp.GPUFanRPM,
		ControlTemp:       temp.ControlTemp,
		ControlSource:     temp.ControlSource,
		SelectedGPUDevice: temp.SelectedGpuDevice,
		GPUDevices:        temp.GpuDevices,
		CPUSensors:        temp.CpuSensors,
		GPUSensors:        temp.GpuSensors,
		CPUPowerSensors:   temp.CpuPowerSensors,
		GPUPowerSensors:   temp.GpuPowerSensors,
		UpdateTime:        temp.UpdateTime,
		BridgeOK:          temp.BridgeOk,
		BridgeMessage:     temp.BridgeMsg,
		CPUTempError:      temp.CPUTempError,
		Bridge:            a.GetBridgeProgramStatus(),
	}
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
		OS: gort.GOOS, Arch: gort.GOARCH, NumCPU: gort.NumCPU(),
		Hardware: a.collectDiagnosticHardware(),
		Debug:    a.GetDebugInfo(), Config: a.GetConfig(),
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
	_ = addPlatformDiagnosticLogs(zw)
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
