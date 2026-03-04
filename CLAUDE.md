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
```

## Config

TOML at `~/.config/flipper/config.toml` (or pass `--config <path>`). All fields have defaults. Environment variables (`FLIPPER_ADDR`, `FLIPPER_SECRET_KEY`, etc.) override file values.

## Conventions

- Go standard library preferred over dependencies
- `slog` for logging
- No database — device state in `devices.json`, images in memory
- Tests use `httptest` and `t.TempDir()`
- Imports in 3 groups separated by blank lines: standard library, external, internal

```go
import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/olivere/flipper/internal/config"
)
```
