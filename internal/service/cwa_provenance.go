package service

import (
	"context"
	"math"
	"sort"
	"time"
)

func DescribeStationCandidates(stations []CWAMarineStation, lat, lng *float64) {
	for i := range stations {
		s := &stations[i]
		s.DistanceKM = nil
		switch {
		case !s.IsActive:
			s.Availability = "inactive"
		case !s.HasSeaTemperature:
			s.Availability = "no_sea_temperature"
		case s.Latitude == nil || s.Longitude == nil:
			s.Availability = "missing_station_coordinates"
		case lat == nil || lng == nil:
			s.Availability = "missing_site_coordinates"
		default:
			s.Availability = "available"
		}
		if lat != nil && lng != nil && s.Latitude != nil && s.Longitude != nil {
			radians := math.Pi / 180
			dlat := (*s.Latitude - *lat) * radians
			dlng := (*s.Longitude - *lng) * radians
			a := math.Pow(math.Sin(dlat/2), 2) + math.Cos(*lat*radians)*math.Cos(*s.Latitude*radians)*math.Pow(math.Sin(dlng/2), 2)
			d := 6371 * 2 * math.Asin(math.Sqrt(math.Min(1, a)))
			s.DistanceKM = &d
		}
	}
	sort.SliceStable(stations, func(i, j int) bool {
		a, b := stations[i], stations[j]
		if (a.Availability == "available") != (b.Availability == "available") {
			return a.Availability == "available"
		}
		if a.DistanceKM == nil {
			return false
		}
		if b.DistanceKM == nil {
			return true
		}
		return *a.DistanceKM < *b.DistanceKM
	})
}

type cwaSyncTracker interface {
	BeginSync(context.Context) (int, error)
	FinishSync(context.Context, int, []string) error
	SyncStatus(context.Context) (any, error)
}

func (s *CWAMarineService) SyncStatus(ctx context.Context) (any, error) {
	if repo, ok := s.Repo.(cwaSyncTracker); ok {
		return repo.SyncStatus(ctx)
	}
	return nil, ErrNotFound
}

// Finish tracking even when the HTTP request was cancelled.
func finishSync(repo cwaSyncTracker, id int, report *CWASyncReport) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := repo.FinishSync(ctx, id, report.Errors); err != nil {
		report.Errors = append(report.Errors, "同步狀態儲存失敗")
	}
}
