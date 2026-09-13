# ScratchPad
A simple self-hosted scratch pad using WebSockets. Allows you to move text between multiple PCs on a local network and syncs edits in real time across the network. Designed to be as lightweight as possible.




https://github.com/user-attachments/assets/f2c80fc5-60e5-416a-940d-48f7640ddd6b




Live edits are propagated using web sockets. (Caution: there is no dispute resolution if multiple users are editing at once. This is intended for you to use as a scratch pad as a single user, synchronously.)

The repo is designed to run two servers: 

- An http server runs on port 63001 and serves a single http file.
- A WebSocket server runs on port 63000 and synchronizes all the client updates.

  __Caution! This repo is not security hardened and uses no encryption. Anything pasted in the scratchpad is transmitted in the clear! Do not use for sensitive information!__

## Dependencies
- Python3
- websockets

## Installation

### General (Windows, Linux, MacOS)

You will need to open ports 63000 and 63001. 

Clone the project and `cd` into the root directory, then run:

```
python3 server.py &
python3 -m http.server 63001 &
trap "kill 0" EXIT INT TERM
```

This spins-up both servers in a single terminal window and will run until the terminal is closed

Now open a web browser and visit `localhost:63001` if you are connecting from the main PC. Or visit `ip_address:63001` if on another PC in your local network.

Type some text and you will see it appear magically in any browser currently visiting the ip address.

### Docker

A container is published to `ghcr.io` and set to track `main`.

In the following command, `sha-*` will change based on the current HEAD of main branch.

Use the current main branch hash with this command to obtain the latest container.

```
docker pull ghcr.io/nwoodsman/scratchpad:sha-6676bae
```

### NixOS

#### Try temporarily

You can temporarily try out ScratchPad. It will be accessible from a browser window on the same PC. However, if you want to access the temporary ScratchPad from another PC on your local network,you will need to open ports 63000 and 63001.

These commands will temporarily open the ports until the next boot.

    sudo nixos-firewall-tool open tcp 63000
    sudo nixos-firewall-tool open tcp 63001

Now you can temporarily run ScratchPad in a shell:

    nix run github:NWoodsman/ScratchPad


#### Installing (in NixOS)

Add ScratchPad to your `flake.nix` and `configuration.nix`. Here is an example flake:


#### `flake.nix`
```nix

{
  description = "main flake example that pulls in ScratchPad";

  inputs = {
    # Keep using your pinned/locked unstable nixpkgs reference
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    
    # Add your python server project repository as a flake input
    ScratchPad.url = "github:NWoodsman/ScratchPad";
  };

  outputs = { self, nixpkgs, ScratchPad, ... }: {
    nixosConfigurations.myhostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux"; # Adjust to your architecture if different
      modules = [
        # 1. Inject the module exposed by your app's flake
        ScratchPad.nixosModules.default
        
        # 2. Pull in your standard system layout file
        ./configuration.nix
        ./hardware-configuration.nix
      ];
    };
  };
}

```
#### `configuration.nix`

```nix

{ config, pkgs, ... }:

{
  # ... Your existing system configurations (timezone, users, bootloader, etc.) ...

  # Enable ScratchPad
  services.Scratchpad.enable = true;
}

```


## Architecture

The scratch pad functions by using the standardized web feature `designMode` available in all browsers which allows editing arbitrary html page content. So we make the entire page a single body `div` that we can edit and paste text into.

The scratch pad content is stored in browser local storage. A timer updates a local `content.txt` backup file every 20 seconds.

The backup file is located:

On Windows: ` C:\Users\<User>\AppData\Local\ScratchPad`
On MacOS: ` ~/Library/Application Support/ScratchPad`
On Linux: `XDG_DATA_HOME` or `~/.local/share/ScratchPad`

The local text is diffed using [https://github.com/jhchen/fast-diff](https://github.com/jhchen/fast-diff) and only diffs are sent through the socket.

The html file uses JetBrains Mono from Google Fonts but will fallback to monospace. 

## License

Apache 2 license to stay in-sync with `fast-diff`.

`fast-diff` Copyright 2014-2023 Jason Chen 

See LICENSE for attribution.





