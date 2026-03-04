package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	if store == nil {
		t.Fatal("Expected non-nil storage")
	}

	if store.Count() != 0 {
		t.Errorf("Expected empty storage, got count %d", store.Count())
	}
}

func TestMarkPosted(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Mark an item as posted
	guid := "test-guid-123"
	err = store.MarkPosted(guid)
	if err != nil {
		t.Fatalf("Failed to mark item as posted: %v", err)
	}

	// Verify it's marked as posted
	if !store.IsPosted(guid) {
		t.Error("Expected item to be marked as posted")
	}

	// Verify count
	if store.Count() != 1 {
		t.Errorf("Expected count 1, got %d", store.Count())
	}
}

func TestIsPosted(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	guid := "test-guid-456"

	// Should not be posted initially
	if store.IsPosted(guid) {
		t.Error("Expected item to not be posted initially")
	}

	// Mark as posted
	store.MarkPosted(guid)

	// Should be posted now
	if !store.IsPosted(guid) {
		t.Error("Expected item to be posted after marking")
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	// Create first storage instance and add items
	store1, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create first storage: %v", err)
	}

	guids := []string{"guid-1", "guid-2", "guid-3"}
	for _, guid := range guids {
		if err := store1.MarkPosted(guid); err != nil {
			t.Fatalf("Failed to mark item as posted: %v", err)
		}
	}

	// Create second storage instance (simulating restart)
	store2, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create second storage: %v", err)
	}

	// Verify all items are still marked as posted
	for _, guid := range guids {
		if !store2.IsPosted(guid) {
			t.Errorf("Expected guid '%s' to be persisted", guid)
		}
	}

	// Verify count
	if store2.Count() != len(guids) {
		t.Errorf("Expected count %d, got %d", len(guids), store2.Count())
	}
}

func TestMarkPosted_Duplicate(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	guid := "duplicate-guid"

	// Mark twice
	store.MarkPosted(guid)
	store.MarkPosted(guid)

	// Should still only count as one
	if store.Count() != 1 {
		t.Errorf("Expected count 1 after duplicate mark, got %d", store.Count())
	}

	// Should still be posted
	if !store.IsPosted(guid) {
		t.Error("Expected item to still be posted")
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "empty.txt")

	// Create empty file
	file, err := os.Create(storagePath)
	if err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}
	file.Close()

	// Load storage
	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to load storage from empty file: %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("Expected count 0 from empty file, got %d", store.Count())
	}
}

func TestLoad_FileWithEmptyLines(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_with_empty_lines.txt")

	// Create file with some empty lines
	content := "guid-1\n\nguid-2\n\n\nguid-3\n"
	if err := os.WriteFile(storagePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load storage
	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to load storage: %v", err)
	}

	// Should only have 3 items (empty lines ignored)
	if store.Count() != 3 {
		t.Errorf("Expected count 3, got %d", store.Count())
	}

	// Verify specific GUIDs
	expectedGuids := []string{"guid-1", "guid-2", "guid-3"}
	for _, guid := range expectedGuids {
		if !store.IsPosted(guid) {
			t.Errorf("Expected guid '%s' to be loaded", guid)
		}
	}
}

func TestCount(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "test_storage.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Initial count should be 0
	if store.Count() != 0 {
		t.Errorf("Expected initial count 0, got %d", store.Count())
	}

	// Add items and check count
	for i := range 5 {
		store.MarkPosted(string(rune('a' + i)))
		expectedCount := i + 1
		if store.Count() != expectedCount {
			t.Errorf("After adding %d items, expected count %d, got %d", expectedCount, expectedCount, store.Count())
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	storagePath := filepath.Join(tempDir, "concurrent_test.txt")

	store, err := New(storagePath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Simulate concurrent access
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			guid := string(rune('a' + id))
			store.MarkPosted(guid)
			_ = store.IsPosted(guid)
			_ = store.Count()
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for range 10 {
		<-done
	}

	// All items should be present
	if store.Count() != 10 {
		t.Errorf("Expected count 10 after concurrent access, got %d", store.Count())
	}
}
