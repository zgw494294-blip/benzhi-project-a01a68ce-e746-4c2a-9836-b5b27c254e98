package domain

import "time"

type SessionStatus string

const (
	StatusDraft      SessionStatus = "draft"
	StatusFrozen     SessionStatus = "frozen"
	StatusCollecting SessionStatus = "collecting"
	StatusVerifying  SessionStatus = "verifying"
	StatusRevealed   SessionStatus = "revealed"
	StatusSealed     SessionStatus = "sealed"
)

type ScaleDimension struct {
	Key   string  `json:"key"`
	Name  string  `json:"name"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Order int     `json:"order"`
}

type Sample struct {
	ID           string    `json:"id"`
	SessionID    string    `json:"sessionID"`
	InternalCode string    `json:"internalCode"`
	DisplayName  string    `json:"displayName"`
	BatchRef     string    `json:"batchRef,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

type BlindAssignment struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionID"`
	ReviewerUserID string    `json:"reviewerUserID"`
	SampleID       string    `json:"sampleID"`
	BlindCode      string    `json:"blindCode"`
	Sequence       int       `json:"sequence"`
	PlanHash       string    `json:"planHash"`
	FrozenAt       time.Time `json:"frozenAt"`
}

type ValidityStatus string

const (
	ValidityValid  ValidityStatus = "valid"
	ValidityVoided ValidityStatus = "voided"
	ValidityRework ValidityStatus = "rework_requested"
)

type Evaluation struct {
	ID             string             `json:"id"`
	SessionID      string             `json:"sessionID"`
	AssignmentID   string             `json:"assignmentID"`
	ReviewerUserID string             `json:"reviewerUserID"`
	Scores         map[string]float64 `json:"scores"`
	Comment        string             `json:"comment,omitempty"`
	StartedAt      time.Time          `json:"startedAt"`
	SubmittedAt    time.Time          `json:"submittedAt"`
	ValidityStatus ValidityStatus     `json:"validityStatus"`
	Revision       int                `json:"revision"`
	ReworkBy       string             `json:"reworkBy,omitempty"`
	ReworkAt       *time.Time         `json:"reworkAt,omitempty"`
}

type FindingStatus string

const (
	FindingOpen     FindingStatus = "open"
	FindingResolved FindingStatus = "resolved"
)

type Resolution string

const (
	ResolutionAccept Resolution = "accept"
	ResolutionVoid   Resolution = "void"
	ResolutionRework Resolution = "rework"
)

type VerificationFinding struct {
	ID           string            `json:"id"`
	SessionID    string            `json:"sessionID"`
	EvaluationID string            `json:"evaluationID,omitempty"`
	AssignmentID string            `json:"assignmentID,omitempty"`
	RuleCode     string            `json:"ruleCode"`
	Severity     string            `json:"severity"`
	Evidence     map[string]string `json:"evidence"`
	Status       FindingStatus     `json:"status"`
	Resolution   Resolution        `json:"resolution,omitempty"`
	ResolvedBy   string            `json:"resolvedBy,omitempty"`
	ResolvedAt   *time.Time        `json:"resolvedAt,omitempty"`
}

type RevealApproval struct {
	UserID     string    `json:"userID"`
	Role       string    `json:"role"`
	ApprovedAt time.Time `json:"approvedAt"`
}

type DimensionConclusion struct {
	DimensionKey string                     `json:"dimensionKey"`
	Dimension    string                     `json:"dimension"`
	SampleMeans  map[string]float64         `json:"sampleMeans"`
	SampleStats  map[string]SampleStatistic `json:"sampleStats,omitempty"`
	WinnerID     string                     `json:"winnerID,omitempty"`
	WinnerIDs    []string                   `json:"winnerIDs,omitempty"`
	Count        int                        `json:"count"`
}

type SampleStatistic struct {
	Mean  float64 `json:"mean"`
	Count int     `json:"count"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
}

type AuditEntry struct {
	Action string            `json:"action"`
	Actor  string            `json:"actor"`
	At     time.Time         `json:"at"`
	Detail map[string]string `json:"detail,omitempty"`
}
