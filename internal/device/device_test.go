package device

import "testing"

func TestBatteryPercent(t *testing.T) {
	tests := []struct {
		voltage string
		want    int
	}{
		{"", -1},
		{"not-a-number", -1},
		{"0", -1},
		{"2.50", 0},   // below empty, clamped
		{"3.00", 0},   // empty
		{"3.60", 50},  // mid
		{"4.20", 100}, // full
		{"4.50", 100}, // above full, clamped
		{" 4.08 ", 90},
	}
	for _, tc := range tests {
		got := Telemetry{BatteryVoltage: tc.voltage}.BatteryPercent()
		if got != tc.want {
			t.Errorf("BatteryPercent(%q) = %d, want %d", tc.voltage, got, tc.want)
		}
	}
}
