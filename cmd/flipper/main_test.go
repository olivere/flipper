package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/olivere/flipper/internal/firmware"
)

func TestFirmwareListSmoke(t *testing.T) {
	body, err := os.ReadFile("../../internal/firmware/testdata/releases.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	origURL := firmware.DefaultURL
	firmware.DefaultURL = srv.URL
	t.Cleanup(func() { firmware.DefaultURL = origURL })

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"firmware", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "VERSION") || !strings.Contains(got, "PUBLISHED") {
		t.Errorf("output missing header row:\n%s", got)
	}
	// VERSION column must hold the v-stripped form so users can copy-paste
	// straight into `flipper firmware update <mac> <version>` (#15).
	var versionRow string
	for line := range strings.SplitSeq(got, "\n") {
		if strings.HasPrefix(line, "1.8.2") {
			versionRow = line
			break
		}
	}
	if versionRow == "" {
		t.Fatalf("no row starts with bare '1.8.2' (column should not include 'v' prefix):\n%s", got)
	}
}

func TestFirmwareListJSONSmoke(t *testing.T) {
	body, err := os.ReadFile("../../internal/firmware/testdata/releases.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	origURL := firmware.DefaultURL
	firmware.DefaultURL = srv.URL
	t.Cleanup(func() { firmware.DefaultURL = origURL })

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"firmware", "list", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var releases []firmware.Release
	if err := json.Unmarshal(out.Bytes(), &releases); err != nil {
		t.Fatalf("decode json output: %v\noutput: %s", err, out.String())
	}
	if len(releases) == 0 || releases[0].Version != "1.8.2" {
		t.Errorf("unexpected json payload: %+v", releases)
	}
}

func TestFirmwareStatusJSONIncludesEmptyLatestOnFetchFailure(t *testing.T) {
	// Set up an empty registry but a stub that always 5xx's on releases.
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	// Pre-seed one device so we get a row in the output.
	devicesJSON := `[{"mac":"AA:BB:CC:DD:EE:FF","api_key":"k","name":"living","first_seen":"2026-05-01T00:00:00Z","last_seen":"2026-05-01T00:00:00Z","telemetry":{"firmware_version":"1.7.4","model":"og"}}]`
	if err := os.MkdirAll(tmp+"/flipper", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp+"/flipper/devices.json", []byte(devicesJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusForbidden)
	}))
	t.Cleanup(srv.Close)

	origURL := firmware.DefaultURL
	firmware.DefaultURL = srv.URL
	t.Cleanup(func() { firmware.DefaultURL = origURL })

	root := newRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs([]string{"firmware", "status", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("decode json: %v\noutput: %s", err, out.String())
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	latest, present := rows[0]["latest"]
	if !present {
		t.Errorf("expected 'latest' field present on fetch failure, got payload: %s", out.String())
	}
	if latest != "" {
		t.Errorf("latest = %v, want empty string", latest)
	}
	if !strings.Contains(errBuf.String(), "could not fetch firmware releases") {
		t.Errorf("expected stderr warning, got: %s", errBuf.String())
	}
}

func TestFirmwareStatusSmoke(t *testing.T) {
	// Point XDG_DATA_HOME at an empty tmpdir so the device registry has
	// no devices to list.
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)

	origURL := firmware.DefaultURL
	firmware.DefaultURL = srv.URL
	t.Cleanup(func() { firmware.DefaultURL = origURL })

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"firmware", "status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "No devices registered") {
		t.Errorf("expected 'No devices registered' fallback, got:\n%s", out.String())
	}
}
