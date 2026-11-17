package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestMemoryCreateAndLookup(t *testing.T) {
	store := NewMemory()
	first := store.Create("user-1", "https://example.com/first")
	second := store.Create("user-1", "https://example.com/second")
	store.Create("user-2", "https://example.org")

	if first.ID == second.ID {
		t.Fatal("short IDs must be unique")
	}
	originalURL, ok := store.OriginalURL(first.ID)
	if !ok || originalURL != first.OriginalURL {
		t.Fatalf("unexpected lookup result: url=%q found=%v", originalURL, ok)
	}
	if _, ok := store.OriginalURL("missing"); ok {
		t.Fatal("unknown short ID must not be found")
	}

	records := store.UserRecords("user-1")
	if len(records) != 2 || records[0] != first || records[1] != second {
		t.Fatalf("unexpected user records: %#v", records)
	}
}

func TestMemoryConcurrentCreate(t *testing.T) {
	store := NewMemory()
	const count = 100

	ids := make(chan string, count)
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			record := store.Create("user", fmt.Sprintf("https://example.com/%d", index))
			ids <- record.ID
		}(i)
	}
	group.Wait()
	close(ids)

	unique := make(map[string]struct{}, count)
	for id := range ids {
		unique[id] = struct{}{}
	}
	if len(unique) != count {
		t.Fatalf("expected %d unique IDs, got %d", count, len(unique))
	}
	if records := store.UserRecords("user"); len(records) != count {
		t.Fatalf("expected %d user records, got %d", count, len(records))
	}
}
