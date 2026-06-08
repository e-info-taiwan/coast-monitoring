package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"coast-monitoring/internal/policy"

	"github.com/google/uuid"
)

const (
	reefCheckSegmentCount              = 4
	reefCheckSubstratePointsPerSegment = 40
	reefCheckRKCReasonThresholdPoints  = 16
)

type ReefCheckRecorderRole string

const (
	ReefCheckRecorderBenthos      ReefCheckRecorderRole = "benthos"
	ReefCheckRecorderFish         ReefCheckRecorderRole = "fish"
	ReefCheckRecorderInvertebrate ReefCheckRecorderRole = "invertebrate"
)

type ReefCheckFishLengthMode string

const (
	ReefCheckFishLengthModeSeparate ReefCheckFishLengthMode = "separate"
	ReefCheckFishLengthModeCombined ReefCheckFishLengthMode = "combined"
)

type ReefCheckModule string

const (
	ReefCheckModuleFish         ReefCheckModule = "fish"
	ReefCheckModuleInvertebrate ReefCheckModule = "invertebrate"
	ReefCheckModuleImpact       ReefCheckModule = "impact"
	ReefCheckModuleRareOrganism ReefCheckModule = "rare_organism"
)

type ReefCheckSurvey struct {
	ID                  uuid.UUID
	SurveyDate          time.Time
	SiteID              uuid.UUID
	DepthM              int
	CountryIsland       string
	TeamLeader          string
	SurveyTime          string
	Visibility          string
	Temperature         string
	GeneralComments     string
	SubstrateComments   string
	RKCReason           string
	RKCBleachingPercent *float64
	FishLengthMode      ReefCheckFishLengthMode
	CreatedBy           uuid.UUID
	UpdatedBy           uuid.UUID
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type ReefCheckSurveyInput struct {
	SurveyDate          time.Time
	SiteID              uuid.UUID
	Site                ReefCheckSiteInput
	DepthM              int
	CountryIsland       string
	TeamLeader          string
	SurveyTime          string
	Visibility          string
	Temperature         string
	GeneralComments     string
	SubstrateComments   string
	RKCReason           string
	RKCBleachingPercent *float64
	FishLengthMode      ReefCheckFishLengthMode
	Recorders           []ReefCheckRecorderInput
	Segments            []ReefCheckSegmentInput
	SubstratePoints     []SubstratePointInput
	SubstrateBleaching  []SubstrateBleachingInput
	MetricCounts        []ReefCheckMetricCountInput
}

type ReefCheckSurveyRecord struct {
	SurveyDate          time.Time
	SiteID              uuid.UUID
	Site                ReefCheckSiteInput
	DepthM              int
	CountryIsland       string
	TeamLeader          string
	SurveyTime          string
	Visibility          string
	Temperature         string
	GeneralComments     string
	SubstrateComments   string
	RKCReason           string
	RKCBleachingPercent *float64
	FishLengthMode      ReefCheckFishLengthMode
	Recorders           []ReefCheckRecorderInput
	Segments            []ReefCheckSegmentInput
	SubstratePoints     []SubstratePointInput
	SubstrateBleaching  []SubstrateBleachingInput
	MetricCounts        []ReefCheckMetricCountInput
	CreatedBy           uuid.UUID
	UpdatedBy           uuid.UUID
}

type ReefCheckRecorderInput struct {
	Role         ReefCheckRecorderRole
	UserID       uuid.UUID
	RecorderName string
}

type ReefCheckSiteInput struct {
	County          string
	LocationName    string
	SiteName        string
	SiteEnglishName string
	Latitude        float64
	Longitude       float64
}

type ReefCheckSegmentInput struct {
	Index  int
	Label  string
	StartM int
	EndM   int
}

type SubstratePointInput struct {
	SegmentIndex int
	PointIndex   int
	TransectM    float64
	Code         string
}

type SubstrateBleachingInput struct {
	SegmentIndex    int
	HCBleachedCount int
	SCBleachedCount int
}

type ReefCheckMetricID struct {
	Module    ReefCheckModule
	MetricKey string
}

type ReefCheckMetricCountInput struct {
	Module       ReefCheckModule
	MetricKey    string
	SegmentIndex int
	Count        int
	Comment      string
}

type ReefCheckSubstrateCode struct {
	Code               string
	DisplayName        string
	NormalizedCategory string
}

type ReefCheckMetricCatalogItem struct {
	Module      ReefCheckModule
	Key         string
	ChineseName string
	EnglishName string
	SizeClass   string
	SortOrder   int
}

type ReefCheckSurveyRepository interface {
	ListReefCheckSurveys(ctx context.Context) ([]ReefCheckSurvey, error)
	ListReefCheckSurveysByCreator(ctx context.Context, creatorID uuid.UUID) ([]ReefCheckSurvey, error)
	CreateReefCheckSurvey(ctx context.Context, record ReefCheckSurveyRecord) (ReefCheckSurvey, error)
}

type ReefCheckSurveyService struct {
	Surveys ReefCheckSurveyRepository
}

func (s ReefCheckSurveyService) Create(ctx context.Context, actor policy.User, input ReefCheckSurveyInput) (ReefCheckSurvey, error) {
	if !policy.CanUseAppAPI(actor) {
		return ReefCheckSurvey{}, ErrForbidden
	}
	record, err := validateReefCheckSurveyInput(actor, input)
	if err != nil {
		return ReefCheckSurvey{}, err
	}
	return s.Surveys.CreateReefCheckSurvey(ctx, record)
}

func (s ReefCheckSurveyService) ListForApp(ctx context.Context, actor policy.User) ([]ReefCheckSurvey, error) {
	if !policy.CanUseAppAPI(actor) {
		return nil, ErrForbidden
	}
	if actor.Role == policy.RoleVolunteer {
		return s.Surveys.ListReefCheckSurveysByCreator(ctx, actor.ID)
	}
	return s.Surveys.ListReefCheckSurveys(ctx)
}

func validateReefCheckSurveyInput(actor policy.User, input ReefCheckSurveyInput) (ReefCheckSurveyRecord, error) {
	if input.SurveyDate.IsZero() {
		return ReefCheckSurveyRecord{}, fmt.Errorf("%w: survey date is required", ErrValidation)
	}
	site, err := validateReefCheckSite(input.SiteID, input.Site)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	if input.DepthM <= 0 {
		return ReefCheckSurveyRecord{}, fmt.Errorf("%w: depth must be greater than zero", ErrValidation)
	}
	if !validFishLengthMode(input.FishLengthMode) {
		return ReefCheckSurveyRecord{}, fmt.Errorf("%w: fish length mode is invalid", ErrValidation)
	}
	recorders, err := validateReefCheckRecorders(input.Recorders)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	segments, err := validateReefCheckSegments(input.Segments)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	points, rkcPoints, err := validateSubstratePoints(input.SubstratePoints)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	bleaching, err := validateSubstrateBleaching(input.SubstrateBleaching)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	counts, err := validateReefCheckMetricCounts(input.MetricCounts)
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	rkcReason := strings.TrimSpace(input.RKCReason)
	if rkcPoints >= reefCheckRKCReasonThresholdPoints && rkcReason == "" {
		return ReefCheckSurveyRecord{}, fmt.Errorf("%w: rkc reason is required when RKC coverage is at least 10%%", ErrValidation)
	}
	rkcBleachingPercent, err := validateOptionalPercent(input.RKCBleachingPercent, "rkc bleaching percent")
	if err != nil {
		return ReefCheckSurveyRecord{}, err
	}
	return ReefCheckSurveyRecord{
		SurveyDate:          input.SurveyDate,
		SiteID:              input.SiteID,
		Site:                site,
		DepthM:              input.DepthM,
		CountryIsland:       strings.TrimSpace(input.CountryIsland),
		TeamLeader:          strings.TrimSpace(input.TeamLeader),
		SurveyTime:          strings.TrimSpace(input.SurveyTime),
		Visibility:          strings.TrimSpace(input.Visibility),
		Temperature:         strings.TrimSpace(input.Temperature),
		GeneralComments:     strings.TrimSpace(input.GeneralComments),
		SubstrateComments:   strings.TrimSpace(input.SubstrateComments),
		RKCReason:           rkcReason,
		RKCBleachingPercent: rkcBleachingPercent,
		FishLengthMode:      input.FishLengthMode,
		Recorders:           recorders,
		Segments:            segments,
		SubstratePoints:     points,
		SubstrateBleaching:  bleaching,
		MetricCounts:        counts,
		CreatedBy:           actor.ID,
		UpdatedBy:           actor.ID,
	}, nil
}

func validateOptionalPercent(value *float64, label string) (*float64, error) {
	if value == nil {
		return nil, nil
	}
	if *value < 0 || *value > 100 {
		return nil, fmt.Errorf("%w: %s must be between 0 and 100", ErrValidation, label)
	}
	normalized := *value
	return &normalized, nil
}

func validateReefCheckSite(siteID uuid.UUID, input ReefCheckSiteInput) (ReefCheckSiteInput, error) {
	if siteID != uuid.Nil {
		return input, nil
	}
	site := ReefCheckSiteInput{
		County:          strings.TrimSpace(input.County),
		LocationName:    strings.TrimSpace(input.LocationName),
		SiteName:        strings.TrimSpace(input.SiteName),
		SiteEnglishName: strings.TrimSpace(input.SiteEnglishName),
		Latitude:        input.Latitude,
		Longitude:       input.Longitude,
	}
	if site.County == "" {
		return ReefCheckSiteInput{}, fmt.Errorf("%w: county is required", ErrValidation)
	}
	if site.LocationName == "" {
		return ReefCheckSiteInput{}, fmt.Errorf("%w: location name is required", ErrValidation)
	}
	if site.SiteName == "" {
		return ReefCheckSiteInput{}, fmt.Errorf("%w: site name is required", ErrValidation)
	}
	if site.SiteEnglishName == "" {
		return ReefCheckSiteInput{}, fmt.Errorf("%w: site english name is required", ErrValidation)
	}
	if site.Latitude == 0 || site.Longitude == 0 {
		return ReefCheckSiteInput{}, fmt.Errorf("%w: latitude and longitude are required", ErrValidation)
	}
	return site, nil
}

func validateReefCheckRecorders(input []ReefCheckRecorderInput) ([]ReefCheckRecorderInput, error) {
	required := map[ReefCheckRecorderRole]bool{
		ReefCheckRecorderBenthos:      false,
		ReefCheckRecorderFish:         false,
		ReefCheckRecorderInvertebrate: false,
	}
	recorders := make([]ReefCheckRecorderInput, 0, len(input))
	for _, recorder := range input {
		if _, ok := required[recorder.Role]; !ok {
			return nil, fmt.Errorf("%w: recorder role is invalid", ErrValidation)
		}
		required[recorder.Role] = true
		recorder.RecorderName = strings.TrimSpace(recorder.RecorderName)
		recorders = append(recorders, recorder)
	}
	for role, seen := range required {
		if !seen {
			return nil, fmt.Errorf("%w: %s recorder is required", ErrValidation, role)
		}
	}
	return recorders, nil
}

func validateReefCheckSegments(input []ReefCheckSegmentInput) ([]ReefCheckSegmentInput, error) {
	if len(input) != reefCheckSegmentCount {
		return nil, fmt.Errorf("%w: exactly four segments are required", ErrValidation)
	}
	seen := make(map[int]bool, reefCheckSegmentCount)
	segments := make([]ReefCheckSegmentInput, 0, len(input))
	for _, segment := range input {
		if !validSegmentIndex(segment.Index) {
			return nil, fmt.Errorf("%w: segment index is invalid", ErrValidation)
		}
		if seen[segment.Index] {
			return nil, fmt.Errorf("%w: duplicate segment index", ErrValidation)
		}
		if strings.TrimSpace(segment.Label) == "" {
			return nil, fmt.Errorf("%w: segment label is required", ErrValidation)
		}
		if segment.EndM <= segment.StartM {
			return nil, fmt.Errorf("%w: segment end must be greater than start", ErrValidation)
		}
		seen[segment.Index] = true
		segment.Label = strings.TrimSpace(segment.Label)
		segments = append(segments, segment)
	}
	return segments, nil
}

func validateSubstratePoints(input []SubstratePointInput) ([]SubstratePointInput, int, error) {
	if len(input) != reefCheckSegmentCount*reefCheckSubstratePointsPerSegment {
		return nil, 0, fmt.Errorf("%w: exactly 160 substrate points are required", ErrValidation)
	}
	seen := make(map[[2]int]bool, len(input))
	points := make([]SubstratePointInput, 0, len(input))
	rkcPoints := 0
	for _, point := range input {
		if !validSegmentIndex(point.SegmentIndex) {
			return nil, 0, fmt.Errorf("%w: substrate segment index is invalid", ErrValidation)
		}
		if point.PointIndex < 1 || point.PointIndex > reefCheckSubstratePointsPerSegment {
			return nil, 0, fmt.Errorf("%w: substrate point index is invalid", ErrValidation)
		}
		transectM, err := substrateTransectMeter(point.SegmentIndex, point.PointIndex)
		if err != nil {
			return nil, 0, err
		}
		if point.TransectM != 0 && math.Abs(point.TransectM-transectM) > 0.0001 {
			return nil, 0, fmt.Errorf("%w: substrate transect meter is invalid", ErrValidation)
		}
		point.TransectM = transectM
		point.Code = strings.TrimSpace(point.Code)
		category, ok := substrateCodeCategories[point.Code]
		if !ok {
			return nil, 0, fmt.Errorf("%w: substrate code is invalid", ErrValidation)
		}
		key := [2]int{point.SegmentIndex, point.PointIndex}
		if seen[key] {
			return nil, 0, fmt.Errorf("%w: duplicate substrate point", ErrValidation)
		}
		seen[key] = true
		if category == "RKC" {
			rkcPoints++
		}
		points = append(points, point)
	}
	return points, rkcPoints, nil
}

func validateSubstrateBleaching(input []SubstrateBleachingInput) ([]SubstrateBleachingInput, error) {
	if len(input) != reefCheckSegmentCount {
		return nil, fmt.Errorf("%w: exactly four substrate bleaching rows are required", ErrValidation)
	}
	seen := make(map[int]bool, reefCheckSegmentCount)
	bleaching := make([]SubstrateBleachingInput, 0, len(input))
	for _, row := range input {
		if !validSegmentIndex(row.SegmentIndex) {
			return nil, fmt.Errorf("%w: bleaching segment index is invalid", ErrValidation)
		}
		if row.HCBleachedCount < 0 || row.SCBleachedCount < 0 {
			return nil, fmt.Errorf("%w: bleaching counts must be zero or greater", ErrValidation)
		}
		if seen[row.SegmentIndex] {
			return nil, fmt.Errorf("%w: duplicate bleaching segment", ErrValidation)
		}
		seen[row.SegmentIndex] = true
		bleaching = append(bleaching, row)
	}
	return bleaching, nil
}

func validateReefCheckMetricCounts(input []ReefCheckMetricCountInput) ([]ReefCheckMetricCountInput, error) {
	counts := make([]ReefCheckMetricCountInput, 0, len(input))
	for _, count := range input {
		if !validReefCheckModule(count.Module) {
			return nil, fmt.Errorf("%w: metric module is invalid", ErrValidation)
		}
		if strings.TrimSpace(count.MetricKey) == "" {
			return nil, fmt.Errorf("%w: metric key is required", ErrValidation)
		}
		if !validSegmentIndex(count.SegmentIndex) {
			return nil, fmt.Errorf("%w: metric segment index is invalid", ErrValidation)
		}
		if count.Count < 0 {
			return nil, fmt.Errorf("%w: metric count must be zero or greater", ErrValidation)
		}
		count.MetricKey = strings.TrimSpace(count.MetricKey)
		count.Comment = strings.TrimSpace(count.Comment)
		counts = append(counts, count)
	}
	return counts, nil
}

type SubstrateSummary struct {
	Categories            map[string]SubstrateCategorySummary
	LiveCoralCoverPercent float64
}

type SubstrateCategorySummary struct {
	CoveragePercent        float64
	SegmentCoveragePercent []float64
	StandardDeviation      float64
	StandardError          float64
}

func CalculateSubstrateSummary(points []SubstratePointInput) (SubstrateSummary, error) {
	validated, _, err := validateSubstratePoints(points)
	if err != nil {
		return SubstrateSummary{}, err
	}
	segmentCounts := map[string][]float64{}
	for _, category := range substrateBaseCategories {
		segmentCounts[category] = make([]float64, reefCheckSegmentCount)
	}
	for _, point := range validated {
		category := substrateCodeCategories[point.Code]
		segmentCounts[category][point.SegmentIndex-1]++
	}
	categories := make(map[string]SubstrateCategorySummary, len(segmentCounts))
	for category, counts := range segmentCounts {
		coverage := make([]float64, reefCheckSegmentCount)
		total := 0.0
		for i, count := range counts {
			coverage[i] = count / float64(reefCheckSubstratePointsPerSegment) * 100
			total += count
		}
		_, sd, se := sampleStats(coverage)
		categories[category] = SubstrateCategorySummary{
			CoveragePercent:        total / float64(reefCheckSegmentCount*reefCheckSubstratePointsPerSegment) * 100,
			SegmentCoveragePercent: coverage,
			StandardDeviation:      sd,
			StandardError:          se,
		}
	}
	return SubstrateSummary{
		Categories:            categories,
		LiveCoralCoverPercent: categories["HC"].CoveragePercent + categories["SC"].CoveragePercent,
	}, nil
}

type MetricSummary struct {
	SegmentCounts     []int
	Average           float64
	StandardDeviation float64
	StandardError     float64
}

type ImpactSummary struct {
	SegmentGrades     []int
	Average           float64
	StandardDeviation float64
	StandardError     float64
}

func CalculateMetricSummaries(counts []ReefCheckMetricCountInput) (map[ReefCheckMetricID]MetricSummary, error) {
	validated, err := validateReefCheckMetricCounts(counts)
	if err != nil {
		return nil, err
	}
	grouped := make(map[ReefCheckMetricID][]int)
	for _, count := range validated {
		if count.Module == ReefCheckModuleImpact {
			continue
		}
		id := ReefCheckMetricID{Module: count.Module, MetricKey: count.MetricKey}
		if grouped[id] == nil {
			grouped[id] = make([]int, reefCheckSegmentCount)
		}
		grouped[id][count.SegmentIndex-1] = count.Count
	}
	summaries := make(map[ReefCheckMetricID]MetricSummary, len(grouped))
	for id, segmentCounts := range grouped {
		values := intsToFloats(segmentCounts)
		avg, sd, se := sampleStats(values)
		summaries[id] = MetricSummary{
			SegmentCounts:     segmentCounts,
			Average:           avg,
			StandardDeviation: sd,
			StandardError:     se,
		}
	}
	return summaries, nil
}

func CalculateImpactSummaries(counts []ReefCheckMetricCountInput) (map[ReefCheckMetricID]ImpactSummary, error) {
	validated, err := validateReefCheckMetricCounts(counts)
	if err != nil {
		return nil, err
	}
	grouped := make(map[ReefCheckMetricID][]int)
	for _, count := range validated {
		if count.Module != ReefCheckModuleImpact {
			continue
		}
		grade, err := ImpactGrade(count.Count)
		if err != nil {
			return nil, err
		}
		id := ReefCheckMetricID{Module: count.Module, MetricKey: count.MetricKey}
		if grouped[id] == nil {
			grouped[id] = make([]int, reefCheckSegmentCount)
		}
		grouped[id][count.SegmentIndex-1] = grade
	}
	summaries := make(map[ReefCheckMetricID]ImpactSummary, len(grouped))
	for id, grades := range grouped {
		values := intsToFloats(grades)
		avg, sd, se := sampleStats(values)
		summaries[id] = ImpactSummary{
			SegmentGrades:     grades,
			Average:           avg,
			StandardDeviation: sd,
			StandardError:     se,
		}
	}
	return summaries, nil
}

func ImpactGrade(count int) (int, error) {
	if count < 0 {
		return 0, fmt.Errorf("%w: impact count must be zero or greater", ErrValidation)
	}
	switch {
	case count == 0:
		return 0, nil
	case count == 1:
		return 1, nil
	case count <= 4:
		return 2, nil
	default:
		return 3, nil
	}
}

func validSegmentIndex(index int) bool {
	return index >= 1 && index <= reefCheckSegmentCount
}

func substrateTransectMeter(segmentIndex, pointIndex int) (float64, error) {
	if !validSegmentIndex(segmentIndex) || pointIndex < 1 || pointIndex > reefCheckSubstratePointsPerSegment {
		return 0, fmt.Errorf("%w: substrate transect meter index is invalid", ErrValidation)
	}
	starts := map[int]float64{1: 0, 2: 25, 3: 50, 4: 75}
	return starts[segmentIndex] + float64(pointIndex-1)*0.5, nil
}

func validFishLengthMode(mode ReefCheckFishLengthMode) bool {
	return mode == ReefCheckFishLengthModeSeparate || mode == ReefCheckFishLengthModeCombined
}

func validReefCheckModule(module ReefCheckModule) bool {
	return module == ReefCheckModuleFish ||
		module == ReefCheckModuleInvertebrate ||
		module == ReefCheckModuleImpact ||
		module == ReefCheckModuleRareOrganism
}

func sampleStats(values []float64) (float64, float64, float64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	avg := sum / float64(len(values))
	if len(values) == 1 {
		return avg, 0, 0
	}
	var squaredDiffs float64
	for _, value := range values {
		diff := value - avg
		squaredDiffs += diff * diff
	}
	sd := math.Sqrt(squaredDiffs / float64(len(values)-1))
	return avg, sd, sd / math.Sqrt(float64(len(values)))
}

func intsToFloats(values []int) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = float64(value)
	}
	return out
}

