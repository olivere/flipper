package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// BrightSkyClient talks to the Bright Sky API (DWD data) for weather forecasts.
type BrightSkyClient struct {
	http *http.Client
}

// NewBrightSkyClient creates a Bright Sky API client.
func NewBrightSkyClient() *BrightSkyClient {
	return &BrightSkyClient{
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Fetch retrieves current weather and daily forecast from Bright Sky.
func (c *BrightSkyClient) Fetch(ctx context.Context, loc Location, forecastDays int, units string) (*WeatherData, error) {
	current, err := c.fetchCurrent(ctx, loc)
	if err != nil {
		return nil, err
	}

	daily, err := c.fetchForecast(ctx, loc, forecastDays)
	if err != nil {
		return nil, err
	}

	data := &WeatherData{
		Location: loc,
		Current:  *current,
		Daily:    daily,
	}

	// Today's high/low from the first daily entry if available.
	if len(daily) > 0 {
		data.Current.HighTemp = daily[0].HighTemp
		data.Current.LowTemp = daily[0].LowTemp
		// Remove today from the daily forecast (renderer expects future days only).
		data.Daily = daily[1:]
	}

	if units == "imperial" {
		convertToImperial(data)
	}

	return data, nil
}

// fetchCurrent retrieves current conditions from Bright Sky.
func (c *BrightSkyClient) fetchCurrent(ctx context.Context, loc Location) (*CurrentWeather, error) {
	params := url.Values{
		"lat": {strconv.FormatFloat(loc.Lat, 'f', 4, 64)},
		"lon": {strconv.FormatFloat(loc.Lon, 'f', 4, 64)},
	}
	u := "https://api.brightsky.dev/current_weather?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("brightsky current request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("brightsky current request failed", "url", u, "error", err)
		return nil, fmt.Errorf("brightsky current fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("brightsky current unexpected status", "url", u, "status", resp.StatusCode)
		return nil, fmt.Errorf("brightsky current: status %d", resp.StatusCode)
	}

	var raw struct {
		Weather struct {
			Temperature   float64 `json:"temperature"`
			Humidity      float64 `json:"relative_humidity"`
			WindSpeed     float64 `json:"wind_speed_10"`
			WindDirection float64 `json:"wind_direction_10"`
			Icon          string  `json:"icon"`
			Condition     string  `json:"condition"`
		} `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("brightsky current decode: %w", err)
	}

	return &CurrentWeather{
		Temperature:   raw.Weather.Temperature,
		Humidity:      int(raw.Weather.Humidity),
		WeatherCode:   iconToWMO(raw.Weather.Icon),
		WindSpeed:     raw.Weather.WindSpeed,
		WindDirection: int(raw.Weather.WindDirection),
	}, nil
}

// fetchForecast retrieves hourly forecast data and aggregates it into daily forecasts.
func (c *BrightSkyClient) fetchForecast(ctx context.Context, loc Location, forecastDays int) ([]DailyForecast, error) {
	now := time.Now()
	today := now.Format("2006-01-02")
	lastDate := now.AddDate(0, 0, forecastDays).Format("2006-01-02")

	params := url.Values{
		"lat":       {strconv.FormatFloat(loc.Lat, 'f', 4, 64)},
		"lon":       {strconv.FormatFloat(loc.Lon, 'f', 4, 64)},
		"date":      {today},
		"last_date": {lastDate},
	}
	u := "https://api.brightsky.dev/weather?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("brightsky forecast request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		slog.Warn("brightsky forecast request failed", "url", u, "error", err)
		return nil, fmt.Errorf("brightsky forecast fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("brightsky forecast unexpected status", "url", u, "status", resp.StatusCode)
		return nil, fmt.Errorf("brightsky forecast: status %d", resp.StatusCode)
	}

	var raw struct {
		Weather []hourlyRecord `json:"weather"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("brightsky forecast decode: %w", err)
	}

	return aggregateDaily(raw.Weather), nil
}

// hourlyRecord is a single hourly weather entry from Bright Sky.
type hourlyRecord struct {
	Timestamp   string   `json:"timestamp"`
	Temperature *float64 `json:"temperature"`
	Icon        string   `json:"icon"`
}

// aggregateDaily groups hourly weather records into daily forecasts.
func aggregateDaily(records []hourlyRecord) []DailyForecast {
	type bucket struct {
		date    time.Time
		minTemp float64
		maxTemp float64
		icons   map[string]int
		hasData bool
	}

	buckets := map[string]*bucket{}
	var order []string

	for _, r := range records {
		t, err := time.Parse(time.RFC3339, r.Timestamp)
		if err != nil {
			continue
		}
		key := t.Format("2006-01-02")

		b, ok := buckets[key]
		if !ok {
			b = &bucket{
				date:    t.Truncate(24 * time.Hour),
				minTemp: math.MaxFloat64,
				maxTemp: -math.MaxFloat64,
				icons:   map[string]int{},
			}
			buckets[key] = b
			order = append(order, key)
		}

		if r.Temperature != nil {
			if *r.Temperature < b.minTemp {
				b.minTemp = *r.Temperature
			}
			if *r.Temperature > b.maxTemp {
				b.maxTemp = *r.Temperature
			}
			b.hasData = true
		}

		if r.Icon != "" {
			b.icons[r.Icon]++
		}
	}

	var days []DailyForecast
	for _, key := range order {
		b := buckets[key]
		if !b.hasData {
			continue
		}
		days = append(days, DailyForecast{
			Date:        b.date,
			WeatherCode: iconToWMO(dominantIcon(b.icons)),
			HighTemp:    b.maxTemp,
			LowTemp:     b.minTemp,
		})
	}
	return days
}

// dominantIcon returns the most frequent icon string from a frequency map.
func dominantIcon(icons map[string]int) string {
	var best string
	var bestCount int
	for icon, count := range icons {
		if count > bestCount {
			best = icon
			bestCount = count
		}
	}
	return best
}

// iconToWMO maps a Bright Sky icon string to a WMO weather code.
func iconToWMO(icon string) int {
	switch icon {
	case "clear-day", "clear-night":
		return 0
	case "partly-cloudy-day", "partly-cloudy-night":
		return 2
	case "cloudy":
		return 3
	case "fog":
		return 45
	case "rain":
		return 61
	case "sleet":
		return 66
	case "snow":
		return 71
	case "hail":
		return 77
	case "thunderstorm":
		return 95
	case "wind":
		return 0
	default:
		return 3 // default to overcast
	}
}

// convertToImperial converts metric weather data to imperial units in place.
func convertToImperial(data *WeatherData) {
	cToF := func(c float64) float64 { return c*9.0/5.0 + 32 }
	kmhToMph := func(k float64) float64 { return k * 0.621371 }

	data.Current.Temperature = cToF(data.Current.Temperature)
	data.Current.ApparentTemp = cToF(data.Current.ApparentTemp)
	data.Current.HighTemp = cToF(data.Current.HighTemp)
	data.Current.LowTemp = cToF(data.Current.LowTemp)
	data.Current.WindSpeed = kmhToMph(data.Current.WindSpeed)

	for i := range data.Daily {
		data.Daily[i].HighTemp = cToF(data.Daily[i].HighTemp)
		data.Daily[i].LowTemp = cToF(data.Daily[i].LowTemp)
	}
}
