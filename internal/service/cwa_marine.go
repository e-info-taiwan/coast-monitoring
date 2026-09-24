package service

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var taiwanLoc = time.FixedZone("CST", 8*3600)

type CWAMarineStation struct {
	ID          string   `json:"id"`
	NameZH      string   `json:"name_zh"`
	NameEN      string   `json:"name_en"`
	StationType string   `json:"station_type"`
	AreaName    string   `json:"area_name"`
	CountyName  string   `json:"county_name"`
	TownName    string   `json:"town_name"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Address     string   `json:"address"`
	IsActive    bool     `json:"is_active"`
}

type CWAObservationRecord struct {
	StationID  string
	ObservedAt time.Time
	SeaTempC   *float64
	AirTempC   *float64
}

type CWATempResult struct {
	StationID   string     `json:"station_id"`
	StationName string     `json:"station_name"`
	SurveyDate  string     `json:"survey_date"`
	EventTime   string     `json:"event_time,omitempty"`
	SeaTempC    *float64   `json:"sea_temp_c"`
	AirTempC    *float64   `json:"air_temp_c,omitempty"`
	ObservedAt  *time.Time `json:"observed_at,omitempty"`
	Source      string     `json:"source"`
	Message     string     `json:"message,omitempty"`
}

type CWASyncReport struct {
	StationsCount     int           `json:"stations_count"`
	ObservationsSaved int           `json:"observations_saved"`
	SitesLinked       int           `json:"sites_linked"`
	Errors            []string      `json:"errors,omitempty"`
	DurationMs        int64         `json:"duration_ms"`
}

type CWAMarineRepository interface {
	UpsertStations(ctx context.Context, stations []CWAMarineStation) error
	ListStations(ctx context.Context) ([]CWAMarineStation, error)
	GetStation(ctx context.Context, id string) (*CWAMarineStation, error)
	UpsertSeaTemperatures(ctx context.Context, records []CWAObservationRecord) (int, error)
	GetSeaTemperature(ctx context.Context, stationID string, dateStr string, timeStr string) (*CWATempResult, error)
	AutoLinkSites(ctx context.Context) (int, error)
}

type CWAScraperClient interface {
	FetchStations(ctx context.Context) ([]CWAMarineStation, error)
	FetchAreaObservations(ctx context.Context) ([]CWAObservationRecord, error)
	FetchStation48Hrs(ctx context.Context, stationID string) ([]CWAObservationRecord, error)
	FetchStation30Days(ctx context.Context, stationID string) ([]CWAObservationRecord, error)
}

type CWAMarineService struct {
	Repo    CWAMarineRepository
	Scraper CWAScraperClient
}

func NewCWAMarineService(repo CWAMarineRepository, scraper CWAScraperClient) *CWAMarineService {
	if scraper == nil {
		scraper = NewCWAScraper(nil, "")
	}
	return &CWAMarineService{
		Repo:    repo,
		Scraper: scraper,
	}
}

func (s *CWAMarineService) ListStations(ctx context.Context) ([]CWAMarineStation, error) {
	return s.Repo.ListStations(ctx)
}

func (s *CWAMarineService) SyncStations(ctx context.Context) (int, error) {
	stations, err := s.Scraper.FetchStations(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch stations from CWA: %w", err)
	}
	if len(stations) == 0 {
		return 0, nil
	}
	if err := s.Repo.UpsertStations(ctx, stations); err != nil {
		return 0, fmt.Errorf("upsert stations: %w", err)
	}
	return len(stations), nil
}

func (s *CWAMarineService) SyncLatestObservations(ctx context.Context) (int, error) {
	records, err := s.Scraper.FetchAreaObservations(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch latest area observations: %w", err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	saved, err := s.Repo.UpsertSeaTemperatures(ctx, records)
	if err != nil {
		return 0, fmt.Errorf("upsert sea temperatures: %w", err)
	}
	return saved, nil
}

func (s *CWAMarineService) SyncStationHistory(ctx context.Context, stationID string, days int) (int, error) {
	var records []CWAObservationRecord
	var err error
	if days > 2 {
		records, err = s.Scraper.FetchStation30Days(ctx, stationID)
	} else {
		records, err = s.Scraper.FetchStation48Hrs(ctx, stationID)
	}
	if err != nil {
		return 0, fmt.Errorf("fetch station %s history: %w", stationID, err)
	}
	if len(records) == 0 {
		return 0, nil
	}
	saved, err := s.Repo.UpsertSeaTemperatures(ctx, records)
	if err != nil {
		return 0, fmt.Errorf("upsert station %s history: %w", stationID, err)
	}
	return saved, nil
}

func (s *CWAMarineService) SyncAll(ctx context.Context) (*CWASyncReport, error) {
	start := time.Now()
	report := &CWASyncReport{}

	// 1. Sync stations
	stCount, err := s.SyncStations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("stations: %v", err))
	} else {
		report.StationsCount = stCount
	}

	// 2. Sync latest observations from all sea areas
	obsCount, err := s.SyncLatestObservations(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("observations: %v", err))
	} else {
		report.ObservationsSaved = obsCount
	}

	// 3. Auto-link sites to nearest stations if not yet linked
	linked, err := s.Repo.AutoLinkSites(ctx)
	if err != nil {
		report.Errors = append(report.Errors, fmt.Sprintf("auto-link sites: %v", err))
	} else {
		report.SitesLinked = linked
	}

	report.DurationMs = time.Since(start).Milliseconds()
	return report, nil
}

func (s *CWAMarineService) GetTemperatureForEvent(ctx context.Context, stationID, dateStr, timeStr string) (*CWATempResult, error) {
	if stationID == "" {
		return nil, fmt.Errorf("%w: 請指定測站代碼", ErrValidation)
	}
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return nil, fmt.Errorf("%w: 日期格式錯誤 (YYYY-MM-DD)", ErrValidation)
	}

	// 1. Look up in database
	res, err := s.Repo.GetSeaTemperature(ctx, stationID, dateStr, timeStr)
	if err == nil && res != nil && res.SeaTempC != nil {
		res.Source = "database"
		return res, nil
	}

	// 2. If not found in database, check if date is within 30 days of today
	now := time.Now().In(taiwanLoc)
	targetDate, _ := time.ParseInLocation("2006-01-02", dateStr, taiwanLoc)
	daysDiff := now.Sub(targetDate).Hours() / 24

	// If within last 30 days and not in future
	if daysDiff >= -1 && daysDiff <= 31 {
		// Attempt live backfill from CWA
		_, _ = s.SyncStationHistory(ctx, stationID, 30)

		// Re-query database
		res2, err2 := s.Repo.GetSeaTemperature(ctx, stationID, dateStr, timeStr)
		if err2 == nil && res2 != nil && res2.SeaTempC != nil {
			res2.Source = "cwa_live"
			return res2, nil
		}
	}

	// Return whatever we have (or station info with null temp)
	if res != nil {
		if res.Message == "" {
			res.Message = "該日期目前無海溫觀測資料"
		}
		return res, nil
	}

	station, err := s.Repo.GetStation(ctx, stationID)
	name := stationID
	if err == nil && station != nil {
		name = station.NameZH
	}

	return &CWATempResult{
		StationID:   stationID,
		StationName: name,
		SurveyDate:  dateStr,
		EventTime:   timeStr,
		SeaTempC:    nil,
		Source:      "none",
		Message:     "查無該日期與測站的海溫資料",
	}, nil
}

// -------------------------------------------------------------------------
// Scraper Implementation
// -------------------------------------------------------------------------

type CWAScraper struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewCWAScraper(client *http.Client, baseURL string) *CWAScraper {
	if client == nil {
		client = &http.Client{
			Timeout: 20 * time.Second,
		}
	}
	if baseURL == "" {
		baseURL = "https://www.cwa.gov.tw"
	}
	return &CWAScraper{
		BaseURL:    baseURL,
		HTTPClient: client,
	}
}

func (c *CWAScraper) get(ctx context.Context, urlPath string, referer string) ([]byte, error) {
	url := urlPath
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = strings.TrimRight(c.BaseURL, "/") + "/" + strings.TrimLeft(urlPath, "/")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	if referer != "" {
		req.Header.Set("Referer", referer)
	} else {
		req.Header.Set("Referer", strings.TrimRight(c.BaseURL, "/")+"/V8/C/M/OBS_Marine.html")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

func (c *CWAScraper) FetchStations(ctx context.Context) ([]CWAMarineStation, error) {
	data, err := c.get(ctx, "/Data/js/marine/MMC_stations.js", "")
	if err != nil {
		return nil, err
	}
	return ParseStationsJS(string(data))
}

var stationLineRe = regexp.MustCompile(`(?m)^[\s\t]*['"]([^'"]+)['"]\s*:\s*\{(.*)\},?$`)
var stationNameCRe = regexp.MustCompile(`['"]stationName['"]\s*:\s*\{[^}]*['"]C['"]\s*:\s*['"]([^'"]+)['"]`)
var stationNameERe = regexp.MustCompile(`['"]stationName['"]\s*:\s*\{[^}]*['"]E['"]\s*:\s*['"]([^'"]+)['"]`)
var areaNameCRe = regexp.MustCompile(`['"]areaName['"]\s*:\s*\{[^}]*['"]C['"]\s*:\s*['"]([^'"]+)['"]`)
var countyNameCRe = regexp.MustCompile(`['"]countyName['"]\s*:\s*\{[^}]*['"]C['"]\s*:\s*['"]([^'"]+)['"]`)
var townNameCRe = regexp.MustCompile(`['"]townName['"]\s*:\s*\{[^}]*['"]C['"]\s*:\s*['"]([^'"]+)['"]`)
var addressCRe = regexp.MustCompile(`['"]stationAddress['"]\s*:\s*\{[^}]*['"]C['"]\s*:\s*['"]([^'"]+)['"]`)
var stationLatRe = regexp.MustCompile(`['"]stationLatitude['"]\s*:\s*['"]([^'"]+)['"]`)
var stationLngRe = regexp.MustCompile(`['"]stationLongitude['"]\s*:\s*['"]([^'"]+)['"]`)

func ParseStationsJS(content string) ([]CWAMarineStation, error) {
	lines := strings.Split(content, "\n")
	var stations []CWAMarineStation
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := stationLineRe.FindStringSubmatch(line)
		if len(matches) < 3 {
			continue
		}
		id := strings.TrimSpace(matches[1])
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		body := matches[2]

		nameZH := findMatch(stationNameCRe, body)
		nameEN := findMatch(stationNameERe, body)
		areaName := findMatch(areaNameCRe, body)
		countyName := findMatch(countyNameCRe, body)
		townName := findMatch(townNameCRe, body)
		addr := findMatch(addressCRe, body)
		latStr := findMatch(stationLatRe, body)
		lngStr := findMatch(stationLngRe, body)

		stType := "潮位站"
		if strings.Contains(nameZH, "浮標") {
			stType = "資料浮標"
		} else if strings.Contains(nameZH, "波浪") {
			stType = "波浪站"
		}

		var latPtr, lngPtr *float64
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil && !math.IsNaN(lat) {
			latPtr = &lat
		}
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil && !math.IsNaN(lng) {
			lngPtr = &lng
		}

		stations = append(stations, CWAMarineStation{
			ID:          id,
			NameZH:      nameZH,
			NameEN:      nameEN,
			StationType: stType,
			AreaName:    areaName,
			CountyName:  countyName,
			TownName:    townName,
			Latitude:    latPtr,
			Longitude:   lngPtr,
			Address:     addr,
			IsActive:    true,
		})
	}

	return stations, nil
}

