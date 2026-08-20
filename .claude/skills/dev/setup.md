# /dev setup — Guided Setup Workflow

Walk the user through getting Flipper running from scratch. Be interactive: after each step, show the result and confirm before continuing. Use `AskUserQuestion` for choices.

Create a task list to track progress through the steps.

## Step 1: Check prerequisites

- Check Go is installed and version >= 1.27 (`go version`)
- Check `make` is available (`make --version`)
- If Go is missing: link to https://go.dev/dl/, offer `brew install go` on macOS
- If make is missing: suggest Xcode command line tools on macOS (`xcode-select --install`)

## Step 2: Build

- Run `make build`
- Verify `bin/flipper` exists
- If build fails: show the error, suggest `go mod tidy`, offer to retry

## Step 3: Configure

Ask the user where to put the config file:
1. **Default path** (recommended): `~/.config/flipper/config.toml` — standard location, used automatically
2. **Local**: `./config.toml` — simpler, requires `--config ./config.toml` flag

Then ask about the secret key:
1. **Generate random** (recommended): generate 32 hex chars via `openssl rand -hex 16`
2. **Use default**: keep `change-me` (fine for local dev, warn it's insecure)

Ask about listen address (default `:3443` is usually fine).

Generate the config file with chosen values. Create the parent directory if needed.

## Step 4: Image directory

Ask where to store source images (default: `~/Pictures/trmnl`).

- Create the directory if it doesn't exist
- Check if it contains at least one supported image (`.png`, `.jpg`, `.jpeg`, `.bmp`)
- If empty, offer choices:
  1. **Generate a test image** — use Go to create a simple PNG:
     ```
     go run -e <<'GOEOF'
     package main
     import (
         "image"
         "image/color"
         "image/png"
         "os"
     )
     func main() {
         img := image.NewRGBA(image.Rect(0, 0, 800, 480))
         for y := 0; y < 480; y++ {
             for x := 0; x < 800; x++ {
                 g := uint8((x + y) % 256)
                 img.Set(x, y, color.RGBA{g, g, g, 255})
             }
         }
         f, _ := os.Create(os.Args[1])
         defer f.Close()
         png.Encode(f, img)
     }
     GOEOF
     ```
     Alternatively, use ImageMagick/`convert` if available, or a simpler approach: write a small Go program to a temp file, run it with the output path as argument.
  2. **Add manually** — tell the user to drop images into the directory and wait for confirmation

## Step 5: Start server

Determine the config flag needed:
- If config is at default path → no flag needed
- If config is at `./config.toml` → `--config ./config.toml`

Run the server in background: `./bin/flipper serve [--config <path>] &`

Wait a couple seconds, then check:
- Is the process still running? (`lsof -i :<port>` or check the PID)
- Can we connect? Try `curl -sk -o /dev/null -w '%{http_code}' -H 'ID: 00:00:00:00:00:00' https://localhost:<port>/api/setup`

If the server fails to start, show the output and help debug.

## Step 6: Provision a device

Ask the user which path:
1. **Real TRMNL device** — Guide them through the full connection flow:
   - Find the local IP: `ipconfig getifaddr en0` (macOS) or `hostname -I` (Linux)
   - IMPORTANT: TRMNL firmware requires **HTTPS**. Plain HTTP does not work. The firmware calls `setInsecure()` so self-signed certificates are accepted.
   - Hold the device button for 5-7 seconds to enter setup mode
   - Connect to the "TRMNL" WiFi hotspot from phone or computer
   - In the config portal: **Advanced > Custom Server > Yes**
   - Enter `https://<local-ip>:<port>` (no trailing slash)
   - Go back, select WiFi network, enter password, click Connect
   - Wait for the device to connect, then press the button to force a refresh
   - Verify by running `./bin/flipper devices` to see the device listed with its telemetry
   - If "API connection cannot be established": check macOS firewall (System Settings > Network > Firewall), verify the server is running and reachable from the network (`curl -sk https://<local-ip>:<port>/api/setup -H 'ID: test'`)
2. **Test without device** — Simulate:
   ```bash
   curl -sk -H "ID: AA:BB:CC:DD:EE:FF" https://localhost:<port>/api/setup
   ```
   Show the returned JSON. Extract and display the `api_key`.

## Step 7: First display fetch

Using the MAC and API key from the previous step:

```bash
curl -sk -H "ID: AA:BB:CC:DD:EE:FF" -H "Access-Token: <api_key>" https://localhost:<port>/api/display
```

Parse the response JSON. Extract `image_url`. Download it:

```bash
curl -sk -o /tmp/flipper-test.bmp "<image_url>"
```

Verify the downloaded file:
- Check file size > 0
- Check magic bytes: BMP starts with `BM` (hex `42 4D`), PNG starts with `\x89PNG`
- Report the file dimensions if possible (`file /tmp/flipper-test.bmp`)

If the user is on macOS, offer to open the image: `open /tmp/flipper-test.bmp`

## Step 8: Success summary

Print a summary:

```
Setup complete!

  Binary:     bin/flipper
  Config:     <config_path>
  Images:     <image_dir> (<N> images)
  Server:     https://localhost:<port>
  Device:     <MAC> (api_key: <key_prefix>...)

To start the server next time:
  ./bin/flipper serve [--config <path>]

To manage devices:
  ./bin/flipper devices                          # list devices + telemetry
  ./bin/flipper devices rename <mac> <name>      # set friendly name
  ./bin/flipper devices remove <mac>             # remove a device

To add more images, drop files into <image_dir>.
The server picks them up automatically on the next display request.

Run /dev doctor at any time to check your environment.
```
