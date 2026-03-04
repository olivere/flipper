package device

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

type Device struct {
	MAC       string    `json:"mac"`
	APIKey    string    `json:"api_key"`
	Name      string    `json:"name"`
	FirstSeen time.Time `json:"first_seen"`
	LastSeen  time.Time `json:"last_seen"`
}

type Registry struct {
	mu        sync.RWMutex
	devices   map[string]*Device // keyed by MAC
	secretKey string
	path      string
}

func NewRegistry(secretKey string) (*Registry, error) {
	path := filepath.Join(xdg.DataHome, "flipper", "devices.json")
	r := &Registry{
		devices:   make(map[string]*Device),
		secretKey: secretKey,
		path:      path,
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Registry) Register(mac string) (*Device, error) {
	mac = normMAC(mac)
	apiKey := deriveKey(r.secretKey, mac)
	now := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[mac]; ok {
		d.LastSeen = now
		_ = r.save()
		return d, nil
	}

	d := &Device{
		MAC:       mac,
		APIKey:    apiKey,
		FirstSeen: now,
		LastSeen:  now,
	}
	r.devices[mac] = d
	if err := r.save(); err != nil {
		return nil, err
	}
	return d, nil
}

func (r *Registry) Authenticate(mac, token string) bool {
	mac = normMAC(mac)
	r.mu.RLock()
	defer r.mu.RUnlock()

	d, ok := r.devices[mac]
	if !ok {
		return false
	}
	return d.APIKey == token
}

func (r *Registry) Touch(mac string) {
	mac = normMAC(mac)
	r.mu.Lock()
	defer r.mu.Unlock()

	if d, ok := r.devices[mac]; ok {
		d.LastSeen = time.Now().UTC()
		_ = r.save()
	}
}

func (r *Registry) load() error {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("load devices: %w", err)
	}
	var devices []*Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return fmt.Errorf("parse devices: %w", err)
	}
	for _, d := range devices {
		r.devices[d.MAC] = d
	}
	return nil
}

func (r *Registry) save() error {
	devices := make([]*Device, 0, len(r.devices))
	for _, d := range r.devices {
		devices = append(devices, d)
	}
	data, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o644)
}

func deriveKey(secret, mac string) string {
	h := sha1.New()
	h.Write([]byte(secret + mac))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func normMAC(mac string) string {
	return strings.ToUpper(strings.TrimSpace(mac))
}