func (c *CWAScraper) FetchAreaObservations(ctx context.Context) ([]CWAObservationRecord, error) {
	areas := []string{"index"}
	for i := 1; i <= 16; i++ {
		areas = append(areas, fmt.Sprintf("OSea%02d", i))
	}

	now := time.Now().In(taiwanLoc)
	var allRecords []CWAObservationRecord
	seen := make(map[string]bool)

	for _, area := range areas {
		path := fmt.Sprintf("/V8/C/M/OBS_Marine/48hrsSeaObs_MOD/%s.html", area)
		data, err := c.get(ctx, path, strings.TrimRight(c.BaseURL, "/")+"/V8/C/M/OBS_Marine.html")
		if err != nil {
			// Some areas might be temporarily unavailable, continue to next
			continue
		}
		records := ParseAreaHTML(string(data), now)
		for _, r := range records {
			key := fmt.Sprintf("%s_%s", r.StationID, r.ObservedAt.Format(time.RFC3339))
			if !seen[key] {
				seen[key] = true
				allRecords = append(allRecords, r)
			}
		}
	}

	return allRecords, nil
}

func (c *CWAScraper) FetchStation48Hrs(ctx context.Context, stationID string) ([]CWAObservationRecord, error) {
	path := fmt.Sprintf("/V8/C/M/OBS_Marine/48hrsSeaObs_MOD/M%s.html", stationID)
	referer := fmt.Sprintf("%s/V8/C/M/OBS_Marine_30day.html?MID=%s", strings.TrimRight(c.BaseURL, "/"), stationID)
	data, err := c.get(ctx, path, referer)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(taiwanLoc)
	return ParseStationHistoryHTML(stationID, string(data), now), nil
}

