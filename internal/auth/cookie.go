package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"
)

const CookieName = "user_id"

// Status describes the result of checking a user cookie.
type Status int

const (
	Valid Status = iota
	Missing
	Malformed
	InvalidSignature
)

// Manager creates and verifies signed user cookies.
type Manager struct {
	secret []byte
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

// UserFromRequest returns the user ID and the result of cookie validation.
func (m *Manager) UserFromRequest(r *http.Request) (string, Status) {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return "", Missing
	}

	return m.Parse(cookie.Value)
}

// Parse verifies a serialized cookie value.
func (m *Manager) Parse(value string) (string, Status) {
	userID, signature, ok := strings.Cut(value, ":")
	if !ok || userID == "" || signature == "" || strings.Contains(signature, ":") {
		return "", Malformed
	}

	actual, err := hex.DecodeString(signature)
	if err != nil || len(actual) != sha256.Size {
		return "", Malformed
	}

	expected := m.signature(userID)
	if !hmac.Equal(expected, actual) {
		return "", InvalidSignature
	}

	return userID, Valid
}

// NewUser creates a random user ID and writes its signed cookie.
func (m *Manager) NewUser(w http.ResponseWriter) (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	userID := hex.EncodeToString(bytes)
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    m.Value(userID),
		Path:     "/",
		MaxAge:   int((30 * 24 * time.Hour).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return userID, nil
}

// Value returns a signed cookie value for a user ID.
func (m *Manager) Value(userID string) string {
	return userID + ":" + hex.EncodeToString(m.signature(userID))
}

func (m *Manager) signature(userID string) []byte {
	h := hmac.New(sha256.New, m.secret)
	_, _ = h.Write([]byte(userID))
	return h.Sum(nil)
}
