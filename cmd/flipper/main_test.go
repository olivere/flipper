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
	if !strings.Contains(got, "v1.8.2") {
		t.Errorf("output missing v1.8.2:\n%s", got)
	}
	if !strings.Contains(got, "VERSION") || !strings.Contains(got, "PUBLISHED") {
		t.Errorf("output missing header row:\n%s", got)
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
