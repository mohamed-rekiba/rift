# Rift

Share your localhost with the world. A simple, self-hosted rift service that exposes your local development server to the internet through SSH.

Think ngrok, but yours.

## What is this?

You're building a web app on your laptop. You want to:
- Show it to a client
- Test it on your phone
- Share it with a teammate
- Connect a webhook

Just run one SSH command and get a public URL. No installation, no accounts, pure SSH.

## Quick Start

**1. Start the server** (on your VPS or server):

```bash
./rift-server
```

**2. Share your local app** (from your laptop):

```bash
# Interactive mode (recommended) - full dashboard
ssh -t -R 0:localhost:3000 user@your-server.com -p 2222

# Simple mode - basic text output
ssh -R 0:localhost:3000 user@your-server.com -p 2222
```

**3. Done!**

With `-t` flag (interactive mode) you get:
- 📊 Real-time request monitoring
- 📈 Live statistics (bytes sent/received, request counts)
- 📱 QR code for mobile access
- ⌨️  Interactive keyboard controls

Without `-t` (simple mode) you get:
- 📝 Basic tunnel info
- 📊 Stats updates every 10 seconds
- 🚀 Lower resource usage

Anyone can now access your local app at the public URL shown.

**👉 See [QUICK_START.md](QUICK_START.md) for detailed testing instructions.**

---

## Installation

### Build from source

```bash
git clone https://github.com/mohamed-rekiba/rift.git
cd rift
go build -o rift-server ./cmd/server
```

### Or download the binary

*(Coming soon - check releases)*

---

## Development

### Prerequisites

**Required:**

