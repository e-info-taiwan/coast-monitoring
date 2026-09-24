package service

import (
	"context"
	"testing"
	"time"
)

const sampleStationsJS = `
var MMC_stations={
  '46761F':{'stationName':{'C':'成功浮球式波浪站','E':"Chenggong Wave Station"},'areaName':{'C':'成功大武沿海','E':"Chenggong Dawu inshore"},'stationChargeIns':{'C':'交通部中央氣象署','E':"Central Weather Administration"},'stationLongitude':'121.4201','stationLatitude':'23.1325','countyName':{'C':'臺東縣','E':"Taitung County"},'townName':{'C':'成功鎮','E':"Chenggong Township"},'stationAddress':{'C':'三仙臺海岬北面約700公尺，水深約28公尺','E':"About 700 meters north of Sanxiantai Cape"}},
  'C4A02':{'stationName':{'C':'龍洞潮位站','E':"Longdong"},'areaName':{'C':'彭佳嶼基隆海面','E':"Pengjiayu-Keelung inshore"},'stationChargeIns':{'C':'交通部中央氣象署','E':"Central Weather Administration"},'stationLongitude':'121.9181','stationLatitude':'25.0975','countyName':{'C':'新北市','E':"New Taipei City"},'townName':{'C':'貢寮區','E':"Gongliao District"},'stationAddress':{'C':'龍洞遊艇港內','E':"Inside Longdong Boat Harbor"}}
};
`

const sampleAreaHTML = `
<tr>
  <th scope="row">
    <a href="#" onclick="stations_profile('C4A02')" role="button" data-toggle="modal" data-target="#st-1">龍洞潮位站
      <br/>24日13時</a>
  </th>
  <td class="is_show">-0.22</td>
  <td class="is_show"></td>
  <td class="is_show"></td>
  <td></td>
  <td><div>1.5</div></td>
  <td><span>北北東</span></td>
  <td><div>2.6</div></td>
  <td>
    <span class="tempC">26.5</span>
    <span class="tempF hide">79.7</span>
  </td>
  <td>
    <span class="tempC">29.1</span>
    <span class="tempF hide">84.4</span>
  </td>
  <td></td>
  <td><div></div></td>
  <td></td>
</tr>
`

const sampleHistoryHTML = `
<tr>
  <th scope="row">
    09/24(四)<br />13:00
  </th>
  <td class="is_show">-0.08</td>
  <td class="is_show"></td>
  <td class="is_show"></td>
  <td></td>
  <td><div>1.0</div></td>
  <td><span>東北</span></td>
  <td><div>1.7</div></td>
  <td>
    <span class="tempC">26.4</span>
    <span class="tempF hide">79.5</span>
  </td>
  <td>
    <span class="tempC">28.8</span>
    <span class="tempF hide">83.8</span>
  </td>
  <td>1014.7</td>
  <td></td>
  <td></td>
</tr>
`

func TestParseStationsJS(t *testing.T) {
	stations, err := ParseStationsJS(sampleStationsJS)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stations))
	}

	st1 := stations[0]
	if st1.ID != "46761F" || st1.NameZH != "成功浮球式波浪站" || st1.StationType != "波浪站" {
		t.Errorf("unexpected station 1: %+v", st1)
	}
	if st1.Latitude == nil || *st1.Latitude != 23.1325 {
		t.Errorf("unexpected lat: %v", st1.Latitude)
	}
	if st1.Longitude == nil || *st1.Longitude != 121.4201 {
		t.Errorf("unexpected lng: %v", st1.Longitude)
	}

	st2 := stations[1]
	if st2.ID != "C4A02" || st2.NameZH != "龍洞潮位站" || st2.StationType != "潮位站" {
		t.Errorf("unexpected station 2: %+v", st2)
	}
}

func TestParseAreaHTML(t *testing.T) {
	refTime := time.Date(2026, 9, 24, 15, 0, 0, 0, taiwanLoc)
	records := ParseAreaHTML(sampleAreaHTML, refTime)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.StationID != "C4A02" {
		t.Errorf("expected station C4A02, got %s", r.StationID)
	}
	if r.SeaTempC == nil || *r.SeaTempC != 26.5 {
		t.Errorf("expected sea temp 26.5, got %v", r.SeaTempC)
	}
	if r.AirTempC == nil || *r.AirTempC != 29.1 {
		t.Errorf("expected air temp 29.1, got %v", r.AirTempC)
	}
	expectedObs := time.Date(2026, 9, 24, 13, 0, 0, 0, taiwanLoc)
	if !r.ObservedAt.Equal(expectedObs) {
		t.Errorf("expected observedAt %v, got %v", expectedObs, r.ObservedAt)
	}
}

