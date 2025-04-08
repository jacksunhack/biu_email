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
	URL      string `json:"url"`      // Ensure JSON tags match if needed
	Accessed bool   `json:"accessed"` // Ensure JSON tags match if needed
}

var (
	shortLinks = make(map[string]ShortLink)
	mu         sync.RWMutex
	// Removed global storageDir and linksFile variables
)

// InitStorage initializes the short link storage by loading links from the configured path.
// It should be called once during application startup after config is loaded.
func InitStorage(config *Config) error {
	storageDir := config.Paths.DataStorageDir // Use configured path
	// linksFile := filepath.Join(storageDir, "shortlinks.json") // Removed, calculated within load/saveLinks

	// Ensure storage directory exists
	if err := os.MkdirAll(storageDir, 0750); err != nil { // Use more restrictive permissions
		log.Printf("[Storage] Error creating storage directory '%s': %v", storageDir, err)
		return fmt.Errorf("failed to ensure storage directory: %w", err)
	}
	log.Printf("[Storage] Ensured storage directory exists: %s", storageDir)

	// Load existing links
	return loadLinks(config) // Pass config to loadLinks
}

// SetShortLink adds or updates a short link and saves it to the file.
func SetShortLink(config *Config, code, url string) error { // Accept config
	mu.Lock()
	defer mu.Unlock()
	shortLinks[code] = ShortLink{
		URL:      url,
		Accessed: false,
	}
	return saveLinks(config) // Pass config to saveLinks
}

// GetShortLink retrieves a short link URL, marks it as accessed, and saves the updated state.
// It returns the URL, a boolean indicating if it was already accessed, and a boolean indicating if the code exists.
func GetShortLink(config *Config, code string) (string, bool, bool) { // Accept config
	mu.Lock() // Need write lock to mark as accessed and save
	defer mu.Unlock()

	link, exists := shortLinks[code]
	if !exists {
		log.Printf("[Storage] Code %s not found in map.", code)
		return "", false, false
	}

	// Check if already accessed
	if link.Accessed {
		log.Printf("[Storage] Link %s was already accessed", code)
		return link.URL, true, true // Return URL even if accessed, but indicate it was accessed
	}

	// Mark as accessed and save
	link.Accessed = true
	shortLinks[code] = link
	err := saveLinks(config) // Pass config to saveLinks
	if err != nil {
		// Log the error, but still return the link for this access attempt
		log.Printf("[Storage] ERROR saving link state after access for code %s: %v", code, err)
	}

	log.Printf("[Storage] Returning first-time access for code %s: %s", code, link.URL)
	return link.URL, false, true
}

// loadLinks loads the short links from the JSON file specified in the config.
func loadLinks(config *Config) error { // Accept config
	// Note: This function assumes the caller (InitStorage) holds the necessary lock if needed,
	// but since InitStorage is called once at startup, direct locking here is fine too.
	// Let's keep the lock here for clarity.
	mu.Lock()
	defer mu.Unlock()

	storageDir := config.Paths.DataStorageDir
	linksFile := filepath.Join(storageDir, "shortlinks.json")
	log.Println("[Storage] Attempting to load links from file:", linksFile)

	// Initialize map first
	shortLinks = make(map[string]ShortLink)

	data, err := os.ReadFile(linksFile) // Use os.ReadFile
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[Storage] Error reading links file '%s': %v. Starting with empty map.", linksFile, err)
			// Return the error instead of just logging
			return fmt.Errorf("error reading links file: %w", err)
		}
		log.Println("[Storage] Links file not found, starting with empty map.")
		return nil // File not existing is not an error for initialization
	}

	if len(data) == 0 {
		log.Println("[Storage] Links file is empty, starting with empty map.")
		return nil
	}

	// Try unmarshaling directly into the current format
	if err := json.Unmarshal(data, &shortLinks); err != nil {
		log.Printf("[Storage] Error unmarshaling links file '%s': %v. Starting with empty map.", linksFile, err)
		shortLinks = make(map[string]ShortLink) // Reset map on error
		// Return the unmarshaling error
		return fmt.Errorf("error parsing links file: %w", err)
	}

	log.Printf("[Storage] Successfully loaded %d short links from file '%s'.", len(shortLinks), linksFile)
	return nil
}

// saveLinks saves the current state of shortLinks to the JSON file specified in the config.
// This function assumes the caller holds the necessary write lock (mu.Lock).
func saveLinks(config *Config) error { // Accept config
	storageDir := config.Paths.DataStorageDir
	linksFile := filepath.Join(storageDir, "shortlinks.json")
	// log.Printf("[Storage] Attempting to save %d links to file: %s", len(shortLinks), linksFile) // Reduce log verbosity

	data, err := json.MarshalIndent(shortLinks, "", "  ")
	if err != nil {
		log.Printf("[Storage] ERROR marshaling links: %v", err)
		return fmt.Errorf("failed to marshal links: %w", err)
	}

	// WriteFile handles file creation/truncation
	if err := os.WriteFile(linksFile, data, 0640); err != nil { // Use more restrictive permissions
		log.Printf("[Storage] ERROR writing links file '%s': %v", linksFile, err)
		return fmt.Errorf("failed to write links file: %w", err)
	}
	// log.Printf("[Storage] Successfully saved links to file.") // Reduce log verbosity
	return nil
}

// Removed CheckStoragePermissions function as InitStorage handles directory creation.
