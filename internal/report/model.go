// Package report carries the abuse-report flow (design §6.6): users
// flag publicly listed coteries, the platform admin triages the inbox
// and decides. Neither the filing nor the decision notifies anyone.
package report

import "time"

// Report statuses (design §6.6): open until an admin resolves or
// dismisses it.
const (
	StatusOpen      = "open"
	StatusResolved  = "resolved"
	StatusDismissed = "dismissed"
)

// Field limits, shared with the migration's CHECK constraints.
const (
	maxReasonLen = 1000
	maxNoteLen   = 1000
)

// Report is one user's flag on one publicly listed coterie — a row in
// the admin's inbox. Deciding never acts on the coterie itself: the
// admin works through the existing surfaces.
type Report struct {
	ID             string     `gorm:"column:id;primaryKey;type:uuid" json:"id"`
	CoterieID      string     `gorm:"column:coterie_id;not null" json:"coterie_id"`
	ReporterID     string     `gorm:"column:reporter_id;not null" json:"reporter_id"`
	Reason         string     `gorm:"column:reason;not null" json:"reason"`
	Status         string     `gorm:"column:status;not null" json:"status"`
	ResolutionNote *string    `gorm:"column:resolution_note" json:"resolution_note,omitempty"`
	DecidedAt      *time.Time `gorm:"column:decided_at" json:"decided_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
}

// TableName aligns the model with the hand-written migration schema.
func (Report) TableName() string { return "reports" }

// ReportView is a report with the identities the admin triages by: the
// flagged coterie and the reporter.
type ReportView struct {
	Report
	CoterieName      string `gorm:"column:coterie_name" json:"coterie_name"`
	CoterieListing   string `gorm:"column:coterie_listing" json:"coterie_listing"`
	ReporterUsername string `gorm:"column:reporter_username" json:"reporter_username"`
	ReporterEmail    string `gorm:"column:reporter_email" json:"reporter_email"`
}

// CreateReportRequest is the payload for POST /coteries/{id}/report.
type CreateReportRequest struct {
	Reason string `json:"reason"`
}

// DecideReportRequest is the payload for the admin resolve|dismiss
// endpoints. The note is optional.
type DecideReportRequest struct {
	Note string `json:"note"`
}
