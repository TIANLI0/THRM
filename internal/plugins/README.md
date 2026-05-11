# Plugins

Optional background integrations live under `internal/plugins`.

Each plugin implements the `plugins.Plugin` lifecycle:

- `Start(ctx)` starts background work.
- `Stop()` releases resources.
- `Status()` reports runtime state for `GetDebugInfo`.

## Lenovo Legion Fn+Q Power Mode

Package: `internal/plugins/fnqpowermode`

The plugin listens for Lenovo Legion power-mode changes produced by `Fn+Q`.

Windows implementation:

- Reads current mode with `root\WMI`, class `LENOVO_GAMEZONE_DATA`, method `GetSmartFanMode`.
- Listens to `root\WMI` event query `SELECT * FROM LENOVO_GAMEZONE_SMART_FAN_MODE_EVENT`.
- Extracts event field `mode`.
- Converts values with `mapped = raw - 1`.

Mapping:

- `0` -> `Quiet`
- `1` -> `Balance`
- `2` -> `Performance`
- `223` -> `Extreme`
- `254` -> `GodMode`

Core broadcasts updates through IPC event `legion-power-mode-update`.

Payload:

```json
{
  "raw": 2,
  "mapped": 1,
  "mode": "Balance",
  "source": "event",
  "timestamp": 1760000000
}
```
