package storage

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

// Storage handles persistence of posted item GUIDs
type Storage struct {
	filePath string
	posted   map[string]bool
	mu       sync.RWMutex
}

// New creates a new storage instance
func New(filePath string) (*Storage, error) {
	s := &Storage{
		filePath: filePath,
		posted:   make(map[string]bool),
	}

	// Load existing posted items
	if err := s.load(); err != nil {
		return nil, fmt.Errorf("failed to load storage: %w", err)
	}

	return s, nil
}

// load reads the posted items from disk
func (s *Storage) load() (err error) {
	file, err := os.Open(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, that's okay
			return nil
		}
		return err
	}
	defer func() {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		guid := scanner.Text()
		if guid != "" {
			s.posted[guid] = true
		}
	}

	return scanner.Err()
}

// save writes the posted items to disk
func (s *Storage) save() (err error) {
	file, err := os.Create(s.filePath)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := file.Close(); err == nil {
			err = cerr
		}
	}()

	writer := bufio.NewWriter(file)
	for guid := range s.posted {
		if _, err := writer.WriteString(guid + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

// IsPosted checks if a GUID has been posted
func (s *Storage) IsPosted(guid string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.posted[guid]
}

// MarkPosted marks a GUID as posted
func (s *Storage) MarkPosted(guid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.posted[guid] = true
	return s.save()
}

// Count returns the number of posted items
func (s *Storage) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.posted)
}
