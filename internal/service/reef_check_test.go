package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

func TestReefCheckCreateStoresCompleteSurvey(t *testing.T) {
	repo := &fakeReefCheckSurveyRepository{}
	svc := ReefCheckSurveyService{Surveys: repo}
	actor := activePolicyVolunteer()
	input := validReefCheckSurveyInput()
	input.GeneralComments = " completed during calm seas "
	input.SubstrateComments = " sheet substrate notes "
	input.RKCReason = " old storm damage "
	input.CountryIsland = " 台灣 "
	input.TeamLeader = " 小鄭教練 "
	input.SurveyTime = " 08:40 "
	input.Visibility = " 15米 "
	input.Temperature = " 28度 "
	rkcBleachingPercent := 25.5
	input.RKCBleachingPercent = &rkcBleachingPercent

	created, err := svc.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("Create error = %v", err)
	}

	if created.ID == uuid.Nil {
		t.Fatal("created survey id is nil")
	}
	if !repo.created {
		t.Fatal("repository was not called")
	}
	if repo.record.CreatedBy != actor.ID {
		t.Fatalf("CreatedBy = %s, want actor %s", repo.record.CreatedBy, actor.ID)
	}
	if repo.record.GeneralComments != "completed during calm seas" {
		t.Fatalf("GeneralComments = %q, want trimmed value", repo.record.GeneralComments)
	}
	if repo.record.SubstrateComments != "sheet substrate notes" {
		t.Fatalf("SubstrateComments = %q, want trimmed value", repo.record.SubstrateComments)
	}
	if repo.record.RKCReason != "old storm damage" {
		t.Fatalf("RKCReason = %q, want trimmed value", repo.record.RKCReason)
	}
	if repo.record.CountryIsland != "台灣" {
		t.Fatalf("CountryIsland = %q, want trimmed value", repo.record.CountryIsland)
	}
	if repo.record.TeamLeader != "小鄭教練" {
		t.Fatalf("TeamLeader = %q, want trimmed value", repo.record.TeamLeader)
	}
	if repo.record.SurveyTime != "08:40" {
		t.Fatalf("SurveyTime = %q, want trimmed value", repo.record.SurveyTime)
	}
	if repo.record.Visibility != "15米" {
		t.Fatalf("Visibility = %q, want trimmed value", repo.record.Visibility)
	}
	if repo.record.Temperature != "28度" {
		t.Fatalf("Temperature = %q, want trimmed value", repo.record.Temperature)
	}
	if repo.record.RKCBleachingPercent == nil || *repo.record.RKCBleachingPercent != 25.5 {
		t.Fatalf("RKCBleachingPercent = %v, want 25.5", repo.record.RKCBleachingPercent)
	}
	if got := len(repo.record.Segments); got != 4 {
		t.Fatalf("segment count = %d, want 4", got)
	}
	if got := len(repo.record.SubstratePoints); got != 160 {
		t.Fatalf("substrate point count = %d, want 160", got)
	}
	wantMeters := map[int]float64{
		0:   0,
		39:  19.5,
		40:  25,
		79:  44.5,
		80:  50,
		119: 69.5,
		120: 75,
		159: 94.5,
	}
	for index, want := range wantMeters {
		if got := repo.record.SubstratePoints[index].TransectM; got != want {
			t.Fatalf("substrate point %d transectM = %.1f, want %.1f", index, got, want)
		}
	}
	if got := len(repo.record.SubstrateBleaching); got != 4 {
		t.Fatalf("substrate bleaching rows = %d, want 4", got)
	}
	if got := len(repo.record.MetricCounts); got != 20 {
		t.Fatalf("metric count rows = %d, want 20", got)
	}
}

