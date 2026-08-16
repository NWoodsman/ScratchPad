{
  description = "Real-time sync server and static file server dev environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux"; # Adjust to aarch64-darwin if on Apple Silicon Mac
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      devShells.${system}.default = pkgs.mkShell {
        buildInputs = [
          (pkgs.python3.withPackages (ps: [
            ps.websockets
          ]))
        ];

        shellHook = ''
          echo "Starting Python WebSocket server and static file server..."
          python3 server.py &
          python3 -m http.server 63001 &
          trap "kill 0" EXIT
        '';
      };
    };
}
