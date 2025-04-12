package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time" // Ensure time is imported

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

// StoredData 定义存储在文件中的数据结构 (文本或文件元数据)
type StoredData struct {
	EncryptedData      string              `json:"encryptedData,omitempty"`      // Base64 encoded (Only for text mode)
	IV                 string              `json:"iv"`                           // Base64 encoded
	Salt               string              `json:"salt"`                         // Base64 encoded
	OriginalFilename   string              `json:"originalFilename,omitempty"`   // Original name of the uploaded file
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"` // Optional password protection details
	ExpiresAt          *time.Time          `json:"expiresAt,omitempty"`          // Primary expiration time
	AccessWindowEndsAt *time.Time          `json:"accessWindowEndsAt,omitempty"` // Access window expiration (set on first access)
	ContentType        string              `json:"contentType,omitempty"`        // MIME type of the content
	FileSize           int64               `json:"fileSize,omitempty"`           // Size of the final merged file (for files)
	FirstAccessedTime  *time.Time          `json:"firstAccessedTime,omitempty"`  // Timestamp of first access (for access window calculation)
}

// StoredMetadata 定义仅包含元数据的文件结构 (用于文件分片上传后)
// 注意: 这个结构体现在与 StoredData 合并，因为字段基本重叠。
// 保留 StoredMetadata 类型别名以兼容旧代码，但内部使用 StoredData。
type StoredMetadata = StoredData // 使用类型别名

// StoreRequest 扩展请求结构以支持密码保护和有效期设置 (用于 /api/store - 文本模式)
type StoreRequest struct {
	EncryptedData      string              `json:"encryptedData"` // Required for text mode
	IV                 string              `json:"iv"`
	Salt               string              `json:"salt"`
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"`
	SetDuration        string              `json:"setDuration,omitempty"` // User-selected duration (e.g., "1h", "24h")
	ContentType        string              `json:"contentType,omitempty"` // Optional: Specify content type (e.g., text/markdown, text/x-python). Defaults to text/plain if empty.
}

// StoreMetadataRequest 定义 /api/store/metadata 的请求结构 (文件模式完成时)
type StoreMetadataRequest struct {
	ID                 string              `json:"id"` // Upload ID becomes the data ID
	IV                 string              `json:"iv"`
	Salt               string              `json:"salt"`
	OriginalFilename   string              `json:"originalFilename"`
	PasswordProtection *PasswordProtection `json:"passwordProtection,omitempty"`
	SetDuration        string              `json:"setDuration,omitempty"` // User-selected duration
	ContentType        string              `json:"contentType"`           // MIME type detected by client or server
	FileSize           int64               `json:"fileSize"`              // Size of the final merged file
}

