# Flipper

TRMNL e-ink display server written in Go. Serves images from a local directory to TRMNL devices over HTTP.

## Project layout

```
cmd/flipper/        CLI entrypoint (cobra)
internal/
  config/           TOML config + env var loading
  device/           Device registry (MAC auth, devices.json)
  display/          Image pipeline (resize, grayscale, dither, encode)
  handler/          HTTP handlers (setup, display, images, log)
  screen/           Screen interface + static screen implementation
  server/           Wires everything, runs HTTP server
```

## Quick reference

```bash
make build          # build to bin/flipper
make test           # run all tests
make setup          # go mod tidy
./bin/flipper serve  # start server (default :3000)
./bin/flipper serve --config ./config.toml  # custom config path
```

## Config

TOML at `$XDG_CONFIG_HOME/flipper/config.toml`. All fields have defaults. Environment variables (`FLIPPER_ADDR`, `FLIPPER_SECRET_KEY`, etc.) override file values.

## Conventions

- Go standard library preferred over dependencies
- `slog` for logging
- No database — device state in `devices.json`, images in memory
- Tests use `httptest` and `t.TempDir()`
