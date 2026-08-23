{ pkgs ? import <nixpkgs> {} }:

let
  pythonWithDeps = pkgs.python3.withPackages (ps: [
    ps.websockets
  ]);
in
pkgs.stdenv.mkDerivation {
  pname = "ScratchPad";
  version = "0.1.1.0";
  src = ./.;

  propagatedBuildInputs = [ pythonWithDeps ];
  dontUnpack = true;

  installPhase = ''
    mkdir -p $out/bin $out/share

    # Copy pure static assets directly to store
    cp $src/server.py $out/share/server.py
    cp $src/index.html $out/share/index.html

    # Clean wrapper that doesn't need to spoof directories
    cat <<EOF > $out/bin/ScratchPad
    #!/bin/sh

    trap "kill 0" EXIT INT TERM
    
    ${pythonWithDeps}/bin/python3 $out/share/server.py &
    ${pythonWithDeps}/bin/python3 -m http.server 63001 --directory $out/share &
    wait
    EOF

    chmod +x $out/bin/ScratchPad
  '';
}
