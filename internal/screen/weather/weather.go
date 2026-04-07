package weather

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"sync"
	"time"

	"github.com/olivere/flipper/internal/config"
	"github.com/olivere/flipper/internal/layout"
	"github.com/olivere/flipper/internal/screen"
)

func init() {
	screen.Register("weather", func(_ *config.Config, params map[string]any) (screen.Screen, error) {
		city := screen.ParamString(params, "city", "")
		lat := screen.ParamFloat(params, "lat", 0)
		lon := screen.ParamFloat(params, "lon", 0)
		if city == "" && (lat == 0 && lon == 0) {
			return nil, fmt.Errorf("weather screen requires a city or lat/lon params")
		}

		refreshStr := screen.ParamString(params, "refresh_interval", "30m")
		refreshInterval, err := time.ParseDuration(refreshStr)
		if err != nil {
			refreshInterval = 30 * time.Minute
		}

		return New(Config{
			City:            city,
			Lat:             lat,
			Lon:             lon,
			ForecastDays:    screen.ParamInt(params, "forecast_days", 5),
			Units:           screen.ParamString(params, "units", "metric"),
			Locale:          screen.ParamString(params, "locale", "en"),
			Service:         screen.ParamString(params, "service", "openmeteo"),
			RefreshInterval: refreshInterval,
		}), nil
	})
}

// Config holds weather screen parameters.
type Config struct {
	City            string
	Lat             float64
	Lon             float64
	ForecastDays    int
	Units           string
	Locale          string
	Service         string // "openmeteo" (default) or "brightsky"
	RefreshInterval time.Duration
}

// Screen renders weather information for a configured city.
type Screen struct {
	cfg     Config
	client  *Client  // shared client for geocoding
	fetcher Fetcher  // weather data fetcher (service-specific)
	mu      sync.Mutex
	loc     *Location
	cache   *WeatherData
	expiry  time.Time
}

func New(cfg Config) *Screen {
	client := NewClient()
	var fetcher Fetcher
	if cfg.Service == "brightsky" {
		fetcher = NewBrightSkyClient()
	} else {
		fetcher = client
	}
	return &Screen{
		cfg:     cfg,
		client:  client,
		fetcher: fetcher,
	}
}

func (s *Screen) Name() string { return "weather" }

func (s *Screen) Render(ctx context.Context, opts screen.RenderOpts) (image.Image, error) {
	data, err := s.getData(ctx)
	if err != nil {
		return s.renderError(opts, err), nil
	}
	return s.renderWeather(opts, data), nil
}

// getData returns cached weather data or fetches fresh data.
func (s *Screen) getData(ctx context.Context) (*WeatherData, error) {
	s.mu.Lock()
	if s.cache != nil && time.Now().Before(s.expiry) {
		data := s.cache
		s.mu.Unlock()
		return data, nil
	}

	// Resolve location on first call: use lat/lon if provided, otherwise geocode.
	if s.loc == nil {
		if s.cfg.Lat != 0 || s.cfg.Lon != 0 {
			name := s.cfg.City
			if name == "" {
				name = fmt.Sprintf("%.2f, %.2f", s.cfg.Lat, s.cfg.Lon)
			}
			s.loc = &Location{Name: name, Country: "", Lat: s.cfg.Lat, Lon: s.cfg.Lon}
		} else {
			loc, err := s.client.Geocode(ctx, s.cfg.City)
			if err != nil {
				cached := s.cache
				s.mu.Unlock()
				if cached != nil {
					slog.Warn("geocode failed, using cache", "city", s.cfg.City, "error", err)
					return cached, nil
				}
				return nil, fmt.Errorf("geocode %q: %w", s.cfg.City, err)
			}
			s.loc = &loc
		}
	}
	loc := *s.loc
	cached := s.cache
	s.mu.Unlock()

	// Fetch outside the lock so concurrent renders aren't blocked.
	data, err := s.fetcher.Fetch(ctx, loc, s.cfg.ForecastDays+1, s.cfg.Units)
	if err != nil {
		if cached != nil {
			slog.Warn("weather fetch failed, using cache", "city", s.cfg.City, "error", err)
			return cached, nil
		}
		return nil, fmt.Errorf("fetch weather: %w", err)
	}

	s.mu.Lock()
	s.cache = data
	s.expiry = time.Now().Add(s.cfg.RefreshInterval)
	s.mu.Unlock()
	return data, nil
}

