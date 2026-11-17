package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"shortener/internal/auth"
	"shortener/internal/storage"
)

const (
	DefaultAddress = ":8080"
	DefaultBaseURL = "http://localhost:8080"
	DefaultSecret  = "super-secret-key"
	maxBodySize    = 1 << 20
)

type App struct {
	storage *storage.Memory
	auth    *auth.Manager
	baseURL string
	mux     *http.ServeMux
}

type shortenRequest struct {
	URL string `json:"url"`
}

type shortenResponse struct {
	Result string `json:"result"`
}

type userURL struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// New constructs the HTTP service from its dependencies.
func New(store *storage.Memory, authManager *auth.Manager, baseURL string) *App {
	a := &App{
		storage: store,
		auth:    authManager,
		baseURL: strings.TrimRight(baseURL, "/"),
		mux:     http.NewServeMux(),
	}
	a.routes()
	return a
}

// NewDefault constructs the service with in-memory storage and local settings.
func NewDefault() *App {
	return New(storage.NewMemory(), auth.NewManager(DefaultSecret), DefaultBaseURL)
}

func (a *App) routes() {
	a.mux.HandleFunc("/api/shorten", a.handleShorten)
	a.mux.HandleFunc("/api/user/urls", a.handleUserURLs)
	a.mux.HandleFunc("/", a.handleRedirect)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.mux.ServeHTTP(w, r)
}

func (a *App) handleShorten(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	request, err := decodeShortenRequest(w, r)
	if err != nil || !validOriginalURL(request.URL) {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userID, status := a.auth.UserFromRequest(r)
	if status != auth.Valid {
		userID, err = a.auth.NewUser(w)
		if err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
	}

	record := a.storage.Create(userID, request.URL)
	response := shortenResponse{Result: a.shortURL(record.ID)}
	writeJSON(w, http.StatusCreated, response)
}

func (a *App) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}

	originalURL, ok := a.storage.OriginalURL(id)
	if !ok {
		http.NotFound(w, r)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
}

func (a *App) handleUserURLs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	userID, status := a.auth.UserFromRequest(r)
	switch status {
	case auth.Malformed:
		http.Error(w, "invalid user cookie", http.StatusUnauthorized)
		return
	case auth.Missing, auth.InvalidSignature:
		if _, err := a.auth.NewUser(w); err != nil {
			http.Error(w, "failed to create user", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	records := a.storage.UserRecords(userID)
	if len(records) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	result := make([]userURL, 0, len(records))
	for _, record := range records {
		result = append(result, userURL{
			ShortURL:    a.shortURL(record.ID),
			OriginalURL: record.OriginalURL,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func decodeShortenRequest(w http.ResponseWriter, r *http.Request) (shortenRequest, error) {
	var request shortenRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		return request, err
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return request, errors.New("request body must contain one JSON object")
	}

	return request, nil
}

func validOriginalURL(rawURL string) bool {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}

	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func (a *App) shortURL(id string) string {
	return a.baseURL + "/" + id
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
