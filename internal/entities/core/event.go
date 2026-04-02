package core

import "time"

const (
	EventStatusDraft      = "draft"
	EventStatusGenerating = "generating"
	EventStatusReady      = "ready"
	EventStatusFailed     = "failed"
	EventStatusCancelled  = "cancelled"
)

type Event struct {
	ID                 string         `json:"id"`
	City               string         `json:"city"`
	EventDate          string         `json:"event_date"`
	EventTime          string         `json:"event_time"`
	ExpectedGuestCount int            `json:"expected_guest_count"`
	Budget             string         `json:"budget"`
	Title              *string        `json:"title"`
	Description        *string        `json:"description"`
	Status             string         `json:"status"`
	SelectedVariantID  *string        `json:"selected_variant_id,omitempty"`
	Variants           []EventVariant `json:"variants,omitempty"`
	AccessRole         *string        `json:"access_role,omitempty"`
	ApprovalStatus     *string        `json:"approval_status,omitempty"`
	AttendanceStatus   *string        `json:"attendance_status,omitempty"`
	InviteToken        *string        `json:"invite_token,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

type EventVariant struct {
	ID              string          `json:"id"`
	EventID         string          `json:"event_id"`
	VariantNumber   int             `json:"variant_number"`
	Title           *string         `json:"title,omitempty"`
	Description     *string         `json:"description,omitempty"`
	Status          string          `json:"status"`
	LLMRequestID    *string         `json:"llm_request_id,omitempty"`
	GenerationError *string         `json:"generation_error,omitempty"`
	Locations       []EventLocation `json:"locations,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type EventLocation struct {
	ID           string     `json:"id"`
	EventID      string     `json:"event_id"`
	VariantID    string     `json:"variant_id"`
	Title        string     `json:"title"`
	ImageURL     *string    `json:"image_url,omitempty"`
	Description  *string    `json:"description,omitempty"`
	Rating       *string    `json:"rating,omitempty"`
	Address      *string    `json:"address,omitempty"`
	WorkingHours *string    `json:"working_hours,omitempty"`
	AvgBill      *string    `json:"avg_bill,omitempty"`
	Cuisine      *string    `json:"cuisine,omitempty"`
	Contacts     *string    `json:"contacts,omitempty"`
	AIComment    *string    `json:"ai_comment,omitempty"`
	AIScore      *string    `json:"ai_score,omitempty"`
	SortOrder    int        `json:"sort_order"`
	Source       string     `json:"source"`
	IsRejected   bool       `json:"is_rejected"`
	RejectedAt   *time.Time `json:"rejected_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