// renderWeather draws the full weather layout.
func (s *Screen) renderWeather(opts screen.RenderOpts, data *WeatherData) image.Image {
	c := layout.New(opts.Width, opts.Height, opts.ColorMode)
	sc := c.Scale()

	content, footer := c.Footer(c.Bounds(), 40)
	footerRight := data.Location.Name
	if data.Location.Country != "" {
		footerRight += ", " + data.Location.Country
	}
	c.DrawFooter(footer, "Weather", footerRight)

	padded := content.InsetAll(int(12 * sc))
	splitW := int(float64(padded.W) * 0.6)
	left, right := padded.SplitV(splitW)

	// Inset panels for spacing.
	left = left.Inset(0, int(8*sc), 0, 0)
	right = right.Inset(0, 0, 0, int(20*sc))

	s.drawToday(c, left, data, sc)
	s.drawForecast(c, right, data, sc)

	return c.Image()
}

// drawToday renders today's weather on the left panel.
func (s *Screen) drawToday(c *layout.Canvas, r layout.Rect, data *WeatherData, sc float64) {
	dc := c.DC()

	iconSize := 100 * sc
	iconCX := float64(r.X) + float64(r.W)*0.5

	iconCY := float64(r.Y) + float64(r.H)*0.18

	dc.SetColor(color.Black)
	drawIcon(dc, data.Current.WeatherCode, iconCX, iconCY, iconSize)

	// Temperature below the icon.
	unit := "C"
	if s.cfg.Units == "imperial" {
		unit = "F"
	}
	tempStr := fmt.Sprintf("%.0f°%s", data.Current.Temperature, unit)
	tempY := float64(r.Y) + float64(r.H)*0.42
	c.DrawTextCenter(tempStr, iconCX, tempY, layout.FontBold, layout.TextHero)

	// Condition text.
	condY := float64(r.Y) + float64(r.H)*0.58
	condition := ConditionText(data.Current.WeatherCode)
	c.DrawTextCenter(condition, iconCX, condY, layout.FontRegular, layout.TextHeading)

	// High / Low.
	hlY := float64(r.Y) + float64(r.H)*0.72
	hlStr := fmt.Sprintf("H: %.0f°  L: %.0f°", data.Current.HighTemp, data.Current.LowTemp)
	c.DrawTextCenter(hlStr, iconCX, hlY, layout.FontRegular, layout.TextBody)

	// Wind.
	windY := float64(r.Y) + float64(r.H)*0.84
	windUnit := "km/h"
	if s.cfg.Units == "imperial" {
		windUnit = "mph"
	}
	windStr := fmt.Sprintf("Wind: %.0f %s %s", data.Current.WindSpeed, windUnit, Compass(data.Current.WindDirection))
	c.DrawTextCenter(windStr, iconCX, windY, layout.FontRegular, layout.TextBody)
}

