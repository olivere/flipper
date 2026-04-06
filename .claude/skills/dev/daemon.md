# /dev daemon — Service Management

Manage Flipper as a system service (launchd on macOS, systemd on Linux).

Route on `$ARGUMENTS` (after stripping the `daemon` prefix):

- `install` → Install and start the service
- `uninstall` → Stop and remove the service
- `start` → Start the service
- `stop` → Stop the service
- `status` → Show service status and recent logs
- `logs` → Tail service logs
- No argument → Print usage

## Platform detection

Detect the platform:
- macOS (`uname -s` = Darwin) → use launchd
- Linux (`uname -s` = Linux) → use systemd (user mode)
- Other → error with message

## Paths

| What | macOS | Linux |
|------|-------|-------|
| Service file | `~/Library/LaunchAgents/com.olivere.flipper.plist` | `~/.config/systemd/user/flipper.service` |
| Log output | `/tmp/flipper.log` | journald (use `journalctl`) |

## Finding the binary

Resolve the flipper binary path in this order:
1. `bin/flipper` in the project directory (if it exists)
2. `$(which flipper)` (if installed globally)
3. Ask the user

Convert to absolute path before writing the service file.

## Install

1. Check if the service is already installed. If so, ask whether to reinstall.
2. Find the flipper binary path (see above).
3. Find the config path: check if `--config` was passed or if `~/.config/flipper/config.toml` exists. If a non-default config path is used, include it in the service command.
4. Generate the service file (see templates below).
5. Write the service file to the appropriate location.
6. Load/enable the service.
7. Verify it started successfully.

### macOS plist template

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.olivere.flipper</string>
    <key>ProgramArguments</key>
    <array>
        <string>BINARY_PATH</string>
        <string>serve</string>
        <!-- include --config CONFIG_PATH if non-default -->
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

### Linux systemd template

```ini
[Unit]
Description=Flipper TRMNL display server
After=network.target

[Service]
ExecStart=BINARY_PATH serve [--config CONFIG_PATH]
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
```

## Uninstall

1. Stop the service if running.
2. Remove the service file.
3. macOS: `launchctl unload <plist>`
4. Linux: `systemctl --user disable --now flipper && systemctl --user daemon-reload`
5. Confirm removal.

## Start / Stop

- macOS: `launchctl load/unload ~/Library/LaunchAgents/com.olivere.flipper.plist`
- Linux: `systemctl --user start/stop flipper`

## Status

- macOS: `launchctl list | grep flipper` and `tail -20 /tmp/flipper.log`
- Linux: `systemctl --user status flipper`

## Logs

- macOS: `tail -f /tmp/flipper.log`
- Linux: `journalctl --user -u flipper -f`
