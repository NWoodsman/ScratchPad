{
  description = "ScratchPad flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      # 1. Standard packages for interactive 'nix run' usage
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.callPackage ./default.nix {};
        });

      # 2. The NixOS module definition
      nixosModules.default = { config, lib, pkgs, ... }:
        let
          cfg = config.services.my-web-app;
          # Build the package inside the module context
          package = pkgs.callPackage ./default.nix {};
        in
        {
          # Schema
          options.services.my-web-app = {
            enable = lib.mkEnableOption "Enable the ScratchPad service";
          };

          # Define what happens when the option is enabled
          config = lib.mkIf cfg.enable {
            # Automatically manage systemd services
            systemd.services.ScratchPad = {
              description = "ScratchPad service";
              
              # Ensure the network is online before starting
              after = [ "network.target" ];
              wantedBy = [ "multi-user.target" ];

              # Run the background service under a safe, unprivileged user profile
              serviceConfig = {
                Type = "simple";
                ExecStart = "${package}/bin/ScratchPad";
                Restart = "always";
                RestartSec = 5;
                
                # Sandboxing security enhancements
                DynamicUser = true;
                ProtectSystem = "strict";
                ProtectHome = true;
              };
            };
          };
        };
    };
}