// drawForecast renders the multi-day forecast on the right panel.
// Layout per row: DayName  Icon  Low° ━━━━bar━━━━ High°
// The bar width represents the day's range relative to the week's overall spread.
func (s *Screen) drawForecast(c *layout.Canvas, r layout.Rect, data *WeatherData, sc float64) {
	dc := c.DC()
	days := data.Daily
	if len(days) > s.cfg.ForecastDays {
		days = days[:s.cfg.ForecastDays]
	}
	if len(days) == 0 {
		return
	}

	// Find the overall min/max across all forecast days for bar scaling.
	weekMin, weekMax := days[0].LowTemp, days[0].HighTemp
	for _, d := range days[1:] {
		if d.LowTemp < weekMin {
			weekMin = d.LowTemp
		}
		if d.HighTemp > weekMax {
			weekMax = d.HighTemp
		}
	}
	weekSpan := weekMax - weekMin
	if weekSpan < 1 {
		weekSpan = 1
	}

	// Fixed row height, vertically centered in the panel.
	rowH := int(28 * sc)
	rowGap := int(14 * sc)
	totalH := len(days)*rowH + (len(days)-1)*rowGap
	startY := r.Y + (r.H-totalH)/2

	rows := make([]layout.Rect, len(days))
	for i := range days {
		rows[i] = layout.Rect{X: r.X, Y: startY + i*(rowH+rowGap), W: r.W, H: rowH}
	}

	// Measure fixed-width columns.
	dayW := 30.0 * sc  // day name
	iconW := 28.0 * sc // icon area
	tempW := 32.0 * sc // temperature text
	gap := 6.0 * sc

	for i, day := range days {
		row := rows[i]
		cy := float64(row.Y) + float64(row.H)/2
		x := float64(row.X)

		// Day name.
		dayName := shortDayName(day.Date, s.cfg.Locale)
		c.DrawText(dayName, x, cy, layout.FontBold, layout.TextCaption)
		x += dayW + gap

		// Icon.
		iconSize := 18 * sc
		dc.SetColor(color.Black)
		drawIcon(dc, day.WeatherCode, x+iconW/2, cy, iconSize)
		x += iconW + gap

		// Low temperature.
		lowStr := fmt.Sprintf("%.0f°", day.LowTemp)
		c.DrawTextRight(lowStr, x+tempW, cy, layout.FontRegular, layout.TextCaption)
		x += tempW + gap

		// Temperature range bar.
		barMaxW := float64(row.X+row.W) - x - tempW - gap
		if barMaxW < 10*sc {
			barMaxW = 10 * sc
		}

		// Bar position and width within the overall week range.
		barStart := (day.LowTemp - weekMin) / weekSpan
		barEnd := (day.HighTemp - weekMin) / weekSpan
		barX := x + barStart*barMaxW
		barW := (barEnd - barStart) * barMaxW
		if barW < 4*sc {
			barW = 4 * sc
		}
		barH := 4.0 * sc
		barR := barH / 2 // rounded ends

		dc.SetColor(color.Black)
		dc.DrawRoundedRectangle(barX, cy-barH/2, barW, barH, barR)
		dc.Fill()

		// High temperature.
		highStr := fmt.Sprintf("%.0f°", day.HighTemp)
		c.DrawText(highStr, float64(row.X+row.W)-tempW, cy, layout.FontBold, layout.TextCaption)
	}
}

// shortDayName returns an abbreviated weekday name for the given locale.
func shortDayName(t time.Time, locale string) string {
	if locale == "de" {
		days := [...]string{"So", "Mo", "Di", "Mi", "Do", "Fr", "Sa"}
		return days[t.Weekday()]
	}
	return t.Format("Mon")
}

// renderError draws a fallback screen when weather data is unavailable.
func (s *Screen) renderError(opts screen.RenderOpts, err error) image.Image {
	c := layout.New(opts.Width, opts.Height, opts.ColorMode)

	content, footer := c.Footer(c.Bounds(), 40)
	c.DrawFooter(footer, "Weather", s.cfg.City)

	cx := float64(content.X) + float64(content.W)/2
	cy := float64(content.Y) + float64(content.H)/2

	c.DrawTextCenter("Weather Unavailable", cx, cy-20*c.Scale(), layout.FontBold, layout.TextTitle)
	c.DrawTextCenter(err.Error(), cx, cy+20*c.Scale(), layout.FontRegular, layout.TextCaption)

	return c.Image()
}