// calculateExpirationTime 根据配置和用户选择计算主有效期时间
func calculateExpirationTime(config *Config, userDurationStr string) (*time.Time, error) {
	if !config.Expiration.Enabled {
		// Expiration disabled, set a very far future time (or return nil if preferred)
		farFuture := time.Now().AddDate(100, 0, 0) // ~100 years
		return &farFuture, nil
	}

	var durationStr string
	if config.Expiration.Mode == "forced" {
		durationStr = config.Expiration.DefaultDuration
	} else if config.Expiration.Mode == "free" {
		if userDurationStr != "" {
			// Attempt to parse the user-provided duration directly
			parsedDuration, err := time.ParseDuration(userDurationStr)
			if err != nil {
				log.Printf("Invalid duration format '%s' provided by client: %v. Using default: %s", userDurationStr, err, config.Expiration.DefaultDuration)
				durationStr = config.Expiration.DefaultDuration
				// Optionally return an error:
				// return nil, fmt.Errorf("无效的有效期格式: %s", userDurationStr)
			} else if parsedDuration <= 0 {
				log.Printf("Non-positive duration '%s' provided by client. Using default: %s", userDurationStr, config.Expiration.DefaultDuration)
				durationStr = config.Expiration.DefaultDuration
				// Optionally return an error:
				// return nil, fmt.Errorf("有效期必须为正数: %s", userDurationStr)
			} else {
				// Optional: Add a maximum duration check if needed
				// maxAllowedDuration := 365 * 24 * time.Hour // Example: 1 year
				// if parsedDuration > maxAllowedDuration {
				// 	log.Printf("Duration '%s' exceeds maximum allowed. Using default: %s", userDurationStr, config.Expiration.DefaultDuration)
				// 	durationStr = config.Expiration.DefaultDuration
				//  // Optionally return an error:
				//  // return nil, fmt.Errorf("有效期超过最大限制")
				// } else {
				durationStr = userDurationStr // Use the valid custom duration
				// }
			}
		} else {
			// If user didn't provide one, use default
			log.Printf("No duration provided by client. Using default: %s", config.Expiration.DefaultDuration)
			durationStr = config.Expiration.DefaultDuration
		}
	} else {
		// Should not happen due to config validation, but handle defensively
		log.Printf("CRITICAL: Invalid expiration mode '%s' found despite validation. Using default duration.", config.Expiration.Mode)
		durationStr = config.Expiration.DefaultDuration
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		// This should ideally not happen due to config validation, but handle defensively
		log.Printf("CRITICAL: Failed to parse configured duration '%s': %v. Using default 24h.", durationStr, err)
		duration = 24 * time.Hour
		// Return the error if strict handling is needed:
		// return nil, fmt.Errorf("无法解析有效期 '%s': %w", durationStr, err)
	}

	expirationTime := time.Now().Add(duration)
	return &expirationTime, nil
}

// StoreDataHandler handles storing encrypted TEXT data sent from the client
func StoreDataHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request StoreRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			log.Printf("[StoreData] Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式", "details": err.Error()})
			return
		}

		// Validate required fields for text mode
		if request.EncryptedData == "" {
			log.Println("[StoreData] Missing encryptedData")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少加密数据"})
			return
		}
		if request.IV == "" {
			log.Println("[StoreData] Missing IV")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少IV"})
			return
		}
		if request.Salt == "" {
			log.Println("[StoreData] Missing salt")
			c.JSON(http.StatusBadRequest, gin.H{"error": "缺少salt"})
			return
		}

		// Generate unique ID
		id := uuid.New().String()
		fileName := id + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName)

		// --- Expiration Logic ---
		expirationTimePtr, err := calculateExpirationTime(config, request.SetDuration)
		if err != nil {
			// Handle error during duration calculation (e.g., invalid user input if not falling back)
			log.Printf("[StoreData:%s] Error calculating expiration: %v", id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("计算有效期失败: %v", err)})
			return
		}
		log.Printf("[StoreData:%s] Calculated expiration time: %v", id, expirationTimePtr)
		// --- End Expiration Logic ---

		// 构建存储数据结构
		data := StoredData{
			EncryptedData:      request.EncryptedData,
			IV:                 request.IV,
			Salt:               request.Salt,
			PasswordProtection: request.PasswordProtection,
			ExpiresAt:          expirationTimePtr, // Store calculated expiration time pointer
			ContentType:        "text/plain",      // Default ContentType for text data
			// File specific fields (OriginalFilename, FileSize) are empty for text
			// Access window fields (FirstAccessedTime, AccessWindowEndsAt) are nil initially
		}
		// Use ContentType from request if provided
		if request.ContentType != "" {
			// Basic validation/sanitization could be added here if needed
			data.ContentType = request.ContentType
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

		log.Printf("[StoreData:%s] Successfully stored text data", id)
		c.JSON(http.StatusOK, gin.H{"id": id})
	}
}

