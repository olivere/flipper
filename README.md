# Flipper

A self-hosted display server for [TRMNL](https://usetrmnl.com) e-ink devices. Point your device at Flipper and it serves images from a local directory — resized, dithered, and encoded for the e-ink display.

## How it works

1. Drop images (PNG, JPG, BMP) into a directory
2. Flipper processes them for the e-ink display (grayscale, dither, resize)
3. Your TRMNL device fetches the next image on each refresh cycle
4. Images rotate in order, no cloud service needed

Flipper supports TRMNL OG (800×480, B&W) and TRMNL X (1872×1404, grayscale) devices. Device capabilities are detected automatically from request headers.

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
| Demo screen | `screens.demo.enabled` | — | `false` |

### Playlist

A playlist defines the order and timing of screens shown on the device. When a `[[playlist]]` section is present in the config, it replaces the `screens.rotate` behavior. Each entry specifies a screen type, an optional duration, and optional parameters.

```toml
[[playlist]]
screen = "news"
duration = "2m"

[[playlist]]
screen = "static"
duration = "60s"

[[playlist]]
screen = "weather"
duration = "2m"
params.city = "Munich"
params.forecast_days = 7

[[playlist]]
screen = "hackernews"
duration = "60s"
```

| Field | Description |
|-------|-------------|
| `screen` | Screen type: `static`, `demo`, `weather`, `hackernews`, `news`, `fcbayern` |
| `duration` | How long to show this screen (e.g. `60s`, `2m`). Optional — defaults to `device.refresh_rate` |
| `params.*` | Screen-specific parameters (e.g. `params.city` for weather) |

The same screen type can appear multiple times with different parameters (e.g. weather for different cities). Unknown screen types are skipped with a warning. When no playlist is defined, the existing `screens.rotate` behavior applies.

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
MAC                NAME         FIRMWARE  BATTERY      RSSI  MODEL  LAST SEEN
AA:BB:CC:DD:EE:FF  —            1.7.4     4.07V (89%)  -65   og     2m ago
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

# Add a LATEST column showing the newest firmware released upstream
./bin/flipper devices --check-updates
```

Device telemetry (firmware version, battery voltage, WiFi RSSI, model) is captured automatically from headers sent by the device on each display request. The battery percentage shown next to the voltage is derived using the formula published in TRMNL's [battery FAQ](https://help.trmnl.com/en/articles/10556850-device-battery-faq) (`pct = (voltage - 3) / 0.012`, clamped to 0–100).

### Firmware

Flipper can surface release metadata from the upstream firmware repository — [usetrmnl/trmnl-firmware](https://github.com/usetrmnl/trmnl-firmware) — so you can tell at a glance whether your devices are running the latest version. These commands are **strictly read-only**: no binaries are downloaded, no `/api/display` fields are added, and nothing about your devices changes. Devices update themselves over-the-air from the official cloud's S3 bucket; Flipper only reports what's available.

```bash
# Show the 10 most recent releases (use --all for the full list, --json for machine output)
./bin/flipper firmware list

# Compare every registered device against the latest non-prerelease
./bin/flipper firmware status
```

The `status` command emits one row per device with a `STATUS` column of `current`, `outdated`, `ahead`, or `unknown`. A device shows up as `unknown` if it hasn't polled yet (no telemetry) or if either side fails semver parsing. If GitHub can't be reached (typically a 60 req/hr rate limit when called repeatedly), Flipper prints a warning to stderr and falls back to `—` for the LATEST column rather than failing the whole command. Release lookups are cached in-process for 15 minutes.

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

### Running as a system service

#### macOS (launchd)

Create `~/Library/LaunchAgents/com.olivere.flipper.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.olivere.flipper</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/flipper</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>ProcessType</key>
    <string>Interactive</string>
    <key>StandardOutPath</key>
    <string>/tmp/flipper.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/flipper.log</string>
</dict>
</plist>
```

`ProcessType: Interactive` prevents macOS App Nap from throttling the server
when the display sleeps. Without it, image processing can exceed the device's
read timeout.

Adjust the path to the `flipper` binary as needed. Then:

```bash
# Install and start
launchctl load ~/Library/LaunchAgents/com.olivere.flipper.plist

# Stop and uninstall
launchctl unload ~/Library/LaunchAgents/com.olivere.flipper.plist

# Check status
launchctl list | grep flipper
```

#### Linux (systemd)

Create `~/.config/systemd/user/flipper.service`:

```ini
[Unit]
Description=Flipper TRMNL display server
After=network.target

[Service]
ExecStart=/usr/local/bin/flipper serve
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

Adjust the path to the `flipper` binary as needed. Then:

```bash
# Install and start
systemctl --user daemon-reload
systemctl --user enable --now flipper

# Stop
systemctl --user stop flipper

# Uninstall
systemctl --user disable --now flipper

# Check status / logs
systemctl --user status flipper
journalctl --user -u flipper -f
```

> **Using Claude Code?** Run `/dev daemon install` to generate and install the service file automatically, or `/dev daemon uninstall` to remove it.

## License

Copyright (c) 2026 Oliver Eilhard. All rights reserved. See [LICENSE](LICENSE).
