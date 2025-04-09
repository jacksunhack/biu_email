package main

import (
	"log"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// isValidUUID checks if the provided string is a valid UUID and doesn't contain path traversal characters.
// Used for data IDs (text or file metadata).
func IsValidUUID(id string) bool {
	// Basic check for path traversal characters
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		log.Printf("[Validation] Invalid characters found in UUID: %s", id)
		return false
	}
	// Try parsing as UUID using the imported library
	_, err := uuid.Parse(id)
	if err != nil {
		log.Printf("[Validation] Invalid UUID format: %s, Error: %v", id, err)
	}
	return err == nil
}

// isValidUploadID checks if the provided string is a valid upload ID format (e.g., hex)
// and doesn't contain path traversal characters. Used for chunk upload IDs.
func IsValidUploadID(id string) bool {
	// Basic check for path traversal characters
	if strings.Contains(id, "..") || strings.Contains(id, "/") || strings.Contains(id, "\\") {
		log.Printf("[Validation] Invalid characters found in upload ID: %s", id)
		return false
	}
	// Check if it looks like a hex string (assuming MD5 hash length)
	// Note: This assumes upload IDs are always 32-char hex strings. Adjust if format changes.
	if len(id) != 32 {
		log.Printf("[Validation] Invalid length for upload ID: %s (expected 32)", id)
		return false
	}
	// Use a precompiled regex for efficiency if called frequently
	// var validHexRegex = regexp.MustCompile(`^[a-fA-F0-9]+$`)
	// return validHexRegex.MatchString(id)
	// For one-off checks, MatchString is fine:
	match, _ := regexp.MatchString(`^[a-fA-F0-9]+$`, id) // Use regexp.MatchString
	if !match {
		log.Printf("[Validation] Invalid hex format for upload ID: %s", id)
	}
	return match
}
