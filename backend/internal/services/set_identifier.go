package services

// SetIDCandidate represents a possible set match
type SetIDCandidate struct {
	SetID string  `json:"set_id"`
	Score float64 `json:"score"`
}

// SetIDResult represents the result of set identification
type SetIDResult struct {
	BestSetID     string           `json:"best_set_id"`
	Confidence    float64          `json:"confidence"`
	LowConfidence bool             `json:"low_confidence"`
	Candidates    []SetIDCandidate `json:"candidates"`
	TimingsMS     map[string]int   `json:"timings_ms"`
}

// SetIdentifierService is a stub - the Python identifier service has been removed.
// Set identification now relies on OCR text parsing and set total inference.
type SetIdentifierService struct{}

// NewSetIdentifierService creates a stub service (does nothing)
func NewSetIdentifierService() *SetIdentifierService {
	return &SetIdentifierService{}
}

// IsConfigured always returns false - service has been removed
func (s *SetIdentifierService) IsConfigured() bool {
	return false
}

// IsHealthy always returns false - service has been removed
func (s *SetIdentifierService) IsHealthy() bool {
	return false
}

// GamesLoaded returns an empty list - service has been removed
func (s *SetIdentifierService) GamesLoaded() []string {
	return nil
}
