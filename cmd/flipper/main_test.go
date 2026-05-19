package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// seedSingleDevice writes a devices.json under tmp/flipper with one
// pre-registered TRMNL_X device on firmware 1.7.4. Tests that need a
// known device reuse this so the firmware update / cancel / status
// commands have something to act on.
func seedSingleDevice(t *testing.T, tmp string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(tmp, "flipper"), 0o755); err != nil {
		t.Fatal(err)
	}
	devicesJSON := `[{"mac":"AA:BB:CC:DD:EE:FF","api_key":"k","name":"living","first_seen":"2026-05-01T00:00:00Z","last_seen":"2026-05-01T00:00:00Z","telemetry":{"firmware_version":"1.7.4","model":"TRMNL_X"}}]`
	if err := os.WriteFile(filepath.Join(tmp, "flipper", "devices.json"), []byte(devicesJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), errBuf.String(), err
}

func TestFirmwareImportAndBinaries(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	bin := filepath.Join(t.TempDir(), "fw.bin")
	if err := os.WriteFile(bin, []byte("firmware-bytes-PAYLOAD"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runCmd(t, "firmware", "import", bin, "--version", "1.8.2", "--model", "TRMNL_X")
	if err != nil {
		t.Fatalf("import: %v\n%s", err, out)
	}
	if !strings.Contains(out, "FW-1.8.2.TRMNL_X.bin") {
		t.Errorf("import output should mention generated filename, got:\n%s", out)
	}

	out, _, err = runCmd(t, "firmware", "binaries", "--json")
	if err != nil {
		t.Fatalf("binaries: %v\n%s", err, out)
	}
	var bins []firmware.Binary
	if err := json.Unmarshal([]byte(out), &bins); err != nil {
		t.Fatalf("decode binaries json: %v\n%s", err, out)
	}
	if len(bins) != 1 || bins[0].Version != "1.8.2" || bins[0].Model != "TRMNL_X" {
		t.Errorf("unexpected binaries: %+v", bins)
	}
}

func TestFirmwareUpdateRefusesUnknownDevice(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	_, _, err := runCmd(t, "firmware", "update", "AA:BB:CC:DD:EE:FF", "1.8.2", "--yes")
	if err == nil {
		t.Fatal("expected error for unknown device")
	}
	if !strings.Contains(err.Error(), "not registered") {
		t.Errorf("error should mention 'not registered', got: %v", err)
	}
}

func TestFirmwareUpdateRefusesMissingBinary(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	seedSingleDevice(t, tmp)

	_, _, err := runCmd(t, "firmware", "update", "AA:BB:CC:DD:EE:FF", "1.8.2", "--yes")
	if err == nil {
		t.Fatal("expected error when binary not imported")
	}
	if !strings.Contains(err.Error(), "no binary") {
		t.Errorf("error should mention 'no binary', got: %v", err)
	}
}

func TestFirmwareUpdateRefusesModelMismatch(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	seedSingleDevice(t, tmp) // device reports model "TRMNL_X"

	// Import a binary for a different model.
	bin := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(bin, []byte("bytes"), 0o644)
	if _, _, err := runCmd(t, "firmware", "import", bin, "--version", "1.8.2", "--model", "og"); err != nil {
		t.Fatalf("import: %v", err)
	}

	_, _, err := runCmd(t, "firmware", "update", "AA:BB:CC:DD:EE:FF", "1.8.2", "--yes")
	if err == nil {
		t.Fatal("expected mismatch error — binary is for 'og' but device is 'TRMNL_X'")
	}
}

func TestFirmwareUpdateRefusesSameVersionWithoutForce(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	seedSingleDevice(t, tmp) // device on 1.7.4

	bin := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(bin, []byte("bytes"), 0o644)
	if _, _, err := runCmd(t, "firmware", "import", bin, "--version", "1.7.4", "--model", "TRMNL_X"); err != nil {
		t.Fatalf("import: %v", err)
	}

	_, _, err := runCmd(t, "firmware", "update", "AA:BB:CC:DD:EE:FF", "1.7.4", "--yes")
	if err == nil {
		t.Fatal("expected refusal when device already on target version (no --force)")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should suggest --force, got: %v", err)
	}
}

func TestFirmwareUpdateArmsAndShowsInArmedAndStatus(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	seedSingleDevice(t, tmp)

	bin := filepath.Join(t.TempDir(), "fw.bin")
	_ = os.WriteFile(bin, []byte("bytes"), 0o644)
	if _, _, err := runCmd(t, "firmware", "import", bin, "--version", "1.8.2", "--model", "TRMNL_X"); err != nil {
		t.Fatalf("import: %v", err)
	}

	out, _, err := runCmd(t, "firmware", "update", "AA:BB:CC:DD:EE:FF", "1.8.2", "--yes")
	if err != nil {
		t.Fatalf("arm: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Armed FW-1.8.2.TRMNL_X.bin") {
		t.Errorf("expected armed message, got:\n%s", out)
	}

	armedOut, _, err := runCmd(t, "firmware", "armed", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var armed []map[string]any
	_ = json.Unmarshal([]byte(armedOut), &armed)
	if len(armed) != 1 || armed[0]["mac"] != "AA:BB:CC:DD:EE:FF" || armed[0]["version"] != "1.8.2" {
		t.Errorf("unexpected armed output: %s", armedOut)
	}

	// Set up GitHub stub for `firmware status`.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	origURL := firmware.DefaultURL
	firmware.DefaultURL = srv.URL
	t.Cleanup(func() { firmware.DefaultURL = origURL })

	statusOut, _, err := runCmd(t, "firmware", "status", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(statusOut), &rows); err != nil {
		t.Fatalf("decode status json: %v\n%s", err, statusOut)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	if _, present := rows[0]["armed"]; !present {
		t.Errorf("status JSON must always include 'armed' field, got: %s", statusOut)
	}
	if rows[0]["armed"] != "1.8.2" {
		t.Errorf("armed = %v, want 1.8.2", rows[0]["armed"])
	}

	// Table form must show ARMED column because at least one device is armed.
	statusTable, _, err := runCmd(t, "firmware", "status")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statusTable, "ARMED") {
		t.Errorf("status table must include ARMED column when arms exist:\n%s", statusTable)
	}

	// Cancel and re-check.
	if _, _, err := runCmd(t, "firmware", "cancel", "AA:BB:CC:DD:EE:FF"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	armedOut, _, _ = runCmd(t, "firmware", "armed", "--json")
	if strings.Contains(armedOut, "AA:BB:CC:DD:EE:FF") {
		t.Errorf("cancel did not clear arm, armed output: %s", armedOut)
	}
}

func TestFirmwareDisabled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("FLIPPER_FIRMWARE_ENABLED", "false")

	for _, args := range [][]string{
		{"firmware", "import", "x", "--version", "1.0.0", "--model", "og"},
		{"firmware", "update", "AA:BB:CC:DD:EE:FF", "1.0.0", "--yes"},
		{"firmware", "cancel", "AA:BB:CC:DD:EE:FF"},
		{"firmware", "armed"},
		{"firmware", "binaries"},
		{"firmware", "remove", "1.0.0", "--model", "og"},
	} {
		_, _, err := runCmd(t, args...)
		if err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Errorf("%v should error with 'disabled', got: %v", args, err)
		}
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
