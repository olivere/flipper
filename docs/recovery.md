# USB recovery for TRMNL devices

If a firmware update goes wrong — the device boots into a broken state, the screen stays frozen, or it stops checking in to Flipper — you can re-flash it over USB. This guide is the escape hatch.

It is written so you can follow it under stress. Skim **Before anything goes wrong** today, while the device is still working. The rest is the emergency procedure.

The TRMNL OG uses an **ESP32-C3** with native USB. No special driver is required on macOS, Linux, or modern Windows 10/11. You just need a USB-C cable that carries data (some cheap cables are power-only — if you're unsure, test it with another device first).

---

## Before anything goes wrong

Do this once, today, while everything still works. The whole point is to have the recovery binary on disk *before* you need it.

### 1. Install `esptool`

`esptool.py` is Espressif's official flasher. The cleanest way to install it is `pipx`, which keeps it isolated from your system Python.

**macOS** (with Homebrew):

```bash
brew install esptool
```

Homebrew installs both `esptool` and `esptool.py` entry points, so commands in this doc that say `esptool.py` work as-is. If you'd rather keep esptool in its own isolated venv (instead of letting Homebrew manage the Python deps), use pipx instead:

```bash
brew install pipx
pipx install esptool
```

**Linux** (Debian/Ubuntu):

```bash
sudo apt install pipx
pipx ensurepath
pipx install esptool
```

**Windows** (PowerShell, with Python ≥ 3.9 installed from python.org):

```powershell
python -m pip install --user pipx
python -m pipx ensurepath
# close and reopen PowerShell so PATH is picked up
pipx install esptool
```

Verify:

```bash
esptool.py version
# expect something like: esptool.py v4.8.x (or newer)
```

If `pipx` isn't available, `pip install --user esptool` works too — but installs into your default Python and can collide with other tools.

### 2. Download the rollback binary

Save the firmware your device is **currently** running, so a recovery puts it back to a known-working state. For this device the current version is `FW1.7.4`.

```bash
curl -fLo ~/trmnl-og-FW1.7.4.bin https://trmnl.com/firmware/trmnl/FW1.7.4.bin
```

Verify the SHA-256 — if this doesn't match, do not flash; re-download:

```
8a7e592e34026b1af71729943e2ca7eae6103c4a52553500a855d109e3e6fa45  trmnl-og-FW1.7.4.bin
```

```bash
# macOS
shasum -a 256 ~/trmnl-og-FW1.7.4.bin

# Linux
sha256sum ~/trmnl-og-FW1.7.4.bin
```

```powershell
# Windows PowerShell
Get-FileHash -Algorithm SHA256 $HOME\trmnl-og-FW1.7.4.bin
```

You can also pre-fetch the version you're upgrading **to** (`FW1.8.2`) the same way, so the recovery binary and the target binary both live on disk before you arm the OTA. That makes it possible to either roll back or retry the upgrade without internet access.

### 3. Confirm USB works while the device is still alive

Plug the TRMNL into your computer with a known-good USB-C cable and check that it shows up as a serial device. You don't need to flash anything — this just confirms the cable and port work, so you don't discover a bad cable mid-emergency.

**macOS:**

```bash
ls /dev/cu.usbmodem*
# expect at least one entry, e.g. /dev/cu.usbmodem14101
```

**Linux:**

```bash
ls /dev/ttyACM* /dev/ttyUSB* 2>/dev/null
# expect /dev/ttyACM0 (most common for ESP32-C3 native USB)
# Linux users: you may need to add your user to the dialout group:
#   sudo usermod -aG dialout $USER  (log out / back in to apply)
```

**Windows:**

Open Device Manager → Ports (COM & LPT). Look for a new COM port (e.g. `COM5`) when you plug the cable in.

If nothing appears: try a different cable, then a different USB port (some hubs don't pass data).

---

## In an emergency

You arm an OTA, the device flashes, and now it's stuck. Here's the recovery path.

### 1. Put the device into download mode

The ESP32-C3 enters download mode when **GPIO9 (the BOOT pin) is held low at power-on**. On the TRMNL OG this is exposed via the circular **boot button on the back, just below the power slide switch**. See TRMNL's official guide for the per-model button procedure:

https://help.trmnl.com/en/articles/11936721-put-your-trmnl-in-flashing-mode

**Basic method (TRMNL OG):**

1. Connect the USB-C cable to the device.
2. Slide the power switch to OFF.
3. Hold the boot button (circular, below the power switch).
4. Slide the power switch back to ON while still holding the boot button.
5. Release the button.

The screen stays blank when you're in download mode — that's the signal.

**If the basic method doesn't work** ("stubborn method"): power off, hold the boot button, plug in USB-C while still holding it, slide power to ON while still holding it, then release.

If both fail, TRMNL recommends leaving the device powered off for 10–15 minutes and trying again.

### 2. Find the serial port

The port name will likely be different from the one you saw in step 3 above — download mode enumerates as a different USB endpoint.

**macOS:**

```bash
ls /dev/cu.usbmodem*
# pick the one that appeared after plugging in / triggering download mode
```

**Linux:**

```bash
dmesg | tail -10
# look for a line like "cdc_acm 1-1.4:1.0: ttyACM0: USB ACM device"
```

**Windows:**

Device Manager → Ports — note the COM port that appears.

### 3. Confirm esptool can talk to it

Before writing anything, ask the device to identify itself. If this succeeds, you're in download mode and the cable is fine; if it fails, fix that before continuing.

```bash
# macOS / Linux — substitute your actual port
esptool.py --chip esp32c3 --port /dev/cu.usbmodem14101 chip_id
```

```powershell
# Windows
esptool.py --chip esp32c3 --port COM5 chip_id
```

Expected output mentions `Chip is ESP32-C3` and a MAC address. If it hangs or prints `Failed to connect`, the device isn't in download mode — go back to step 1.

### 4. Re-flash the rollback binary

This writes a known-good firmware to the device, starting at flash offset `0x0` (the TRMNL bins from `trmnl.com/firmware/` are combined images that include bootloader + partition table + app).

```bash
# macOS / Linux
esptool.py --chip esp32c3 --port /dev/cu.usbmodem14101 --baud 460800 \
  write_flash --flash_mode keep --flash_freq keep --flash_size keep \
  0x0 ~/trmnl-og-FW1.7.4.bin
```

```powershell
# Windows
esptool.py --chip esp32c3 --port COM5 --baud 460800 `
  write_flash --flash_mode keep --flash_freq keep --flash_size keep `
  0x0 $HOME\trmnl-og-FW1.7.4.bin
```

`--flash_mode keep --flash_freq keep --flash_size keep` tells esptool to honour the values embedded in the binary header — the safest defaults when you don't know your device's exact flash config.

If `460800` baud fails, drop it to `--baud 115200`. Slower but more tolerant of bad cables.

### 5. Reboot and verify

After the flash completes, esptool will reset the device automatically. Unplug USB, power-cycle the device, and watch Flipper's log:

```bash
./bin/flipper firmware status
# wait for the device to poll once (refresh_rate seconds; default 900)
# FIRMWARE column should show 1.7.4 again
```

> **Expect a WiFi setup screen.** `write_flash 0x0 <combined-image>` overwrites the entire flash region the image covers — including the `nvs` partition where WiFi credentials, the API token, and other device preferences live. So after a successful recovery flash, the device boots into setup mode with no saved network. This is expected, not a sign of a failed recovery: rerun the captive-portal setup from the README ("Connecting a TRMNL device") and point Custom Server at your Flipper again. The MAC stays the same, so your `devices.json` entry resumes and the derived API key matches automatically.

Once you've re-entered WiFi credentials and pointed the device at Flipper, `firmware status` should show your device polling and reporting `1.7.4`. You're back to exactly where you were before the failed OTA, modulo a few minutes of captive-portal setup.

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `Failed to connect to ESP32-C3: Wrong boot mode` | Device isn't in download mode | Re-do step 1; hold the button *while* plugging in, not after |
| `serial.serialutil.SerialException: [Errno 13] Permission denied: '/dev/ttyACM0'` (Linux) | User not in `dialout` group | `sudo usermod -aG dialout $USER`, then log out and back in |
| `A fatal error occurred: Could not open port ...` | Wrong port name, or another process (Arduino IDE, screen, minicom) is holding it | Close other serial tools; re-check the port name |
| No USB device appears at all when plugging in | Power-only cable, dead port, or dead device | Try another cable + port; test the cable with a phone first |
| Flash succeeds but the screen stays blank | Wrong binary for this hardware variant | Confirm you flashed the `trmnl/` (OG) bin, not `trmnl_x/` or `trmnl_4clr/` |
| `Detected flash size XX does not match the size in the image header` | Binary header doesn't match physical flash | Remove the `--flash_size keep` flag and let esptool detect: it'll print the detected size and you can re-run with `--flash_size <size>` |

If you're still stuck after the above, the device's bootloader itself may have been corrupted. The recovery in that case is the same `esptool.py write_flash 0x0 <bin>` — the combined image at offset 0 rewrites the bootloader too. The only way to truly brick an ESP32-C3 is to write garbage directly into the read-only mask ROM, and `esptool.py write_flash` cannot do that.

---

## Why not vendor `esptool.py` in this repo?

It's a multi-module Python package (~thousands of LoC) maintained by Espressif at https://github.com/espressif/esptool, distributed via PyPI, and updated regularly to support new chip revisions. Pinning a snapshot in this repo would freeze us on an old version that may not speak to future hardware, and would saddle Flipper — a single Go binary — with a Python dependency tree. Installing via `pipx` is the supported path and stays out of Flipper's way.
