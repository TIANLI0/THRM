//go:build windows

package fnqpowermode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"

	"github.com/TIANLI0/BS2PRO-Controller/internal/plugins"
)

type Plugin struct {
	logger interface {
		Debug(string, ...any)
		Error(string, ...any)
		Info(string, ...any)
	}
	onModeChange func(PowerModeState)

	mutex   sync.Mutex
	cancel  context.CancelFunc
	cmd     *exec.Cmd
	running bool
	lastErr string
}

func New(options Options) *Plugin {
	return &Plugin{
		logger:       options.Logger,
		onModeChange: options.OnModeChange,
	}
}

func (p *Plugin) ID() string {
	return PluginID
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Start(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if _, err := exec.LookPath("powershell.exe"); err != nil {
		p.lastErr = err.Error()
		return fmt.Errorf("powershell.exe not found: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.running = true
	p.lastErr = ""

	go p.run(runCtx)
	return nil
}

func (p *Plugin) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return nil
	}
	if p.cancel != nil {
		p.cancel()
	}
	if p.cmd != nil && p.cmd.Process != nil {
		_ = p.cmd.Process.Kill()
	}
	p.running = false
	p.cmd = nil
	p.cancel = nil
	return nil
}

func (p *Plugin) Status() plugins.Status {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return plugins.Status{
		ID:        p.ID(),
		Name:      p.Name(),
		Running:   p.running,
		LastError: p.lastErr,
	}
}

func (p *Plugin) run(ctx context.Context) {
	defer func() {
		p.mutex.Lock()
		p.running = false
		p.cmd = nil
		p.mutex.Unlock()
	}()

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", powerShellListenerScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		p.setLastError(err)
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		p.setLastError(err)
		return
	}

	p.mutex.Lock()
	p.cmd = cmd
	p.mutex.Unlock()

	if err := cmd.Start(); err != nil {
		p.setLastError(err)
		return
	}

	go p.scanErrors(stderr)

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		p.handleLine(scanner.Bytes())
	}
	if err := scanner.Err(); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
		p.setLastError(err)
	}

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		p.setLastError(err)
	}
}

func (p *Plugin) scanErrors(pipe interface{ Read([]byte) (int, error) }) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		p.setLastError(fmt.Errorf("%s", scanner.Text()))
	}
}

func (p *Plugin) handleLine(line []byte) {
	var event struct {
		Type      string `json:"type"`
		Raw       int    `json:"raw"`
		Mapped    int    `json:"mapped"`
		Mode      string `json:"mode"`
		Source    string `json:"source"`
		Timestamp int64  `json:"timestamp"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(line, &event); err != nil {
		p.setLastError(err)
		return
	}

	switch event.Type {
	case "mode":
		state := PowerModeState{
			Raw:       event.Raw,
			Mapped:    event.Mapped,
			Mode:      event.Mode,
			Source:    event.Source,
			Timestamp: event.Timestamp,
		}
		if p.onModeChange != nil {
			p.onModeChange(state)
		}
	case "error":
		p.setLastError(fmt.Errorf("%s", event.Message))
	}
}

func (p *Plugin) setLastError(err error) {
	if err == nil {
		return
	}

	p.mutex.Lock()
	p.lastErr = err.Error()
	p.mutex.Unlock()

	if p.logger != nil {
		p.logger.Debug("Lenovo Legion Fn+Q plugin: %v", err)
	}
}

const powerShellListenerScript = `
$ErrorActionPreference = 'Stop'

$pollIntervalSeconds = 2
$lastRaw = $null
$nextPollAt = [DateTime]::UtcNow
$eventRegistered = $false

function Get-ModeName([int]$mapped) {
    switch ($mapped) {
        0 { 'Quiet'; break }
        1 { 'Balance'; break }
        2 { 'Performance'; break }
        223 { 'Extreme'; break }
        254 { 'GodMode'; break }
        default { 'Unknown' }
    }
}

function Read-SmartFanModeRaw() {
    $data = Get-WmiObject -Namespace 'root\WMI' -Class 'LENOVO_GAMEZONE_DATA' -ErrorAction Stop | Select-Object -First 1
    if ($null -eq $data) {
        throw 'LENOVO_GAMEZONE_DATA not found'
    }

    $result = $data.GetSmartFanMode()
    if ($null -eq $result -or $null -eq $result.Data) {
        throw 'GetSmartFanMode returned empty data'
    }

    return [int]$result.Data
}

function Emit-Mode([int]$raw, [string]$source, [bool]$force) {
    if (-not $force -and $null -ne $script:lastRaw -and $script:lastRaw -eq $raw) {
        return
    }

    $script:lastRaw = $raw
    $mapped = $raw - 1
    [pscustomobject]@{
        type = 'mode'
        raw = $raw
        mapped = $mapped
        mode = Get-ModeName $mapped
        source = $source
        timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    } | ConvertTo-Json -Compress
}

function Emit-Error([string]$message) {
    [pscustomobject]@{
        type = 'error'
        message = $message
        timestamp = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
    } | ConvertTo-Json -Compress
}

try {
    Emit-Mode (Read-SmartFanModeRaw) 'current' $true
} catch {
    Emit-Error ("GetSmartFanMode failed: " + $_.Exception.Message)
}

$sourceIdentifier = "BS2PRO.LenovoLegionFnQ.$PID"
try {
    Register-WmiEvent -Namespace 'root\WMI' -Query 'SELECT * FROM LENOVO_GAMEZONE_SMART_FAN_MODE_EVENT' -SourceIdentifier $sourceIdentifier | Out-Null
    $eventRegistered = $true
} catch {
    Emit-Error ("LENOVO_GAMEZONE_SMART_FAN_MODE_EVENT registration failed: " + $_.Exception.Message)
}

try {
    while ($true) {
        if ($eventRegistered) {
            $event = Wait-Event -SourceIdentifier $sourceIdentifier -Timeout 1
            if ($null -ne $event) {
                try {
                    $raw = [int]$event.SourceEventArgs.NewEvent.Properties['mode'].Value
                    Emit-Mode $raw 'event' $false
                } catch {
                    Emit-Error ("event parse failed: " + $_.Exception.Message)
                } finally {
                    Remove-Event -EventIdentifier $event.EventIdentifier -ErrorAction SilentlyContinue
                }
            }
        } else {
            Start-Sleep -Seconds 1
        }

        $now = [DateTime]::UtcNow
        if ($now -ge $nextPollAt) {
            try {
                Emit-Mode (Read-SmartFanModeRaw) 'poll' $false
            } catch {
                Emit-Error ("GetSmartFanMode poll failed: " + $_.Exception.Message)
            }
            $nextPollAt = $now.AddSeconds($pollIntervalSeconds)
        }
    }
} finally {
    if ($eventRegistered) {
        Unregister-Event -SourceIdentifier $sourceIdentifier -ErrorAction SilentlyContinue
    }
}
`
