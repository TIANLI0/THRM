{ self }:
{
  config,
  lib,
  pkgs,
  ...
}:

let
  cfg = config.programs.thrm;
in
{
  options.programs.thrm = {
    enable = lib.mkEnableOption "THRM, a controller for Flydigi BS-series laptop coolers";

    package = lib.mkOption {
      type = lib.types.package;
      default = self.packages.${pkgs.stdenv.hostPlatform.system}.default;
      description = "The THRM package to install.";
    };
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [ cfg.package ];

    # Installs the Flydigi fan udev rules shipped by the package.
    services.udev.packages = [ cfg.package ];

    # RAPL energy_uj is root-only (0400) on current kernels.  A store path
    # cannot carry file capabilities (/nix/store is read-only), so expose the
    # helper through a capability wrapper on the writable wrapper directory.
    security.wrappers.thrm-rapl-helper = {
      source = "${cfg.package}/bin/thrm-rapl-helper";
      capabilities = "cap_dac_read_search+ep";
    };

    # The package wrapper also probes this path, but setting it explicitly
    # keeps working when security.wrapperDir has been customised.
    environment.sessionVariables.THRM_RAPL_HELPER = "${config.security.wrapperDir}/thrm-rapl-helper";
  };
}
