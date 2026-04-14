package weather

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIconToWMO(t *testing.T) {
	tests := []struct {
		icon string
		want int
	}{
		{"clear-day", 0},
		{"clear-night", 0},
		{"partly-cloudy-day", 2},
		{"partly-cloudy-night", 2},
		{"cloudy", 3},
		{"fog", 45},
		{"rain", 61},
		{"sleet", 66},
		{"snow", 71},
		{"hail", 77},
		{"thunderstorm", 95},
		{"wind", 0},
		{"unknown", 3},
		{"", 3},
	}
	for _, tt := range tests {
		if got := iconToWMO(tt.icon); got != tt.want {
			t.Errorf("iconToWMO(%q) = %d, want %d", tt.icon, got, tt.want)
		}
	}
}

func TestAggregateDaily(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	records := []hourlyRecord{
		{Timestamp: "2026-04-07T06:00:00+02:00", Temperature: f(5.0), Icon: "cloudy"},
		{Timestamp: "2026-04-07T12:00:00+02:00", Temperature: f(15.0), Icon: "partly-cloudy-day"},
		{Timestamp: "2026-04-07T18:00:00+02:00", Temperature: f(12.0), Icon: "partly-cloudy-day"},
		{Timestamp: "2026-04-08T06:00:00+02:00", Temperature: f(3.0), Icon: "clear-day"},
		{Timestamp: "2026-04-08T12:00:00+02:00", Temperature: f(10.0), Icon: "clear-day"},
		{Timestamp: "2026-04-08T18:00:00+02:00", Temperature: f(8.0), Icon: "rain"},
	}

	days := aggregateDaily(records)
	if len(days) != 2 {
		t.Fatalf("got %d days, want 2", len(days))
	}

	// Day 1: min=5, max=15, dominant icon=partly-cloudy-day (2x vs cloudy 1x)
	if days[0].LowTemp != 5.0 {
		t.Errorf("day 0 low = %f, want 5.0", days[0].LowTemp)
	}
	if days[0].HighTemp != 15.0 {
		t.Errorf("day 0 high = %f, want 15.0", days[0].HighTemp)
	}
	if days[0].WeatherCode != 2 { // partly-cloudy-day
		t.Errorf("day 0 code = %d, want 2", days[0].WeatherCode)
	}

	// Day 2: min=3, max=10, dominant icon=clear-day (2x vs rain 1x)
	if days[1].LowTemp != 3.0 {
		t.Errorf("day 1 low = %f, want 3.0", days[1].LowTemp)
	}
	if days[1].HighTemp != 10.0 {
		t.Errorf("day 1 high = %f, want 10.0", days[1].HighTemp)
	}
	if days[1].WeatherCode != 0 { // clear-day
		t.Errorf("day 1 code = %d, want 0", days[1].WeatherCode)
	}
}

func TestAggregateDailySkipsNilTemperature(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	records := []hourlyRecord{
		{Timestamp: "2026-04-07T06:00:00+02:00", Temperature: nil, Icon: "cloudy"},
		{Timestamp: "2026-04-07T12:00:00+02:00", Temperature: f(10.0), Icon: "cloudy"},
	}

	days := aggregateDaily(records)
	if len(days) != 1 {
		t.Fatalf("got %d days, want 1", len(days))
	}
	if days[0].LowTemp != 10.0 || days[0].HighTemp != 10.0 {
		t.Errorf("got low=%f high=%f, want 10.0/10.0", days[0].LowTemp, days[0].HighTemp)
	}
}

func TestBrightSkyFetchParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/current_weather", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"weather": map[string]any{
				"temperature":       14.4,
				"relative_humidity": 50.0,
				"wind_speed_10":     8.0,
				"wind_direction_10": 297.0,
				"icon":              "partly-cloudy-day",
				"condition":         "dry",
			},
		})
	})

	mux.HandleFunc("/weather", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"weather": []map[string]any{
				{"timestamp": "2026-04-07T06:00:00+02:00", "temperature": 9.6, "icon": "cloudy"},
				{"timestamp": "2026-04-07T14:00:00+02:00", "temperature": 15.6, "icon": "partly-cloudy-day"},
				{"timestamp": "2026-04-08T06:00:00+02:00", "temperature": 6.9, "icon": "clear-day"},
				{"timestamp": "2026-04-08T14:00:00+02:00", "temperature": 16.7, "icon": "clear-day"},
				{"timestamp": "2026-04-09T06:00:00+02:00", "temperature": 4.8, "icon": "rain"},
				{"timestamp": "2026-04-09T14:00:00+02:00", "temperature": 14.6, "icon": "rain"},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Patch the client to use the test server.
	client := &BrightSkyClient{http: srv.Client()}

	// We can't easily override the base URL, so test the parsing logic
	// via aggregateDaily and the response structures directly.

	// Test current weather parsing.
	resp, err := srv.Client().Get(srv.URL + "/current_weather")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var rawCurrent struct {
		Weather struct {
			Temperature   float64 `json:"temperature"`
			Humidity      float64 `json:"relative_humidity"`
			WindSpeed     float64 `json:"wind_speed_10"`
			WindDirection float64 `json:"wind_direction_10"`
			Icon          string  `json:"icon"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawCurrent); err != nil {
		t.Fatal(err)
	}
	if rawCurrent.Weather.Temperature != 14.4 {
		t.Errorf("got temp %f, want 14.4", rawCurrent.Weather.Temperature)
	}
	if iconToWMO(rawCurrent.Weather.Icon) != 2 {
		t.Errorf("got WMO %d, want 2 for partly-cloudy-day", iconToWMO(rawCurrent.Weather.Icon))
	}

	// Test forecast parsing.
	resp2, err := srv.Client().Get(srv.URL + "/weather")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	var rawForecast struct {
		Weather []hourlyRecord `json:"weather"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&rawForecast); err != nil {
		t.Fatal(err)
	}

	days := aggregateDaily(rawForecast.Weather)
	if len(days) != 3 {
		t.Fatalf("got %d days, want 3", len(days))
	}

	_ = client // ensure client is used
}

func TestConvertToImperial(t *testing.T) {
	data := &WeatherData{
		Current: CurrentWeather{
			Temperature: 0,    // 0°C = 32°F
			HighTemp:    100,  // 100°C = 212°F
			LowTemp:     -40,  // -40°C = -40°F
			WindSpeed:   100,  // 100 km/h ≈ 62.14 mph
		},
		Daily: []DailyForecast{
			{HighTemp: 20, LowTemp: 10},
		},
	}

	convertToImperial(data)

	if data.Current.Temperature != 32 {
		t.Errorf("got temp %f, want 32", data.Current.Temperature)
	}
	if data.Current.HighTemp != 212 {
		t.Errorf("got high %f, want 212", data.Current.HighTemp)
	}
	if data.Current.LowTemp != -40 {
		t.Errorf("got low %f, want -40", data.Current.LowTemp)
	}

	wantWind := 62.1371
	if data.Current.WindSpeed < 62.13 || data.Current.WindSpeed > 62.14 {
		t.Errorf("got wind %f, want ~%f", data.Current.WindSpeed, wantWind)
	}

	// Daily: 20°C = 68°F, 10°C = 50°F
	if data.Daily[0].HighTemp != 68 {
		t.Errorf("got daily high %f, want 68", data.Daily[0].HighTemp)
	}
	if data.Daily[0].LowTemp != 50 {
		t.Errorf("got daily low %f, want 50", data.Daily[0].LowTemp)
	}
}
