# Code Signing Policy

*中文摘要：THRM 的 Windows 发布包使用维护者的 Certum 代码签名证书签名，由 GitHub Actions 从本仓库公开源码自动构建并签名。只签名本项目自己的程序；第三方组件保持原样。*

## Signing certificate

Windows releases are signed with the maintainer's **code-signing certificate issued by Certum**
(Asseco Data Systems).

- Issuer: `Certum Code Signing 2021 CA`
- Thumbprint (SHA-1): `DB7AE67D50D4D210C244324C54BC2FB82FDA57C4`
- Timestamps: Certum RFC 3161 (`http://time.certum.pl`), so signatures stay valid after the certificate expires

The private key never leaves Certum's cloud HSM (SimplySign). The signing workflow logs in with credentials stored
only in the `codesign` GitHub environment of this repository, which admits only `main` and `v*`/`V*` tags. Pull
requests (including those from forks) cannot reach it. The shared signing actions are public:
<https://github.com/TIANLI0/codesign>.

## Source code

All signed binaries are built exclusively from the public source code in this repository:
<https://github.com/TIANLI0/THRM>, licensed under the [MIT License](LICENSE).

## Build and signing process

Builds are fully automated and publicly auditable; there are no local or manual builds in the release path.

- **Workflow:** [`.github/workflows/build-and-release.yml`](.github/workflows/build-and-release.yml)
- **Signed runs:** pushed `v*`/`V*` tags and manual preview releases (`workflow_dispatch` with publishing)
- **Unsigned runs:** pull requests and ordinary branch builds (development artifacts)
- **Logs:** every run is public at [Actions](https://github.com/TIANLI0/THRM/actions/workflows/build-and-release.yml)

In a signed run the `build-windows` job:

1. Builds TempBridge and THRM (no signing session exists yet, so no build tool can use the certificate).
2. Logs in to Certum SimplySign.
3. Signs `THRM.exe`, `THRM Core.exe` and `bridge/THRM TempBridge.exe`, and verifies signer and timestamp.
4. Rebuilds the installer (FluentSetup, [`build/windows/installer/fluentsetup.json`](build/windows/installer/fluentsetup.json))
   from those signed binaries. FluentSetup signs `THRM-amd64-installer.exe` and the `uninstall.exe` it installs.
5. Creates `THRM-windows-portable.zip` from the signed files, then logs out.

Tags containing `-test` are published as GitHub pre-releases with a test notice. They are signed like any other
release but are not production releases.

### Signed artifacts

| Artifact | Description |
| --- | --- |
| `THRM-amd64-installer.exe` | Installer (recommended), built with FluentSetup; its `uninstall.exe` is signed too |
| `THRM.exe` | Main application (GUI) |
| `THRM Core.exe` | Background core service |
| `bridge/THRM TempBridge.exe` | Windows temperature bridge |
| `THRM-windows-portable.zip` | Portable archive containing the signed binaries and unsigned third-party components |

Linux artifacts (`.tar.gz`, `.deb`, Arch package) are not Authenticode-signed.

### Third-party components (not signed by us)

| Component | Origin | Signing |
| --- | --- | --- |
| `LibreHardwareMonitorLib.dll` and its NuGet dependencies | [LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor) (MPL-2.0) and NuGet | Not signed by us |
| `PawnIO_setup.exe` | [PawnIO](https://pawnio.eu/) | Signed by its vendor; hash-pinned in `build/windows/pawnio.env` |
| WebView2 bootstrapper inside the installer | Microsoft | Signed by Microsoft |

## Distribution

Signed binaries are distributed **only** through [GitHub Releases](https://github.com/TIANLI0/THRM/releases)
(and the in-app updater, which downloads from there). Builds obtained anywhere else are not endorsed by this project.

## Privacy

See [PRIVACY.md](PRIVACY.md).

## Reporting

To report a signed binary you believe to be malicious or tampered with, open an issue at
<https://github.com/TIANLI0/THRM/issues> or contact the maintainer at the address listed in the
[README](README.md#作者与联系).
