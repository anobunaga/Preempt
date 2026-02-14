package wal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Entry represents a single write-ahead log entry
type Entry struct {
	MessageID string    `json:"message_id"`
	Timestamp time.Time `json:"timestamp"`
	Payload   []byte    `json:"payload"`
	Status    string    `json:"status"` // "pending" or "committed"
}

// WAL implements write-ahead logging for durability
type WAL struct {
	file          *os.File
	writer        *bufio.Writer
	mu            sync.Mutex
	path          string
	committedIDs  map[string]bool
	compactTicker *time.Ticker
}

// NewWAL creates a new write-ahead log at the specified path
func NewWAL(path string) (*WAL, error) {
	// Ensure directory exists
	dir := path[:len(path)-len("/store.wal")]
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	// Open or create WAL file
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	wal := &WAL{
		file:         file,
		writer:       bufio.NewWriter(file),
		path:         path,
		committedIDs: make(map[string]bool),
	}

	// Start background compaction every 5 minutes
	wal.compactTicker = time.NewTicker(5 * time.Minute)
	go wal.autoCompact()

	return wal, nil
}

// Write appends an entry to the WAL and forces disk sync
func (w *WAL) Write(messageID string, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	entry := Entry{
		MessageID: messageID,
		Timestamp: time.Now(),
		Payload:   payload,
		Status:    "pending",
	}

	// Serialize entry as JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal WAL entry: %w", err)
	}

	// Write entry + newline
	if _, err := w.writer.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write to WAL: %w", err)
	}

	// Flush buffer
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush WAL: %w", err)
	}

	// Force OS to write to disk (critical for durability!)
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL to disk: %w", err)
	}

	return nil
}

// Remove marks a WAL entry as committed (safe to delete during compaction)
func (w *WAL) Remove(messageID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.committedIDs[messageID] = true

	// Trigger compaction if too many committed entries
	if len(w.committedIDs) > 1000 {
		go w.Compact()
	}

	return nil
}

// GetPending returns all uncommitted entries from the WAL
func (w *WAL) GetPending() ([]Entry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No WAL file yet
		}
		return nil, fmt.Errorf("failed to open WAL for reading: %w", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip corrupted entries
			continue
		}

		// Only return entries that haven't been committed
		if !w.committedIDs[entry.MessageID] && entry.Status == "pending" {
			entries = append(entries, entry)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading WAL: %w", err)
	}

	return entries, nil
}

// Compact removes committed entries from the WAL
func (w *WAL) Compact() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Read all entries
	file, err := os.Open(w.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to compact
		}
		return fmt.Errorf("failed to open WAL for compaction: %w", err)
	}

	var pendingEntries []Entry
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		var entry Entry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue // Skip corrupted entries
		}

		// Keep only uncommitted entries
		if !w.committedIDs[entry.MessageID] {
			pendingEntries = append(pendingEntries, entry)
		}
	}
	file.Close()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading WAL during compaction: %w", err)
	}

	// Write compacted WAL to temp file
	tempPath := w.path + ".tmp"
	tempFile, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp WAL file: %w", err)
	}
	defer tempFile.Close()

	tempWriter := bufio.NewWriter(tempFile)

	for _, entry := range pendingEntries {
		data, _ := json.Marshal(entry)
		tempWriter.Write(append(data, '\n'))
	}

	tempWriter.Flush()
	tempFile.Sync()

	// Close current WAL
	w.writer.Flush()
	w.file.Close()

	// Atomic rename
	if err := os.Rename(tempPath, w.path); err != nil {
		return fmt.Errorf("failed to rename compacted WAL: %w", err)
	}

	// Reopen WAL
	w.file, err = os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to reopen WAL after compaction: %w", err)
	}
	w.writer = bufio.NewWriter(w.file)

	// Clear committed IDs map
	w.committedIDs = make(map[string]bool)

	return nil
}

// autoCompact runs compaction periodically
func (w *WAL) autoCompact() {
	for range w.compactTicker.C {
		if err := w.Compact(); err != nil {
			// Log error but don't stop compaction
			fmt.Printf("WAL compaction error: %v\n", err)
		}
	}
}

// Close stops the WAL and closes the file
func (w *WAL) Close() error {
	if w.compactTicker != nil {
		w.compactTicker.Stop()
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.writer.Flush(); err != nil {
		return err
	}

	return w.file.Close()
}
