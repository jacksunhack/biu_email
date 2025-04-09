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

type Server struct {
	config *Config
}

type PasswordProtection struct {
	Data string `json:"data"`
	IV   string `json:"iv"`
	Salt string `json:"salt"`
}

// StoredData 定义旧的存储在文件中的数据结构 (主要用于文本模式)
type StoredData struct {
	EncryptedData      string              `json:"encryptedData"`    // Base64 encoded (Only for text mode now)
	IV                 string              `json:"iv"`               // Base64 encoded
	Salt               string              `json:"salt"`             // Base64 encoded
	OriginalFilename   string              `json:"originalFilename"` // Should be empty for text mode
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"`
}

// StoredMetadata 定义仅包含元数据的文件结构 (用于文件分片上传后)
// Added PasswordProtection field to handle password-protected files correctly.
type StoredMetadata struct {
	ID                 string              `json:"id"`
	IV                 string              `json:"iv"`
	Salt               string              `json:"salt"`
	OriginalFilename   string              `json:"originalFilename"`
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"` // Added this field
}

// StoreRequest 扩展请求结构以支持密码保护
type StoreRequest struct {
	EncryptedData      string              `json:"encryptedData"`
	IV                 string              `json:"iv"`
	Salt               string              `json:"salt"`
	OriginalFilename   string              `json:"originalFilename,omitempty"`
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"`
}

// Removed local isValidUUID function, will use IsValidUUID from utils.go

// Note: ensureDataStorageDir function is now defined and called in main.go

// StoreDataHandler handles storing encrypted data sent from the client
// StoreDataHandler handles storing encrypted TEXT data sent from the client (legacy/text mode)
// StoreDataHandler handles storing encrypted TEXT data sent from the client (legacy/text mode)
func StoreDataHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request StoreRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			log.Printf("[StoreData] Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式", "details": err.Error()})
			return
		}

		// 改进的数据验证
		if len(request.EncryptedData) == 0 {
			log.Println("[StoreData] Missing encryptedData")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少加密数据"})
			return
		}

		if len(request.IV) == 0 {
			log.Println("[StoreData] Missing IV")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少IV"})
			return
		}

		if len(request.Salt) == 0 {
			log.Println("[StoreData] Missing salt")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少salt"})
			return
		}

		// Generate unique ID
		id := uuid.New().String()
		fileName := id + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName)

		// 构建存储数据结构
		data := StoredData{
			EncryptedData:      request.EncryptedData,
			IV:                 request.IV,
			Salt:               request.Salt,
			PasswordProtection: request.PasswordProtection,
		}

		// 确保目录存在
		if err := os.MkdirAll(config.Paths.DataStorageDir, 0750); err != nil {
			log.Printf("[StoreData:%s] Error creating directory %s: %v", id, config.Paths.DataStorageDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法创建存储目录"})
			return
		}

		// 序列化数据
		jsonData, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			log.Printf("[StoreData:%s] Error marshaling JSON: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法序列化数据"})
			return
		}

		// 写入文件
		if err := os.WriteFile(filePath, jsonData, 0640); err != nil {
			log.Printf("[StoreData:%s] Error writing file %s: %v", id, filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "无法保存数据"})
			return
		}

		log.Printf("[StoreData:%s] Successfully stored data", id)
		c.JSON(http.StatusOK, gin.H{"id": id})
	}
}

// GetDataHandler handles retrieving stored METADATA (IV, Salt, OriginalFilename) by ID
// OR retrieving TEXT data (IV, Salt, EncryptedData)
func GetDataHandler(config *Config) gin.HandlerFunc {
	// Revised GetDataHandler logic based on biu/ reference and password protection integration
	return func(c *gin.Context) {
		id := c.Param("id")
		if !IsValidUUID(id) {
			log.Printf("[GetData] 无效的ID格式: %s", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的数据ID"})
			return
		}

		log.Printf("[GetData] 尝试获取数据，ID: %s", id)
		dataPath := filepath.Join(config.Paths.DataStorageDir, id+".json")

		log.Printf("[GetData] 尝试读取文件: %s", dataPath)
		jsonData, err := os.ReadFile(dataPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[GetData] 数据文件不存在: %s", dataPath)
				c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
				return
			}
			log.Printf("[GetData] 读取数据文件失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "读取数据失败"})
			return
		}

		// log.Printf("[GetData:%s] Raw JSON data read from file: %s", id, string(jsonData))

		// Attempt 1: Parse as StoredData (handles text, potentially password-protected)
		var textData StoredData
		errText := json.Unmarshal(jsonData, &textData)

		if errText == nil {
			// Successfully parsed as StoredData, now check specifics
			if textData.PasswordProtection != nil {
				// Password protected data (could be text or file, parsed as StoredData)
				log.Printf("[GetData:%s] 检测到密码保护的数据 (解析为 StoredData)", id)
				response := gin.H{
					"needPassword":       true,
					"iv":                 textData.IV,
					"salt":               textData.Salt,
					"passwordProtection": textData.PasswordProtection,
				}
				if textData.OriginalFilename != "" {
					response["originalFilename"] = textData.OriginalFilename
				}
				if textData.EncryptedData != "" {
					response["encryptedData"] = textData.EncryptedData
				}
				c.JSON(http.StatusOK, response)
				return
			} else if textData.EncryptedData != "" && textData.OriginalFilename == "" {
				// Non-password-protected text data
				log.Printf("[GetData:%s] 返回文本数据 (无密码)", id)
				c.JSON(http.StatusOK, gin.H{
					"iv":            textData.IV,
					"salt":          textData.Salt,
					"encryptedData": textData.EncryptedData,
				})
				return
			}
			// If it parsed as StoredData but doesn't fit above criteria,
			// it might be file metadata that partially matched. Proceed to Attempt 2.
			log.Printf("[GetData:%s] Parsed as StoredData but not valid text/password format. Trying StoredMetadata.", id)
		}

		// Attempt 2: Parse as StoredMetadata (handles files, potentially password-protected)
		var fileMeta StoredMetadata
		errFile := json.Unmarshal(jsonData, &fileMeta)

		if errFile == nil && fileMeta.OriginalFilename != "" {
			// Successfully parsed as StoredMetadata and has an original filename
			if fileMeta.PasswordProtection != nil {
				// Password-protected file metadata
				log.Printf("[GetData:%s] 检测到密码保护的文件元数据", id)
				c.JSON(http.StatusOK, gin.H{
					"needPassword":       true,
					"iv":                 fileMeta.IV,
					"salt":               fileMeta.Salt,
					"passwordProtection": fileMeta.PasswordProtection,
					"originalFilename":   fileMeta.OriginalFilename,
				})
				return
			} else {
				// Non-password-protected file metadata
				log.Printf("[GetData:%s] 返回文件元数据 (无密码)", id)
				c.JSON(http.StatusOK, gin.H{
					"iv":               fileMeta.IV,
					"salt":             fileMeta.Salt,
					"originalFilename": fileMeta.OriginalFilename,
				})
				return
			}
		}

		// Failure: Neither format matched correctly
		log.Printf("[GetData:%s] 无法确定数据类型或格式无效. RawData='%s', ParseTextErr=%v, ParseFileErr=%v", id, string(jsonData), errText, errFile)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "存储的数据格式无效"})
	}
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

// Removed redundant handleGetData function

// End of file
