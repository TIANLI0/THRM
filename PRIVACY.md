# Privacy Policy

*中文摘要：THRM 不收集、不上传、不出售任何个人数据。所有配置、日志和温度历史都只保存在你自己的电脑上。程序唯一的网络连接是向 GitHub 检查新版本。*

Last updated: 2026-08-07

## Summary

**THRM does not collect, transmit, or sell any personal data.**

THRM has no analytics, no telemetry, no crash reporting, no advertising identifiers, and no
user accounts. There is no THRM server. All data the application produces stays on your
computer.

## Data stored on your device

THRM stores the following locally. None of it leaves your machine unless you choose to share it.

| Data | Contents | Location |
| --- | --- | --- |
| Configuration | Fan curves, profiles, RGB settings, application preferences | Windows: application data / installation directory · Linux: XDG state directory |
| Logs | Diagnostic messages produced at runtime | `logs/` under the same directory · Linux may use `journald` |
| Temperature history | Recorded CPU/GPU/device temperature and fan speed samples | `telemetry/history.bin` under the data directory |

Despite its filename, `telemetry/history.bin` is a **local-only** temperature history file used
to draw charts in the interface. It is never uploaded.

## Data read from your system

To do its job, THRM reads hardware and system information locally, including CPU and GPU
temperatures, fan speeds, power draw, connected HID/BLE device identifiers, and the name of the
foreground process when per-application profiles are enabled. This information is used only to
drive fan control and the on-screen display. It is not stored beyond the local files listed
above and is never transmitted.

## Network connections

THRM makes network requests to exactly one destination:

| Destination | Purpose | Data sent |
| --- | --- | --- |
| `github.com` | Check for a newer release, and download the installer if you choose to update | Nothing beyond what any HTTPS request necessarily reveals: your IP address and a standard user agent |

No identifier, hardware profile, or usage data is attached to this request. Update checking can
be disabled in the application settings; when disabled, THRM makes no network connections at
all.

GitHub is an independent service and its handling of that request is governed by the
[GitHub Privacy Statement](https://docs.github.com/site-policy/privacy-policies/github-general-privacy-statement).

## Diagnostic packages

The application can export a diagnostic package (a `.zip` containing hardware information and
recent log files) to help investigate a problem. This is **always** initiated by you, saved to a
location you choose, and never uploaded automatically.

Before attaching a diagnostic package to a public issue report, please review its contents —
logs may contain device identifiers, file paths, and the names of running applications.

## Third-party components

THRM bundles [LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor)
and [PawnIO](https://pawnio.eu/) to read hardware sensors on Windows. Both operate locally and
are not used to collect or transmit data.

## Children's privacy

THRM does not knowingly collect data from anyone, including children, because it does not
collect data at all.

## Changes

Changes to this policy are published in this file and tracked in the repository's Git history.

## Contact

Questions about this policy: open an issue at <https://github.com/TIANLI0/THRM/issues> or
contact the maintainer at the address listed in the [README](README.md#作者与联系).

## Related

- [Code Signing Policy](CODE_SIGNING_POLICY.md)
