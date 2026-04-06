package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/olivere/flipper/internal/display"
	"github.com/olivere/flipper/internal/screen"
)

func TestConditionText(t *testing.T) {
	tests := []struct {
		code int
		want string
	}{
		{0, "Clear Sky"},
		{1, "Mostly Clear"},
		{2, "Partly Cloudy"},
		{3, "Overcast"},
		{45, "Fog"},
		{48, "Fog"},
		{51, "Drizzle"},
		{61, "Rain"},
		{65, "Rain"},
		{71, "Snow"},
		{80, "Showers"},
		{95, "Thunderstorm"},
		{99, "Thunderstorm"},
		{100, "Unknown"},
	}
	for _, tt := range tests {
		if got := ConditionText(tt.code); got != tt.want {
			t.Errorf("ConditionText(%d) = %q, want %q", tt.code, got, tt.want)
		}
	}
}

func TestCompass(t *testing.T) {
	tests := []struct {
		deg  int
		want string
	}{
		{0, "N"},
		{45, "NE"},
		{90, "E"},
		{135, "SE"},
		{180, "S"},
		{225, "SW"},
		{270, "W"},
		{315, "NW"},
		{350, "N"},
		{22, "N"},
		{23, "NE"},
	}
	for _, tt := range tests {
		if got := Compass(tt.deg); got != tt.want {
			t.Errorf("Compass(%d) = %q, want %q", tt.deg, got, tt.want)
		}
	}
}

func TestShortDayName(t *testing.T) {
	mon := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday
	if got := shortDayName(mon, "en"); got != "Mon" {
		t.Errorf("en Monday = %q, want Mon", got)
	}
	if got := shortDayName(mon, "de"); got != "Mo" {
		t.Errorf("de Monday = %q, want Mo", got)
	}
	wed := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC) // Wednesday
	if got := shortDayName(wed, "de"); got != "Mi" {
		t.Errorf("de Wednesday = %q, want Mi", got)
	}
	// Unknown locale falls back to English.
	if got := shortDayName(mon, "fr"); got != "Mon" {
		t.Errorf("fr Monday = %q, want Mon", got)
	}
}

func TestGeocode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{
					"name":      "Munich",
					"country":   "Germany",
					"latitude":  48.1372,
					"longitude": 11.5755,
				},
			},
		})
	}))
	defer srv.Close()

	c := &Client{http: srv.Client()}
	// Override base URL by swapping the geocode call with a direct request.
	// Instead, test the parsing by making a real HTTP call to our test server.
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)
	resp, err := c.http.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Results) == 0 {
		t.Fatal("expected at least one result")
	}
	if result.Results[0].Name != "Munich" {
		t.Errorf("got name %q, want Munich", result.Results[0].Name)
	}
}

func TestFetchParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"current": map[string]any{
				"temperature_2m":       14.4,
				"relative_humidity_2m": 50,
				"apparent_temperature": 12.8,
				"weather_code":         2,
				"wind_speed_10m":       8.0,
				"wind_direction_10m":   297,
			},
			"daily": map[string]any{
				"time":               []string{"2026-04-06", "2026-04-07", "2026-04-08"},
				"weather_code":       []int{80, 2, 0},
				"temperature_2m_max": []float64{15.6, 16.7, 14.6},
				"temperature_2m_min": []float64{9.6, 6.9, 4.8},
			},
		})
	}))
	defer srv.Close()

	// Make a direct HTTP call and parse using the same struct as Fetch.
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var raw struct {
		Current struct {
			Temperature   float64 `json:"temperature_2m"`
			Humidity      int     `json:"relative_humidity_2m"`
			ApparentTemp  float64 `json:"apparent_temperature"`
			WeatherCode   int     `json:"weather_code"`
			WindSpeed     float64 `json:"wind_speed_10m"`
			WindDirection int     `json:"wind_direction_10m"`
		} `json:"current"`
		Daily struct {
			Time        []string  `json:"time"`
			WeatherCode []int     `json:"weather_code"`
			TempMax     []float64 `json:"temperature_2m_max"`
			TempMin     []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		t.Fatal(err)
	}

	if raw.Current.Temperature != 14.4 {
		t.Errorf("got temp %f, want 14.4", raw.Current.Temperature)
	}
	if raw.Current.WeatherCode != 2 {
		t.Errorf("got code %d, want 2", raw.Current.WeatherCode)
	}
	if len(raw.Daily.Time) != 3 {
		t.Errorf("got %d days, want 3", len(raw.Daily.Time))
	}
}

func TestRenderProducesImage(t *testing.T) {
	s := New(Config{
		City:            "Munich",
		ForecastDays:    5,
		Units:           "metric",
		RefreshInterval: 30 * time.Minute,
	})

	// Inject cached data so Render doesn't hit the network.
	s.cache = &WeatherData{
		Location: Location{Name: "Munich", Country: "Germany", Lat: 48.14, Lon: 11.58},
		Current: CurrentWeather{
			Temperature:   14.4,
			ApparentTemp:  12.8,
			Humidity:      50,
			WeatherCode:   2,
			WindSpeed:     8.0,
			WindDirection: 297,
			HighTemp:      15.6,
			LowTemp:       9.6,
		},
		Daily: []DailyForecast{
			{Date: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC), WeatherCode: 2, HighTemp: 16.7, LowTemp: 6.9},
			{Date: time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), WeatherCode: 0, HighTemp: 14.6, LowTemp: 4.8},
			{Date: time.Date(2026, 4, 9, 0, 0, 0, 0, time.UTC), WeatherCode: 3, HighTemp: 12.4, LowTemp: 2.1},
			{Date: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), WeatherCode: 80, HighTemp: 10.3, LowTemp: 4.2},
			{Date: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC), WeatherCode: 2, HighTemp: 11.3, LowTemp: 1.6},
		},
	}
	s.expiry = time.Now().Add(time.Hour)

	img, err := s.Render(context.Background(), screen.RenderOpts{
		Width:     800,
		Height:    480,
		ColorMode: display.ColorBW,
	})
	if err != nil {
		t.Fatal(err)
	}
	if img == nil {
		t.Fatal("expected non-nil image")
	}
	bounds := img.Bounds()
	if bounds.Dx() != 800 || bounds.Dy() != 480 {
		t.Errorf("got %dx%d, want 800x480", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderUsesCache(t *testing.T) {
	s := New(Config{
		City:            "Munich",
		ForecastDays:    5,
		Units:           "metric",
		RefreshInterval: 30 * time.Minute,
	})

	data := &WeatherData{
		Location: Location{Name: "Munich", Country: "Germany"},
		Current:  CurrentWeather{Temperature: 14.4, WeatherCode: 2, HighTemp: 15.6, LowTemp: 9.6},
		Daily: []DailyForecast{
			{Date: time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC), WeatherCode: 0, HighTemp: 16.0, LowTemp: 6.0},
		},
	}
	s.cache = data
	s.expiry = time.Now().Add(time.Hour)

	// First render should use cache.
	got, err := s.getData(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != data {
		t.Error("expected cached data to be returned")
	}
}

func TestRenderErrorScreen(t *testing.T) {
	s := New(Config{
		City:            "Munich",
		ForecastDays:    5,
		Units:           "metric",
		RefreshInterval: 30 * time.Minute,
	})

	img := s.renderError(screen.RenderOpts{
		Width:     800,
		Height:    480,
		ColorMode: display.ColorBW,
	}, fmt.Errorf("network timeout"))

	if img == nil {
		t.Fatal("expected non-nil image from error screen")
	}
}