func TestParseStationHistoryHTML(t *testing.T) {
	refTime := time.Date(2026, 9, 24, 15, 0, 0, 0, taiwanLoc)
	records := ParseStationHistoryHTML("C4A02", sampleHistoryHTML, refTime)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if r.StationID != "C4A02" {
		t.Errorf("expected station C4A02, got %s", r.StationID)
	}
	if r.SeaTempC == nil || *r.SeaTempC != 26.4 {
		t.Errorf("expected sea temp 26.4, got %v", r.SeaTempC)
	}
	if r.AirTempC == nil || *r.AirTempC != 28.8 {
		t.Errorf("expected air temp 28.8, got %v", r.AirTempC)
	}
	expectedObs := time.Date(2026, 9, 24, 13, 0, 0, 0, taiwanLoc)
	if !r.ObservedAt.Equal(expectedObs) {
		t.Errorf("expected observedAt %v, got %v", expectedObs, r.ObservedAt)
	}
}

type mockCWAMarineRepo struct {
	stations []CWAMarineStation
	temps    map[string]*CWATempResult
}

func (m *mockCWAMarineRepo) UpsertStations(ctx context.Context, stations []CWAMarineStation) error {
	m.stations = stations
	return nil
}

func (m *mockCWAMarineRepo) ListStations(ctx context.Context) ([]CWAMarineStation, error) {
	return m.stations, nil
}

func (m *mockCWAMarineRepo) GetStation(ctx context.Context, id string) (*CWAMarineStation, error) {
	for _, s := range m.stations {
		if s.ID == id {
			return &s, nil
		}
	}
	return nil, nil
}

func (m *mockCWAMarineRepo) UpsertSeaTemperatures(ctx context.Context, records []CWAObservationRecord) (int, error) {
	return len(records), nil
}

func (m *mockCWAMarineRepo) GetSeaTemperature(ctx context.Context, stationID string, dateStr string, timeStr string) (*CWATempResult, error) {
	key := stationID + "_" + dateStr
	if res, ok := m.temps[key]; ok {
		return res, nil
	}
	return nil, nil
}

func (m *mockCWAMarineRepo) AutoLinkSites(ctx context.Context) (int, error) {
	return 5, nil
}

type mockCWAScraper struct{}

func (s *mockCWAScraper) FetchStations(ctx context.Context) ([]CWAMarineStation, error) {
	return []CWAMarineStation{{ID: "C4A02", NameZH: "龍洞潮位站"}}, nil
}

func (s *mockCWAScraper) FetchAreaObservations(ctx context.Context) ([]CWAObservationRecord, error) {
	temp := 26.5
	return []CWAObservationRecord{{
		StationID:  "C4A02",
		ObservedAt: time.Now(),
		SeaTempC:   &temp,
	}}, nil
}

func (s *mockCWAScraper) FetchStation48Hrs(ctx context.Context, stationID string) ([]CWAObservationRecord, error) {
	temp := 26.5
	return []CWAObservationRecord{{
		StationID:  stationID,
		ObservedAt: time.Now(),
		SeaTempC:   &temp,
	}}, nil
}

func (s *mockCWAScraper) FetchStation30Days(ctx context.Context, stationID string) ([]CWAObservationRecord, error) {
	return s.FetchStation48Hrs(ctx, stationID)
}

func TestCWAMarineServiceSyncAll(t *testing.T) {
	repo := &mockCWAMarineRepo{temps: make(map[string]*CWATempResult)}
	scraper := &mockCWAScraper{}
	svc := NewCWAMarineService(repo, scraper)

	report, err := svc.SyncAll(context.Background())
	if err != nil {
		t.Fatalf("SyncAll failed: %v", err)
	}

	if report.StationsCount != 1 {
		t.Errorf("expected 1 station, got %d", report.StationsCount)
	}
	if report.ObservationsSaved != 1 {
		t.Errorf("expected 1 observation saved, got %d", report.ObservationsSaved)
	}
	if report.SitesLinked != 5 {
		t.Errorf("expected 5 sites linked, got %d", report.SitesLinked)
	}
	if len(report.Errors) != 0 {
		t.Errorf("unexpected errors: %v", report.Errors)
	}
}

func TestGetTemperatureForEventFromDB(t *testing.T) {
	temp := 27.2
	repo := &mockCWAMarineRepo{
		stations: []CWAMarineStation{{ID: "C4A02", NameZH: "龍洞潮位站"}},
		temps: map[string]*CWATempResult{
			"C4A02_2026-09-24": {
				StationID:   "C4A02",
				StationName: "龍洞潮位站",
				SurveyDate:  "2026-09-24",
				EventTime:   "13:00",
				SeaTempC:    &temp,
			},
		},
	}
	svc := NewCWAMarineService(repo, &mockCWAScraper{})

	res, err := svc.GetTemperatureForEvent(context.Background(), "C4A02", "2026-09-24", "13:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SeaTempC == nil || *res.SeaTempC != 27.2 {
		t.Errorf("expected temp 27.2, got %v", res.SeaTempC)
	}
	if res.Source != "database" {
		t.Errorf("expected source database, got %s", res.Source)
	}
}
