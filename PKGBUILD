# Maintainer: TIANLI0 <wutianli@tianli0.top>

pkgname=thrm-bin
pkgver=3.6.3
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
  'thrm::build/thrm'
  'thrm-core::build/thrm-core'
  '99-flydigi-fan.rules::scripts/99-flydigi-fan.rules'
  'thrm.desktop::packaging/linux/thrm.desktop'
  'thrm.png::frontend/public/brand/appicon.png'
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
