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

## Ecosystem

| Repository | Description |
|---|---|
| [cgram-proto](https://github.com/isalikov/cgram-proto) | Protobuf schema definitions |
| [cgram-server](https://github.com/isalikov/cgram-server) | Stateless relay server |
| **cgram-cli** | Terminal client (this repo) |

The server acts as a stateless relay — it never sees message content. All encryption and decryption happens client-side.

## Quick Start

### Prerequisites

- Go 1.22+
- Running cgram-server instance

### Installation

```bash
git clone https://github.com/isalikov/cgram-cli.git
cd cgram-cli
cp .env.example .env
make build
./bin/cgram
```

### Development

```bash
make dev
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `SERVER_ADDR` | `127.0.0.1:8080` | Server WebSocket address |
| `DB_PATH` | `cgram.db` | SQLite database path |

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
| `:add <user_id>` | Add a contact |
| `:delete` | Delete selected contact |
| `:quit` / `:q` | Quit application |
| `:help` | Show help |

## Protocol

Communication uses WebSocket with binary Protobuf frames. Each frame is a `Frame` message containing a `request_id` for request-response correlation and a `payload` oneof with typed messages for auth, messaging, and key exchange.

### Authentication Flow

1. **Register** — sends username, password verifier, Ed25519 public key
2. **Login** — sends username, auth message; receives session token
3. **Upload pre-keys** — sends X3DH pre-key bundle for key exchange

### Message Flow

1. Sender fetches recipient's pre-key bundle
2. X3DH key agreement derives a shared secret
3. Message payload encrypted with NaCl secretbox
4. Encrypted envelope sent via server relay
5. Recipient decrypts using shared secret + ratchet index

## Development Commands

```
make build    Build binary to ./bin/cgram
make run      Run without building (go run)
make dev      Run with .env loaded
make clean    Remove ./bin directory
make test     Run tests
make help     Show this help
```

## License

MIT
