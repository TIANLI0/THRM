# Maintainer: TIANLI0 <wutianli@tianli0.top>

pkgname=thrm-bin
pkgver="${THRM_PKGVER:-3.6.3}"
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

# This PKGBUILD packages the binaries already produced by build.sh/CI. It does
# not download a stale release or require Go/Bun on the Arch packaging host.
source=()
sha256sums=()

package() {
  local artifact_dir="${THRM_ARTIFACT_DIR:-${startdir}/build}"
  if [[ "${artifact_dir}" != /* ]]; then
    artifact_dir="${startdir}/${artifact_dir}"
  fi

  local artifact
  for artifact in thrm thrm-core; do
    if [[ ! -x "${artifact_dir}/${artifact}" ]]; then
      error "missing executable build artifact: ${artifact_dir}/${artifact}"
      return 1
    fi
  done

  install -Dm755 "${artifact_dir}/thrm" "${pkgdir}/usr/bin/thrm"
  install -Dm755 "${artifact_dir}/thrm-core" "${pkgdir}/usr/bin/thrm-core"
  install -Dm644 "${startdir}/scripts/99-flydigi-fan.rules" \
    "${pkgdir}/usr/lib/udev/rules.d/99-flydigi-fan.rules"
  install -Dm644 "${startdir}/packaging/linux/thrm.desktop" \
    "${pkgdir}/usr/share/applications/thrm.desktop"
  install -Dm644 "${startdir}/frontend/public/brand/appicon.png" \
    "${pkgdir}/usr/share/icons/hicolor/256x256/apps/thrm.png"
  install -Dm644 "${startdir}/LICENSE" \
    "${pkgdir}/usr/share/licenses/${pkgname}/LICENSE"
}
