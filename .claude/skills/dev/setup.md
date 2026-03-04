# /dev setup — Guided Setup Workflow

Walk the user through getting Flipper running from scratch. Be interactive: after each step, show the result and confirm before continuing. Use `AskUserQuestion` for choices.

Create a task list to track progress through the steps.

## Step 1: Check prerequisites

- Check Go is installed and version >= 1.26 (`go version`)
- Check `make` is available (`make --version`)
- If Go is missing: link to https://go.dev/dl/, offer `brew install go` on macOS
- If make is missing: suggest Xcode command line tools on macOS (`xcode-select --install`)

## Step 2: Build

- Run `make build`
- Verify `bin/flipper` exists
- If build fails: show the error, suggest `go mod tidy`, offer to retry

## Step 3: Configure

Ask the user where to put the config file:
1. **XDG path** (default): `$XDG_CONFIG_HOME/flipper/config.toml` — standard location, used automatically
2. **Local**: `./config.toml` — simpler, requires `--config ./config.toml` flag

Then ask about the secret key:
1. **Generate random** (recommended): generate 32 hex chars via `openssl rand -hex 16`
2. **Use default**: keep `change-me` (fine for local dev, warn it's insecure)

Ask about listen address (default `:3000` is usually fine).

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
- If config is at XDG default → no flag needed
- If config is at `./config.toml` → `--config ./config.toml`

Run the server in background: `./bin/flipper serve [--config <path>] &`

Wait a couple seconds, then check:
- Is the process still running? (`lsof -i :<port>` or check the PID)
- Can we connect? Try `curl -s -o /dev/null -w '%{http_code}' -H 'ID: 00:00:00:00:00:00' http://localhost:<port>/api/setup`

If the server fails to start, show the output and help debug.

## Step 6: Provision a device

Ask the user which path:
1. **Real TRMNL device** — Guide them to:
   - Open the device's WiFi config page
   - Set the server URL to `http://<local-ip>:<port>`
   - The device will call `/api/setup` on its own
   - Verify by checking the server logs or `devices.json`
2. **Test without device** (recommended for initial setup) — Simulate:
   ```bash
   curl -s -H "ID: AA:BB:CC:DD:EE:FF" http://localhost:<port>/api/setup
   ```
   Show the returned JSON. Extract and display the `api_key`.

## Step 7: First display fetch

Using the MAC and API key from the previous step:

```bash
curl -s -H "ID: AA:BB:CC:DD:EE:FF" -H "Access-Token: <api_key>" http://localhost:<port>/api/display
```

Parse the response JSON. Extract `image_url`. Download it:

```bash
curl -s -o /tmp/flipper-test.bmp "<image_url>"
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
  Server:     http://localhost:<port>
  Device:     <MAC> (api_key: <key_prefix>...)

To start the server next time:
  ./bin/flipper serve [--config <path>]

To add more images, drop files into <image_dir>.
The server picks them up automatically on the next display request.

Run /dev doctor at any time to check your environment.
```
