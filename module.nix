{ config, lib, pkgs, ... }:

let
  cfg = config.services.ScratchPad;
in
{
  options.services.ScratchPad = {
    enable = lib.mkEnableOption "ScratchPad";

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      defaultText = lib.literalMD "the ScratchPad package from this flake";
      description = "The ScratchPad package to run.";
    };

    listenAddress = lib.mkOption {
      type = lib.types.str;
      default = "";
      example = "127.0.0.1";
      description = ''
        Address to bind. Empty listens on all interfaces.
        Wrap IPv6 addresses in brackets, e.g. `[::1]`.
      '';
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 63001;
      description = "Port serving both the page and the WebSocket.";
    };

    openFirewall = lib.mkOption {
      type = lib.types.bool;
      default = false;
      description = "Open {option}`services.ScratchPad.port` in the firewall.";
    };

    allowedOrigins = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      example = [ "pad.example.com" ];
      description = ''
        Extra origin hosts allowed to open the WebSocket, e.g. when served
        behind a reverse proxy under a different hostname. Same-origin
        requests are always allowed.
      '';
    };
  };

  config = lib.mkIf cfg.enable {
    networking.firewall.allowedTCPPorts = lib.mkIf cfg.openFirewall [ cfg.port ];

    systemd.services.scratchpad = {
      description = "ScratchPad";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      environment = {
        SCRATCHPAD_LISTEN = "${cfg.listenAddress}:${toString cfg.port}";
        SCRATCHPAD_DATA_DIR = "/var/lib/ScratchPad";
        SCRATCHPAD_ALLOWED_ORIGINS = lib.concatStringsSep "," cfg.allowedOrigins;
      };

      serviceConfig = {
        ExecStart = lib.getExe cfg.package;
        Restart = "on-failure";
        StateDirectory = "ScratchPad";
        StateDirectoryMode = "0700";

        # Hardening
        DynamicUser = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        PrivateTmp = true;
        PrivateDevices = true;
        NoNewPrivileges = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictAddressFamilies = [ "AF_INET" "AF_INET6" ];
        RestrictNamespaces = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";
        CapabilityBoundingSet = "";
      };
    };
  };
}