- **Go 1.21+** - [Install Go](https://go.dev/doc/install)

**Optional (recommended):**

- **air** - Hot reload for development
- **delve** - Go debugger
- **gopls** - Go language server (for IDE support)
- **staticcheck** - Static analysis tool
- **golangci-lint** - Fast linters runner
- **goimports** - Code formatter with import organization

### Install Development Tools

```bash
# Install all development tools
go install github.com/air-verse/air@latest
go install github.com/go-delve/delve/cmd/dlv@latest
go install golang.org/x/tools/gopls@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install golang.org/x/tools/cmd/goimports@latest

# Verify installations
air -v
dlv version
gopls version
staticcheck -version
golangci-lint --version
goimports -h
```

### Setup

```bash
# Clone the repository
git clone https://github.com/mohamed-rekiba/rift.git
cd rift

# Install dependencies
make install

# Run the server
make run
```

### Development Workflow

**Hot reload (recommended):**

```bash
# Automatically rebuilds and restarts on file changes
make dev
# Or directly:
air
```

**Manual build and run:**

```bash
# Build and run
make run

# Build and run with debug logging
make run-debug

# Build only
make build
```

### Running Tests

```bash
# Run all tests with race detector
make test

# Run tests with coverage report
make test-cover
# Opens coverage.html in your browser
```

### Code Quality

Before committing, run all checks:

```bash
# Run all checks (format, vet, lint, test)
make check
```

Individual checks:

```bash
# Format code with gofmt
make fmt

# Format code with goimports (recommended)
make format

# Run go vet
make vet

# Run all linters (go vet, staticcheck, golangci-lint)
make lint
```

### Make Commands Reference

| Command | Description |
|---------|-------------|
| `make dev` | Start with hot reload (requires air) |
| `make build` | Build the binary |
| `make run` | Build and run with default settings |
| `make run-debug` | Build and run with debug logging |
| `make test` | Run tests with race detector |
| `make test-cover` | Run tests with coverage report |
| `make fmt` | Format code with gofmt |
| `make format` | Format code with goimports (recommended) |
| `make vet` | Run go vet |
| `make lint` | Run all linters (vet, staticcheck, golangci-lint) |
| `make check` | Run all checks (fmt, vet, lint, test) |
| `make clean` | Remove build artifacts |
| `make install` | Download dependencies |
| `make help` | Show all available commands |

### Project Structure

```
rift/
├── cmd/server/          # Application entry point
│   └── main.go
├── internal/            # Private application code
│   ├── config/          # Configuration loading
│   ├── proxy/           # HTTP reverse proxy
│   ├── registry/        # Tunnel registry
│   ├── ssh/             # SSH server
│   └── tui/             # Interactive dashboard
├── pkg/models/          # Shared domain models
├── web/                 # Embedded web assets
├── .air.toml            # Hot reload configuration
├── .env.example         # Example environment file
├── go.mod               # Go module definition
└── Makefile             # Build automation
```

---

## Configuration

Three ways to configure, pick what works for you:

### 1. Just run it (easiest)

```bash
./rift-server
```

Uses sensible defaults. Perfect for trying it out.

### 2. Use command-line flags

```bash
./rift-server -ssh-addr :2222 -http-addr :8080 -domain example.com
```

### 3. Use environment variables or .env file

```bash
# Create .env file
cp .env.example .env

# Edit it
vim .env

# Run
./rift-server
```

**Priority:** Flags override environment variables override .env file.

### Available Options

| Option | Flag | Environment | Default | Description |
|--------|------|-------------|---------|-------------|
| SSH Address | `-ssh-addr` | `SSH_ADDR` | `:2222` | Where SSH server listens |
| HTTP Address | `-http-addr` | `HTTP_ADDR` | `:8080` | Where HTTP proxy listens |
| Domain | `-domain` | `BASE_DOMAIN` | `localhost` | Base domain for rifts |
| Log Level | `-log-level` | `LOG_LEVEL` | `info` | debug, info, warn, error |

See [CONFIGURATION.md](CONFIGURATION.md) for all options including timeouts and cleanup intervals.

---

## Usage Examples

### Share your React dev server

```bash
# Start React app (usually on port 3000)
npm start

# In another terminal
ssh -t -R 0:localhost:3000 user@rift.example.com -p 2222
```

### Test webhooks locally

```bash
# Start your local server that receives webhooks
python -m http.server 8080

# Create rift
ssh -t -R 0:localhost:8080 user@rift.example.com -p 2222

# Use the public URL in your webhook settings
```

### Show your work to a client

```bash
# Start your local web server
./start-server.sh

# Create rift
ssh -t -R 0:localhost:8000 user@rift.example.com -p 2222

# Share the URL with your client
```

### Keep the rift alive

Use `autossh` to automatically reconnect:

```bash
autossh -M 0 -t -R 0:localhost:3000 user@rift.example.com -p 2222
```

**Note:** The `-t` flag forces PTY allocation, enabling the interactive dashboard. Without it, you'll see simple text output instead.

---

## How It Works

```
┌──────────────────────────────────────────────────────────┐
│  You: localhost:3000                                     │
│  (your laptop)                                           │
└──────────────────┬───────────────────────────────────────┘
                   │
                   │ SSH rift
                   │ (encrypted)
                   │
┌──────────────────▼───────────────────────────────────────┐
│  rift Server                                           │
│  - Accepts SSH connections                               │
│  - Generates random subdomain (abc12345)                 │
│  - Routes HTTP requests → your SSH rift → your laptop  │
└──────────────────┬───────────────────────────────────────┘
                   │
                   │ HTTP
                   │
┌──────────────────▼───────────────────────────────────────┐
│  Anyone on the internet                                  │
│  http://abc12345.example.com                             │
└──────────────────────────────────────────────────────────┘
```

When someone visits your public URL:
1. Request hits the rift server
2. Server forwards it through the SSH rift
3. Request arrives at your localhost:3000
4. Your app responds
5. Response goes back through the rift
6. Visitor sees your page

All traffic is encrypted through SSH.

---

## Server Setup

### For Production

**1. Get a server** (any VPS works - DigitalOcean, AWS, Linode, etc.)

**2. Point a domain at it:**

```
A    rift.example.com     → 1.2.3.4
A    *.rift.example.com   → 1.2.3.4
```

The wildcard (`*`) is important - it lets subdomains work.

**3. Run the server:**

```bash
./rift-server -domain rift.example.com
```

**4. Done!** Users can now:

```bash
ssh -R 0:localhost:8000 user@rift.example.com -p 2222
```

### With Docker

```bash
docker build -t rift-server .
docker run -p 2222:2222 -p 80:8080 \
  -e BASE_DOMAIN=rift.example.com \
  rift-server
```

### With systemd

Create `/etc/systemd/system/rift.service`:

```ini
[Unit]
Description=Rift rift Service
After=network.target

[Service]
Type=simple
User=rift
ExecStart=/usr/local/bin/rift-server -domain rift.example.com
Restart=always

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl enable rift
sudo systemctl start rift
```

---

## Features

- ✅ Works with any local port
- ✅ Instant setup, one SSH command
- ✅ Automatic subdomain assignment
- ✅ Handles multiple tunnels simultaneously
- ✅ Graceful shutdown (press Ctrl+C)
- ✅ Configurable timeouts and limits
- ✅ Clean, structured logging
- ✅ Self-hosted, no external dependencies

---

## Stopping Things

### Stop a tunnel

Press **Ctrl+C** in the SSH session. The tunnel closes immediately.

### Stop the server

Press **Ctrl+C** where the server is running. It will:
1. Stop accepting new connections
2. Notify active users
3. Clean up resources
4. Exit cleanly

---

## Security Notes

**Current status:** This is an MVP. It works great for:
- Local development
- Trusted teams
- Internal networks
- Personal use

**Not recommended for:**
- Public production use without adding authentication
- Untrusted users

**Why?** Currently anyone can create a tunnel if they can reach your server.

**Coming soon:** SSH key authentication, rate limiting, user accounts.

For now, use firewall rules or VPN to restrict access.

---

## Troubleshooting

### "Connection refused"

Server isn't running or firewall is blocking. Check:

```bash
# Is the server running?
ps aux | grep rift-server

# Are the ports listening?
netstat -tuln | grep 2222
netstat -tuln | grep 8080
```

### "Permission denied"

Server might need root to bind to ports 22 or 80. Either:
- Use higher ports (2222, 8080) - easier
- Run with sudo - not recommended
- Use `setcap` - better for production

### URL doesn't resolve

Check your DNS:

```bash
# Should return your server IP
dig abc12345.rift.example.com

# Wildcard working?
dig anything.rift.example.com
```

### rift closes immediately

Check server logs:

```bash
./rift-server -log-level debug
```

---

## What's Next

- [ ] SSH key authentication
- [ ] Web dashboard to see active rifts
- [ ] Custom subdomain names
- [ ] HTTPS/TLS support
- [ ] TCP and UDP rifts (not just HTTP)
- [ ] Traffic inspection and replay

---

## Documentation

- **[CONFIGURATION.md](CONFIGURATION.md)** - All configuration options
- **[IMPLEMENTATION.md](IMPLEMENTATION.md)** - Architecture and code details
- **[CHANGELOG.md](CHANGELOG.md)** - Version history

---

## Contributing

Found a bug? Want a feature? Pull requests welcome!

1. Fork it
2. Create a branch (`git checkout -b feature/amazing`)
3. Commit changes (`git commit -am 'Add something amazing'`)
4. Push (`git push origin feature/amazing`)
5. Open a Pull Request

---

## License

MIT - Use it however you want.

---

## Credits

Built with:
- [gliderlabs/ssh](https://github.com/gliderlabs/ssh) - SSH server in Go
- [joho/godotenv](https://github.com/joho/godotenv) - .env file support
- Go's excellent standard library

Inspired by ngrok, Pinggy, and localtunnel.

---

**Made with ☕ by humans, for humans.**

Got questions? Open an issue.
