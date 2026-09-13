{
  description = "ScratchPad";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }: 
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f (import nixpkgs { inherit system; }));
    in {
      
      packages = forAllSystems (pkgs: {
        # Explicitly pass 'self' as the source context for local flake execution
        default = pkgs.callPackage ./package.nix {};
      });

      nixosModules.default = import ./module.nix;
    };
}
