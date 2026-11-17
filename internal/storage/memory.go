package storage

import (
	"sync"
)

// Record is a shortened URL owned by a user.
type Record struct {
	ID          string
	OriginalURL string
}

// Memory stores shortened URLs in process memory.
type Memory struct {
	mu        sync.RWMutex
	nextID    uint64
	links     map[string]string
	userLinks map[string][]Record
}

func NewMemory() *Memory {
	return &Memory{
		links:     make(map[string]string),
		userLinks: make(map[string][]Record),
	}
}

// Create saves a URL and returns its unique short ID.
func (m *Memory) Create(userID, originalURL string) Record {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	id := encodeID(m.nextID)
	record := Record{ID: id, OriginalURL: originalURL}
	m.links[id] = originalURL
	m.userLinks[userID] = append(m.userLinks[userID], record)

	return record
}

// OriginalURL finds the original URL by its short ID.
func (m *Memory) OriginalURL(id string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	originalURL, ok := m.links[id]
	return originalURL, ok
}

// UserRecords returns a copy of all URLs created by a user.
func (m *Memory) UserRecords(userID string) []Record {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.userLinks[userID]
	result := make([]Record, len(records))
	copy(result, records)
	return result
}

func encodeID(value uint64) string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	encoded := make([]byte, 0, 6)
	for value > 0 {
		encoded = append(encoded, alphabet[value%uint64(len(alphabet))])
		value /= uint64(len(alphabet))
	}
	for len(encoded) < 6 {
		encoded = append(encoded, alphabet[0])
	}

	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}

	return string(encoded)
}
