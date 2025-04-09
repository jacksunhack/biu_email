package main

import (
	"encoding/json"
	"fmt" // Added for fmt.Sprintf

	// "io" // Removed unused import
	// "io/ioutil" // Replaced with os package functions
	"log"
	"net/http"
	"os"
	"path/filepath"

	"strings" // Import strings

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Removed const dataStorageDir and finalUploadDir, will use config values from *Config.
// StoredData 定义旧的存储在文件中的数据结构 (主要用于文本模式)
type StoredData struct {
	EncryptedData    string `json:"encryptedData"`    // Base64 encoded (Only for text mode now)
	IV               string `json:"iv"`               // Base64 encoded
	Salt             string `json:"salt"`             // Base64 encoded
	OriginalFilename string `json:"originalFilename"` // Should be empty for text mode
}

// StoredMetadata 定义仅包含元数据的文件结构 (用于文件分片上传后)
type StoredMetadata struct {
	ID               string `json:"id"`               // Corresponds to uploadId
	IV               string `json:"iv"`               // Base64 encoded
	Salt             string `json:"salt"`             // Base64 encoded
	OriginalFilename string `json:"originalFilename"` // Original filename from upload
}

// Removed local isValidUUID function, will use IsValidUUID from utils.go

// Note: ensureDataStorageDir function is now defined and called in main.go

// StoreDataHandler handles storing encrypted data sent from the client
// StoreDataHandler handles storing encrypted TEXT data sent from the client (legacy/text mode)
// StoreDataHandler handles storing encrypted TEXT data sent from the client (legacy/text mode)
func StoreDataHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		var requestData StoredData
		if err := c.ShouldBindJSON(&requestData); err != nil {
			log.Printf("[StoreData] Error binding JSON: %v", err)                                           // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()}) // Slightly clearer message
			return
		}

		// Basic validation
		if requestData.EncryptedData == "" || requestData.IV == "" || requestData.Salt == "" {
			log.Println("[StoreData] Missing required fields (encryptedData, iv, salt)")                                                // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields", "fields": []string{"encryptedData", "iv", "salt"}}) // More structured error
			return
		}

		// Log original filename if provided (useful for files)
		// OriginalFilename should not be present in text mode
		if requestData.OriginalFilename != "" {
			log.Printf("Warning: StoreDataHandler received OriginalFilename for text data? Filename: %s", requestData.OriginalFilename)
			// Decide if this is an error or just ignore it. Ignoring for now.
			requestData.OriginalFilename = "" // Ensure it's not saved for text
		}

		// Generate unique ID
		id := uuid.New().String()
		fileName := id + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName) // Use config path

		// log.Printf("Attempting to store data with ID: %s to %s", id, filePath) // Reduce verbose logging

		// Marshal the received data (including optional filename) to JSON
		jsonData, err := json.MarshalIndent(requestData, "", "  ")
		if err != nil {
			log.Printf("[StoreData:%s] Error marshaling data: %v", id, err)                                // Add context and ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error preparing data"}) // More generic internal error
			return
		}

		// Ensure the directory exists (although called in main, belt-and-suspenders)
		// Re-checking permissions/existence might be needed in a concurrent env
		if err := os.MkdirAll(config.Paths.DataStorageDir, 0750); err != nil { // Use config path
			log.Printf("[StoreData:%s] Error ensuring data storage directory '%s': %v", id, config.Paths.DataStorageDir, err) // Add ID
			// Don't necessarily fail here, WriteFile below might still work if dir exists
			// but logging the error is important.
		}

		// Write the JSON data to the file
		// Use os.WriteFile instead of ioutil.WriteFile
		if err := os.WriteFile(filePath, jsonData, 0640); err != nil { // Use more restrictive permissions
			log.Printf("[StoreData:%s] Error writing data file %s: %v", id, filePath, err)              // Add ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error saving data"}) // More generic internal error
			return
		}

		log.Printf("[StoreData:%s] Successfully stored text data", id) // Add context and ID
		c.JSON(http.StatusOK, gin.H{"id": id})
	} // Close returned handler
}

