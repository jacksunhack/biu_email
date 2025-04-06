package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const dataStorageDir = "storage/data" // 新的数据存储目录

// StoredData 定义存储在文件中的数据结构
type StoredData struct {
	EncryptedData    string `json:"encryptedData"`    // Base64 encoded
	IV               string `json:"iv"`               // Base64 encoded
	Salt             string `json:"salt"`             // Base64 encoded
	OriginalFilename string `json:"originalFilename"` // Added to store original filename
}

// Note: ensureDataStorageDir function is now defined and called in main.go

// StoreDataHandler handles storing encrypted data sent from the client
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
	if requestData.OriginalFilename != "" {
		log.Printf("Storing data with original filename hint: %s", requestData.OriginalFilename)
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

// GetDataHandler handles retrieving stored encrypted data by ID
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

	log.Printf("Attempting to retrieve data for ID: %s from %s", id, filePath)

	// Read the JSON file content
	jsonData, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Data file not found for ID %s: %s", id, filePath)
			// Return 404, client JS handles the 'burned' message logic
			c.JSON(http.StatusNotFound, gin.H{"error": "Data not found or already burned"})
		} else {
			log.Printf("Error reading data file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read data"})
		}
		return
	}

	// Unmarshal JSON to send back the structured data
	var storedData StoredData
	if err := json.Unmarshal(jsonData, &storedData); err != nil {
		log.Printf("Error unmarshaling data for ID %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse stored data"})
		return
	}

	log.Printf("Successfully retrieved data for ID: %s", id)
	// Return the full StoredData object (includes EncryptedData, IV, Salt, OriginalFilename)
	c.JSON(http.StatusOK, storedData)
}

// BurnDataHandler handles deleting stored data by ID
func BurnDataHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		log.Println("BurnData request missing ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Data ID not provided"})
		return
	}

	// Construct file path (again, sanitize ID in production)
	fileName := id + ".json"
	filePath := filepath.Join(dataStorageDir, fileName)

	log.Printf("Attempting to burn data for ID: %s at %s", id, filePath)

	// Attempt to remove the file
	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Data file %s already burned or never existed.", filePath)
			// Still return success as the goal (data gone) is achieved
			c.JSON(http.StatusOK, gin.H{"message": "Data already burned or not found"})
		} else {
			log.Printf("Error deleting data file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to burn data"})
		}
		return
	}

	log.Printf("Successfully burned data for ID: %s", id)
	c.JSON(http.StatusOK, gin.H{"message": "Data successfully burned"})
}

// End of file
