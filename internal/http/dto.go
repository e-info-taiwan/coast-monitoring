package httpx

type CurrentUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type SessionResponse struct {
	Authenticated bool                 `json:"authenticated"`
	User          *CurrentUserResponse `json:"user,omitempty"`
	CSRFToken     string               `json:"csrfToken,omitempty"`
}

type AdminUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	HasGoogle   bool   `json:"hasGoogle"`
	HasPassword bool   `json:"hasPassword"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveUserRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Password string `json:"password"`
}

type CatalogResponse struct {
	ID          string `json:"id"`
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type SaveCatalogRequest struct {
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
}

type ObservationResponse struct {
	ID         string `json:"id"`
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	ObserverID string `json:"observerId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type AdminSaveObservationRequest struct {
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	ObserverID string `json:"observerId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
}

type AppObservationResponse struct {
	ID         string `json:"id"`
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type SaveObservationRequest struct {
	ObservedOn string `json:"observedOn"`
	LocationID string `json:"locationId"`
	SpeciesID  string `json:"speciesId"`
	Count      int    `json:"count"`
	Notes      string `json:"notes"`
}

type ReefCheckConfigResponse struct {
	Segments        []ReefCheckSegmentResponse       `json:"segments"`
	SubstrateCodes  []ReefCheckSubstrateCodeResponse `json:"substrateCodes"`
	Metrics         []ReefCheckMetricResponse        `json:"metrics"`
	FishLengthModes []string                         `json:"fishLengthModes"`
}

type ReefCheckSegmentResponse struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	StartM int    `json:"startM"`
	EndM   int    `json:"endM"`
}

type ReefCheckSubstrateCodeResponse struct {
	Code               string `json:"code"`
	DisplayName        string `json:"displayName"`
	NormalizedCategory string `json:"normalizedCategory"`
}

type ReefCheckMetricResponse struct {
	Module      string `json:"module"`
	Key         string `json:"key"`
	ChineseName string `json:"chineseName"`
	EnglishName string `json:"englishName"`
	SizeClass   string `json:"sizeClass"`
	SortOrder   int    `json:"sortOrder"`
}

type ReefCheckSurveyResponse struct {
	ID                  string   `json:"id"`
	SurveyDate          string   `json:"surveyDate"`
	SiteID              string   `json:"siteId"`
	DepthM              int      `json:"depthM"`
	CountryIsland       string   `json:"countryIsland"`
	TeamLeader          string   `json:"teamLeader"`
	SurveyTime          string   `json:"surveyTime"`
	Visibility          string   `json:"visibility"`
	Temperature         string   `json:"temperature"`
	GeneralComments     string   `json:"generalComments"`
	SubstrateComments   string   `json:"substrateComments"`
	RKCReason           string   `json:"rkcReason"`
	RKCBleachingPercent *float64 `json:"rkcBleachingPercent,omitempty"`
	FishLengthMode      string   `json:"fishLengthMode"`
	CreatedAt           string   `json:"createdAt"`
	UpdatedAt           string   `json:"updatedAt"`
}

type SaveReefCheckSurveyRequest struct {
	SurveyDate          string                        `json:"surveyDate"`
	SiteID              string                        `json:"siteId"`
	Site                ReefCheckSiteRequest          `json:"site"`
	DepthM              int                           `json:"depthM"`
	CountryIsland       string                        `json:"countryIsland"`
	TeamLeader          string                        `json:"teamLeader"`
	SurveyTime          string                        `json:"surveyTime"`
	Visibility          string                        `json:"visibility"`
	Temperature         string                        `json:"temperature"`
	GeneralComments     string                        `json:"generalComments"`
	SubstrateComments   string                        `json:"substrateComments"`
	RKCReason           string                        `json:"rkcReason"`
	RKCBleachingPercent *float64                      `json:"rkcBleachingPercent"`
	FishLengthMode      string                        `json:"fishLengthMode"`
	Recorders           []ReefCheckRecorderRequest    `json:"recorders"`
	Segments            []ReefCheckSegmentRequest     `json:"segments"`
	SubstratePoints     []SubstratePointRequest       `json:"substratePoints"`
	SubstrateBleaching  []SubstrateBleachingRequest   `json:"substrateBleaching"`
	MetricCounts        []ReefCheckMetricCountRequest `json:"metricCounts"`
}

type ReefCheckSiteRequest struct {
	County          string  `json:"county"`
	LocationName    string  `json:"locationName"`
	SiteName        string  `json:"siteName"`
	SiteEnglishName string  `json:"siteEnglishName"`
	Latitude        float64 `json:"latitude"`
	Longitude       float64 `json:"longitude"`
}

type ReefCheckRecorderRequest struct {
	Role         string `json:"role"`
	UserID       string `json:"userId"`
	RecorderName string `json:"recorderName"`
}

type ReefCheckSegmentRequest struct {
	Index  int    `json:"index"`
	Label  string `json:"label"`
	StartM int    `json:"startM"`
	EndM   int    `json:"endM"`
}

type SubstratePointRequest struct {
	SegmentIndex int     `json:"segmentIndex"`
	PointIndex   int     `json:"pointIndex"`
	TransectM    float64 `json:"transectM"`
	Code         string  `json:"code"`
}

type SubstrateBleachingRequest struct {
	SegmentIndex    int `json:"segmentIndex"`
	HCBleachedCount int `json:"hcBleachedCount"`
	SCBleachedCount int `json:"scBleachedCount"`
}

type ReefCheckMetricCountRequest struct {
	Module       string `json:"module"`
	MetricKey    string `json:"metricKey"`
	SegmentIndex int    `json:"segmentIndex"`
	Count        int    `json:"count"`
	Comment      string `json:"comment"`
}

type AuditLogResponse struct {
	ID          string `json:"id"`
	Action      string `json:"action"`
	TargetTable string `json:"targetTable"`
	TargetID    string `json:"targetId"`
	ActorUserID string `json:"actorUserId,omitempty"`
	ActorEmail  string `json:"actorEmail"`
	BeforeData  any    `json:"beforeData,omitempty"`
	AfterData   any    `json:"afterData,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	IP          string `json:"ip"`
	UserAgent   string `json:"userAgent"`
	LoggedAt    string `json:"loggedAt"`
}