var substrateBaseCategories = []string{"HC", "SC", "RKC", "NIA", "SP", "RC", "RB", "SD", "SI", "OT"}

var substrateCodeCategories = map[string]string{
	"1":  "HC",
	"2":  "SC",
	"3":  "RKC",
	"4":  "NIA",
	"5":  "SP",
	"6":  "RC",
	"7":  "RB",
	"8":  "SD",
	"9":  "SI",
	"0":  "OT",
	"a":  "HC",
	"b":  "HC",
	"c":  "HC",
	"91": "HC",
	"92": "SC",
	"93": "RKC",
	"94": "NIA",
	"95": "SP",
	"96": "RC",
	"97": "RB",
	"98": "SD",
}

func ReefCheckDefaultSegments() []ReefCheckSegmentInput {
	return []ReefCheckSegmentInput{
		{Index: 1, Label: "0-20m", StartM: 0, EndM: 20},
		{Index: 2, Label: "25-45m", StartM: 25, EndM: 45},
		{Index: 3, Label: "50-70m", StartM: 50, EndM: 70},
		{Index: 4, Label: "75-95m", StartM: 75, EndM: 95},
	}
}

func ReefCheckSubstrateCodeCatalog() []ReefCheckSubstrateCode {
	return []ReefCheckSubstrateCode{
		{Code: "1", DisplayName: "硬珊瑚", NormalizedCategory: "HC"},
		{Code: "2", DisplayName: "軟珊瑚", NormalizedCategory: "SC"},
		{Code: "3", DisplayName: "新死珊瑚", NormalizedCategory: "RKC"},
		{Code: "4", DisplayName: "藻類", NormalizedCategory: "NIA"},
		{Code: "5", DisplayName: "海綿", NormalizedCategory: "SP"},
		{Code: "6", DisplayName: "岩石", NormalizedCategory: "RC"},
		{Code: "7", DisplayName: "碎石", NormalizedCategory: "RB"},
		{Code: "8", DisplayName: "沙", NormalizedCategory: "SD"},
		{Code: "9", DisplayName: "泥沙", NormalizedCategory: "SI"},
		{Code: "0", DisplayName: "其他", NormalizedCategory: "OT"},
		{Code: "a", DisplayName: "硬珊瑚 -a", NormalizedCategory: "HC"},
		{Code: "b", DisplayName: "硬珊瑚 -b", NormalizedCategory: "HC"},
		{Code: "c", DisplayName: "硬珊瑚 -c", NormalizedCategory: "HC"},
		{Code: "91", DisplayName: "泥沙 (硬珊瑚)", NormalizedCategory: "HC"},
		{Code: "92", DisplayName: "泥沙 (軟珊瑚)", NormalizedCategory: "SC"},
		{Code: "93", DisplayName: "泥沙 (新死珊瑚)", NormalizedCategory: "RKC"},
		{Code: "94", DisplayName: "泥沙 (藻類)", NormalizedCategory: "NIA"},
		{Code: "95", DisplayName: "泥沙 (海綿)", NormalizedCategory: "SP"},
		{Code: "96", DisplayName: "泥沙 (岩石)", NormalizedCategory: "RC"},
		{Code: "97", DisplayName: "泥沙 (碎石)", NormalizedCategory: "RB"},
		{Code: "98", DisplayName: "泥沙 (沙)", NormalizedCategory: "SD"},
	}
}

