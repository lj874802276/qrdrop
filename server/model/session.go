package model

import "time"

// SessionStatus describes the lifecycle of a drop inbox.
type SessionStatus string

const (
	// StatusOpen means the inbox accepts uploads.
	StatusOpen SessionStatus = "open"
	// StatusClosed means the host closed the inbox; it is cleaned up shortly after.
	StatusClosed SessionStatus = "closed"
)

// Session is a temporary drop inbox identified by a random token.
type Session struct {
	Token     string        `json:"token"`
	Status    SessionStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
	ExpiresAt time.Time     `json:"expires_at"`
	ClosedAt  *time.Time    `json:"closed_at,omitempty"`
	// SaveDir is the save-location preference snapshotted when the inbox was
	// created, so files stay traceable even after the global setting changes.
	SaveDir string `json:"save_dir"`
	Files   []FileMeta `json:"files"`
}

// FileMeta describes a received file. Path is relative to the data directory
// and is intentionally never exposed to clients.
type FileMeta struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Size       int64     `json:"size"`
	Path       string    `json:"-"`
	UploadedAt time.Time `json:"uploaded_at"`
}
