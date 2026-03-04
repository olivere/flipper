# Flipper

A self-hosted display server for [TRMNL](https://usetrmnl.com) e-ink devices. Point your device at Flipper and it serves images from a local directory — resized, dithered, and encoded for the e-ink display.

## How it works

1. Drop images (PNG, JPG, BMP) into a directory
2. Flipper processes them for the e-ink display (grayscale, dither, resize)
3. Your TRMNL device fetches the next image on each refresh cycle
4. Images rotate in order, no cloud service needed

## Getting started

You need Go 1.26+ and `make`.

```bash
# Build
make build

# Create a config file (optional — defaults work for local dev)
mkdir -p ~/.config/flipper
cat > ~/.config/flipper/config.toml <<'EOF'
[server]
addr       = ":3443"
secret_key = "change-me"
setup_mode = true

[screens.static]
dir = "~/Pictures/trmnl"
EOF

# Add some images
mkdir -p ~/Pictures/trmnl
# drop a few .png or .jpg files in there

# Run (starts with HTTPS by default)
./bin/flipper serve
```

Then register a device and fetch a display:

```bash
# Register (returns an API key)
curl -sk -H "ID: AA:BB:CC:DD:EE:FF" https://localhost:3443/api/setup

# Fetch display
curl -sk -H "ID: AA:BB:CC:DD:EE:FF" -H "Access-Token: <api_key>" https://localhost:3443/api/display
```

### Using Claude Code?

Run `/dev setup` for an interactive guided walkthrough that handles all of the above. Run `/dev doctor` at any time to check your environment and fix issues.

## Configuration

Flipper reads TOML config from `~/.config/flipper/config.toml` (or pass `--config <path>`). Every field has a sensible default and can be overridden with environment variables. To edit the config in your `$EDITOR`:

```bash
./bin/flipper config edit
```

| Setting | Config key | Env var | Default |
|---------|-----------|---------|---------|
| Listen address | `server.addr` | `FLIPPER_ADDR` | `:3443` |
| Secret key | `server.secret_key` | `FLIPPER_SECRET_KEY` | `change-me` |
| Allow new devices | `server.setup_mode` | `FLIPPER_SETUP_MODE` | `true` |
| Disable TLS | `server.tls.disabled` | `FLIPPER_TLS_DISABLED` | `false` |
| TLS cert file | `server.tls.cert_file` | `FLIPPER_TLS_CERT_FILE` | (auto-generated) |
| TLS key file | `server.tls.key_file` | `FLIPPER_TLS_KEY_FILE` | (auto-generated) |
| Display width | `device.width` | `FLIPPER_WIDTH` | `800` |
| Display height | `device.height` | `FLIPPER_HEIGHT` | `480` |
| Output format | `device.format` | `FLIPPER_FORMAT` | `bmp` |
| Refresh interval (s) | `device.refresh_rate` | `FLIPPER_REFRESH_RATE` | `900` |
| Rotate screens | `screens.rotate` | `FLIPPER_SCREENS_ROTATE` | `false` |
| Image directory | `screens.static.dir` | `FLIPPER_STATIC_DIR` | `~/Pictures/trmnl` |

### HTTPS

Flipper serves HTTPS by default using an auto-generated self-signed certificate. TRMNL devices require HTTPS but skip certificate validation (`setInsecure()` in the firmware), so self-signed works fine. Plain HTTP does **not** work with TRMNL devices. The certificate includes `localhost`, `127.0.0.1`, and all local network IPs in its SANs.

To use plain HTTP instead (e.g. behind your own reverse proxy that terminates TLS):

```toml
[server.tls]
disabled = true
```

Or: `FLIPPER_TLS_DISABLED=true ./bin/flipper serve`

To use your own certificate:

```toml
[server.tls]
cert_file = "/path/to/cert.pem"
key_file  = "/path/to/key.pem"
```

### Connecting a TRMNL device

1. Start Flipper: `./bin/flipper serve`
2. Find your local IP: `ipconfig getifaddr en0` (macOS)
3. On the device, hold the button for 5-7 seconds to enter setup mode
4. Connect to the "TRMNL" WiFi hotspot from your phone or computer
5. In the portal, go to **Advanced > Custom Server > Yes**
6. Enter `https://<your-local-ip>:3443` (no trailing slash)
7. Go back, select your WiFi network, enter password, click Connect
8. Press the device button to force an immediate refresh

The device will call `/api/setup` to register, then `/api/display` to fetch its first image. Check registration with `flipper devices`:

```bash
$ ./bin/flipper devices
MAC                NAME         FIRMWARE  BATTERY  RSSI  MODEL  LAST SEEN
AA:BB:CC:DD:EE:FF  —            1.7.4     4.07     -65   og     2m ago
```

### Managing devices

```bash
# List all devices (table)
./bin/flipper devices

# List all devices (JSON)
./bin/flipper devices --json

# Give a device a friendly name
./bin/flipper devices rename AA:BB:CC:DD:EE:FF "Living Room"

# Remove a device from the registry
./bin/flipper devices remove AA:BB:CC:DD:EE:FF
```

Device telemetry (firmware version, battery voltage, WiFi RSSI, model) is captured automatically from headers sent by the device on each display request.

### Reverse proxy (advanced)

If you prefer to terminate TLS externally (e.g. with Caddy or nginx), disable Flipper's built-in TLS and set up your proxy to forward to the plain HTTP port. Flipper detects the `X-Forwarded-Proto` header and generates correct `https://` image URLs when proxied.

## API

| Endpoint | Method | Headers | Description |
|----------|--------|---------|-------------|
| `/api/setup` | GET | `ID: <MAC>` | Register a device (requires `setup_mode`) |
| `/api/display` | GET | `ID: <MAC>`, `Access-Token: <key>` | Get next display image URL |
| `/api/log` | POST | `ID: <MAC>` (optional) | Accept device log messages |
| `/images/{filename}` | GET | — | Serve processed images |

### Token adoption

When a TRMNL device migrates from another server (e.g. the TRMNL cloud), it may send an API key that doesn't match the one Flipper derived during setup. While `setup_mode` is enabled, Flipper automatically adopts the device's token on first contact, so devices work without manual key reconfiguration. Disable `setup_mode` after onboarding to lock down token adoption.

## FAQ

### Images show ghosting or overlay of previous images

This is an e-ink partial refresh artifact, not a server issue. E-ink displays have two refresh modes: full refresh (flashes black/white to fully clear the screen) and partial refresh (only updates changed pixels, which leaves remnants of the previous image). The TRMNL firmware controls which mode is used — there is no server-side field to force a full refresh.

Things that may help:

- **Increase `refresh_rate`** — very low values like 30s are aggressive and may cause the firmware to skip full refreshes. Try 120s or higher for sustained use.
- **Use high-contrast source images** — images with large solid areas transition more cleanly between refreshes.
- **Press the device button** — a manual refresh typically triggers a full screen clear.

### What device telemetry is available?

The TRMNL device sends the following HTTP headers on each `/api/display` request:

| Header | Description | Example |
|--------|-------------|---------|
| `Battery-Voltage` | Battery level in volts | `4.07` |
| `FW-Version` | Firmware version | `1.7.4` |
| `Model` | Device model identifier | — |
| `RSSI` | WiFi signal strength (dBm) | `-65` |
| `Refresh-Rate` | Current refresh rate (seconds) | `900` |
| `Width` | Display width (pixels) | `800` |
| `Height` | Display height (pixels) | `480` |

The device also sends detailed log entries via `POST /api/log` including free heap size, wake reason, wifi status, and retry counts.

### What `special_function` values does the firmware support?

The `/api/display` response can include a `special_function` string field. Supported values:

| Value | Description |
|-------|-------------|
| `none` | No special function (default) |
| `identify` | Show identification screen |
| `sleep` | Put device to sleep for 8 hours |
| `add_wifi` | Activate WiFi captive portal |
| `restart_playlist` | Restart playlist from first position |
| `rewind` | Go back to previous screen |
| `send_to_me` | Email current screen to user |
| `guest_mode` | Switch to guest display mode |

## License

Copyright (c) 2026 Oliver Eilhard. All rights reserved. See [LICENSE](LICENSE).
