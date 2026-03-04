# Flipper

TRMNL e-ink display server written in Go. Serves images from a local directory to TRMNL devices over HTTPS.

## Project layout

```
cmd/flipper/        CLI entrypoint (cobra)
internal/
  config/           TOML config + env var loading
  device/           Device registry (MAC auth, devices.json)
  display/          Image pipeline (resize, grayscale, dither, encode)
  handler/          HTTP handlers (setup, display, images, log)
  screen/           Screen interface + static screen implementation
  selfcert/         Self-signed TLS certificate generation
  server/           Wires everything, runs HTTP server
  xdg/              XDG Base Directory paths (~/.config, ~/.local/share)
```

## Quick reference

```bash
make build          # build to bin/flipper
make test           # run all tests
make setup          # go mod tidy
./bin/flipper serve  # start server (default :3443, HTTPS)
./bin/flipper serve --config ./config.toml  # custom config path
./bin/flipper devices          # list registered devices + telemetry
./bin/flipper devices --json   # machine-readable device list
./bin/flipper devices rename <mac> <name>   # set friendly name
./bin/flipper devices remove <mac>          # remove a device
./bin/flipper config edit      # open config in $EDITOR
```

## TLS

The server runs HTTPS with a self-signed certificate by default. The `selfcert` package generates an in-memory ECDSA P-256 cert on startup, including the machine's local IPs as SANs. TRMNL devices connect to `https://<ip>:3443` and accept the self-signed cert. This works as-is on the local network; do not disable TLS or add a reverse proxy unless there is a specific reason.

## Config

TOML at `~/.config/flipper/config.toml` (or pass `--config <path>`). All fields have defaults. Environment variables (`FLIPPER_ADDR`, `FLIPPER_SECRET_KEY`, etc.) override file values.

## Conventions

- Go standard library preferred over dependencies
- `slog` for logging
- No database — device state in `devices.json`, images in memory
- Tests use `httptest` and `t.TempDir()`
- When adding or changing CLI commands or features, update README.md, CLAUDE.md quick reference, and `.claude/skills/dev/` accordingly
- Imports in 3 groups separated by blank lines: standard library, external, internal

```go
import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/olivere/flipper/internal/config"
)
```
