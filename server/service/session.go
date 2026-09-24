package service

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"time"

	"github.com/yourname/qrdrop/server/model"
)

// tokenPattern is the only shape a session token may take. Token values are
// used as directory names under the data dir, so this guard is load-bearing.
var tokenPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

// ValidToken reports whether s is a well-formed session token.
func ValidToken(s string) bool { return tokenPattern.MatchString(s) }

// GenerateToken returns a 128-bit random session token as hex.
func GenerateToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// NewSession creates an open inbox that expires after ttl. saveDirPref is
// snapshotted onto the session so its files stay traceable even if the global
// save-location setting changes later.
func NewSession(ttl time.Duration, saveDirPref string) (*model.Session, error) {
	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &model.Session{
		Token:     token,
		Status:    model.StatusOpen,
		SaveDir:   saveDirPref,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		Files:     []model.FileMeta{},
	}, nil
}
