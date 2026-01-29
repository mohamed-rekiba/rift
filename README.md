# Rift

Share your localhost with the world. Self-hosted SSH tunnels that expose your localhost to the world. Like ngrok, but yours.

Think ngrok, but yours.

## Why Rift?

You're building something on your laptop and need to:
- Show it to a client
- Test it on your phone
- Share it with a teammate
- Hook up a webhook

One SSH command, instant public URL. No signup, no client install, just SSH.

## Getting Started

**1. Start the server** (on your VPS):

```bash
./rift-server
```

**2. Create a tunnel** (from your laptop):

```bash
ssh -t -R 0:localhost:3000 user@your-server.com -p 2222
```

**3. That's it.** Share the URL and anyone can reach your local app.

The `-t` flag gives you a nice dashboard with live stats and a QR code. Skip it for simple text output.

## Installation

### Build from source

```bash
git clone https://github.com/mohamed-rekiba/rift.git
cd rift
go build -o rift-server ./cmd/server
```

### Download a release

Grab the latest binary from [GitHub Releases](https://github.com/mohamed-rekiba/rift/releases).

**macOS note:** If you see the "unidentified developer" warning:

```bash
xattr -d com.apple.quarantine rift-server-*
chmod +x rift-server-*
```

## Configuration

Rift works out of the box with sensible defaults. Customize if you need to:

```bash
# Flags
./rift-server -domain tunnel.example.com -ssh-addr :2222 -http-addr :80

# Or environment variables
BASE_DOMAIN=tunnel.example.com ./rift-server

# Or a .env file
cp .env.example .env && vim .env
```

| Option | Flag | Env Var | Default |
|--------|------|---------|---------|
| SSH port | `-ssh-addr` | `SSH_ADDR` | `:2222` |
| HTTP port | `-http-addr` | `HTTP_ADDR` | `:8080` |
| Domain | `-domain` | `BASE_DOMAIN` | `localhost` |
| Log level | `-log-level` | `LOG_LEVEL` | `info` |

Flags take priority over env vars over defaults.

## Usage Examples

### Share your dev server

```bash
npm start  # Running on port 3000
ssh -t -R 0:localhost:3000 user@tunnel.example.com -p 2222
```

### Test webhooks locally

```bash
python -m http.server 8080
ssh -t -R 0:localhost:8080 user@tunnel.example.com -p 2222
# Point your webhook to the public URL
```

### Keep it alive with autossh

```bash
autossh -M 0 -t -R 0:localhost:3000 user@tunnel.example.com -p 2222
```

## Interactive Dashboard

When you use `-t`, you get a live dashboard:

| Key | What it does |
|-----|--------------|
| `q` | Quit |
| `?` | Toggle help |
| `↑/↓` | Browse requests |
| `Enter` | View details |
| `Esc` | Close details |

You'll see incoming requests in real-time, bytes transferred, and a QR code for quick mobile testing.

## How It Works

```
Your laptop (localhost:3000)
        │
        │ SSH tunnel (encrypted)
        ▼
Rift Server (your VPS)
        │
        │ HTTP
        ▼
Anyone on the internet
(http://abc123.tunnel.example.com)
```

Requests hit your server, travel through the SSH tunnel, reach your local app, and responses go back the same way. All encrypted.

## Server Setup

### DNS

Point your domain at your server:

```
A    tunnel.example.com     → YOUR_SERVER_IP
A    *.tunnel.example.com   → YOUR_SERVER_IP
```

The wildcard lets subdomains work.

### Run it

```bash
./rift-server -domain tunnel.example.com
```

### Keep it running (systemd)

Create `/etc/systemd/system/rift.service`:

```ini
[Unit]
Description=Rift Tunnel Service
After=network.target

[Service]
Type=simple
User=rift
ExecStart=/usr/local/bin/rift-server -domain tunnel.example.com
Restart=always

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now rift
```

### Docker

```bash
docker build -t rift-server .
docker run -p 2222:2222 -p 80:8080 -e BASE_DOMAIN=tunnel.example.com rift-server
```

## Development

### Prerequisites

- Go 1.21+
- Optional: `air` for hot reload, `golangci-lint` for linting

### Quick setup

```bash
git clone https://github.com/mohamed-rekiba/rift.git
cd rift
make install  # Download dependencies
make dev      # Start with hot reload
```

### Commands

| Command | Description |
|---------|-------------|
| `make dev` | Hot reload development |
| `make build` | Build binary |
| `make test` | Run tests |
| `make lint` | Run linters |
| `make format` | Format code |

### Project layout

```
rift/
├── cmd/server/       # Entry point
├── internal/
│   ├── cli/          # Flags, config, version
│   ├── proxy/        # HTTP reverse proxy
│   ├── registry/     # Tunnel management
│   ├── ssh/          # SSH server
│   └── tui/          # Interactive dashboard
├── pkg/models/       # Shared types
└── web/              # Embedded assets
```

## Troubleshooting

**"Connection refused"** — Server not running or port blocked. Check `netstat -tuln | grep 2222`.

**"Permission denied"** — Use higher ports (2222, 8080) or give the binary capabilities with `setcap`.

**URL doesn't work** — Check DNS: `dig anything.tunnel.example.com` should return your server IP.

**Tunnel drops immediately** — Run with `-log-level debug` and check what's happening.

## Security

This is built for development and trusted environments. Currently anyone who can reach your server can create tunnels.

Good for:
- Local development
- Internal teams
- Personal projects

Coming soon: SSH key auth, rate limiting.

For now, use firewall rules to limit who can connect.

## Roadmap

- [ ] SSH key authentication
- [ ] Custom subdomain names
- [ ] HTTPS support
- [ ] TCP tunnels (not just HTTP)
- [ ] Web dashboard

## Contributing

PRs welcome! Fork it, make a branch, submit a PR.

```bash
git checkout -b feature/cool-thing
git commit -am 'Add cool thing'
git push origin feature/cool-thing
```

## License

MIT — do whatever you want with it.

## Credits

Built with [gliderlabs/ssh](https://github.com/gliderlabs/ssh) and Go's standard library.

Inspired by ngrok, localtunnel, and the desire to own your tools.

---

Questions? [Open an issue](https://github.com/mohamed-rekiba/rift/issues).
