{
  description = "THRM — open-source controller for Flydigi BS-series laptop coolers";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs =
    { self, nixpkgs }:
    let
      systems = [
        "x86_64-linux"
        "aarch64-linux"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      packages = forAllSystems (pkgs: rec {
        thrm = pkgs.callPackage ./nix/thrm.nix { };
        default = thrm;
      });

      nixosModules.default = import ./nix/module.nix { inherit self; };

      checks = forAllSystems (pkgs: {
        default = self.packages.${pkgs.stdenv.hostPlatform.system}.thrm;
      });

      apps = forAllSystems (pkgs: {
        default = {
          type = "app";
          program = "${self.packages.${pkgs.stdenv.hostPlatform.system}.thrm}/bin/thrm";
          meta.description = "THRM — Flydigi BS-series laptop cooler controller";
        };
        thrm = self.apps.${pkgs.stdenv.hostPlatform.system}.default;
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          name = "thrm-flake";
          nativeBuildInputs = with pkgs; [
            go_1_27
            bun
            nodejs
            wails3
            pkg-config
            nixfmt
            curl
            git
          ];
          buildInputs = with pkgs; [
            gtk4
            webkitgtk_6_0
            libsoup_3
            hidapi
            udev
            libx11
            gdk-pixbuf
            glib
          ];
        };
      });

      formatter = forAllSystems (pkgs: pkgs.nixfmt);
    };
}
