# Maintainer: TIANLI0 <wutianli@tianli0.top>

pkgname=thrm-bin
# 版本以 wails.json 的 productVersion 为唯一来源，避免再出现"忘了同步这一行导致发版被卡"。
# CI 打 Arch 包时会把这一行整行 sed 成字面量：nightly 需要 0.0.0.rYYYYMMDD 这种 makepkg
# 能接受的形式，不是 wails.json 里的值。
pkgver=$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "${startdir}/wails.json" 2>/dev/null)
: "${pkgver:=0.0.0}"
pkgrel=1
pkgdesc='Flydigi BS-series laptop cooler controller (prebuilt)'
arch=('x86_64')
url='https://github.com/TIANLI0/THRM'
license=('MIT')
depends=(
  'gdk-pixbuf2'
  'glib2'
  'glibc'
  'gtk3'
  'libx11'
  'libsoup3'
  'systemd-libs'
  'webkit2gtk-4.1'
)
optdepends=('bluez: BS1 BLE support')
provides=("thrm=${pkgver}")
conflicts=('thrm')
options=('!strip' '!debug')

# This PKGBUILD packages the binaries already produced by build.sh/CI. makepkg
# imports every payload into $srcdir before package() runs, including in a
# clean chroot.
source=(
  "thrm::file://${startdir}/build/thrm"
  "thrm-core::file://${startdir}/build/thrm-core"
  "99-flydigi-fan.rules::file://${startdir}/scripts/99-flydigi-fan.rules"
  "thrm.desktop::file://${startdir}/packaging/linux/thrm.desktop"
  "thrm.png::file://${startdir}/build/appicon.png"
  'LICENSE'
)
sha256sums=('SKIP' 'SKIP' 'SKIP' 'SKIP' 'SKIP' 'SKIP')

package() {
  install -Dm755 "${srcdir}/thrm" "${pkgdir}/usr/bin/thrm"
  install -Dm755 "${srcdir}/thrm-core" "${pkgdir}/usr/bin/thrm-core"
  install -Dm644 "${srcdir}/99-flydigi-fan.rules" \
    "${pkgdir}/usr/lib/udev/rules.d/99-flydigi-fan.rules"
  install -Dm644 "${srcdir}/thrm.desktop" \
    "${pkgdir}/usr/share/applications/thrm.desktop"
  install -Dm644 "${srcdir}/thrm.png" \
    "${pkgdir}/usr/share/icons/hicolor/256x256/apps/thrm.png"
  install -Dm644 "${srcdir}/LICENSE" \
    "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
}
