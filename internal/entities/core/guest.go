package core

import "time"

type ApprovalStatus string
type AttendanceStatus string

const (
	ApprovalPending  ApprovalStatus = "pending"
	ApprovalApproved ApprovalStatus = "approved"
	ApprovalRejected ApprovalStatus = "rejected"

	AttendancePending   AttendanceStatus = "pending"
	AttendanceConfirmed AttendanceStatus = "confirmed"
	AttendanceDeclined  AttendanceStatus = "declined"
)

type EventGuest struct {
	ID               string           `json:"id"`
	EventID          string           `json:"event_id"`
	UserID           *string          `json:"user_id,omitempty"`
	FullName         string           `json:"full_name"`
	Phone            *string          `json:"phone,omitempty"`
	ApprovalStatus   ApprovalStatus   `json:"approval_status"`
	AttendanceStatus AttendanceStatus `json:"attendance_status"`
	PlusOneCount     int              `json:"plus_one_count"`
	CreatedAt        time.Time        `json:"created_at"`
}

type EventGuestStats struct {
	PendingApproval   int `json:"pending_approval"`
	Approved          int `json:"approved"`
	Rejected          int `json:"rejected"`
	AttendancePending int `json:"attendance_pending"`
	Confirmed         int `json:"confirmed"`
	Declined          int `json:"declined"`
}
