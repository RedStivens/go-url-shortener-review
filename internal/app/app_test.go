package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"shortener/internal/auth"
	"shortener/internal/storage"
)

const testBaseURL = "http://short.test"

func newTestApp() (*App, *auth.Manager) {
	authManager := auth.NewManager("test-secret")
	return New(storage.NewMemory(), authManager, testBaseURL), authManager
}

func TestCreateRedirectAndListUserURLs(t *testing.T) {
	service, authManager := newTestApp()
	originalURL := "https://example.com/some/long/path?value=1"

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"`+originalURL+`"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	service.ServeHTTP(createRecorder, createRequest)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if contentType := createRecorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("unexpected content type %q", contentType)
	}
	var createResponse shortenResponse
	if err := json.NewDecoder(createRecorder.Body).Decode(&createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if !strings.HasPrefix(createResponse.Result, testBaseURL+"/") {
		t.Fatalf("unexpected short URL %q", createResponse.Result)
	}

	cookies := createRecorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected a new user cookie, got %d", len(cookies))
	}
	userID, status := authManager.Parse(cookies[0].Value)
	if status != auth.Valid || userID == "" {
		t.Fatalf("cookie is not signed correctly: id=%q status=%v", userID, status)
	}

	shortID := strings.TrimPrefix(createResponse.Result, testBaseURL+"/")
	redirectRecorder := httptest.NewRecorder()
	redirectRequest := httptest.NewRequest(http.MethodGet, "/"+shortID, nil)
	service.ServeHTTP(redirectRecorder, redirectRequest)
	if redirectRecorder.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected status 307, got %d", redirectRecorder.Code)
	}
	if location := redirectRecorder.Header().Get("Location"); location != originalURL {
		t.Fatalf("expected redirect to %q, got %q", originalURL, location)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	listRequest.AddCookie(cookies[0])
	service.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var urls []userURL
	if err := json.NewDecoder(listRecorder.Body).Decode(&urls); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(urls) != 1 || urls[0].ShortURL != createResponse.Result || urls[0].OriginalURL != originalURL {
		t.Fatalf("unexpected URL list: %#v", urls)
	}
}

func TestUnknownShortURL(t *testing.T) {
	service, _ := newTestApp()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestUserURLsWithoutCookieCreatesUserAndReturnsNoContent(t *testing.T) {
	service, authManager := newTestApp()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected a new cookie, got %d", len(cookies))
	}
	if _, status := authManager.Parse(cookies[0].Value); status != auth.Valid {
		t.Fatalf("expected a valid signed cookie, got status %v", status)
	}
}

func TestUserURLsWithMalformedCookieReturnsUnauthorized(t *testing.T) {
	service, _ := newTestApp()

	for _, value := range []string{"", "missing-signature", ":signature"} {
		t.Run(value, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
			request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: value})

			service.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d", recorder.Code)
			}
		})
	}
}

func TestUserURLsWithInvalidSignatureCreatesNewUser(t *testing.T) {
	service, authManager := newTestApp()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	request.AddCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: "user-123:" + strings.Repeat("0", 64),
	})

	service.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected a replacement cookie, got %d", len(cookies))
	}
	if userID, status := authManager.Parse(cookies[0].Value); status != auth.Valid || userID == "user-123" {
		t.Fatalf("expected a new valid user, got id=%q status=%v", userID, status)
	}
}

func TestUserURLsAreIsolated(t *testing.T) {
	service, authManager := newTestApp()
	firstUser := &http.Cookie{Name: auth.CookieName, Value: authManager.Value("first-user")}
	secondUser := &http.Cookie{Name: auth.CookieName, Value: authManager.Value("second-user")}

	createForUser(t, service, firstUser, "https://example.com/first")
	createForUser(t, service, secondUser, "https://example.com/second")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/user/urls", nil)
	request.AddCookie(firstUser)
	service.ServeHTTP(recorder, request)

	var urls []userURL
	if err := json.NewDecoder(recorder.Body).Decode(&urls); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(urls) != 1 || urls[0].OriginalURL != "https://example.com/first" {
		t.Fatalf("expected only the first user's URL, got %#v", urls)
	}
}

func TestInvalidRequestsDoNotCrashService(t *testing.T) {
	service, _ := newTestApp()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
		status int
	}{
		{name: "invalid JSON", method: http.MethodPost, path: "/api/shorten", body: `{`, status: http.StatusBadRequest},
		{name: "empty URL", method: http.MethodPost, path: "/api/shorten", body: `{"url":""}`, status: http.StatusBadRequest},
		{name: "relative URL", method: http.MethodPost, path: "/api/shorten", body: `{"url":"/local"}`, status: http.StatusBadRequest},
		{name: "wrong method", method: http.MethodGet, path: "/api/shorten", status: http.StatusMethodNotAllowed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			service.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, recorder.Code)
			}
		})
	}
}

func createForUser(t *testing.T, service *App, cookie *http.Cookie, originalURL string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/shorten", strings.NewReader(`{"url":"`+originalURL+`"}`))
	request.AddCookie(cookie)
	service.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create URL for user: status %d: %s", recorder.Code, recorder.Body.String())
	}
}