func TestReefCheckCreateRequiresCoreFields(t *testing.T) {
	actor := activePolicyVolunteer()
	tests := []struct {
		name   string
		mutate func(*ReefCheckSurveyInput)
	}{
		{name: "survey date", mutate: func(input *ReefCheckSurveyInput) { input.SurveyDate = time.Time{} }},
		{name: "site id", mutate: func(input *ReefCheckSurveyInput) { input.SiteID = uuid.Nil }},
		{name: "depth", mutate: func(input *ReefCheckSurveyInput) { input.DepthM = 0 }},
		{name: "fish length mode", mutate: func(input *ReefCheckSurveyInput) { input.FishLengthMode = "" }},
		{name: "benthos recorder", mutate: func(input *ReefCheckSurveyInput) { input.Recorders = input.Recorders[1:] }},
		{name: "fish recorder", mutate: func(input *ReefCheckSurveyInput) {
			input.Recorders = append(input.Recorders[:1], input.Recorders[2:]...)
		}},
		{name: "invertebrate recorder", mutate: func(input *ReefCheckSurveyInput) { input.Recorders = input.Recorders[:2] }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validReefCheckSurveyInput()
			tt.mutate(&input)

			_, err := ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}.Create(context.Background(), actor, input)

			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Create error = %v, want %v", err, ErrValidation)
			}
		})
	}
}

func TestReefCheckCreateRequiresFourCompleteSubstrateSegments(t *testing.T) {
	actor := activePolicyVolunteer()
	tests := []struct {
		name   string
		mutate func(*ReefCheckSurveyInput)
	}{
		{name: "missing segment metadata", mutate: func(input *ReefCheckSurveyInput) { input.Segments = input.Segments[:3] }},
		{name: "segment outside range", mutate: func(input *ReefCheckSurveyInput) { input.Segments[3].Index = 5 }},
		{name: "missing one substrate point", mutate: func(input *ReefCheckSurveyInput) { input.SubstratePoints = input.SubstratePoints[:159] }},
		{name: "duplicate point index", mutate: func(input *ReefCheckSurveyInput) { input.SubstratePoints[159] = input.SubstratePoints[0] }},
		{name: "point outside range", mutate: func(input *ReefCheckSurveyInput) { input.SubstratePoints[0].PointIndex = 41 }},
		{name: "missing bleaching row", mutate: func(input *ReefCheckSurveyInput) { input.SubstrateBleaching = input.SubstrateBleaching[:3] }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validReefCheckSurveyInput()
			tt.mutate(&input)

			_, err := ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}.Create(context.Background(), actor, input)

			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Create error = %v, want %v", err, ErrValidation)
			}
		})
	}
}

func TestReefCheckCreateRejectsIncorrectSubstrateTransectMeter(t *testing.T) {
	input := validReefCheckSurveyInput()
	input.SubstratePoints[1].TransectM = 9.5

	_, err := ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}.Create(context.Background(), activePolicyVolunteer(), input)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create error = %v, want %v", err, ErrValidation)
	}
}

func TestReefCheckCreateRejectsInvalidSubstrateCodeAndNegativeCounts(t *testing.T) {
	actor := activePolicyVolunteer()
	tests := []struct {
		name   string
		mutate func(*ReefCheckSurveyInput)
	}{
		{name: "invalid substrate code", mutate: func(input *ReefCheckSurveyInput) { input.SubstratePoints[0].Code = "unknown" }},
		{name: "negative hc bleaching", mutate: func(input *ReefCheckSurveyInput) { input.SubstrateBleaching[0].HCBleachedCount = -1 }},
		{name: "negative sc bleaching", mutate: func(input *ReefCheckSurveyInput) { input.SubstrateBleaching[0].SCBleachedCount = -1 }},
		{name: "negative metric count", mutate: func(input *ReefCheckSurveyInput) { input.MetricCounts[0].Count = -1 }},
		{name: "unknown module", mutate: func(input *ReefCheckSurveyInput) { input.MetricCounts[0].Module = "weather" }},
		{name: "metric segment outside range", mutate: func(input *ReefCheckSurveyInput) { input.MetricCounts[0].SegmentIndex = 0 }},
		{name: "negative rkc bleaching percentage", mutate: func(input *ReefCheckSurveyInput) {
			negative := -0.1
			input.RKCBleachingPercent = &negative
		}},
		{name: "rkc bleaching percentage over 100", mutate: func(input *ReefCheckSurveyInput) {
			tooHigh := 100.1
			input.RKCBleachingPercent = &tooHigh
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validReefCheckSurveyInput()
			tt.mutate(&input)

			_, err := ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}.Create(context.Background(), actor, input)

			if !errors.Is(err, ErrValidation) {
				t.Fatalf("Create error = %v, want %v", err, ErrValidation)
			}
		})
	}
}

