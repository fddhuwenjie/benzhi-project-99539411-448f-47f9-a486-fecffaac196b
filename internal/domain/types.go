package domain

import "time"

type State string

const (
	StateDraft              State = "DRAFT"
	StateBaselined          State = "BASELINED"
	StateImaged             State = "IMAGED"
	StateAnalyzed           State = "ANALYZED"
	StateCorrectionRequired State = "CORRECTION_REQUIRED"
	StateReviewReady        State = "REVIEW_READY"
	StateVerified           State = "VERIFIED"
	StateSealed             State = "SEALED"
)

type MarkerKind string

const (
	MarkerNone    MarkerKind = "NONE"
	MarkerMissing MarkerKind = "MISSING"
	MarkerFalse   MarkerKind = "FALSE"
)

type DendroBatch struct {
	BatchID       string            `json:"batch_id"`
	SiteCode      string            `json:"site_code"`
	Species       string            `json:"species"`
	SampledAt     time.Time         `json:"sampled_at"`
	OperatorID    string            `json:"operator_id"`
	ReviewerID    string            `json:"reviewer_id,omitempty"`
	State         State             `json:"state"`
	Revision      int64             `json:"revision"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	SealedAt      *time.Time        `json:"sealed_at,omitempty"`
	Cores         []CoreSample      `json:"cores"`
	Observations  []RingObservation `json:"observations"`
	Findings      []QualityFinding  `json:"findings"`
	Review        *ReviewSeal       `json:"review,omitempty"`
	LastRuleRunAt *time.Time        `json:"last_rule_run_at,omitempty"`
}

type CoreSample struct {
	CoreID            string     `json:"core_id"`
	BatchID           string     `json:"batch_id"`
	TreeCode          string     `json:"tree_code"`
	RadiusCode        string     `json:"radius_code"`
	PreparationMethod string     `json:"preparation_method,omitempty"`
	ImageDigest       string     `json:"image_digest,omitempty"`
	MicronsPerPixel   float64    `json:"microns_per_pixel,omitempty"`
	CapturedAt        *time.Time `json:"captured_at,omitempty"`
}

type RingObservation struct {
	ObservationID    string     `json:"observation_id"`
	CoreID           string     `json:"core_id"`
	RingIndex        int        `json:"ring_index"`
	CalendarYear     int        `json:"calendar_year"`
	WidthMicrons     float64    `json:"width_microns"`
	BoundaryPosition float64    `json:"boundary_position"`
	MarkerKind       MarkerKind `json:"marker_kind"`
	MarkerNote       string     `json:"marker_note,omitempty"`
	AnchorGroup      string     `json:"anchor_group,omitempty"`
	SupersedesID     string     `json:"supersedes_id,omitempty"`
	RecordedAt       time.Time  `json:"recorded_at"`
}

type FindingStatus string

const (
	FindingOpen     FindingStatus = "OPEN"
	FindingResolved FindingStatus = "RESOLVED"
)

type QualityFinding struct {
	FindingID        string        `json:"finding_id"`
	BatchID          string        `json:"batch_id"`
	CoreID           string        `json:"core_id,omitempty"`
	ObservationID    string        `json:"observation_id,omitempty"`
	RuleCode         string        `json:"rule_code"`
	Severity         string        `json:"severity"`
	Message          string        `json:"message"`
	Status           FindingStatus `json:"status"`
	BeforeValue      any           `json:"before_value,omitempty"`
	AfterValue       any           `json:"after_value,omitempty"`
	ResolutionReason string        `json:"resolution_reason,omitempty"`
	ResolvedAt       *time.Time    `json:"resolved_at,omitempty"`
	CreatedAt        time.Time     `json:"created_at"`
}

type ReviewSeal struct {
	SealID           string    `json:"seal_id"`
	BatchID          string    `json:"batch_id"`
	ReviewerID       string    `json:"reviewer_id"`
	Decision         string    `json:"decision"`
	ReviewNote       string    `json:"review_note"`
	VerifiedRevision int64     `json:"verified_revision"`
	ManifestDigest   string    `json:"manifest_digest,omitempty"`
	EventChainDigest string    `json:"event_chain_digest,omitempty"`
	SignedAt         time.Time `json:"signed_at"`
}

type AuditEvent struct {
	EventID        string    `json:"event_id"`
	BatchID        string    `json:"batch_id"`
	Revision       int64     `json:"revision"`
	RequestID      string    `json:"request_id"`
	Action         string    `json:"action"`
	ActorID        string    `json:"actor_id"`
	Reason         string    `json:"reason,omitempty"`
	OccurredAt     time.Time `json:"occurred_at"`
	PreviousDigest string    `json:"previous_digest"`
	Payload        any       `json:"payload,omitempty"`
	Digest         string    `json:"digest"`
}

type Manifest struct {
	SchemaVersion    string         `json:"schema_version"`
	BatchID          string         `json:"batch_id"`
	SiteCode         string         `json:"site_code"`
	Species          string         `json:"species"`
	SampledAt        time.Time      `json:"sampled_at"`
	OperatorID       string         `json:"operator_id"`
	ReviewerID       string         `json:"reviewer_id"`
	SealedRevision   int64          `json:"sealed_revision"`
	SealedAt         time.Time      `json:"sealed_at"`
	Cores            []ManifestCore `json:"cores"`
	EvidenceDigest   string         `json:"evidence_digest"`
	EventChainDigest string         `json:"event_chain_digest"`
	ManifestDigest   string         `json:"manifest_digest"`
}

type ManifestCore struct {
	CoreID          string                `json:"core_id"`
	TreeCode        string                `json:"tree_code"`
	RadiusCode      string                `json:"radius_code"`
	ImageDigest     string                `json:"image_digest"`
	MicronsPerPixel float64               `json:"microns_per_pixel"`
	Sequence        []ManifestObservation `json:"sequence"`
}

type ManifestObservation struct {
	RingIndex        int        `json:"ring_index"`
	CalendarYear     int        `json:"calendar_year"`
	WidthMicrons     float64    `json:"width_microns"`
	BoundaryPosition float64    `json:"boundary_position"`
	MarkerKind       MarkerKind `json:"marker_kind"`
	AnchorGroup      string     `json:"anchor_group,omitempty"`
}