func (c *CWAScraper) FetchStation30Days(ctx context.Context, stationID string) ([]CWAObservationRecord, error) {
	path := fmt.Sprintf("/V8/C/M/OBS_Marine/30daysSeaObs_MOD/M%s.html", stationID)
	referer := fmt.Sprintf("%s/V8/C/M/OBS_Marine_30day.html?MID=%s", strings.TrimRight(c.BaseURL, "/"), stationID)
	data, err := c.get(ctx, path, referer)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(taiwanLoc)
	return ParseStationHistoryHTML(stationID, string(data), now), nil
}

// -------------------------------------------------------------------------
// HTML Parsers
// -------------------------------------------------------------------------

var trRegex = regexp.MustCompile(`(?s)<tr>(.*?)</tr>`)
var areaStationProfileRe = regexp.MustCompile(`stations_profile\(['"]([^'"]+)['"]\)`)
var areaTimeRe = regexp.MustCompile(`<br\s*/?>\s*(\d{1,2})日(\d{1,2})時`)
var tempCRe = regexp.MustCompile(`<span class=['"]tempC['"]>([^<]+)</span>`)
var historyTimeRe = regexp.MustCompile(`scope=['"]row['"]>\s*(\d{2})/(\d{2})[^\n<]*<br\s*/?>\s*(\d{2}):(\d{2})`)

