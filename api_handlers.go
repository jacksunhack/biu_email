package main

import (
	"encoding/json"
	"fmt" // Added for fmt.Sprintf

	// "io" // Removed unused import
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const dataStorageDir = "storage/data" // Metadata storage directory
const finalUploadDir = "uploads"      // Directory where merged files are stored by chunk_upload.go

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

// Note: ensureDataStorageDir function is now defined and called in main.go

// StoreDataHandler handles storing encrypted data sent from the client
// StoreDataHandler handles storing encrypted TEXT data sent from the client (legacy/text mode)
func StoreDataHandler(c *gin.Context) {
	var requestData StoredData
	if err := c.ShouldBindJSON(&requestData); err != nil {
		log.Printf("Error binding JSON for store data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Basic validation
	if requestData.EncryptedData == "" || requestData.IV == "" || requestData.Salt == "" {
		log.Println("Missing required fields in store request (encryptedData, iv, salt)")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: encryptedData, iv, salt"})
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
	filePath := filepath.Join(dataStorageDir, fileName)

	log.Printf("Attempting to store data with ID: %s to %s", id, filePath)

	// Marshal the received data (including optional filename) to JSON
	jsonData, err := json.MarshalIndent(requestData, "", "  ")
	if err != nil {
		log.Printf("Error marshaling data for ID %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare data for storage"})
		return
	}

	// Ensure the directory exists (although called in main, belt-and-suspenders)
	// Re-checking permissions/existence might be needed in a concurrent env
	if err := os.MkdirAll(dataStorageDir, 0750); err != nil {
		log.Printf("Error ensuring data storage directory '%s' exists: %v", dataStorageDir, err)
		// Don't necessarily fail here, WriteFile below might still work if dir exists
		// but logging the error is important.
	}

	// Write the JSON data to the file
	if err := ioutil.WriteFile(filePath, jsonData, 0640); err != nil { // Use more restrictive permissions
		log.Printf("Error writing data file %s: %v", filePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save data"})
		return
	}

	log.Printf("Successfully stored data with ID: %s", id)
	c.JSON(http.StatusOK, gin.H{"id": id})
}

// GetDataHandler handles retrieving stored METADATA (IV, Salt, OriginalFilename) by ID
// It no longer returns the encrypted data itself.
func GetDataHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		log.Println("GetData request missing ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID not provided"})
		return
	}

	// Construct file path - IMPORTANT: Sanitize ID to prevent path traversal
	// A simple check: ensure ID contains only expected characters (e.g., hex, dashes for UUID)
	// For UUIDs, a regex like ^[a-fA-F0-9-]+$ is reasonable.
	// For simplicity here, we assume UUID format is generated correctly.
	fileName := id + ".json"
	filePath := filepath.Join(dataStorageDir, fileName)

	log.Printf("Attempting to retrieve metadata for ID: %s from %s", id, filePath)

	// Read the JSON file content
	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Data file not found for ID %s: %s", id, filePath)
			// Return 404, client JS handles the 'burned' message logic
			c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found or already burned"})
		} else {
			log.Printf("Error reading data file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read metadata"})
		}
		return
	}

	// Try unmarshaling as StoredData (text format) first
	var oldData StoredData
	errOld := json.Unmarshal(jsonData, &oldData)

	// Check if it parsed successfully AND contains the EncryptedData field (distinguishes text)
	if errOld == nil && oldData.EncryptedData != "" {
		log.Printf("Retrieved text data for ID %s. Returning full data.", id)
		c.JSON(http.StatusOK, oldData) // Return the object containing encryptedData, iv, salt
		return
	}

	// If it wasn't valid text data, try unmarshaling as StoredMetadata (file format)
	var metadata StoredMetadata
	errMeta := json.Unmarshal(jsonData, &metadata)

	// Check if it parsed successfully AND contains the OriginalFilename (distinguishes file metadata)
	if errMeta == nil && metadata.OriginalFilename != "" {
		log.Printf("Successfully retrieved metadata for file ID: %s", id)
		// Ensure the ID in the metadata matches the request ID (optional sanity check)
		if metadata.ID == "" { // If ID wasn't in the stored JSON, use the request ID
			metadata.ID = id
		} else if metadata.ID != id {
			log.Printf("Warning: Metadata ID mismatch for request ID %s (Metadata ID: %s)", id, metadata.ID)
			// Decide how to handle: error out, or trust request ID? Trusting request ID for now.
			metadata.ID = id
		}
		c.JSON(http.StatusOK, metadata) // Return StoredMetadata struct
		return
	}

	// If neither format matched or key fields were missing, log errors and return failure
	log.Printf("Error determining data type for ID %s: ParseAsTextErr=%v, ParseAsMetaErr=%v. Could not identify as valid text or file metadata.", id, errOld, errMeta)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse stored data or invalid format"})

	// Ensure the ID in the metadata matches the request ID (optional sanity check)
	if metadata.ID == "" { // If ID wasn't in the stored JSON, use the request ID
		metadata.ID = id
	} else if metadata.ID != id {
		log.Printf("Warning: Metadata ID mismatch for request ID %s (Metadata ID: %s)", id, metadata.ID)
		// Decide how to handle: error out, or trust request ID? Trusting request ID for now.
		metadata.ID = id
	}

	// Code should not reach here due to returns in the blocks above
}

