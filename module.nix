{ config, lib, pkgs, ... }:

with lib;

let
  cfg = config.services.ScratchPad;
  scratchPadPkg = pkgs.callPackage ./package.nix {};
in {
  options.services.ScratchPad = {
    enable = mkEnableOption "Enable ScratchPad";
  };

  config = mkIf cfg.enable {
    # Automatically open the configured port in the firewall
    networking.firewall.allowedTCPPorts = [63000 63001 ];
    
    
    # Service 1: The WebSocket Backend (server.py)
    systemd.services.ScratchPad-WebSockets = {
      description = "Scratchpad WebSockets Backend Server";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];
      
      environment = {
        DATA_DIR = "/var/lib/ScratchPad";  
      };

      serviceConfig = {
        # Directly execute server.py using the bundled python symlink
        ExecStart = "${scratchPadPkg}/bin/python3 ${scratchPadPkg}/share/ScratchPad/server.py";
        WorkingDirectory = "${scratchPadPkg}/share/ScratchPad";
        Restart = "always";
        
        StateDirectory = "ScratchPad";
        
        # Security hardening
        DynamicUser = true;
        ProtectSystem = "full";
      };
    };

    # Service 2: The HTTP Frontend (http.server)
    systemd.services.ScratchPad-HTTP = {
      description = "ScratchPad HTTP Frontend Server";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig = {
        # Directly execute the HTTP module
        ExecStart = "${scratchPadPkg}/bin/python3 -m http.server 63001";
        WorkingDirectory = "${scratchPadPkg}/share/ScratchPad";
        Restart = "always";
        
        # Security hardening
        DynamicUser = true;
        ProtectSystem = "full";
      };
    };
    
  };
}
