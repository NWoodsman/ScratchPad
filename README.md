# ScratchPad
A simple self-hosted scratch pad using WebSockets. Allows you to move text between multiple PCs on a local network and syncs edits in real time across the network. Designed to be as lightweight as possible.


<video autoplay loop muted playsinline width="800">
  <source src="ScratchPad_clip.mp4" type="video/mp4">
  Your browser does not support the video tag.
</video>


Live edits are propagated using web sockets. (Caution: there is no dispute resolution if multiple users are editing at once. This is intended for you to use as a scratch pad as a single user, synchronously.)

The repo is designed to run two servers: 

- An http server runs on port 63001 and serves a single http file.
- A WebSocket server runs on port 63000 and synchronizes all the client updates.

## Dependencies
- Python3
- websockets

## Installation

### Prerequisites
You will need to open ports 63000 and 63001.

If using in NixOS, these commands should be run first will temporarily open the ports until the next boot.

    sudo nixos-firewall-tool open tcp 63000
    sudo nixos-firewall-tool open tcp 63001

If you would like to make it permanent, edit your config.  

### Installing

The repo is designed for easy deployment in a nix dev shell. 

Clone the project and `cd` into the root directory, then run

    nix develop

This should spin up both servers. 

#### Other distros

If you are not on NixOS, you will need to run the command in `flake.nix`, namely:

    python3 server.py &
    python3 -m http.server 63001 &
    trap "kill 0" EXIT

This spins-up both servers in a single terminal window and will run until the terminal is closed

Now open a web browser and visit `localhost:63001` if you are connecting from the main PC. Or visit `ip_address:63001` if on another PC in your local network.

Type some text and you will see it appear magically in any browser currently visiting the ip address.

## Architecture

The scratch pad functions by using the standardized web feature `designMode` available in all browsers which allows editing arbitrary html page content. So we make the entire page a single body `div` that we can edit and paste text into.

The scratch pad content is stored in browser local storage. A timer updates a local `content.txt` backup file every 20 seconds.

The local text is diffed using [https://github.com/jhchen/fast-diff](https://github.com/jhchen/fast-diff) and only diffs are sent through the socket.

The html file uses JetBrains Mono from Google Fonts but will fallback to monospace. 

## License

Apache 2 license to stay in-sync with `fast-diff`.

`fast-diff` Copyright 2014-2023 Jason Chen 

See LICENSE for attribution.





