# /dev doctor — Diagnostic & Repair

Run all checks, collect results, then print a summary table. For each failure: explain what's wrong, offer to fix automatically or guide through a manual fix. Use `AskUserQuestion` when the repair approach is ambiguous.

## Execution approach

1. Run all checks in order (some later checks depend on earlier ones)
2. Track each check's result: PASS, WARN, or FAIL
3. After all checks complete, print the summary table
4. For any WARN or FAIL, offer repair in order

## Checks

### Build & Dependencies

#### 1. Go installed
- Run `go version`
- PASS: Go is installed, version >= 1.27
- FAIL: Go not found or version too old
- Repair: Link to https://go.dev/dl/. On macOS, offer `brew install go`.

#### 2. Dependencies intact
- Run `go mod verify`
- PASS: All modules verified
- FAIL: Verification failed
- Repair: Run `go mod tidy && go mod download`

#### 3. Build succeeds
- Run `make build`
- PASS: `bin/flipper` exists and was just built
- FAIL: Build errors
- Repair: Show the error output. Suggest `go mod tidy`. If that doesn't help, use WebSearch to look up the error.

#### 4. Tests pass
- Run `make test`
- PASS: All tests pass
- WARN: Tests fail (non-blocking — doctor continues)
- Repair: Show failing test output. Offer to investigate.

### Configuration

#### 5. Config file exists
- Check for config in order: `./config.toml`, then `~/.config/flipper/config.toml`
- Also check if `FLIPPER_ADDR` or `FLIPPER_SECRET_KEY` env vars are set (user might be using env-only config)
- PASS: Config file found (report which path)
- WARN: No config file but env vars are set
- FAIL: No config file and no env vars
- Repair: Offer to generate one (same flow as `/dev setup` step 3)

#### 6. Config parses correctly
- Try to parse the config file. The simplest check: run `./bin/flipper serve --config <path>` very briefly and see if it errors on config parsing. Alternatively, use a quick Go snippet or just read the TOML and check for obvious syntax errors.
- Actually, the simplest approach: just read the file and verify it's valid TOML structure. Look for obvious issues like missing quotes, bad section headers.
- PASS: No parse errors
- FAIL: Parse error
- Repair: Show the error and the problematic line. Offer to fix if it's a common issue.

#### 7. Secret key is not default
- Read the config, check if `secret_key` equals `"change-me"`
- Also check `FLIPPER_SECRET_KEY` env var
- PASS: Custom secret key is set
- WARN: Using default `change-me` (works but insecure, fine for local dev)
- Repair: Generate a random key with `openssl rand -hex 16`, offer to update config.

#### 8. Static image directory exists
- Read `screens.static.dir` from config (or `FLIPPER_STATIC_DIR`)
- Expand `~` to home directory
- PASS: Directory exists
- FAIL: Directory doesn't exist or path not configured
- Repair: Create the directory.

#### 9. Static directory has images
- Check for `.png`, `.jpg`, `.jpeg`, `.bmp` files in the directory
- PASS: At least one image found (report count)
- WARN: No images found
- Repair: Offer to generate a test image (gradient PNG via Go).

### Runtime (only if server appears to be running)

Check if something is listening on the configured port first. If not, skip runtime checks and note that the server isn't running.

#### 10. Server is listening
- Check with `lsof -i :<port>` or `curl -sk -o /dev/null -w '%{http_code}' https://localhost:<port>/api/setup -H 'ID: 00:00:00:00:00:00'`
- PASS: Server responds
- SKIP: Server not running (not an error — just note it)
- Repair: Offer to start it.

#### 11. Setup endpoint responds
- `curl -sk -H 'ID: 00:00:00:00:00:00' https://localhost:<port>/api/setup`
- PASS: Returns JSON with `api_key`
- WARN: Returns 403 (setup_mode is disabled — not necessarily wrong)
- FAIL: Error or unexpected response
- Repair: If 403, ask if they want to enable setup_mode. Otherwise, investigate.

#### 12. Display endpoint works
- Need a registered device. Check `devices.json` for any device.
- If a device exists, use its MAC and api_key to call `/api/display`
- PASS: Returns 200 with valid JSON containing `image_url`
- SKIP: No registered devices (suggest running `/dev setup` step 6)
- FAIL: Returns error
- Repair: Depends on the error. 401 = bad API key (re-register). 500 = server error (check logs).

#### 13. Image URL is downloadable
- Fetch the `image_url` from the display response
- PASS: Returns 200 with image data, correct magic bytes (BMP: `42 4D`, PNG: `89 50`)
- FAIL: 404 or invalid data
- Repair: Image cache may have expired. Re-fetch display to regenerate.

### Data

#### 14. Data directory is writable
- Check `~/.local/share/flipper/` exists and is writable
- PASS: Directory exists and is writable
- FAIL: Missing or not writable
- Repair: Create directory or fix permissions.

#### 15. Devices file is valid
- If `~/.local/share/flipper/devices.json` exists, check it's valid JSON
- PASS: Valid JSON array
- SKIP: File doesn't exist (no devices registered yet — that's fine)
- FAIL: Invalid JSON
- Repair: Show the content, offer to back up and reset.

#### 16. Device list works
- Run `./bin/flipper devices --json` and check it returns valid JSON
- PASS: Command succeeds and returns a JSON array
- SKIP: Binary not built or no devices registered
- FAIL: Command errors
- Repair: Rebuild binary (`make build`). If devices.json is corrupt, offer to back up and reset.

## Summary table format

After all checks, print:

```
Flipper Doctor Summary
======================

Build & Dependencies
  [PASS] Go installed (go1.27.x)
  [PASS] Dependencies verified
  [PASS] Build succeeds
  [PASS] Tests pass

Configuration
  [PASS] Config file found (~/.config/flipper/config.toml)
  [PASS] Config parses correctly
  [WARN] Secret key is default "change-me"
  [PASS] Image directory exists (~/Pictures/trmnl)
  [PASS] Image directory has 3 images

Runtime
  [PASS] Server listening on :3443
  [PASS] Setup endpoint responds
  [PASS] Display endpoint works
  [PASS] Image URL downloadable

Data
  [PASS] Data directory writable
  [PASS] Devices file valid (1 device)

Result: 14 passed, 1 warning, 0 failed
```

Then, for each WARN/FAIL, offer repair in sequence.
