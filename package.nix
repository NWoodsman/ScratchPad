{ lib, buildGoModule }:

buildGoModule {
  pname = "scratchpad";
  version = "0.2.0";

  # Only the files the build needs, so README/video edits don't trigger rebuilds.
  src = lib.fileset.toSource {
    root = ./.;
    fileset = lib.fileset.unions [
      ./go.mod
      ./go.sum
      ./web
      (lib.fileset.fileFilter (file: file.hasExt "go") ./.)
    ];
  };

  vendorHash = "sha256-1ckWAhiyEgGgIk/9Y/bgj4msc4eNVpsRgyIpxK8hDks=";

  ldflags = [ "-s" "-w" ];

  meta = {
    description = "Self-hosted scratch pad that syncs text between browsers in real time";
    homepage = "https://github.com/NWoodsman/ScratchPad";
    license = lib.licenses.asl20;
    mainProgram = "ScratchPad";
  };
}