func ReefCheckMetricCatalog() []ReefCheckMetricCatalogItem {
	return []ReefCheckMetricCatalogItem{
		{Module: ReefCheckModuleFish, Key: "butterflyfish", ChineseName: "蝶魚", EnglishName: "Butterflyfish", SortOrder: 10},
		{Module: ReefCheckModuleFish, Key: "butterflyfish_less5", ChineseName: "蝶魚 <5", EnglishName: "Butterflyfish", SizeClass: "<5", SortOrder: 20},
		{Module: ReefCheckModuleFish, Key: "sweetlips", ChineseName: "石鱸", EnglishName: "Sweetlips", SortOrder: 30},
		{Module: ReefCheckModuleFish, Key: "sweetlips_juv", ChineseName: "石鱸 juvenile", EnglishName: "Sweetlips juvenile", SizeClass: "juvenile", SortOrder: 40},
		{Module: ReefCheckModuleFish, Key: "snapper", ChineseName: "笛鯛", EnglishName: "Snapper", SortOrder: 50},
		{Module: ReefCheckModuleFish, Key: "snapper_less20", ChineseName: "笛鯛 <20", EnglishName: "Snapper", SizeClass: "<20", SortOrder: 60},
		{Module: ReefCheckModuleFish, Key: "barramundi_cod", ChineseName: "老鼠斑", EnglishName: "Barramundi cod", SortOrder: 70},
		{Module: ReefCheckModuleFish, Key: "humphead_wrasse", ChineseName: "蘇眉", EnglishName: "Humphead wrasse", SortOrder: 80},
		{Module: ReefCheckModuleFish, Key: "bumphead_parrotfish", ChineseName: "隆頭鸚哥", EnglishName: "Bumphead parrotfish", SortOrder: 90},
		{Module: ReefCheckModuleFish, Key: "other_parrotfish", ChineseName: "鸚哥魚", EnglishName: "Other parrotfish", SortOrder: 100},
		{Module: ReefCheckModuleFish, Key: "other_parrotfish_less20", ChineseName: "鸚哥魚 <20", EnglishName: "Other parrotfish", SizeClass: "<20", SortOrder: 110},
		{Module: ReefCheckModuleFish, Key: "moray_eel", ChineseName: "裸胸鯙", EnglishName: "Moray eel", SortOrder: 120},
		{Module: ReefCheckModuleFish, Key: "grouper_less30", ChineseName: "石斑魚 <30", EnglishName: "Grouper", SizeClass: "<30", SortOrder: 130},
		{Module: ReefCheckModuleFish, Key: "grouper_30_40", ChineseName: "石斑魚 30-40", EnglishName: "Grouper", SizeClass: "30-40", SortOrder: 140},
		{Module: ReefCheckModuleFish, Key: "grouper_40_50", ChineseName: "石斑魚 40-50", EnglishName: "Grouper", SizeClass: "40-50", SortOrder: 150},
		{Module: ReefCheckModuleFish, Key: "grouper_50_60", ChineseName: "石斑魚 50-60", EnglishName: "Grouper", SizeClass: "50-60", SortOrder: 160},
		{Module: ReefCheckModuleFish, Key: "grouper_60", ChineseName: "石斑魚 >60", EnglishName: "Grouper", SizeClass: ">60", SortOrder: 170},
		{Module: ReefCheckModuleFish, Key: "others", ChineseName: "其他魚類", EnglishName: "Other fish", SortOrder: 180},

		{Module: ReefCheckModuleInvertebrate, Key: "banded_coral_shrimp", ChineseName: "珊瑚蝦", EnglishName: "Banded coral shrimp", SortOrder: 10},
		{Module: ReefCheckModuleInvertebrate, Key: "diadema", ChineseName: "魔鬼海膽 Diadema", EnglishName: "Diadema", SortOrder: 20},
		{Module: ReefCheckModuleInvertebrate, Key: "echinothrix", ChineseName: "刺冠海膽 Echinothrix", EnglishName: "Echinothrix", SortOrder: 30},
		{Module: ReefCheckModuleInvertebrate, Key: "pencil_urchin", ChineseName: "鉛筆海膽", EnglishName: "Pencil urchin", SortOrder: 40},
		{Module: ReefCheckModuleInvertebrate, Key: "collector_urchin", ChineseName: "收集海膽", EnglishName: "Collector urchin", SortOrder: 50},
		{Module: ReefCheckModuleInvertebrate, Key: "seacucumber", ChineseName: "海參", EnglishName: "Sea cucumber", SortOrder: 60},
		{Module: ReefCheckModuleInvertebrate, Key: "crown_of_thorns", ChineseName: "棘冠海星", EnglishName: "Crown of thorns", SortOrder: 70},
		{Module: ReefCheckModuleInvertebrate, Key: "triton", ChineseName: "法螺", EnglishName: "Triton", SortOrder: 80},
		{Module: ReefCheckModuleInvertebrate, Key: "lobster", ChineseName: "龍蝦", EnglishName: "Lobster", SortOrder: 90},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_less10", ChineseName: "硨磲貝 <10", EnglishName: "Giant clam", SizeClass: "<10", SortOrder: 100},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_10_20", ChineseName: "硨磲貝 10-20", EnglishName: "Giant clam", SizeClass: "10-20", SortOrder: 110},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_20_30", ChineseName: "硨磲貝 20-30", EnglishName: "Giant clam", SizeClass: "20-30", SortOrder: 120},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_30_40", ChineseName: "硨磲貝 30-40", EnglishName: "Giant clam", SizeClass: "30-40", SortOrder: 130},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_40_50", ChineseName: "硨磲貝 40-50", EnglishName: "Giant clam", SizeClass: "40-50", SortOrder: 140},
		{Module: ReefCheckModuleInvertebrate, Key: "giantclam_50", ChineseName: "硨磲貝 >50", EnglishName: "Giant clam", SizeClass: ">50", SortOrder: 150},

		{Module: ReefCheckModuleImpact, Key: "boat_anchor", ChineseName: "船錨", EnglishName: "Boat anchor", SortOrder: 10},
		{Module: ReefCheckModuleImpact, Key: "dynamite", ChineseName: "炸魚", EnglishName: "Dynamite", SortOrder: 20},
		{Module: ReefCheckModuleImpact, Key: "other_coral_damage", ChineseName: "其他珊瑚損害", EnglishName: "Other coral damage", SortOrder: 30},
		{Module: ReefCheckModuleImpact, Key: "fishnet", ChineseName: "漁網", EnglishName: "Fishnet", SortOrder: 40},
		{Module: ReefCheckModuleImpact, Key: "trash", ChineseName: "垃圾", EnglishName: "Trash", SortOrder: 50},
		{Module: ReefCheckModuleImpact, Key: "general_trash", ChineseName: "一般垃圾", EnglishName: "General trash", SortOrder: 60},
		{Module: ReefCheckModuleImpact, Key: "bleaching_population_percent", ChineseName: "白化比例 population", EnglishName: "Bleaching population percent", SortOrder: 70},
		{Module: ReefCheckModuleImpact, Key: "bleaching_colony_percent", ChineseName: "白化比例 colony", EnglishName: "Bleaching colony percent", SortOrder: 80},
		{Module: ReefCheckModuleImpact, Key: "disease_coral_black_band", ChineseName: "黑帶病", EnglishName: "Black band disease", SortOrder: 90},
		{Module: ReefCheckModuleImpact, Key: "disease_coral_white_band", ChineseName: "白帶病", EnglishName: "White band disease", SortOrder: 100},

		{Module: ReefCheckModuleRareOrganism, Key: "sharks", ChineseName: "鯊魚", EnglishName: "Sharks", SortOrder: 10},
		{Module: ReefCheckModuleRareOrganism, Key: "turtles", ChineseName: "海龜", EnglishName: "Turtles", SortOrder: 20},
		{Module: ReefCheckModuleRareOrganism, Key: "mantas", ChineseName: "鬼蝠魟", EnglishName: "Mantas", SortOrder: 30},
		{Module: ReefCheckModuleRareOrganism, Key: "other", ChineseName: "其他罕見生物", EnglishName: "Other rare organism", SortOrder: 40},
	}
}