func ParseAreaHTML(htmlContent string, now time.Time) []CWAObservationRecord {
	var records []CWAObservationRecord
	rows := trRegex.FindAllStringSubmatch(htmlContent, -1)

	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		content := row[1]
		stMatch := areaStationProfileRe.FindStringSubmatch(content)
		if len(stMatch) < 2 {
			continue
		}
		stationID := strings.TrimSpace(stMatch[1])
		if stationID == "" {
			continue
		}

		timeMatch := areaTimeRe.FindStringSubmatch(content)
		if len(timeMatch) < 3 {
			continue
		}
		day, err1 := strconv.Atoi(timeMatch[1])
		hour, err2 := strconv.Atoi(timeMatch[2])
		if err1 != nil || err2 != nil {
			continue
		}

		// Calculate observed timestamp
		var obsTime time.Time
		if day == now.Day() {
			obsTime = time.Date(now.Year(), now.Month(), day, hour, 0, 0, 0, taiwanLoc)
		} else if day < now.Day() {
			obsTime = time.Date(now.Year(), now.Month(), day, hour, 0, 0, 0, taiwanLoc)
		} else {
			// Day > now.Day() -> previous month
			prevMonth := now.AddDate(0, -1, 0)
			obsTime = time.Date(prevMonth.Year(), prevMonth.Month(), day, hour, 0, 0, 0, taiwanLoc)
		}

		// Extract temperatures
		temps := tempCRe.FindAllStringSubmatch(content, -1)
		var seaTemp, airTemp *float64
		if len(temps) > 0 {
			seaTemp = parseTemp(temps[0][1])
		}
		if len(temps) > 1 {
			airTemp = parseTemp(temps[1][1])
		}

		records = append(records, CWAObservationRecord{
			StationID:  stationID,
			ObservedAt: obsTime,
			SeaTempC:   seaTemp,
			AirTempC:   airTemp,
		})
	}

	return records
}

func ParseStationHistoryHTML(stationID, htmlContent string, now time.Time) []CWAObservationRecord {
	var records []CWAObservationRecord
	rows := trRegex.FindAllStringSubmatch(htmlContent, -1)

	for _, row := range rows {
		if len(row) < 2 {
			continue
		}
		content := row[1]
		timeMatch := historyTimeRe.FindStringSubmatch(content)
		if len(timeMatch) < 5 {
			continue
		}

		month, err1 := strconv.Atoi(timeMatch[1])
		day, err2 := strconv.Atoi(timeMatch[2])
		hour, err3 := strconv.Atoi(timeMatch[3])
		minute, err4 := strconv.Atoi(timeMatch[4])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}

		year := now.Year()
		if month == 12 && now.Month() == 1 {
			year--
		} else if month == 1 && now.Month() == 12 {
			year++
		}

		obsTime := time.Date(year, time.Month(month), day, hour, minute, 0, 0, taiwanLoc)

		temps := tempCRe.FindAllStringSubmatch(content, -1)
		var seaTemp, airTemp *float64
		if len(temps) > 0 {
			seaTemp = parseTemp(temps[0][1])
		}
		if len(temps) > 1 {
			airTemp = parseTemp(temps[1][1])
		}

		records = append(records, CWAObservationRecord{
			StationID:  stationID,
			ObservedAt: obsTime,
			SeaTempC:   seaTemp,
			AirTempC:   airTemp,
		})
	}

	return records
}

func parseTemp(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || s == "--" {
		return nil
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil && !math.IsNaN(v) {
		return &v
	}
	return nil
}

func findMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}
