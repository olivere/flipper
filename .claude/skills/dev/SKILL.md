---
name: dev
description: "Development helper: `/dev setup` guides you from zero to image-on-device, `/dev doctor` diagnoses and repairs your environment"
argument-hint: <setup|doctor>
disable-model-invocation: true
allowed-tools: Bash, Read, Write, Edit, Glob, Grep, WebSearch, WebFetch, AskUserQuestion, TaskCreate, TaskUpdate, TaskList
---

# /dev — Flipper Development Helper

Route on `$ARGUMENTS`:

- If `$ARGUMENTS` starts with `setup` → Read and follow `.claude/skills/dev/setup.md`
- If `$ARGUMENTS` starts with `doctor` → Read and follow `.claude/skills/dev/doctor.md`
- Otherwise → Print usage:

```
Usage: /dev <command>

Commands:
  setup    Interactive guided walkthrough from zero to "image on device"
  doctor   Diagnostic sweep — checks everything works, offers repair when it doesn't
```

---

## Project Reference (shared context for both sub-commands)

### Paths

| What | Default location | Override |
|------|-----------------|----------|
| Binary | `bin/flipper` | Build output |
| Config | `$XDG_CONFIG_HOME/flipper/config.toml` | `--config <path>` flag |
| Device data | `$XDG_DATA_HOME/flipper/devices.json` | `XDG_DATA_HOME` env var |
| Image source | `~/Pictures/trmnl` | `screens.static.dir` in config |

On macOS, XDG defaults: config = `~/Library/Application Support`, data = `~/Library/Application Support`.

### Config format (TOML)

```toml
[server]
addr       = ":3000"
secret_key = "change-me"
setup_mode = true

[device]
width        = 800
height       = 480
format       = "bmp"
refresh_rate = 900

[screens]
rotate = false

[screens.static]
dir = "~/Pictures/trmnl"
```

Environment variable overrides: `FLIPPER_ADDR`, `FLIPPER_SECRET_KEY`, `FLIPPER_SETUP_MODE`, `FLIPPER_WIDTH`, `FLIPPER_HEIGHT`, `FLIPPER_FORMAT`, `FLIPPER_REFRESH_RATE`, `FLIPPER_STATIC_DIR`.

### Device protocol

**Register** — `GET /api/setup`
- Header: `ID: <MAC>` (e.g. `AA:BB:CC:DD:EE:FF`)
- Requires `setup_mode = true`
- Returns: `{"status":"ok","api_key":"<hex>"}`
- API key = lowercase hex of `SHA1(secret_key + uppercase_MAC)`

**Display** — `GET /api/display`
- Headers: `ID: <MAC>`, `Access-Token: <api_key>`
- Optional: `WIDTH`, `HEIGHT`
- Returns JSON with `image_url`, `refresh_rate` (string), etc.

**Log** — `POST /api/log`
- Header: `ID: <MAC>` (optional)
- Body: JSON or raw text (max 1 MB)

**Image serving** — `GET /images/{filename}`
- Images are in-memory cache, not on disk
- Content-Type: `image/bmp` or `image/png`

### Build targets

```
make install   # go install ./cmd/flipper
make setup     # go mod tidy
make build     # go build -o bin/flipper ./cmd/flipper
make test      # go test ./...
```

### Supported image formats (input)

`.png`, `.jpg`, `.jpeg`, `.bmp` — the static screen serves files from the configured directory in lexicographic order (round-robin).

### No health endpoint

There is no `/health` route. To check if the server is up, curl any endpoint (e.g. `GET /api/setup` with an ID header) or just check if the port is open.
