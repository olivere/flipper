// Package config loads Flipper configuration from a TOML file with
// environment variable overrides.
//
// Resolution order:
//  1. Built-in defaults (addr :3443, 800×480 BMP, etc.)
//  2. TOML file at the path given via --config, or
//     ~/.config/flipper/config.toml (XDG_CONFIG_HOME)
//  3. Environment variables (FLIPPER_ADDR, FLIPPER_SECRET_KEY, …)
//
// Each layer overrides only the fields it sets; unset fields keep the
// value from the previous layer.
package config
