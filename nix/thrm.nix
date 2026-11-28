{
  lib,
  stdenvNoCC,
  buildGoModule,
  fetchFromGitHub,
  pkg-config,
  makeWrapper,
  bun,
  nodejs,
  gtk4,
  webkitgtk_6_0,
  libsoup_3,
  hidapi,
  udev,
  libx11,
  gdk-pixbuf,
  glib,
  # Version of the upstream release tag to package (v${version}).
  version ? "3.9.0",
  # Hash of the unpacked GitHub source tarball.
  srcHash ? "sha256-6zX7tNJutSd0KNn1VLBtXla6StfpXsfQMRgXE7f3ON4=",
  # Hash of the Go module dependency vendor tree.
  vendorHash ? "sha256-/eyFSBWn8iYBhDq8nUV/IwOHtBYzDfhhgzvWqcNz2gM=",
  # Hash of the `frontend/node_modules` tree produced by `bun install`.
  frontendDepsHash ? "sha256-Aabx1FflTvcsZjSvOolVfjOke3b3TQKikNwgLc4XcFs=",
}:

let
  src = fetchFromGitHub {
    owner = "TIANLI0";
    repo = "THRM";
    rev = "v${version}";
    hash = srcHash;
  };

  # Next.js 16 needs a fully populated node_modules.  bun install must run
  # before the sandboxed (network-less) frontend build, so vendor it through a
  # fixed-output derivation.
  frontendDeps = stdenvNoCC.mkDerivation {
    pname = "thrm-frontend-deps";
    inherit version src;

    nativeBuildInputs = [
      bun
      nodejs
    ];

    dontConfigure = true;
    dontFixup = true;
    dontPatchELF = true;

    buildPhase = ''
      runHook preBuild

      export HOME=$TMPDIR
      export BUN_INSTALL_CACHE_DIR=$TMPDIR/bun-cache
      pushd frontend
      bun install --frozen-lockfile --no-progress
      popd

      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -a frontend/node_modules $out/node_modules
      runHook postInstall
    '';

    outputHashMode = "recursive";
    outputHashAlgo = "sha256";
    outputHash = frontendDepsHash;
  };

  # Static export of the Next.js UI.  main.go embeds `all:frontend/dist`, so
  # this has to be ready before the Go build starts.
  frontend = stdenvNoCC.mkDerivation {
    pname = "thrm-frontend";
    inherit version src;

    nativeBuildInputs = [
      bun
      nodejs
    ];

    dontConfigure = true;
    dontFixup = true;
    dontPatchELF = true;

    buildPhase = ''
      runHook preBuild

      export HOME=$TMPDIR
      export NEXT_TELEMETRY_DISABLED=1
      cp -a ${frontendDeps}/node_modules frontend/node_modules
      patchShebangs frontend/node_modules
      pushd frontend
      bun run build
      popd

      runHook postBuild
    '';

    installPhase = ''
      runHook preInstall
      mkdir -p $out
      cp -a frontend/dist/. $out/
      runHook postInstall
    '';
  };
in
buildGoModule {
  pname = "thrm";
  inherit version src;

  vendorHash = vendorHash;

  # The Wails v3 Linux backend links GTK 4 + WebKitGTK 6.0 (the upstream
  # Taskfile's default `production` configuration); go-hid needs hidapi and
  # libudev, and global shortcuts need X11.
  nativeBuildInputs = [
    pkg-config
    makeWrapper
  ];
  buildInputs = [
    gtk4
    webkitgtk_6_0
    libsoup_3
    hidapi
    udev
    libx11
    gdk-pixbuf
    glib
  ];

  env.CGO_ENABLED = "1";

  subPackages = [
    "."
    "cmd/core"
    "cmd/rapl-helper"
  ];
  tags = [ "production" ];
  ldflags = [
    "-s"
    "-w"
    "-X"
    "github.com/TIANLI0/THRM/internal/version.BuildVersion=${version}"
  ];

  # Upstream tests need a live desktop session / device tree; not run in the
  # packaging sandbox.
  doCheck = false;

  # The dependency vendor tree must not build the frontend first.
  overrideModAttrs = _final: _prev: {
    preBuild = "";
  };

  preBuild = ''
    cp -a ${frontend} frontend/dist
  '';

  postInstall = ''
    mv $out/bin/THRM $out/bin/thrm
    mv $out/bin/core $out/bin/thrm-core
    mv $out/bin/rapl-helper $out/bin/thrm-rapl-helper

    # The NVML provider dlopen()s libnvidia-ml.so.1.  On NixOS the NVIDIA
    # userspace libraries live in /run/opengl-driver/lib and are not part of
    # the default loader search path, so the core service must be given that
    # directory explicitly.  Wrapping both entry points also covers the case
    # where thrm-core is started directly by a user service/autostart entry.
    wrapProgram $out/bin/thrm \
      --suffix LD_LIBRARY_PATH : /run/opengl-driver/lib
    wrapProgram $out/bin/thrm-core \
      --suffix LD_LIBRARY_PATH : /run/opengl-driver/lib

    install -Dm644 ${src}/scripts/99-flydigi-fan.rules \
      $out/lib/udev/rules.d/99-flydigi-fan.rules

    install -Dm644 ${src}/packaging/linux/thrm.desktop $out/share/applications/thrm.desktop
    substituteInPlace $out/share/applications/thrm.desktop \
      --replace-fail "/usr/bin/thrm" "$out/bin/thrm"

    install -Dm644 ${src}/build/appicon.png \
      $out/share/icons/hicolor/1024x1024/apps/thrm.png
    install -Dm644 ${src}/build/appicon.png \
      $out/share/pixmaps/thrm.png

    install -Dm644 ${src}/LICENSE \
      $out/share/licenses/thrm/LICENSE
  '';

  meta = {
    description = "Third-party controller for Flydigi BS-series laptop coolers";
    homepage = "https://github.com/TIANLI0/THRM";
    downloadPage = "https://github.com/TIANLI0/THRM/releases";
    license = lib.licenses.mit;
    sourceProvenance = [ lib.sourceTypes.fromSource ];
    platforms = lib.platforms.linux;
    mainProgram = "thrm";
  };
}