// GetDataHandler handles retrieving stored METADATA (IV, Salt, OriginalFilename) by ID
// It no longer returns the encrypted data itself.
// GetDataHandler handles retrieving stored METADATA (IV, Salt, OriginalFilename) or TEXT data by ID
func GetDataHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		id := c.Param("id")
		if id == "" {
			log.Println("[GetData] Request missing ID parameter")                          // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID parameter is required"}) // Clearer message
			return
		}
		// --- Path Traversal Mitigation ---
		// 1. Validate ID format
		if !IsValidUUID(id) { // Use shared function
			log.Printf("[GetData:%s] Invalid ID format received", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		// 2. Clean the ID part of the path (though UUID format should prevent '..')
		cleanID := filepath.Clean(id)
		if cleanID != id || strings.Contains(cleanID, "..") { // Double check after cleaning
			log.Printf("[GetData:%s] Potential path traversal detected after cleaning ID ('%s' -> '%s')", id, id, cleanID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		// --- End Mitigation ---

		// Construct file path - IMPORTANT: Sanitize ID to prevent path traversal
		// A simple check: ensure ID contains only expected characters (e.g., hex, dashes for UUID)
		// For UUIDs, a regex like ^[a-fA-F0-9-]+$ is reasonable.
		// For simplicity here, we assume UUID format is generated correctly.
		fileName := id + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName) // Use config path

		// log.Printf("Attempting to retrieve metadata for ID: %s from %s", id, filePath) // Reduce verbose logging

		// Read the JSON file content
		// Use os.ReadFile instead of ioutil.ReadFile
		jsonData, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[GetData:%s] Metadata file not found: %s", id, filePath) // Add context and ID
				// Return 404, client JS handles the 'burned' message logic
				c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found or already burned"})
			} else {
				log.Printf("[GetData:%s] Error reading metadata file %s: %v", id, filePath, err)                 // Add context and ID
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error reading metadata"}) // More generic internal error
			}
			return
		}

		// Log the raw data being processed
		log.Printf("[GetData:%s] Raw JSON data read from file: %s", id, string(jsonData))

		// Try unmarshaling as StoredData (text format) first
		var oldData StoredData
		errOld := json.Unmarshal(jsonData, &oldData)

		// Check if it parsed successfully AND contains the EncryptedData field (distinguishes text)
		if errOld == nil && oldData.EncryptedData != "" {
			log.Printf("[GetData:%s] Retrieved text data (legacy format)", id) // Add context and ID
			c.JSON(http.StatusOK, oldData)                                     // Return the object containing encryptedData, iv, salt
			return
		}

		// If it wasn't valid text data, try unmarshaling as StoredMetadata (file format)
		var metadata StoredMetadata
		errMeta := json.Unmarshal(jsonData, &metadata)
		log.Printf("[GetData:%s] Attempted parsing as StoredMetadata, error: %v", id, errMeta) // Log metadata parse attempt error

		// Check if it parsed successfully AND contains the OriginalFilename (distinguishes file metadata)
		if errMeta == nil && metadata.OriginalFilename != "" {
			log.Printf("[GetData:%s] Retrieved file metadata", id) // Add context and ID
			// Ensure the ID in the metadata matches the request ID (optional sanity check)
			if metadata.ID == "" { // If ID wasn't in the stored JSON, use the request ID
				metadata.ID = id
			} else if metadata.ID != id {
				log.Printf("[GetData:%s] Warning: Metadata ID mismatch (Metadata has ID: %s)", id, metadata.ID) // Add context and ID
				// Decide how to handle: error out, or trust request ID? Trusting request ID for now.
				metadata.ID = id
			}
			// Successfully parsed as file metadata, return it.
			c.JSON(http.StatusOK, metadata) // Return StoredMetadata struct
			return                          // IMPORTANT: Return after successful handling
		}

		// If neither format matched or key fields were missing, log errors and return failure
		log.Printf("[GetData:%s] Error determining data type. RawData='%s', ParseAsTextErr=%v, ParseAsMetaErr=%v", id, string(jsonData), errOld, errMeta) // Log raw data on final failure
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: Invalid stored data format"})                                       // Keep user-facing error generic but specific

		// Ensure the ID in the metadata matches the request ID (optional sanity check)
		if metadata.ID == "" { // If ID wasn't in the stored JSON, use the request ID
			metadata.ID = id
		} else if metadata.ID != id {
			// This block seems redundant after the previous metadata.ID check, consider removing or refactoring.
			// Keeping it for now, but adding context/ID to log.
			log.Printf("[GetData:%s] Warning: Metadata ID mismatch (after parsing attempts, Metadata has ID: %s)", id, metadata.ID)
			// Decide how to handle: error out, or trust request ID? Trusting request ID for now.
			metadata.ID = id
		}

		// Code should not reach here due to returns in the blocks above
	} // Close returned handler
}

