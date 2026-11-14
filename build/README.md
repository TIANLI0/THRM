# Build Directory

Build configuration and assets for the THRM GUI (Wails v3). Run the tasks from the repository root
with `wails3 task <name>` — see [`Taskfile.yml`](../Taskfile.yml).

* `config.yml` - Wails v3 project configuration. `info.version` is the single source of the THRM
  version; `build.bat`, `build.sh`, CI and `PKGBUILD` all read it. `dev_mode` drives `wails3 dev`.
* `appicon.png` - Application icon, also embedded as the Linux window icon.
* `bin` - Output directory.
* `.wails` - Assets rendered from `config.yml` during a build (ignored by git).

## Windows

* `icon.ico` - Executable and installer icon.
* `info.json` - Version resource template; `@VERSION@` is replaced with `info.version` at build time.
  It deliberately declares `FileVersion` and the fixed `product_version`, which Wails' own template
  omits (Windows then shows no version details at all).
* `wails.exe.manifest` - Application manifest. Maintained by hand because THRM requires
  administrator rights; it is not regenerated from `config.yml`.
* `installer/*` - The installer, built with [FluentSetup](https://github.com/TIANLI0/FluentSetup):
  `fluentsetup.json` (install folder, PawnIO, autostart, upgrades from NSIS installs, uninstall cleanup),
  `locales/*.json` (option texts and license per language) and `resources/license_*.txt`. `wails3 task package`
  runs `fluentsetup build fluentsetup.json -D Version=<config.yml version> -D PawnIOVersion=<pawnio.env> -o ../../bin`;
  FluentSetup downloads Microsoft's WebView2 bootstrapper itself (`"auto": "bundle"`).
