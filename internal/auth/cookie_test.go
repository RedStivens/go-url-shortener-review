package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignedCookie(t *testing.T) {
	manager := NewManager("test-secret")
	value := manager.Value("user-123")

	userID, status := manager.Parse(value)
	if status != Valid {
		t.Fatalf("expected a valid cookie, got status %v", status)
	}
	if userID != "user-123" {
		t.Fatalf("expected user-123, got %q", userID)
	}

	tampered := "another-user" + value[strings.IndexByte(value, ':'):]
	if _, status := manager.Parse(tampered); status != InvalidSignature {
		t.Fatalf("expected a tampered cookie to be rejected, got status %v", status)
	}
}

func TestMalformedCookie(t *testing.T) {
	manager := NewManager("test-secret")

	values := []string{"", "user", ":signature", "user:not-hex", "user:abcd:extra"}
	for _, value := range values {
		t.Run(value, func(t *testing.T) {
			if _, status := manager.Parse(value); status != Malformed {
				t.Fatalf("expected malformed status for %q, got %v", value, status)
			}
		})
	}
}

func TestNewUserSetsValidUniqueCookies(t *testing.T) {
	manager := NewManager("test-secret")
	firstRecorder := httptest.NewRecorder()
	secondRecorder := httptest.NewRecorder()

	firstID, err := manager.NewUser(firstRecorder)
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	secondID, err := manager.NewUser(secondRecorder)
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	if firstID == secondID {
		t.Fatal("expected unique user IDs")
	}

	cookies := firstRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	if cookies[0].Name != CookieName || !cookies[0].HttpOnly || cookies[0].Path != "/" {
		t.Fatalf("unexpected cookie settings: %#v", cookies[0])
	}
	parsedID, status := manager.Parse(cookies[0].Value)
	if status != Valid || parsedID != firstID {
		t.Fatalf("new cookie is not valid: id=%q status=%v", parsedID, status)
	}
}