func TestReefCheckCreateRequiresRKCReasonWhenRKCCoverageIsAtLeastTenPercent(t *testing.T) {
	input := validReefCheckSurveyInput()
	input.RKCReason = ""
	for i := range 16 {
		input.SubstratePoints[i].Code = "3"
	}

	_, err := ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}.Create(context.Background(), activePolicyVolunteer(), input)

	if !errors.Is(err, ErrValidation) {
		t.Fatalf("Create error = %v, want %v", err, ErrValidation)
	}

	input.RKCReason = "typhoon waves"
	if _, err := (ReefCheckSurveyService{Surveys: &fakeReefCheckSurveyRepository{}}).Create(context.Background(), activePolicyVolunteer(), input); err != nil {
		t.Fatalf("Create with RKC reason error = %v", err)
	}
}

func TestDisabledActorCannotCreateReefCheckSurveyAndDoesNotWrite(t *testing.T) {
	repo := &fakeReefCheckSurveyRepository{}
	svc := ReefCheckSurveyService{Surveys: repo}
	actor := activePolicyVolunteer()
	actor.Status = policy.StatusDisabled

	_, err := svc.Create(context.Background(), actor, validReefCheckSurveyInput())

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("Create error = %v, want %v", err, ErrForbidden)
	}
	if repo.created {
		t.Fatal("repository was called for disabled actor")
	}
}

func TestSubstrateSummaryCalculatesCoverageSDSEAndLiveCoralCover(t *testing.T) {
	points := makeSubstratePoints(func(segment, point int) string {
		switch {
		case point <= 10:
			return "1"
		case point <= 20:
			return "2"
		case point <= 25:
			return "3"
		case point <= 30:
			return "4"
		case point <= 35:
			return "6"
		default:
			return "8"
		}
	})

	summary, err := CalculateSubstrateSummary(points)
	if err != nil {
		t.Fatalf("CalculateSubstrateSummary error = %v", err)
	}

	assertFloatNear(t, summary.Categories["HC"].CoveragePercent, 25)
	assertFloatNear(t, summary.Categories["SC"].CoveragePercent, 25)
	assertFloatNear(t, summary.Categories["RKC"].CoveragePercent, 12.5)
	assertFloatNear(t, summary.LiveCoralCoverPercent, 50)
	assertFloatNear(t, summary.Categories["HC"].SegmentCoveragePercent[0], 25)
	assertFloatNear(t, summary.Categories["HC"].StandardDeviation, 0)
	assertFloatNear(t, summary.Categories["HC"].StandardError, 0)
}