// GetDataHandler handles retrieving stored data (text or file metadata) by ID,
// applying expiration checks (primary and access window).
func GetDataHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !IsValidUUID(id) {
			log.Printf("[GetData:%s] Invalid ID format received.", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的数据ID"})
			return
		}

		dataPath := filepath.Join(config.Paths.DataStorageDir, id+".json")
		log.Printf("[GetData:%s] Attempting to read metadata file: %s", id, dataPath)

		jsonData, err := os.ReadFile(dataPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[GetData:%s] Metadata file not found (likely burned or invalid ID): %s", id, dataPath)
				c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
			} else {
				log.Printf("[GetData:%s] Error reading metadata file %s: %v", id, dataPath, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取数据失败"})
			}
			return
		}

		// Unmarshal into the unified StoredData struct
		var metadata StoredData
		if err := json.Unmarshal(jsonData, &metadata); err != nil {
			log.Printf("[GetData:%s] Error unmarshaling metadata from %s: %v", id, dataPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "存储的数据格式无效"})
			return
		}

		now := time.Now()
		needsUpdate := false // Flag to indicate if metadata file needs to be rewritten

		// --- Primary Expiration Check ---
		if metadata.ExpiresAt != nil && now.After(*metadata.ExpiresAt) {
			log.Printf("[GetData:%s] Primary expiration time (%s) has passed. Burning data.", id, (*metadata.ExpiresAt).Format(time.RFC3339))
			go burnData(config, id) // Burn in background
			c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
			return
		}

		// --- Access Window Logic ---
		if config.Expiration.Enabled && config.Expiration.AccessWindow.Enabled {
			if metadata.FirstAccessedTime == nil {
				// First access: Calculate and set access window expiry
				log.Printf("[GetData:%s] First access detected. Calculating access window.", id)

				isTextData := metadata.OriginalFilename == "" // Determine if it's text data
				fileExt := ""
				if !isTextData {
					fileExt = strings.ToLower(filepath.Ext(metadata.OriginalFilename))
					if len(fileExt) > 0 {
						fileExt = fileExt[1:] // Remove leading dot
					}
				}
				fileSizeMB := float64(metadata.FileSize) / (1024 * 1024) // FileSize is 0 for text
				accessWindowDurationStr := config.Expiration.AccessWindow.DefaultDuration

				// Find matching rule
				for _, rule := range config.Expiration.AccessWindow.Rules {
					typeMatch := false
					for _, t := range rule.Type {
						// Handle wildcard, specific extension match, or 'text' type
						if t == "*" || (t == "text" && isTextData) || (!isTextData && t == fileExt) {
							typeMatch = true
							break
						}
					}
					if !typeMatch {
						continue
					}

					sizeMatch := true // Assume size matches unless rule specifies otherwise
					if !isTextData {  // Size rules only apply to files
						if rule.MinSizeMB > 0 && fileSizeMB < float64(rule.MinSizeMB) {
							sizeMatch = false
						}
						if rule.MaxSizeMB > 0 && fileSizeMB >= float64(rule.MaxSizeMB) {
							sizeMatch = false
						}
					} else if rule.MinSizeMB > 0 || rule.MaxSizeMB > 0 {
						// If size limits are set on a rule matching 'text', it won't apply
						sizeMatch = false
					}

					if sizeMatch {
						accessWindowDurationStr = rule.Duration
						log.Printf("[GetData:%s] Matched access window rule: Type=%v, SizeMB=%.2f -> Duration=%s", id, rule.Type, fileSizeMB, rule.Duration)
						break // Use first matching rule
					}
				}

				accessWindowDuration, err := time.ParseDuration(accessWindowDurationStr)
				if err != nil {
					log.Printf("[GetData:%s] CRITICAL: Failed to parse access window duration '%s': %v. Using default.", id, accessWindowDurationStr, err)
					// Attempt to parse default duration as fallback
					defaultAccessDur, defaultErr := time.ParseDuration(config.Expiration.AccessWindow.DefaultDuration)
					if defaultErr != nil {
						log.Printf("[GetData:%s] CRITICAL: Failed to parse DEFAULT access window duration '%s': %v. Using 10m.", id, config.Expiration.AccessWindow.DefaultDuration, defaultErr)
						defaultAccessDur = 10 * time.Minute // Absolute fallback
					}
					accessWindowDuration = defaultAccessDur
				}

				calculatedAccessWindowEnd := now.Add(accessWindowDuration)
				finalAccessWindowEnd := calculatedAccessWindowEnd
				// Ensure access window doesn't exceed primary expiry
				if metadata.ExpiresAt != nil && metadata.ExpiresAt.Before(calculatedAccessWindowEnd) {
					finalAccessWindowEnd = *metadata.ExpiresAt
					log.Printf("[GetData:%s] Access window expiry (%s) capped by primary expiry (%s)", id, calculatedAccessWindowEnd.Format(time.RFC3339), finalAccessWindowEnd.Format(time.RFC3339))
				}

				// Update metadata in memory
				metadata.FirstAccessedTime = &now
				metadata.AccessWindowEndsAt = &finalAccessWindowEnd
				needsUpdate = true // Mark for rewrite

				log.Printf("[GetData:%s] Access window set. FirstAccess: %s, AccessWindowEndsAt: %s", id, now.Format(time.RFC3339), finalAccessWindowEnd.Format(time.RFC3339))

			} else {
				// Subsequent access: Check if access window has expired
				if metadata.AccessWindowEndsAt != nil && now.After(*metadata.AccessWindowEndsAt) {
					log.Printf("[GetData:%s] Access window expired at %s. Burning data.", id, (*metadata.AccessWindowEndsAt).Format(time.RFC3339))
					go burnData(config, id) // Burn in background
					c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
					return
				}
				// Log subsequent access within window (optional)
				// log.Printf("[GetData:%s] Subsequent access within window. AccessWindowEndsAt: %s", id, (*metadata.AccessWindowEndsAt).Format(time.RFC3339))
			}
		}

		// --- Update Metadata File if Necessary ---
		if needsUpdate {
			updatedJsonData, marshalErr := json.MarshalIndent(metadata, "", "  ") // Marshal the updated StoredData
			if marshalErr != nil {
				log.Printf("[GetData:%s] CRITICAL: Failed to marshal updated metadata for saving: %v", id, marshalErr)
				// Don't fail the request, but log the error. The access window won't be persisted.
			} else {
				writeErr := os.WriteFile(dataPath, updatedJsonData, 0640)
				if writeErr != nil {
					log.Printf("[GetData:%s] CRITICAL: Failed to write updated metadata file %s: %v", id, dataPath, writeErr)
					// Don't fail the request, but log the error.
				} else {
					log.Printf("[GetData:%s] Successfully updated metadata file with access times.", id)
				}
			}
		}

		// --- Prepare and Send Response ---
		response := gin.H{
			"iv":          metadata.IV,
			"salt":        metadata.Salt,
			"contentType": metadata.ContentType, // Include ContentType
		}
		if metadata.PasswordProtection != nil {
			response["needPassword"] = true
			response["passwordProtection"] = metadata.PasswordProtection
		}

		isTextData := metadata.OriginalFilename == ""
		if !isTextData { // File data
			response["originalFilename"] = metadata.OriginalFilename
			log.Printf("[GetData:%s] Returning metadata for file: %s", id, metadata.OriginalFilename)
		} else { // Text data
			response["encryptedData"] = metadata.EncryptedData
			log.Printf("[GetData:%s] Returning encrypted text data.", id)
		}

		c.JSON(http.StatusOK, response)
	}
}

