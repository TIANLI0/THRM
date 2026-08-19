# Code Signing Policy

*中文摘要：本文档说明 THRM 的 Windows 发布包如何被构建、审核和签名。THRM 的代码签名由 [SignPath.io](https://signpath.io/) 免费提供，证书由 [SignPath Foundation](https://signpath.org/) 签发。*

## Status

**Current status: the SignPath Foundation review is complete and the THRM OSS organization is
active.** The repository is wired for automatic SignPath signing on release-shaped GitHub
Actions runs. The current OSS organization uses a self-signed test certificate while SignPath
completes the production certificate setup.

The self-signed certificate is suitable for testing only: it is not trusted by Windows until
the certificate is installed on the test device, and it must not be treated as a production
release certificate. The workflow can switch to the production policy without changing the
artifact flow once SignPath makes that policy available.

## Signing provider

Free code signing provided by [SignPath.io](https://signpath.io/), certificate by
[SignPath Foundation](https://signpath.org/).

The signing certificate is issued in the name of **SignPath Foundation**, not in the name of
the THRM project or its maintainer. SignPath Foundation acts as the publisher of record: it
vouches that the signed binary was built by an automated pipeline from the public source code
of this repository. The private key is held in SignPath's HSM and is never accessible to the
THRM project team.

## Source code

All signed binaries are built exclusively from the public source code in this repository:

<https://github.com/TIANLI0/THRM>

The project is licensed under the [MIT License](LICENSE) with no commercial dual-licensing and
no proprietary components of our own.

### Third-party components

| Component | Origin | Signing |
| --- | --- | --- |
| `LibreHardwareMonitorLib.dll` | [LibreHardwareMonitor](https://github.com/LibreHardwareMonitor/LibreHardwareMonitor) (MPL-2.0) | Redistributed upstream build; not signed by us |
| `PawnIO_setup.exe` | [PawnIO](https://pawnio.eu/) | Redistributed upstream installer, signed by its own vendor; not signed by us |

These are unmodified upstream redistributions. We do not sign third-party binaries under the
SignPath Foundation certificate.

## Build process

Builds are fully automated and publicly auditable. There are no local or manual builds in the
release path.

- **Build system:** GitHub Actions
- **Workflow:** [`.github/workflows/build-and-release.yml`](.github/workflows/build-and-release.yml)
- **Trigger:** a version tag pushed to the repository
- **Logs:** every run is public at
  [Actions](https://github.com/TIANLI0/THRM/actions/workflows/build-and-release.yml)

Tags containing `signpath-test` or ending in `-test` are deliberately published as GitHub
pre-releases. Their release notes carry a visible test notice explaining that the artifacts use
the self-signed test certificate and are not production releases.

Release-shaped Windows runs perform three SignPath requests in sequence:

1. Sign `THRM.exe`.
2. Sign `THRM Core.exe`.
3. Rebuild the NSIS installer from those signed binaries, then sign
   `THRM-amd64-installer.exe`.

The workflow uploads each executable as a non-archived GitHub Actions artifact, waits for the
SignPath request to complete, and replaces the local build output with the returned signed
artifact. The portable ZIP is created only after these replacements, so it contains the signed
application binaries. Pull requests and ordinary branch builds remain unsigned development
artifacts.

All signed artifacts carry product name and version metadata, generated from
[`build/windows/info.json`](build/windows/info.json) and the version declared in
[`wails.json`](wails.json).

### Signed artifacts

| Artifact | Description |
| --- | --- |
| `THRM-amd64-installer.exe` | NSIS installer (recommended) |
| `THRM.exe` | Main application (GUI) |
| `THRM Core.exe` | Background core service |
| `THRM-windows-portable.zip` | Portable archive containing the signed application binaries and unsigned third-party components |

Linux artifacts (`.tar.gz`, `.deb`) are not covered by this certificate.

## Team roles

The current `test-signing` policy has no approval gate, so the configured GitHub Actions release
workflow submits and completes test signing automatically. When SignPath enables the production
policy, its approval requirements must be reflected here before switching the workflow policy.

| Role | Held by |
| --- | --- |
| **Author** (may commit / open pull requests) | [TIANLI0](https://github.com/TIANLI0) and [project contributors](https://github.com/TIANLI0/THRM/graphs/contributors) |
| **Reviewer** (reviews pull requests) | [TIANLI0](https://github.com/TIANLI0) |
| **Approver** (approves signing of a release) | [TIANLI0](https://github.com/TIANLI0) |

All team members with repository or SignPath access are required to enable multi-factor
authentication on both GitHub and SignPath.

## Distribution

Signed binaries are distributed **only** through official channels:

- [GitHub Releases](https://github.com/TIANLI0/THRM/releases) — primary
- [AUR: `thrm-bin`](https://aur.archlinux.org/packages/thrm-bin) — Linux, unsigned

Builds obtained anywhere else are not endorsed by this project. THRM does not sign or
distribute builds produced by third parties.

Development builds published from pull requests or ordinary branch pushes are **not signed** and
are not covered by this policy. Manual preview releases and tagged releases use the configured
SignPath policy; while the test certificate is active, those artifacts are test-signed rather
than production-signed.

## Privacy

See [PRIVACY.md](PRIVACY.md).

## Reporting

To report a signed binary you believe to be malicious, tampered with, or otherwise in
violation of the [SignPath Foundation terms](https://signpath.org/terms), open an issue at
<https://github.com/TIANLI0/THRM/issues> or contact the maintainer at the address listed in the
[README](README.md#作者与联系).
