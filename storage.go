package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type ShortLink struct {
	URL      string
	Accessed bool
}

var (
	shortLinks = make(map[string]ShortLink)
	mu         sync.RWMutex
	storageDir = "/app/storage" // Use absolute path inside container
	linksFile  = filepath.Join(storageDir, "shortlinks.json")
)

// CheckStoragePermissions checks if storage directory is writable
func CheckStoragePermissions() error {
	mu.Lock()
	defer mu.Unlock()

	// Create directory if not exists
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %v", err)
	}

	// Check if directory is writable by creating a test file
	testFile := filepath.Join(storageDir, ".test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("storage directory is not writable: %v", err)
	}
	_ = os.Remove(testFile)
	return nil
}

func init() {
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		log.Printf("Error creating storage directory: %v", err)
	}
	loadLinks()
}

func SetShortLink(code, url string) {
	mu.Lock()
	defer mu.Unlock()
	shortLinks[code] = ShortLink{
		URL:      url,
		Accessed: false,
	}
	saveLinks()
}

func GetShortLink(code string) (string, bool, bool) {
	mu.Lock() // Need write lock to mark as accessed
	defer mu.Unlock()

	link, exists := shortLinks[code]
	if !exists {
		log.Printf("[Storage] Code %s not found in map.", code)
		return "", false, false
	}

	// Check if already accessed
	if link.Accessed {
		log.Printf("[Storage] Link %s was already accessed", code)
		return "", true, true
	}

	// Mark as accessed and save
	link.Accessed = true
	shortLinks[code] = link
	saveLinks()

	log.Printf("[Storage] Returning first-time access for code %s: %s", code, link.URL)
	return link.URL, false, true
}

func loadLinks() {
	mu.Lock() // Use write lock for loading as it modifies the map
	defer mu.Unlock()
	log.Println("[Storage] Attempting to load links from file:", linksFile)
	data, err := os.ReadFile(linksFile)
	// Initialize map first in case of errors or empty file
	shortLinks = make(map[string]ShortLink)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Storage] Error reading links file: %v. Starting with empty map.", err)
		} else {
			log.Println("[Storage] Links file not found, starting with empty map.")
		}
		return // Start with empty map
	}

	// Check if file is empty
	if len(data) == 0 {
		log.Println("[Storage] Links file is empty, starting with empty map.")
		return
	}

	// Try to unmarshal into new format first
	if err := json.Unmarshal(data, &shortLinks); err != nil {
		// Fallback to old format (map[string]string)
		var oldLinks map[string]string
		if err := json.Unmarshal(data, &oldLinks); err != nil {
			log.Printf("[Storage] Error unmarshaling links file: %v. Starting with empty map.", err)
			shortLinks = make(map[string]ShortLink)
			return
		}
		// Convert old format to new
		for k, v := range oldLinks {
			shortLinks[k] = ShortLink{
				URL:      v,
				Accessed: false, // Assume not accessed before
			}
		}
		log.Printf("[Storage] Converted %d legacy links to new format", len(oldLinks))
	}
	log.Printf("[Storage] Successfully loaded %d short links from file.", len(shortLinks))
}

// saveLinks is called internally by SetShortLink which already holds the write lock.
// No need for separate locking here.
func saveLinks() {
	log.Printf("[Storage] Attempting to save %d links to file: %s", len(shortLinks), linksFile)
	// Marshal the current state of the global shortLinks map directly
	data, err := json.MarshalIndent(shortLinks, "", "  ")
	if err != nil {
		log.Printf("[Storage] Error marshaling links: %v", err)
		return
	}

	// WriteFile handles file creation/truncation
	if err := os.WriteFile(linksFile, data, 0644); err != nil {
		log.Printf("[Storage] Error writing links file: %v", err)
	} else {
		log.Printf("[Storage] Successfully saved links to file.")
	}
}