// BurnDataHandler handles deleting stored metadata AND the corresponding merged file (if applicable)
// BurnDataHandler handles deleting stored metadata AND the corresponding merged file (if applicable)
func BurnDataHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		id := c.Param("id")
		if id == "" {
			log.Println("[BurnData] Request missing ID parameter")                         // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID parameter is required"}) // Clearer message
			return
		}
		// --- Path Traversal Mitigation ---
		if !IsValidUUID(id) { // Use shared function
			log.Printf("[BurnData:%s] Invalid ID format received", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		cleanID := filepath.Clean(id)
		if cleanID != id || strings.Contains(cleanID, "..") {
			log.Printf("[BurnData:%s] Potential path traversal detected after cleaning ID ('%s' -> '%s')", id, id, cleanID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		// --- End Mitigation ---

		// Construct file path (again, sanitize ID in production)
		metaFileName := id + ".json"
		metaFilePath := filepath.Join(config.Paths.DataStorageDir, metaFileName) // Use config path

		log.Printf("[BurnData:%s] Attempting to burn data (Metadata: %s)", id, metaFilePath) // Add context and ID

		// 1. Try to read metadata first to get original filename (needed for deleting merged file)
		var originalFilename string
		// Use os.ReadFile instead of ioutil.ReadFile
		jsonData, err := os.ReadFile(metaFilePath)
		if err == nil {
			var metadata StoredMetadata
			if json.Unmarshal(jsonData, &metadata) == nil {
				originalFilename = metadata.OriginalFilename
			} else {
				// Try old format for text data
				var oldData StoredData
				if json.Unmarshal(jsonData, &oldData) == nil {
					originalFilename = oldData.OriginalFilename // Will be empty for text
				} else {
					log.Printf("[BurnData:%s] Error unmarshaling metadata file %s, cannot determine original filename.", id, metaFilePath) // Add ID
					// Proceed to delete metadata file anyway, but cannot delete merged file if it exists.
				}
			}
		} else if !os.IsNotExist(err) {
			log.Printf("[BurnData:%s] Error reading metadata file %s before delete: %v", id, metaFilePath, err) // Add ID
			// Proceed to attempt delete anyway.
		}

		// 2. Attempt to remove the metadata file
		errMeta := os.Remove(metaFilePath)
		metaDeleted := false
		if errMeta == nil {
			log.Printf("[BurnData:%s] Successfully deleted metadata file: %s", id, metaFilePath) // Add ID
			metaDeleted = true
		} else if os.IsNotExist(errMeta) {
			log.Printf("[BurnData:%s] Metadata file %s already deleted or never existed.", id, metaFilePath) // Add ID
			metaDeleted = true                                                                               // Consider it deleted if not found
		} else {
			log.Printf("[BurnData:%s] Error deleting metadata file %s: %v", id, metaFilePath, errMeta) // Add ID
			// Don't return yet, try deleting the merged file if we have the name
		}

		// 3. Attempt to remove the merged file if originalFilename is known (i.e., it was likely a file upload)
		mergedFileDeleted := true // Assume deleted if not a file upload or filename unknown
		var errMerged error       // Declare errMerged outside the if block
		if originalFilename != "" {
			mergedFilePath := filepath.Join(config.Paths.FinalUploadDir, id, originalFilename)                // Use config path
			log.Printf("[BurnData:%s] Attempting to delete merged file directory for %s", id, mergedFilePath) // Add ID
			errMerged = os.RemoveAll(filepath.Join(config.Paths.FinalUploadDir, id))                          // Assign to the outer errMerged
			if errMerged == nil {
				log.Printf("[BurnData:%s] Successfully deleted merged file directory: %s", id, filepath.Join(config.Paths.FinalUploadDir, id)) // Add ID
				mergedFileDeleted = true
			} else if os.IsNotExist(errMerged) {
				log.Printf("[BurnData:%s] Merged file directory %s already deleted or never existed.", id, filepath.Join(config.Paths.FinalUploadDir, id)) // Add ID
				mergedFileDeleted = true
			} else {
				log.Printf("[BurnData:%s] Error deleting merged file directory %s: %v", id, filepath.Join(config.Paths.FinalUploadDir, id), errMerged) // Add ID
				mergedFileDeleted = false
			}
		}

		// 4. Determine final status
		if metaDeleted && mergedFileDeleted {
			c.JSON(http.StatusOK, gin.H{"message": "Data successfully burned"})
		} else {
			// Report error if either deletion failed (and the file wasn't already gone)
			errorMsg := "Failed to burn data completely."
			if !metaDeleted {
				errorMsg += " Metadata deletion failed."
			}
			if !mergedFileDeleted && originalFilename != "" {
				errorMsg += " Merged file deletion failed."
			}
			// Provide more details in the error response
			details := map[string]string{}
			if !metaDeleted {
				details["metadata_error"] = fmt.Sprintf("Failed to delete %s: %v", metaFilePath, errMeta)
			}
			if !mergedFileDeleted && originalFilename != "" {
				details["merged_file_error"] = fmt.Sprintf("Failed to delete directory %s: %v", filepath.Join(config.Paths.FinalUploadDir, id), errMerged)
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg, "details": details})
			return // Return after sending error
		}

		// If successful, only send JSON once. Removed redundant log/JSON call from original code.
	} // Close returned handler
}

// StoreMetadataHandler handles storing metadata after chunk upload completes
// StoreMetadataHandler handles storing metadata after chunk upload completes
func StoreMetadataHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		var metadata StoredMetadata
		if err := c.ShouldBindJSON(&metadata); err != nil {
			log.Printf("[StoreMetadata] Error binding JSON: %v", err)                                       // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()}) // Clearer message
			return
		}

		// Basic validation
		if metadata.ID == "" || metadata.IV == "" || metadata.Salt == "" || metadata.OriginalFilename == "" {
			log.Printf("[StoreMetadata:%s] Missing required fields (id, iv, salt, originalFilename)", metadata.ID)                               // Add context and ID if available
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields", "fields": []string{"id", "iv", "salt", "originalFilename"}}) // More structured error
			return
		}
		// --- Path Traversal Mitigation for ID in metadata ---
		// Validate the ID format received in the metadata payload
		if !IsValidUUID(metadata.ID) { // Use shared function
			log.Printf("[StoreMetadata] Invalid ID format received in metadata payload: %s", metadata.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format in payload"})
			return
		}
		cleanID := filepath.Clean(metadata.ID)
		if cleanID != metadata.ID || strings.Contains(cleanID, "..") {
			log.Printf("[StoreMetadata:%s] Potential path traversal detected after cleaning ID from payload ('%s' -> '%s')", metadata.ID, metadata.ID, cleanID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format in payload"})
			return
		}
		// --- End Mitigation ---

		// Construct metadata file path
		fileName := metadata.ID + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName) // Use config path

		// log.Printf("Attempting to store metadata for ID: %s to %s", metadata.ID, filePath) // Reduce verbose logging

		// Check if merged file exists before saving metadata (important!)
		mergedFilePath := filepath.Join(config.Paths.FinalUploadDir, metadata.ID, metadata.OriginalFilename) // Use config path
		if _, err := os.Stat(mergedFilePath); os.IsNotExist(err) {
			log.Printf("[StoreMetadata:%s] Error: Merged file %s not found. Cannot store metadata.", metadata.ID, mergedFilePath) // Add ID
			c.JSON(http.StatusPreconditionFailed, gin.H{"error": "Merged file not found, cannot save metadata."})                 // Use 412 Precondition Failed
			return
		} else if err != nil {
			log.Printf("[StoreMetadata:%s] Error checking merged file %s: %v", metadata.ID, mergedFilePath, err)         // Add ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error checking merged file status."}) // More generic internal error
			return
		}

		// Marshal the metadata to JSON
		jsonData, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			log.Printf("[StoreMetadata:%s] Error marshaling metadata: %v", metadata.ID, err)                   // Add ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error preparing metadata"}) // More generic internal error
			return
		}

		// Ensure the directory exists
		if err := os.MkdirAll(config.Paths.DataStorageDir, 0750); err != nil { // Use config path
			log.Printf("[StoreMetadata:%s] Error ensuring data storage directory '%s': %v", metadata.ID, config.Paths.DataStorageDir, err) // Add ID
			// Fail if we can't ensure the directory
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error ensuring storage directory"}) // More generic internal error
			return
		}

		// Write the JSON metadata to the file
		// Use os.WriteFile instead of ioutil.WriteFile
		if err := os.WriteFile(filePath, jsonData, 0640); err != nil {
			log.Printf("[StoreMetadata:%s] Error writing metadata file %s: %v", metadata.ID, filePath, err) // Add ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error saving metadata"}) // More generic internal error
			return
		}

		log.Printf("[StoreMetadata:%s] Successfully stored metadata", metadata.ID) // Add context and ID
		c.JSON(http.StatusOK, gin.H{"message": "Metadata successfully stored", "id": metadata.ID})
	} // Close returned handler
}

