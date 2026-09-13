{ pkgs ? import <nixpkgs> {} }:

let
  pythonWithDeps = pkgs.python3.withPackages (ps: [
    ps.websockets
    ps.platformdirs
  ]);
  
in
pkgs.stdenv.mkDerivation {
  pname = "ScratchPad";
  
  # 0.x.x.x = this software is in beta; stable but bugfixing
  # x.1.0.0 = no major feature changes or breaking changes
  # x.x.2.x = the nix module system is being refactored
  # x.x.x.0 = first impl of said refactoring
  
  version = "0.1.2.0";
  
  src = ./.;

  installPhase = ''
    mkdir -p $out/share/ScratchPad
    
    cp index.html $out/share/ScratchPad
    cp server.py $out/share/ScratchPad
    
    # Generate the launch script
    
    mkdir -p $out/bin
    
    ln -s ${pythonWithDeps}/bin/python3 $out/bin/python3
    
    cat <<EOF > $out/bin/ScratchPad
    #!/usr/bin/env ${pythonWithDeps}/bin/python3
    import subprocess
    import signal
    import sys
    import os

    # 1. Start both servers as child processes
    backend = subprocess.Popen(["${pythonWithDeps}/bin/python3", "$out/share/ScratchPad/server.py"], cwd="$out/share/ScratchPad")
    frontend = subprocess.Popen(["${pythonWithDeps}/bin/python3", "-m", "http.server", "63001"], cwd="$out/share/ScratchPad")

    # 2. Define a unified cleanup function
    def cleanup_and_exit(signum, frame):
        print("\n[ScratchPad] Signal received. Killing background processes and exiting cleanly...")
        backend.terminate()
        frontend.terminate()
        sys.exit(0)

    # 3. Handle Ctrl+C (SIGINT), Terminate (SIGTERM), AND Ctrl+Z (SIGTSTP)
    signal.signal(signal.SIGINT, cleanup_and_exit)
    signal.signal(signal.SIGTERM, cleanup_and_exit)
    signal.signal(signal.SIGTSTP, cleanup_and_exit) # <--- This successfully catches Ctrl+Z!

    # 4. Keep this manager script alive while the servers run
    try:
        frontend.wait()
    except Exception:
        cleanup_and_exit(None, None)
    EOF
    
    chmod +x $out/bin/ScratchPad
  '';
}
