# ScratchPad
A simple self-hosted scratch pad. Move text between PCs on your local network, with edits synced in real time to every open browser. It's a single small binary with no runtime dependencies.




https://github.com/user-attachments/assets/f2c80fc5-60e5-416a-940d-48f7640ddd6b




(Caution: there's no merging if several people edit at once. The last edit wins. It's meant as a scratch pad for one person at a time.)

One process on **port 63001** serves the page and the WebSocket that keeps browsers in sync.

__Caution! There is no encryption or login. Anything pasted in the scratchpad is sent in the clear to anyone who can reach the port. Don't use it for sensitive information!__ (Other *websites* can't reach it: the server rejects WebSocket connections from other origins.)

## Running

Open port 63001 in your firewall, start ScratchPad, then visit `localhost:63001` on the same PC or `<ip-address>:63001` from another PC on your network.

### From source (Windows, Linux, macOS)

Requires [Go](https://go.dev/dl/) 1.24 or newer.

```
go build
./ScratchPad
```

To cross-compile, e.g. for Windows: `GOOS=windows GOARCH=amd64 go build`.

### Options

| Flag | Environment variable | Default |
|---|---|---|
| `-listen` | `SCRATCHPAD_LISTEN` | `:63001` (all interfaces) |
| `-data-dir` | `SCRATCHPAD_DATA_DIR` | OS user data dir (see below) |
| `-allowed-origins` | `SCRATCHPAD_ALLOWED_ORIGINS` | none (same-origin only) |
| `-max-mib` | `SCRATCHPAD_MAX_MIB` | `0` (no limit) |
| `-save-interval` | | `5s` |

`-allowed-origins` is only needed behind a reverse proxy that serves the page under a different hostname than the one ScratchPad sees, e.g. `-allowed-origins pad.example.com`. HTTPS reverse proxies work: the page uses `wss://` automatically.

### Docker

An image for `linux/amd64` and `linux/arm64` is published to `ghcr.io` from `main`:

```
docker run -d -p 63001:63001 -v scratchpad-data:/data ghcr.io/nwoodsman/scratchpad:latest
```

Or use the included [`docker-compose.yml`](docker-compose.yml): `docker compose up -d`.

### NixOS

#### Try temporarily

    nix run github:NWoodsman/ScratchPad

To reach it from another PC, open the port until the next boot:

    sudo nixos-firewall-tool open tcp 63001

#### Installing

Add ScratchPad to your `flake.nix`:

```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    ScratchPad.url = "github:NWoodsman/ScratchPad";
  };

  outputs = { self, nixpkgs, ScratchPad, ... }: {
    nixosConfigurations.myhostname = nixpkgs.lib.nixosSystem {
      system = "x86_64-linux";
      modules = [
        ScratchPad.nixosModules.default
        ./configuration.nix
        ./hardware-configuration.nix
      ];
    };
  };
}
```

and enable it in `configuration.nix`:

```nix
{
  services.ScratchPad = {
    enable = true;
    openFirewall = true;      # default: false
    # port = 63001;
    # listenAddress = "";     # empty = all interfaces
    # allowedOrigins = [ ];   # extra hostnames, for reverse proxies
  };
}
```

Content is stored in `/var/lib/ScratchPad`.

## Storage

The server keeps the text in memory and writes it to `content.txt` within a few seconds of a change, and again on shutdown. Writes are atomic, so a crash can't leave a half-written file.

`content.txt` is stored in:

- Windows: `%LOCALAPPDATA%\ScratchPad`
- macOS: `~/Library/Application Support/ScratchPad`
- Linux: `$XDG_DATA_HOME/ScratchPad`, or `~/.local/share/ScratchPad`
- Docker: the `/data` volume
- NixOS module: `/var/lib/ScratchPad`

## How it works

The page is a single full-screen `<textarea>` built into the binary. On every edit, the browser sends the whole text over the WebSocket, and the server forwards it to every other open browser. When text arrives from another browser, your cursor and scroll position stay where they were.

If the connection drops, a "disconnected" badge appears and the page keeps reconnecting. Anything you type while offline is sent once the connection is back, replacing the server's copy.

The page uses JetBrains Mono if it's installed locally and otherwise falls back to your system's monospace font. It makes no external requests, so it works on networks without internet access.

## Development

```
nix develop        # or install Go yourself
go test -race ./...
```

If you change Go dependencies, update `vendorHash` in `package.nix`. To get the new hash, set it to `lib.fakeHash`, run `nix build`, and copy the hash it reports.

## License

Apache 2.0.
