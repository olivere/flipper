# Flipper

A self-hosted display server for [TRMNL](https://usetrmnl.com) e-ink devices. Point your device at Flipper and it serves images from a local directory — resized, dithered, and encoded for the e-ink display.

## How it works

1. Drop images (PNG, JPG, BMP) into a directory
2. Flipper processes them for the e-ink display (grayscale, dither, resize)
3. Your TRMNL device fetches the next image on each refresh cycle
4. Images rotate in order, no cloud service needed

Flipper supports TRMNL OG (800×480, B&W) and TRMNL X (1872×1404, 16-level grayscale) devices. Device capabilities are detected automatically from request headers. When multiple devices poll the same server, each advances through its playlist independently — and each device can be assigned its own playlist (see [Per-device playlists](#per-device-playlists)).

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
| Firmware OTA | `firmware.enabled` | `FLIPPER_FIRMWARE_ENABLED` | `true` |

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

### Per-device playlists

Multiple devices can each have their own playlist. Define named playlists with `[[playlists.<name>]]` sections (same entry format as `[[playlist]]`), then assign a device by MAC address:

```toml
# Default playlist for any device without an assignment
[[playlist]]
screen = "news"
duration = "1m"

# Named playlist
[[playlists.office]]
screen = "weather"
duration = "2m"
params.city = "Munich"

[[playlists.office]]
screen = "hackernews"
duration = "60s"

# Assign a device (MAC is case-insensitive)
[devices."AA:BB:CC:DD:EE:FF"]
playlist = "office"
```

Rules:

- A device with a `playlist` assignment uses that named playlist; all other devices use the top-level `[[playlist]]`.
- If there is no top-level `[[playlist]]` and exactly one named playlist exists, it serves all devices — the `playlist` assignment is optional in a single-playlist setup.
- Misconfigurations are startup errors rather than silent surprises: an assignment referencing an unknown playlist name (even when no playlists are defined at all), two `[devices]` sections that resolve to the same MAC after normalization, and a playlist with no usable entries all refuse to start.
- With multiple named playlists and no top-level `[[playlist]]`, unassigned devices fall back to `screens.rotate` behavior — a startup warning points this out. Defined-but-unassigned playlists are also flagged with a warning.
- Every device advances through its playlist independently; two devices on the same playlist each see the full sequence.

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

Flipper can surface release metadata from the upstream firmware repository — [usetrmnl/trmnl-firmware](https://github.com/usetrmnl/trmnl-firmware) — and, if you bring your own .bin file, dispatch it to a specific device as a one-shot OTA. The metadata commands are strictly read-only; the apply commands actively flash devices and are described below.

```bash
# Show the 10 most recent releases (use --all for the full list, --json for machine output)
./bin/flipper firmware list

# Compare every registered device against the latest non-prerelease
./bin/flipper firmware status
```

The `status` command emits one row per device with a `STATUS` column of `current`, `outdated`, `ahead`, or `unknown`. A device shows up as `unknown` if it hasn't polled yet (no telemetry) or if either side fails semver parsing. If GitHub can't be reached (typically a 60 req/hr rate limit when called repeatedly), Flipper prints a warning to stderr and falls back to `—` for the LATEST column rather than failing the whole command. Release lookups are cached in-process for 15 minutes. When any device has a pending update, an extra `ARMED` column appears showing the queued version.

#### Applying firmware updates

> **Warning:** Flashing the wrong firmware can brick a device. Flipper enforces a one-shot, model-matched dispatch model to keep failures contained — but the bytes you import are yours, and TRMNL's official OTA pipeline isn't publicly reachable. Verify the binary you import out-of-band.

##### Sourcing the binary

TRMNL upstream releases on GitHub carry no `.bin` attachments. Their official cloud serves OTA binaries from a private S3 bucket. There are two practical sources, and they require *different* binary formats — getting this wrong is the most common cause of "the device downloaded the firmware and then nothing happened":

| Use case | Binary format | Where to get it |
|---|---|---|
| **USB recovery** (`esptool write_flash 0x0`) | **Combined image** — bootloader + partition table + app, written from flash offset 0x0 | Public CDN: `https://trmnl.com/firmware/{model}/{version}.bin` (same one used by `trmnl.com/flash`) |
| **OTA via Flipper** (this feature) | **App-only image** — starts at the app partition offset; ESP-IDF's `Update.begin()` rejects anything else, *silently* | Extract from the combined image, **or** build from source |

The combined image you can download from `trmnl.com/firmware/...` is correct for [`docs/recovery.md`](docs/recovery.md) but **wrong for OTA**. If you arm it, the device will dutifully download the bytes, fail verification with no log entry, and keep running the old firmware. (You won't see this in Flipper either — TRMNL devices don't ack flash outcomes.)

To extract the app-only image from a combined image, strip the first 64 KiB (bootloader + partition-table area; the app sits at flash offset `0x10000`):

```bash
# Download the combined image
curl -fLo trmnl-og-FW1.8.2.bin https://trmnl.com/firmware/trmnl/FW1.8.2.bin

# Strip the first 64 KiB to get the OTA-ready app
dd if=trmnl-og-FW1.8.2.bin of=trmnl-og-FW1.8.2-app.bin bs=4096 skip=16

# Sanity check: first byte must be 0xE9 (ESP image magic), and the entry-point
# bytes at offset 0x04-0x07 (little-endian) should land in the app IRAM range
# — roughly 0x4038xxxx for ESP32-C3 apps (vs 0x403cxxxx for the bootloader).
xxd trmnl-og-FW1.8.2-app.bin | head -1
# expect: 00000000: e9.. .... f61d 3840 ...   ← entry point 0x40381df6 → app
```

Alternatively, build from source: clone [`usetrmnl/trmnl-firmware`](https://github.com/usetrmnl/trmnl-firmware), check out the version tag, install PlatformIO, run `pio run -e trmnl` (for OG; use `trmnl_x` for TRMNL X). The output at `.pio/build/<env>/firmware.bin` is the app-only image, guaranteed correct.

##### Applying the update

The device model passed via `--model` must match what the target device reports in its `Model` header (visible in `flipper firmware status`) — e.g. `og` for TRMNL OG, `trmnl_x` for TRMNL X. A mismatch at dispatch time drops the arm with a logged error.

```bash
# 1. Import the OTA-ready binary you just extracted (or built).
./bin/flipper firmware import ./trmnl-og-FW1.8.2-app.bin --version 1.8.2 --model og

# 2. List the local store (also accepts --json).
./bin/flipper firmware binaries

# 3. Arm a one-shot OTA for one device. Prompts for confirmation; use --yes to skip.
./bin/flipper firmware update AA:BB:CC:DD:EE:FF 1.8.2

# 4. Inspect what's armed across the fleet.
./bin/flipper firmware armed

# 5. Disarm if you change your mind before the device polls.
./bin/flipper firmware cancel AA:BB:CC:DD:EE:FF

# 6. Remove a binary from the local store.
./bin/flipper firmware remove 1.8.2 --model og
```

The flash actually runs the next time the device polls `/api/display`: Flipper serves a response with `update_firmware=true` and a `firmware_url` pointing at `/firmware/{filename}`, the device downloads and flashes, and the arm is consumed in the same step. Importantly:

- **One-shot, never auto-retried.** A failed flash does not re-arm itself. The operator must explicitly arm again. This prevents the brick-loop where a device repeatedly tries to flash a bad binary on every wake.
- **Model match enforced at dispatch.** Even if the right binary is imported, the arm's recorded model must match the `Model` header the device sends; mismatched arms are dropped with a logged error.
- **`/firmware/{filename}` is intentionally unauthenticated.** TRMNL devices don't send `ID` / `Access-Token` headers when fetching the firmware URL, so gating this endpoint by auth would block every real OTA. The security boundary is `/api/display` — it decides which binary gets dispatched to which device. The bytes themselves are not secret (they're the same builds published at `trmnl.com/firmware/`); only filenames already in the local store are served, and every request is logged with its source address.
- **No success/failure callback.** TRMNL devices do not report flash outcomes. The signal a flash worked is that the next `/api/display` poll's `FW-Version` header reflects the new version — visible in `flipper firmware status`.

Binaries and their manifest live under `$XDG_DATA_HOME/flipper/firmware/`; the pending-arm state lives in `$XDG_DATA_HOME/flipper/firmware-pending.json` (with a file lock so concurrent CLI and server processes don't lose arms via a lost-update race).

If a flash leaves a device in a broken state, the recovery path is a USB reflash with `esptool.py`. See [`docs/recovery.md`](docs/recovery.md) for the full procedure on macOS, Linux, and Windows — read it once *before* you arm your first OTA, so the rollback binary is on disk when you need it.

To disable the apply side entirely (the read-only `list`/`status` commands keep working), set `[firmware] enabled = false` in config. With that, the `/firmware/{filename}` route is not registered, `/api/display` skips the pending-arm check, and the apply commands refuse to run.

### Reverse proxy (advanced)

If you prefer to terminate TLS externally (e.g. with Caddy or nginx), disable Flipper's built-in TLS and set up your proxy to forward to the plain HTTP port. Flipper detects the `X-Forwarded-Proto` header and generates correct `https://` image URLs when proxied.

## API

| Endpoint | Method | Headers | Description |
|----------|--------|---------|-------------|
| `/api/setup` | GET | `ID: <MAC>` | Register a device (requires `setup_mode`) |
| `/api/display` | GET | `ID: <MAC>`, `Access-Token: <key>` | Get next display image URL |
| `/api/log` | POST | `ID: <MAC>` (optional) | Accept device log messages |
| `/images/{filename}` | GET | — | Serve processed images |
| `/firmware/{filename}` | GET | — | Serve an imported firmware binary (only when `[firmware] enabled = true`; unauthenticated — see "Applying firmware updates" for why) |

### Token adoption

When a TRMNL device migrates from another server (e.g. the TRMNL cloud), it may send an API key that doesn't match the one Flipper derived during setup. While `setup_mode` is enabled, Flipper automatically adopts the device's token on first contact, so devices work without manual key reconfiguration. Disable `setup_mode` after onboarding to lock down token adoption.

### `/api/display` response shape

`refresh_rate` is emitted as a **JSON number** to match the firmware's parser test fixtures (`refresh_rate: 123456` in `test/test_parse_api_display/`). Most TRMNL firmware versions appear to tolerate either form, but matching the test-fixture shape is the conservative choice. If you fork Flipper or build your own server, mirror this:

```json
{"status":0,"image_url":"...","refresh_rate":900,"update_firmware":false,"firmware_url":null,"reset_firmware":false}
```

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

MIT — see [LICENSE](LICENSE).

The embedded [Inter](https://github.com/rsms/inter) and [JetBrains Mono](https://github.com/JetBrains/JetBrainsMono) fonts are licensed under the SIL Open Font License 1.1 — see [internal/layout/fonts](internal/layout/fonts).