// burnData 销毁数据文件和相关资源
func burnData(config *Config, id string) error {
	log.Printf("[BurnData:%s] Starting burn process.", id)
	// 删除元数据文件
	metaFilePath := filepath.Join(config.Paths.DataStorageDir, id+".json")
	log.Printf("[BurnData:%s] Attempting to remove metadata file: %s", id, metaFilePath)
	errMeta := os.Remove(metaFilePath)
	if errMeta != nil && !os.IsNotExist(errMeta) {
		log.Printf("[BurnData:%s] Failed to remove metadata file: %v", id, errMeta)
		// Continue to attempt deleting other files
	} else if errMeta == nil {
		log.Printf("[BurnData:%s] Successfully removed metadata file.", id)
	} else { // err is os.IsNotExist
		log.Printf("[BurnData:%s] Metadata file did not exist.", id)
	}

	// 删除上传目录（如果存在），带重试逻辑
	uploadDir := filepath.Join(config.Paths.FinalUploadDir, id)
	log.Printf("[BurnData:%s] Attempting to remove upload directory with retries: %s", id, uploadDir)
	var errUpload error
	maxRetries := 5
	retryDelay := 1 * time.Second
	for i := 0; i < maxRetries; i++ {
		errUpload = os.RemoveAll(uploadDir)
		if errUpload == nil || os.IsNotExist(errUpload) {
			break // Success or directory doesn't exist
		}
		log.Printf("[BurnData:%s] Attempt %d/%d: Failed to remove upload directory: %v. Retrying after %v...", id, i+1, maxRetries, errUpload, retryDelay)
		time.Sleep(retryDelay)
	}

	if errUpload != nil && !os.IsNotExist(errUpload) {
		log.Printf("[BurnData:%s] Failed to remove upload directory after %d retries: %v", id, maxRetries, errUpload)
		// Continue
	} else if errUpload == nil {
		log.Printf("[BurnData:%s] Successfully removed upload directory.", id)
	} else { // err is os.IsNotExist
		log.Printf("[BurnData:%s] Upload directory did not exist.", id)
	}

	// 删除临时文件目录（如果存在）
	tempDir := filepath.Join(config.Paths.TempChunkDir, id)
	log.Printf("[BurnData:%s] Attempting to remove temporary directory: %s", id, tempDir)
	errTemp := os.RemoveAll(tempDir)
	if errTemp != nil && !os.IsNotExist(errTemp) {
		log.Printf("[BurnData:%s] Failed to remove temporary directory: %v", id, errTemp)
		// Continue
	} else if errTemp == nil {
		log.Printf("[BurnData:%s] Successfully removed temporary directory.", id)
	} else { // err is os.IsNotExist
		log.Printf("[BurnData:%s] Temporary directory did not exist.", id)
	}

	// Return the first significant error encountered
	if errMeta != nil && !os.IsNotExist(errMeta) {
		log.Printf("[BurnData:%s] Burn process completed with error (metadata).", id)
		return fmt.Errorf("failed to remove metadata file: %w", errMeta)
	}
	if errUpload != nil && !os.IsNotExist(errUpload) {
		log.Printf("[BurnData:%s] Burn process completed with error (upload dir).", id)
		return fmt.Errorf("failed to remove upload directory: %w", errUpload)
	}
	if errTemp != nil && !os.IsNotExist(errTemp) {
		log.Printf("[BurnData:%s] Burn process completed with error (temp dir).", id)
		return fmt.Errorf("failed to remove temporary directory: %w", errTemp)
	}

	log.Printf("[BurnData:%s] Burn process completed successfully.", id)
	return nil
}