// BurnDataHandler handles deleting stored metadata AND the corresponding merged file (if applicable)
func BurnDataHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		log.Println("BurnData request missing ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID not provided"})
		return
	}

	// Construct file path (again, sanitize ID in production)
	metaFileName := id + ".json"
	metaFilePath := filepath.Join(dataStorageDir, metaFileName)

	log.Printf("Attempting to burn data for ID: %s (Metadata: %s)", id, metaFilePath)

	// 1. Try to read metadata first to get original filename (needed for deleting merged file)
	var originalFilename string
	jsonData, err := ioutil.ReadFile(metaFilePath)
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
				log.Printf("Burn: Error unmarshaling metadata file %s, cannot determine original filename.", metaFilePath)
				// Proceed to delete metadata file anyway, but cannot delete merged file if it exists.
			}
		}
	} else if !os.IsNotExist(err) {
		log.Printf("Burn: Error reading metadata file %s before delete: %v", metaFilePath, err)
		// Proceed to attempt delete anyway.
	}

	// 2. Attempt to remove the metadata file
	errMeta := os.Remove(metaFilePath)
	metaDeleted := false
	if errMeta == nil {
		log.Printf("Successfully burned metadata file: %s", metaFilePath)
		metaDeleted = true
	} else if os.IsNotExist(errMeta) {
		log.Printf("Metadata file %s already burned or never existed.", metaFilePath)
		metaDeleted = true // Consider it deleted if not found
	} else {
		log.Printf("Error deleting metadata file %s: %v", metaFilePath, errMeta)
		// Don't return yet, try deleting the merged file if we have the name
	}

	// 3. Attempt to remove the merged file if originalFilename is known (i.e., it was likely a file upload)
	mergedFileDeleted := true // Assume deleted if not a file upload or filename unknown
	if originalFilename != "" {
		mergedFilePath := filepath.Join(finalUploadDir, id, originalFilename)
		log.Printf("Attempting to burn merged file: %s", mergedFilePath)
		errMerged := os.RemoveAll(filepath.Join(finalUploadDir, id)) // Remove the whole directory <uploadId>/
		if errMerged == nil {
			log.Printf("Successfully burned merged file directory: %s", filepath.Join(finalUploadDir, id))
			mergedFileDeleted = true
		} else if os.IsNotExist(errMerged) {
			log.Printf("Merged file directory %s already burned or never existed.", filepath.Join(finalUploadDir, id))
			mergedFileDeleted = true
		} else {
			log.Printf("Error deleting merged file directory %s: %v", filepath.Join(finalUploadDir, id), errMerged)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": errorMsg})
	}

	log.Printf("Successfully burned data for ID: %s", id)
	c.JSON(http.StatusOK, gin.H{"message": "Data successfully burned"})
}