func TestMetricSummaryCalculatesAverageSDAndSEAcrossFourSegments(t *testing.T) {
	counts := []ReefCheckMetricCountInput{
		{Module: ReefCheckModuleFish, MetricKey: "butterflyfish", SegmentIndex: 1, Count: 2},
		{Module: ReefCheckModuleFish, MetricKey: "butterflyfish", SegmentIndex: 2, Count: 4},
		{Module: ReefCheckModuleFish, MetricKey: "butterflyfish", SegmentIndex: 3, Count: 6},
		{Module: ReefCheckModuleFish, MetricKey: "butterflyfish", SegmentIndex: 4, Count: 8},
		{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster", SegmentIndex: 1, Count: 0},
		{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster", SegmentIndex: 2, Count: 0},
		{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster", SegmentIndex: 3, Count: 1},
		{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster", SegmentIndex: 4, Count: 1},
	}

	summary, err := CalculateMetricSummaries(counts)
	if err != nil {
		t.Fatalf("CalculateMetricSummaries error = %v", err)
	}

	fish := summary[ReefCheckMetricID{Module: ReefCheckModuleFish, MetricKey: "butterflyfish"}]
	assertFloatNear(t, fish.Average, 5)
	assertFloatNear(t, fish.StandardDeviation, math.Sqrt(20.0/3.0))
	assertFloatNear(t, fish.StandardError, math.Sqrt(20.0/3.0)/2)

	invert := summary[ReefCheckMetricID{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster"}]
	assertFloatNear(t, invert.Average, 0.5)
	assertFloatNear(t, invert.StandardDeviation, math.Sqrt(1.0/3.0))
	assertFloatNear(t, invert.StandardError, math.Sqrt(1.0/3.0)/2)
}

func TestImpactSummaryGradesCountsBeforeAverageSDAndSE(t *testing.T) {
	counts := []ReefCheckMetricCountInput{
		{Module: ReefCheckModuleImpact, MetricKey: "trash", SegmentIndex: 1, Count: 0},
		{Module: ReefCheckModuleImpact, MetricKey: "trash", SegmentIndex: 2, Count: 1},
		{Module: ReefCheckModuleImpact, MetricKey: "trash", SegmentIndex: 3, Count: 4},
		{Module: ReefCheckModuleImpact, MetricKey: "trash", SegmentIndex: 4, Count: 5},
	}

	summary, err := CalculateImpactSummaries(counts)
	if err != nil {
		t.Fatalf("CalculateImpactSummaries error = %v", err)
	}

	trash := summary[ReefCheckMetricID{Module: ReefCheckModuleImpact, MetricKey: "trash"}]
	if want := []int{0, 1, 2, 3}; !sameInts(trash.SegmentGrades, want) {
		t.Fatalf("SegmentGrades = %v, want %v", trash.SegmentGrades, want)
	}
	assertFloatNear(t, trash.Average, 1.5)
	assertFloatNear(t, trash.StandardDeviation, math.Sqrt(5.0/3.0))
	assertFloatNear(t, trash.StandardError, math.Sqrt(5.0/3.0)/2)
}

func TestImpactGradeBoundaries(t *testing.T) {
	tests := []struct {
		count int
		want  int
	}{
		{count: 0, want: 0},
		{count: 1, want: 1},
		{count: 2, want: 2},
		{count: 4, want: 2},
		{count: 5, want: 3},
		{count: 12, want: 3},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.count+'0')), func(t *testing.T) {
			got, err := ImpactGrade(tt.count)
			if err != nil {
				t.Fatalf("ImpactGrade error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ImpactGrade(%d) = %d, want %d", tt.count, got, tt.want)
			}
		})
	}

	if _, err := ImpactGrade(-1); !errors.Is(err, ErrValidation) {
		t.Fatalf("ImpactGrade(-1) error = %v, want %v", err, ErrValidation)
	}
}

func validReefCheckSurveyInput() ReefCheckSurveyInput {
	return ReefCheckSurveyInput{
		SurveyDate:     time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC),
		SiteID:         uuid.New(),
		DepthM:         5,
		FishLengthMode: ReefCheckFishLengthModeSeparate,
		Recorders: []ReefCheckRecorderInput{
			{Role: ReefCheckRecorderBenthos, RecorderName: "Benthos Recorder"},
			{Role: ReefCheckRecorderFish, RecorderName: "Fish Recorder"},
			{Role: ReefCheckRecorderInvertebrate, RecorderName: "Invert Recorder"},
		},
		Segments: []ReefCheckSegmentInput{
			{Index: 1, Label: "0-20m", StartM: 0, EndM: 20},
			{Index: 2, Label: "25-45m", StartM: 25, EndM: 45},
			{Index: 3, Label: "50-70m", StartM: 50, EndM: 70},
			{Index: 4, Label: "75-95m", StartM: 75, EndM: 95},
		},
		SubstratePoints: makeSubstratePoints(func(segment, point int) string {
			if point <= 10 {
				return "1"
			}
			if point <= 20 {
				return "2"
			}
			if point <= 25 {
				return "4"
			}
			if point <= 30 {
				return "6"
			}
			return "8"
		}),
		SubstrateBleaching: []SubstrateBleachingInput{
			{SegmentIndex: 1, HCBleachedCount: 0, SCBleachedCount: 0},
			{SegmentIndex: 2, HCBleachedCount: 0, SCBleachedCount: 0},
			{SegmentIndex: 3, HCBleachedCount: 1, SCBleachedCount: 0},
			{SegmentIndex: 4, HCBleachedCount: 0, SCBleachedCount: 1},
		},
		MetricCounts: validMetricCounts(),
	}
}

func makeSubstratePoints(codeFor func(segment, point int) string) []SubstratePointInput {
	points := make([]SubstratePointInput, 0, 160)
	for segment := 1; segment <= 4; segment++ {
		for point := 1; point <= 40; point++ {
			points = append(points, SubstratePointInput{
				SegmentIndex: segment,
				PointIndex:   point,
				Code:         codeFor(segment, point),
			})
		}
	}
	return points
}

func validMetricCounts() []ReefCheckMetricCountInput {
	metrics := []ReefCheckMetricID{
		{Module: ReefCheckModuleFish, MetricKey: "butterflyfish"},
		{Module: ReefCheckModuleFish, MetricKey: "grouper_30_40"},
		{Module: ReefCheckModuleInvertebrate, MetricKey: "lobster"},
		{Module: ReefCheckModuleImpact, MetricKey: "trash"},
		{Module: ReefCheckModuleRareOrganism, MetricKey: "turtles"},
	}
	counts := make([]ReefCheckMetricCountInput, 0, len(metrics)*4)
	for _, metric := range metrics {
		for segment := 1; segment <= 4; segment++ {
			counts = append(counts, ReefCheckMetricCountInput{
				Module:       metric.Module,
				MetricKey:    metric.MetricKey,
				SegmentIndex: segment,
				Count:        segment - 1,
			})
		}
	}
	return counts
}

func activePolicyVolunteer() policy.User {
	return policy.User{
		ID:     uuid.New(),
		Email:  "volunteer@example.com",
		Name:   "Volunteer",
		Role:   policy.RoleVolunteer,
		Status: policy.StatusActive,
	}
}

type fakeReefCheckSurveyRepository struct {
	created bool
	record  ReefCheckSurveyRecord
}

func (r *fakeReefCheckSurveyRepository) ListReefCheckSurveys(ctx context.Context) ([]ReefCheckSurvey, error) {
	return nil, nil
}

func (r *fakeReefCheckSurveyRepository) ListReefCheckSurveysByCreator(ctx context.Context, creatorID uuid.UUID) ([]ReefCheckSurvey, error) {
	return nil, nil
}

func (r *fakeReefCheckSurveyRepository) CreateReefCheckSurvey(ctx context.Context, record ReefCheckSurveyRecord) (ReefCheckSurvey, error) {
	r.created = true
	r.record = record
	return ReefCheckSurvey{
		ID:        uuid.New(),
		CreatedBy: record.CreatedBy,
		UpdatedBy: record.UpdatedBy,
	}, nil
}

func assertFloatNear(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("got %.12f, want %.12f", got, want)
	}
}

func sameInts(left, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
