# cgram-cli

Terminal client for **cgram** — an anonymous end-to-end encrypted messenger.

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and the [Nord](https://www.nordtheme.com/) color palette.

## Features

- End-to-end encryption (X3DH key exchange + NaCl secretbox)
- WebSocket + Protobuf binary protocol
- Local message history (SQLite)
- OS notifications (macOS, Linux, Windows)
- Vim-style command mode
- Auto-reconnect on connection loss
- Nord dark theme

## Ecosystem

| Repository | Description |
|---|---|
| [cgram-proto](https://github.com/isalikov/cgram-proto) | Protobuf schema definitions |
| [cgram-server](https://github.com/isalikov/cgram-server) | Stateless relay server |
| **cgram-cli** | Terminal client (this repo) |

The server acts as a stateless relay — it never sees message content. All encryption and decryption happens client-side.

## Installation

### macOS (Homebrew)

```bash
brew tap isalikov/tap
brew install cgram
```

### macOS (manual)

```bash
# Apple Silicon (M1/M2/M3/M4)
curl -Lo cgram https://github.com/isalikov/cgram-cli/releases/latest/download/cgram-darwin-arm64

# Intel
curl -Lo cgram https://github.com/isalikov/cgram-cli/releases/latest/download/cgram-darwin-amd64

chmod +x cgram
sudo mv cgram /usr/local/bin/
```

### Linux

```bash
# x86_64
curl -Lo cgram https://github.com/isalikov/cgram-cli/releases/latest/download/cgram-linux-amd64

# ARM64
curl -Lo cgram https://github.com/isalikov/cgram-cli/releases/latest/download/cgram-linux-arm64

chmod +x cgram
sudo mv cgram /usr/local/bin/
```

### Windows

Download `cgram-windows-amd64.exe` from the [Releases](https://github.com/isalikov/cgram-cli/releases/latest) page, rename to `cgram.exe` and add to `PATH`.

### Docker

```bash
docker run -it ghcr.io/isalikov/cgram-cli:latest
```

### Build from source

```bash
git clone https://github.com/isalikov/cgram-cli.git
cd cgram-cli
make install
```

This builds and installs `cgram` to `/usr/local/bin/`. To remove: `make uninstall`.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `SERVER_ADDR` | `127.0.0.1:8080` | Server WebSocket address |
| `DATA_DIR` | `~/.cgram` | Local data directory |

Configuration is loaded from environment variables or a `.env` file in the working directory.

## Keyboard Shortcuts

| Key | Action |
|---|---|
| `Tab` | Switch between contacts and chat |
| `j/k`, `Up/Down` | Navigate contacts / scroll messages |
| `Enter` | Open chat / send message |
| `Shift+Enter` | New line in message |
| `:` | Command mode |
| `?` | Help overlay |
| `Esc` | Close overlay |
| `Ctrl+C` | Quit |

### Commands

| Command | Description |
|---|---|
| `:add <username>` | Add a contact |
| `:rename <name>` | Rename selected contact |
| `:delete` | Delete selected contact (`:delete!` to confirm) |
| `:id` | Show your user ID |
| `:help` | Show help |
| `:quit` / `:q` | Quit |

## Architecture

```
cgram-cli/
├── cmd/cgram/main.go              — entry point
├── internal/
│   ├── tui/                       — Bubble Tea UI
│   │   ├── app.go                 — main model, screen management
│   │   ├── welcome.go             — login/register screen
│   │   ├── chat.go                — message display + input
│   │   ├── contacts.go            — contact list panel
│   │   ├── input.go               — multiline text input
│   │   ├── notification.go        — toast notifications
│   │   ├── help.go                — hotkey overlay
│   │   ├── styles.go              — Nord theme styles
│   │   └── keys.go                — key bindings
│   ├── client/client.go           — WebSocket client with reconnect
│   ├── crypto/crypto.go           — E2E encryption (X3DH, NaCl)
│   ├── store/store.go             — SQLite local storage
│   ├── notify/notify.go           — OS notifications
│   └── config/config.go           — environment configuration
├── Makefile
├── Dockerfile
└── .github/workflows/release.yml
```

## Protocol

Communication uses WebSocket with binary Protobuf frames. Each frame contains a `request_id` for request-response correlation and a `payload` oneof with typed messages.

### Authentication

1. **Register** — sends username, password verifier, Ed25519 public key
2. **Login** — sends username, auth message; receives session token
3. **Upload pre-keys** — sends X3DH pre-key bundle for key exchange

### Messaging

1. Sender fetches recipient's pre-key bundle
2. X3DH key agreement derives a shared secret
3. Message encrypted with NaCl secretbox (XSalsa20 + Poly1305)
4. Encrypted envelope sent via server relay
5. Recipient decrypts using shared secret + ratchet index

## Development

```
make build      Build binary to ./bin/cgram
make install    Build and install to /usr/local/bin/cgram
make uninstall  Remove from /usr/local/bin/cgram
make run        Run without building
make dev        Run with .env loaded
make clean      Remove ./bin directory
make test       Run tests
make vendor     Download dependencies to vendor/
make help       Show available targets
```

## License

MIT