// DownloadHandler handles downloading the merged encrypted file
// DownloadHandler handles downloading the merged encrypted file
func DownloadHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		id := c.Param("id")
		if id == "" {
			log.Println("[Download] Request missing ID parameter")                         // Add context
			c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID parameter is required"}) // Clearer message
			return
		}
		// --- Path Traversal Mitigation ---
		if !IsValidUUID(id) { // Use shared function
			log.Printf("[Download:%s] Invalid ID format received", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		cleanID := filepath.Clean(id)
		if cleanID != id || strings.Contains(cleanID, "..") {
			log.Printf("[Download:%s] Potential path traversal detected after cleaning ID ('%s' -> '%s')", id, id, cleanID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}
		// --- End Mitigation ---

		// 1. Read metadata to get the original filename
		metaFileName := id + ".json"
		metaFilePath := filepath.Join(config.Paths.DataStorageDir, metaFileName) // Use config path
		// log.Printf("Download: Attempting to read metadata for ID: %s from %s", id, metaFilePath) // Reduce verbose logging

		// Use os.ReadFile instead of ioutil.ReadFile
		jsonData, err := os.ReadFile(metaFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[Download:%s] Metadata file not found: %s", id, metaFilePath) // Add context and ID
				c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found (file may be burned or text-only)"})
			} else {
				log.Printf("[Download:%s] Error reading metadata file %s: %v", id, metaFilePath, err)            // Add context and ID
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error reading metadata"}) // More generic internal error
			}
			return
		}

		var metadata StoredMetadata
		if err := json.Unmarshal(jsonData, &metadata); err != nil {
			// Could be text data (old format) which doesn't have a separate download file
			log.Printf("[Download:%s] Error unmarshaling metadata: %v. Assuming text data or invalid state.", id, err)     // Add context and ID
			c.JSON(http.StatusNotFound, gin.H{"error": "Cannot download: data format indicates text-only or is invalid."}) // Clearer message
			return
		}

		if metadata.OriginalFilename == "" {
			log.Printf("[Download:%s] Metadata does not contain an original filename. Assuming text data.", id)                 // Add context and ID
			c.JSON(http.StatusNotFound, gin.H{"error": "Cannot download: no file associated with this ID (likely text data)."}) // Clearer message
			return
		}

		// 2. Construct path to the merged file
		mergedFilePath := filepath.Join(config.Paths.FinalUploadDir, id, metadata.OriginalFilename) // Use config path
		// log.Printf("Download: Attempting to serve merged file: %s", mergedFilePath) // Reduce verbose logging

		// 3. Check if file exists
		fileInfo, err := os.Stat(mergedFilePath)
		if os.IsNotExist(err) {
			log.Printf("[Download:%s] Merged file not found: %s", id, mergedFilePath)                                  // Add context and ID
			c.JSON(http.StatusNotFound, gin.H{"error": "Cannot download: encrypted file not found (already burned?)"}) // Clearer message
			return
		} else if err != nil {
			log.Printf("[Download:%s] Error stating merged file %s: %v", id, mergedFilePath, err)                    // Add context and ID
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error accessing encrypted file"}) // More generic internal error
			return
		}

		// 4. Stream the file
		// Set headers for download
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.enc\"", metadata.OriginalFilename)) // Suggest adding .enc extension
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Use ServeFile for efficient streaming
		c.File(mergedFilePath)
		log.Printf("[Download:%s] Started streaming file %s", id, mergedFilePath) // Add context and ID

		// Note: After c.File(), you cannot reliably write JSON errors if streaming fails midway.
		// Gin handles logging errors during file serving internally to some extent.
	} // Close returned handler
}

// End of file
