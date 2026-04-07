package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Fetcher retrieves weather data for a location.
type Fetcher interface {
	Fetch(ctx context.Context, loc Location, forecastDays int, units string) (*WeatherData, error)
}

// Client talks to the Open-Meteo API for geocoding and weather forecasts.
type Client struct {
	http *http.Client
}

// NewClient creates a weather API client with sensible defaults.
func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Location holds geocoded city information.
type Location struct {
	Name    string
	Country string
	Lat     float64
	Lon     float64
}

// WeatherData holds the complete weather response for rendering.
type WeatherData struct {
	Location Location
	Current  CurrentWeather
	Daily    []DailyForecast
}

// CurrentWeather holds current conditions.
type CurrentWeather struct {
	Temperature   float64
	ApparentTemp  float64
	Humidity      int
	WeatherCode   int
	WindSpeed     float64
	WindDirection int
	HighTemp      float64
	LowTemp       float64
}

// DailyForecast holds a single day's forecast.
type DailyForecast struct {
	Date        time.Time
	WeatherCode int
	HighTemp    float64
	LowTemp     float64
}

// Geocode resolves a city name to coordinates using Open-Meteo's geocoding API.
func (c *Client) Geocode(ctx context.Context, city string) (Location, error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?name=" + url.QueryEscape(city) + "&count=1&language=en"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Location{}, fmt.Errorf("geocode request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("geocode request failed", "url", u, "error", err)
		return Location{}, fmt.Errorf("geocode fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("geocode unexpected status", "url", u, "status", resp.StatusCode)
		return Location{}, fmt.Errorf("geocode: status %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name      string  `json:"name"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Location{}, fmt.Errorf("geocode decode: %w", err)
	}
	if len(result.Results) == 0 {
		return Location{}, fmt.Errorf("geocode: no results for %q", city)
	}

	r := result.Results[0]
	return Location{
		Name:    r.Name,
		Country: r.Country,
		Lat:     r.Latitude,
		Lon:     r.Longitude,
	}, nil
}

// Fetch retrieves current weather and daily forecast from Open-Meteo.
func (c *Client) Fetch(ctx context.Context, loc Location, forecastDays int, units string) (*WeatherData, error) {
	params := url.Values{
		"latitude":      {strconv.FormatFloat(loc.Lat, 'f', 4, 64)},
		"longitude":     {strconv.FormatFloat(loc.Lon, 'f', 4, 64)},
		"current":       {"temperature_2m,relative_humidity_2m,apparent_temperature,weather_code,wind_speed_10m,wind_direction_10m"},
		"daily":         {"weather_code,temperature_2m_max,temperature_2m_min"},
		"timezone":      {"auto"},
		"forecast_days": {strconv.Itoa(forecastDays)},
	}
	if units == "imperial" {
		params.Set("temperature_unit", "fahrenheit")
		params.Set("wind_speed_unit", "mph")
	}

	u := "https://api.open-meteo.com/v1/forecast?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("forecast request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("forecast request failed", "url", u, "error", err)
		return nil, fmt.Errorf("forecast fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("forecast unexpected status", "url", u, "status", resp.StatusCode)
		return nil, fmt.Errorf("forecast: status %d", resp.StatusCode)
	}

	var raw struct {
		Current struct {
			Temperature      float64 `json:"temperature_2m"`
			Humidity         int     `json:"relative_humidity_2m"`
			ApparentTemp     float64 `json:"apparent_temperature"`
			WeatherCode      int     `json:"weather_code"`
			WindSpeed        float64 `json:"wind_speed_10m"`
			WindDirection    int     `json:"wind_direction_10m"`
		} `json:"current"`
		Daily struct {
			Time        []string  `json:"time"`
			WeatherCode []int     `json:"weather_code"`
			TempMax     []float64 `json:"temperature_2m_max"`
			TempMin     []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("forecast decode: %w", err)
	}

	data := &WeatherData{
		Location: loc,
		Current: CurrentWeather{
			Temperature:   raw.Current.Temperature,
			ApparentTemp:  raw.Current.ApparentTemp,
			Humidity:      raw.Current.Humidity,
			WeatherCode:   raw.Current.WeatherCode,
			WindSpeed:     raw.Current.WindSpeed,
			WindDirection: raw.Current.WindDirection,
		},
	}

	// Today's high/low from the first daily entry.
	if len(raw.Daily.TempMax) > 0 {
		data.Current.HighTemp = raw.Daily.TempMax[0]
		data.Current.LowTemp = raw.Daily.TempMin[0]
	}

	// Build daily forecast (skip today = index 0).
	for i := 1; i < len(raw.Daily.Time); i++ {
		t, err := time.Parse("2006-01-02", raw.Daily.Time[i])
		if err != nil {
			continue
		}
		data.Daily = append(data.Daily, DailyForecast{
			Date:        t,
			WeatherCode: raw.Daily.WeatherCode[i],
			HighTemp:    raw.Daily.TempMax[i],
			LowTemp:     raw.Daily.TempMin[i],
		})
	}

	return data, nil
}

// ConditionText returns a human-readable condition from a WMO weather code.
func ConditionText(code int) string {
	switch {
	case code == 0:
		return "Clear Sky"
	case code == 1:
		return "Mostly Clear"
	case code == 2:
		return "Partly Cloudy"
	case code == 3:
		return "Overcast"
	case code >= 45 && code <= 48:
		return "Fog"
	case code >= 51 && code <= 55:
		return "Drizzle"
	case code >= 56 && code <= 57:
		return "Freezing Drizzle"
	case code >= 61 && code <= 65:
		return "Rain"
	case code >= 66 && code <= 67:
		return "Freezing Rain"
	case code >= 71 && code <= 75:
		return "Snow"
	case code == 77:
		return "Snow Grains"
	case code >= 80 && code <= 82:
		return "Showers"
	case code >= 85 && code <= 86:
		return "Snow Showers"
	case code >= 95 && code <= 99:
		return "Thunderstorm"
	default:
		return "Unknown"
	}
}

// Compass returns a cardinal/intercardinal direction from degrees.
func Compass(deg int) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	idx := ((deg + 22) % 360) / 45
	if idx < 0 || idx >= len(dirs) {
		return "N"
	}
	return dirs[idx]
}
