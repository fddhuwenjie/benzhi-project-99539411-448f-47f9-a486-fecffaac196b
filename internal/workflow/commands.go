package workflow

import (
	"encoding/json"
	"time"

	"dendro-chronology-workbench/internal/domain"
)

type CommandMeta struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	ActorID          string `json:"actor_id"`
}

type CreateBatchCommand struct {
	CommandMeta
	BatchID    string              `json:"batch_id"`
	SiteCode   string              `json:"site_code"`
	Species    string              `json:"species"`
	SampledAt  time.Time           `json:"sampled_at"`
	OperatorID string              `json:"operator_id"`
	Cores      []domain.CoreSample `json:"cores"`
}

type ImageInput struct {
	CoreID            string    `json:"core_id"`
	PreparationMethod string    `json:"preparation_method"`
	ImageDigest       string    `json:"image_digest"`
	MicronsPerPixel   float64   `json:"microns_per_pixel"`
	CapturedAt        time.Time `json:"captured_at"`
}

type RegisterImagesCommand struct {
	CommandMeta
	Images []ImageInput `json:"images"`
}
type SubmitObservationsCommand struct {
	CommandMeta
	Observations []domain.RingObservation `json:"observations"`
}
type ValidateCommand struct{ CommandMeta }

type Replacement struct {
	WidthMicrons     *float64           `json:"width_microns,omitempty"`
	CalendarYear     *int               `json:"calendar_year,omitempty"`
	BoundaryPosition *float64           `json:"boundary_position,omitempty"`
	MarkerKind       *domain.MarkerKind `json:"marker_kind,omitempty"`
	MarkerNote       *string            `json:"marker_note,omitempty"`
	AnchorGroup      *string            `json:"anchor_group,omitempty"`
}

type CorrectFindingCommand struct {
	CommandMeta
	FindingID   string      `json:"finding_id"`
	Reason      string      `json:"reason"`
	Replacement Replacement `json:"replacement"`
}

type CorrectionItem struct {
	FindingID   string      `json:"finding_id"`
	Reason      string      `json:"reason"`
	Replacement Replacement `json:"replacement"`
}

type CorrectFindingsCommand struct {
	CommandMeta
	Items []CorrectionItem `json:"items"`
}

type CorrectionItemResult struct {
	FindingID     string `json:"finding_id"`
	ObservationID string `json:"observation_id,omitempty"`
	SupersedesID  string `json:"supersedes_id,omitempty"`
	Status        string `json:"status"`
}

type ReviewCommand struct {
	CommandMeta
	ReviewerID        string `json:"reviewer_id"`
	Decision          string `json:"decision"`
	Note              string `json:"note"`
	InspectedRevision *int64 `json:"inspected_revision,omitempty"`
}
type SealCommand struct{ CommandMeta }

type Result struct {
	StatusCode int             `json:"-"`
	Body       json.RawMessage `json:"-"`
	Replayed   bool            `json:"-"`
}

type CommandResponse struct {
	Batch       *domain.DendroBatch    `json:"batch"`
	RuleResult  *domain.RuleResult     `json:"rule_result,omitempty"`
	Inspection  *EvidenceInspection    `json:"inspection,omitempty"`
	Manifest    *domain.Manifest       `json:"manifest,omitempty"`
	Corrections []CorrectionItemResult `json:"corrections,omitempty"`
	Replayed    bool                   `json:"replayed,omitempty"`
}