// BurnDataHandler handles deleting stored metadata AND the corresponding merged file (if applicable)
func BurnDataHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !IsValidUUID(id) { // Use shared function and check format first
			log.Printf("[BurnData:%s] Invalid ID format received", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}

		// Call the burnData function
		err := burnData(config, id) // Call the actual burning logic

		if err != nil {
			// Log the specific error from burnData
			log.Printf("[BurnData:%s] Burn process failed: %v", id, err)
			// Return a generic server error to the client
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to burn data completely."})
		} else {
			// Success
			log.Printf("[BurnData:%s] Data successfully burned via API request.", id)
			c.JSON(http.StatusOK, gin.H{"message": "Data successfully burned"})
		}
	}
}

// StoreMetadataHandler handles storing metadata after chunk upload completes
func StoreMetadataHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestData StoreMetadataRequest // Use the specific request struct
		if err := c.ShouldBindJSON(&requestData); err != nil {
			log.Printf("[StoreMetadata] Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format", "details": err.Error()})
			return
		}

		// Basic validation
		if requestData.ID == "" || requestData.IV == "" || requestData.Salt == "" || requestData.OriginalFilename == "" || requestData.ContentType == "" {
			log.Printf("[StoreMetadata:%s] Missing required fields (id, iv, salt, originalFilename, contentType)", requestData.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required fields", "fields": []string{"id", "iv", "salt", "originalFilename", "contentType"}})
			return
		}
		// Validate the ID format received in the metadata payload
		if !IsValidUUID(requestData.ID) {
			log.Printf("[StoreMetadata] Invalid ID format received in metadata payload: %s", requestData.ID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format in payload"})
			return
		}
		// Path traversal check (redundant if IsValidUUID is strict, but good practice)
		cleanID := filepath.Clean(requestData.ID)
		if cleanID != requestData.ID || strings.Contains(cleanID, "..") {
			log.Printf("[StoreMetadata:%s] Potential path traversal detected after cleaning ID from payload ('%s' -> '%s')", requestData.ID, requestData.ID, cleanID)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format in payload"})
			return
		}

		// Construct metadata file path
		id := requestData.ID // Use ID from request
		fileName := id + ".json"
		filePath := filepath.Join(config.Paths.DataStorageDir, fileName)

		// Check if merged file exists before saving metadata (important!)
		mergedFilePath := filepath.Join(config.Paths.FinalUploadDir, id, requestData.OriginalFilename)
		fileInfo, err := os.Stat(mergedFilePath)
		if os.IsNotExist(err) {
			log.Printf("[StoreMetadata:%s] Error: Merged file %s not found. Cannot store metadata.", id, mergedFilePath)
			c.JSON(http.StatusPreconditionFailed, gin.H{"error": "Merged file not found, cannot save metadata."})
			return
		} else if err != nil {
			log.Printf("[StoreMetadata:%s] Error checking merged file %s: %v", id, mergedFilePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error checking merged file status."})
			return
		}
		log.Printf("[StoreMetadata:%s] Merged file found. Size: %d bytes", id, fileInfo.Size())

		// --- Expiration Logic ---
		expirationTimePtr, err := calculateExpirationTime(config, requestData.SetDuration)
		if err != nil {
			log.Printf("[StoreMetadata:%s] Error calculating expiration: %v", id, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("计算有效期失败: %v", err)})
			return
		}
		log.Printf("[StoreMetadata:%s] Calculated expiration time: %v", id, expirationTimePtr)
		// --- End Expiration Logic ---

		// Create the metadata struct to store
		metadata := StoredData{ // Use StoredData struct
			IV:                 requestData.IV,
			Salt:               requestData.Salt,
			OriginalFilename:   requestData.OriginalFilename,
			PasswordProtection: requestData.PasswordProtection,
			ExpiresAt:          expirationTimePtr,
			ContentType:        requestData.ContentType,
			FileSize:           fileInfo.Size(), // Store the actual file size from stat
			// AccessWindowEndsAt and FirstAccessedTime are nil initially
		}

		// Marshal the metadata to JSON
		jsonData, err := json.MarshalIndent(metadata, "", "  ")
		if err != nil {
			log.Printf("[StoreMetadata:%s] Error marshaling metadata: %v", id, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error preparing metadata"})
			return
		}

		// Ensure the directory exists
		if err := os.MkdirAll(config.Paths.DataStorageDir, 0750); err != nil {
			log.Printf("[StoreMetadata:%s] Error ensuring data storage directory '%s': %v", id, config.Paths.DataStorageDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error ensuring storage directory"})
			return
		}

		// Write the JSON metadata to the file
		if err := os.WriteFile(filePath, jsonData, 0640); err != nil {
			log.Printf("[StoreMetadata:%s] Error writing metadata file %s: %v", id, filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error saving metadata"})
			return
		}

		log.Printf("[StoreMetadata:%s] Successfully stored metadata", id)
		c.JSON(http.StatusOK, gin.H{"message": "Metadata successfully stored", "id": id})
	}
}

// DownloadHandler handles downloading the merged encrypted file, checking expiration.
func DownloadHandler(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if !IsValidUUID(id) {
			log.Printf("[Download:%s] Invalid ID format received", id)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data ID format"})
			return
		}

		// 1. Read metadata to get the original filename and check expiration
		metaFileName := id + ".json"
		metaFilePath := filepath.Join(config.Paths.DataStorageDir, metaFileName)

		jsonData, err := os.ReadFile(metaFilePath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("[Download:%s] Metadata file not found: %s", id, metaFilePath)
				c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
			} else {
				log.Printf("[Download:%s] Error reading metadata file %s: %v", id, metaFilePath, err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取元数据失败"})
			}
			return
		}

		var metadata StoredData
		if err := json.Unmarshal(jsonData, &metadata); err != nil {
			log.Printf("[Download:%s] Error unmarshaling metadata: %v. Assuming invalid state.", id, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "数据格式无效或已损坏"})
			return
		}

		// --- Expiration Checks ---
		now := time.Now()
		// Primary Expiration
		if metadata.ExpiresAt != nil && now.After(*metadata.ExpiresAt) {
			log.Printf("[Download:%s] Primary expiration time (%s) has passed. Burning data.", id, (*metadata.ExpiresAt).Format(time.RFC3339))
			go burnData(config, id) // Burn in background
			c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
			return
		}
		// Access Window Expiration (check only, don't set on download)
		if config.Expiration.Enabled && config.Expiration.AccessWindow.Enabled && metadata.AccessWindowEndsAt != nil && now.After(*metadata.AccessWindowEndsAt) {
			log.Printf("[Download:%s] Access window expired at %s. Burning data.", id, (*metadata.AccessWindowEndsAt).Format(time.RFC3339))
			go burnData(config, id) // Burn in background
			c.JSON(http.StatusNotFound, gin.H{"error": "数据不存在或已被销毁"})
			return
		}
		// --- End Expiration Checks ---

		if metadata.OriginalFilename == "" {
			log.Printf("[Download:%s] Metadata indicates text data, cannot download file.", id)
			c.JSON(http.StatusNotFound, gin.H{"error": "无法下载：此ID关联的是文本数据"})
			return
		}

		// 2. Construct path to the merged file
		mergedFilePath := filepath.Join(config.Paths.FinalUploadDir, id, metadata.OriginalFilename)

		// 3. Check if file exists
		fileInfo, err := os.Stat(mergedFilePath)
		if os.IsNotExist(err) {
			log.Printf("[Download:%s] Merged file not found: %s", id, mergedFilePath)
			// Attempt to burn metadata if file is missing (consistency)
			go burnData(config, id)
			c.JSON(http.StatusNotFound, gin.H{"error": "无法下载：加密文件不存在（可能已被销毁）"})
			return
		} else if err != nil {
			log.Printf("[Download:%s] Error stating merged file %s: %v", id, mergedFilePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "访问加密文件时出错"})
			return
		}

		// 4. Stream the file
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		// Use ContentType from metadata if available, otherwise octet-stream
		contentTypeHeader := "application/octet-stream"
		if metadata.ContentType != "" {
			contentTypeHeader = metadata.ContentType
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.enc\"", metadata.OriginalFilename)) // Suggest adding .enc extension
		c.Header("Content-Type", contentTypeHeader)
		c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		c.File(mergedFilePath)
		log.Printf("[Download:%s] Started streaming file %s", id, mergedFilePath)

		// Note: After c.File(), you cannot reliably write JSON errors if streaming fails midway.
	}
}
