# Flipper

A self-hosted display server for [TRMNL](https://usetrmnl.com) e-ink devices. Point your device at Flipper and it serves images from a local directory — resized, dithered, and encoded for the e-ink display.

## How it works

1. Drop images (PNG, JPG, BMP) into a directory
2. Flipper processes them for the e-ink display (grayscale, dither, resize)
3. Your TRMNL device fetches the next image on each refresh cycle
4. Images rotate in order, no cloud service needed

## Getting started

You need Go 1.25+ and `make`.

```bash
# Build
make build

# Create a config file (optional — defaults work for local dev)
mkdir -p ~/.config/flipper
cat > ~/.config/flipper/config.toml <<'EOF'
[server]
addr       = ":3000"
secret_key = "change-me"
setup_mode = true

[screens.static]
dir = "~/Pictures/trmnl"
EOF

# Add some images
mkdir -p ~/Pictures/trmnl
# drop a few .png or .jpg files in there

# Run
./bin/flipper serve
```

Then register a device and fetch a display:

```bash
# Register (returns an API key)
curl -H "ID: AA:BB:CC:DD:EE:FF" http://localhost:3000/api/setup

# Fetch display
curl -H "ID: AA:BB:CC:DD:EE:FF" -H "Access-Token: <api_key>" http://localhost:3000/api/display
```

### Using Claude Code?

Run `/dev setup` for an interactive guided walkthrough that handles all of the above. Run `/dev doctor` at any time to check your environment and fix issues.

## Configuration

Flipper reads TOML config from `$XDG_CONFIG_HOME/flipper/config.toml` (or pass `--config <path>`). Every field has a sensible default and can be overridden with environment variables.

| Setting | Config key | Env var | Default |
|---------|-----------|---------|---------|
| Listen address | `server.addr` | `FLIPPER_ADDR` | `:3000` |
| Secret key | `server.secret_key` | `FLIPPER_SECRET_KEY` | `change-me` |
| Allow new devices | `server.setup_mode` | `FLIPPER_SETUP_MODE` | `true` |
| Display width | `device.width` | `FLIPPER_WIDTH` | `800` |
| Display height | `device.height` | `FLIPPER_HEIGHT` | `480` |
| Output format | `device.format` | `FLIPPER_FORMAT` | `bmp` |
| Refresh interval (s) | `device.refresh_rate` | `FLIPPER_REFRESH_RATE` | `900` |
| Image directory | `screens.static.dir` | `FLIPPER_STATIC_DIR` | `~/Pictures/trmnl` |

## API

| Endpoint | Method | Headers | Description |
|----------|--------|---------|-------------|
| `/api/setup` | GET | `ID: <MAC>` | Register a device (requires `setup_mode`) |
| `/api/display` | GET | `ID: <MAC>`, `Access-Token: <key>` | Get next display image URL |
| `/api/log` | POST | `ID: <MAC>` (optional) | Accept device log messages |
| `/images/{filename}` | GET | — | Serve processed images |

## License

Copyright (c) 2026 Oliver Eilhard. All rights reserved. See [LICENSE](LICENSE).