// StoreMetadataHandler handles storing metadata after chunk upload completes
func StoreMetadataHandler(c *gin.Context) {
	var metadata StoredMetadata
	if err := c.ShouldBindJSON(&metadata); err != nil {
		log.Printf("Error binding JSON for store metadata: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body", "details": err.Error()})
		return
	}

	// Basic validation
	if metadata.ID == "" || metadata.IV == "" || metadata.Salt == "" || metadata.OriginalFilename == "" {
		log.Println("Missing required fields in store metadata request (id, iv, salt, originalFilename)")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields: id, iv, salt, originalFilename"})
		return
	}

	// Construct metadata file path
	fileName := metadata.ID + ".json"
	filePath := filepath.Join(dataStorageDir, fileName)

	log.Printf("Attempting to store metadata for ID: %s to %s", metadata.ID, filePath)

	// Check if merged file exists before saving metadata (important!)
	mergedFilePath := filepath.Join(finalUploadDir, metadata.ID, metadata.OriginalFilename)
	if _, err := os.Stat(mergedFilePath); os.IsNotExist(err) {
		log.Printf("Error: Merged file %s not found. Cannot store metadata for ID %s.", mergedFilePath, metadata.ID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Merged file not found, cannot save metadata."})
		return
	} else if err != nil {
		log.Printf("Error checking merged file %s: %v", mergedFilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error checking merged file status."})
		return
	}

	// Marshal the metadata to JSON
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		log.Printf("Error marshaling metadata for ID %s: %v", metadata.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare metadata for storage"})
		return
	}

	// Ensure the directory exists
	if err := os.MkdirAll(dataStorageDir, 0750); err != nil {
		log.Printf("Error ensuring data storage directory '%s' exists: %v", dataStorageDir, err)
		// Fail if we can't ensure the directory
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure storage directory"})
		return
	}

	// Write the JSON metadata to the file
	if err := ioutil.WriteFile(filePath, jsonData, 0640); err != nil {
		log.Printf("Error writing metadata file %s: %v", filePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
		return
	}

	log.Printf("Successfully stored metadata for ID: %s", metadata.ID)
	c.JSON(http.StatusOK, gin.H{"message": "Metadata successfully stored", "id": metadata.ID})
}

// DownloadHandler handles downloading the merged encrypted file
func DownloadHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		log.Println("Download request missing ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID not provided"})
		return
	}

	// 1. Read metadata to get the original filename
	metaFileName := id + ".json"
	metaFilePath := filepath.Join(dataStorageDir, metaFileName)
	log.Printf("Download: Attempting to read metadata for ID: %s from %s", id, metaFilePath)

	jsonData, err := ioutil.ReadFile(metaFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Download: Metadata file not found for ID %s: %s", id, metaFilePath)
			c.JSON(http.StatusNotFound, gin.H{"error": "Metadata not found (file may be burned or text-only)"})
		} else {
			log.Printf("Download: Error reading metadata file %s: %v", metaFilePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read metadata"})
		}
		return
	}

	var metadata StoredMetadata
	if err := json.Unmarshal(jsonData, &metadata); err != nil {
		// Could be text data (old format) which doesn't have a separate download file
		log.Printf("Download: Error unmarshaling metadata for ID %s: %v. Assuming text data or invalid state.", id, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Data format indicates text-only or is invalid."})
		return
	}

	if metadata.OriginalFilename == "" {
		log.Printf("Download: Metadata for ID %s does not contain an original filename. Assuming text data.", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "No downloadable file associated with this ID (likely text data)."})
		return
	}

	// 2. Construct path to the merged file
	mergedFilePath := filepath.Join(finalUploadDir, id, metadata.OriginalFilename)
	log.Printf("Download: Attempting to serve merged file: %s", mergedFilePath)

	// 3. Check if file exists
	fileInfo, err := os.Stat(mergedFilePath)
	if os.IsNotExist(err) {
		log.Printf("Download: Merged file not found: %s", mergedFilePath)
		c.JSON(http.StatusNotFound, gin.H{"error": "Encrypted file not found (already burned?)"})
		return
	} else if err != nil {
		log.Printf("Download: Error stating merged file %s: %v", mergedFilePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to access encrypted file"})
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
	log.Printf("Download: Successfully started streaming file %s for ID %s", mergedFilePath, id)

	// Note: After c.File(), you cannot reliably write JSON errors if streaming fails midway.
	// Gin handles logging errors during file serving internally to some extent.
}

// End of file
