// Package device manages the TRMNL device registry.
//
// Devices authenticate via MAC address and a derived API key. On first
// contact (setup), a device is registered and its key is derived from
// the server's secret key using SHA-1(secret + MAC). Subsequent requests
// are authenticated by matching the MAC/key pair.
//
// The registry persists to a JSON file at
// ~/.local/share/flipper/devices.json (XDG_DATA_HOME) and is safe for
// concurrent use.
package device
