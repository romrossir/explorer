package models

import "database/sql"

// Component represents a hierarchical component in the system.
type Component struct {
	ID          int64          `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ParentID    sql.NullInt64  `json:"parent_id,omitempty"` // Use sql.NullInt64 for nullable foreign key
	CreatedAt   string         `json:"created_at,omitempty"` // Stored as RFC3339 string, converted from time.Time
	UpdatedAt   string         `json:"updated_at,omitempty"` // Stored as RFC3339 string, converted from time.Time
}
