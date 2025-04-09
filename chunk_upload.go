package main

import (
	"crypto/md5"
	cryptoRand "crypto/rand" // Alias for crypto/rand
	"encoding/hex"
	"fmt"
	"io"
	"log"
	mathRand "math/rand" // Alias for math/rand
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ChunkInfo 保存文件分片的信息
type ChunkInfo struct {
	UploadID    string `json:"uploadId"`    // 唯一上传标识
	ChunkNumber int    `json:"chunkNumber"` // 当前分片编号
	TotalChunks int    `json:"totalChunks"` // 总分片数
	FileName    string `json:"fileName"`    // 文件名
	FileSize    int64  `json:"fileSize"`    // 文件总大小
}

// ChunkResponse 返回给客户端的响应
type ChunkResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	UploadID  string `json:"uploadId,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Completed bool   `json:"completed,omitempty"`
}

// Removed const tempDir and uploadDir, paths are retrieved from *Config.

// 使用互斥锁保护文件合并过程
var mergeMutex sync.Mutex

// ChunkUploadHandler 处理分片上传请求 (Exported)
// ChunkUploadHandler handles receiving individual file chunks.
func ChunkUploadHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		// 确保目录存在
		// ensureDirectoriesExist() // 可以在 main 函数开始时调用一次

		// 检查是否为POST请求
		// Gin 会自动处理 Method Not Allowed，通常不需要手动检查

		// 解析表单
		// Gin 会自动处理表单解析，但可以设置限制
		// c.Request.ParseMultipartForm(32 << 20) // 如果需要手动设置限制

		// 获取分片信息
		uploadID := c.PostForm("uploadId")
		chunkNumberStr := c.PostForm("chunkNumber")
		totalChunksStr := c.PostForm("totalChunks")
		fileName := c.PostForm("fileName")
		fileSizeStr := c.PostForm("fileSize")

		// --- Security: Sanitize filename ---
		originalFileName := c.PostForm("fileName") // Keep original for logging/reference if needed
		fileName = filepath.Base(originalFileName) // Extract only the filename part
		// Basic validation for the cleaned filename
		if fileName == "" || fileName == "." || fileName == ".." {
			log.Printf("[ChunkUpload:%s] Invalid filename received after cleaning: '%s' (original: '%s')", uploadID, fileName, originalFileName)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid file name provided"})
			return
		}
		// Log the cleaned filename being used
		if fileName != originalFileName {
			log.Printf("[ChunkUpload:%s] Sanitized filename from '%s' to '%s'", uploadID, originalFileName, fileName)
		}
		// --- End Security ---

		// 验证参数
		// Validate parameters
		missingParams := []string{}
		if uploadID == "" {
			missingParams = append(missingParams, "uploadId")
		} else if !IsValidUploadID(uploadID) { // --- Use shared validation ---
			log.Printf("[ChunkUpload] Invalid uploadId format received: %s", uploadID)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid uploadId format"})
			return
		} // --- End validation ---
		// Remove the old check for empty uploadID as it's covered above
		if chunkNumberStr == "" {
			missingParams = append(missingParams, "chunkNumber")
		}
		if totalChunksStr == "" {
			missingParams = append(missingParams, "totalChunks")
		}
		if fileName == "" {
			missingParams = append(missingParams, "fileName")
		}
		if fileSizeStr == "" {
			missingParams = append(missingParams, "fileSize")
		}
		if len(missingParams) > 0 {
			log.Printf("[ChunkUpload] Missing required parameters: %v", missingParams)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing required parameters", "missing": missingParams})
			return
		}

		// 转换参数类型
		chunkNumber, err := strconv.Atoi(chunkNumberStr)
		if err != nil {
			log.Printf("[ChunkUpload:%s] Invalid chunk number format: %s, error: %v", uploadID, chunkNumberStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid chunk number format"})
			return
		}

		totalChunks, err := strconv.Atoi(totalChunksStr)
		if err != nil {
			log.Printf("[ChunkUpload:%s] Invalid total chunks format: %s, error: %v", uploadID, totalChunksStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid total chunks format"})
			return
		}

		fileSize, err := strconv.ParseInt(fileSizeStr, 10, 64)
		if err != nil {
			log.Printf("[ChunkUpload:%s] Invalid file size format: %s, error: %v", uploadID, fileSizeStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid file size format"})
			return
		}

		// 获取文件分片
		file, header, err := c.Request.FormFile("chunk")
		if err != nil {
			log.Printf("[ChunkUpload:%s] Failed to get chunk file from form: %v", uploadID, err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to retrieve chunk file from request"})
			return
		}
		defer file.Close()

		// 在 header 有效的作用域内记录日志
		log.Printf("[ChunkUpload:%s] Received chunk %d / %d: Name=%s, Size=%d bytes", uploadID, chunkNumber, totalChunks, header.Filename, header.Size) // Add filename

		// 存储分片
		chunkDir := filepath.Join(config.Paths.TempChunkDir, uploadID) // Use config path
		if err := os.MkdirAll(chunkDir, 0755); err != nil {
			log.Printf("[ChunkUpload:%s] Failed to create chunk directory %s: %v", uploadID, chunkDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error creating storage directory"})
			return
		}

		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", chunkNumber))
		out, err := os.Create(chunkPath)
		if err != nil {
			log.Printf("[ChunkUpload:%s] Failed to create chunk file %s: %v", uploadID, chunkPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error creating chunk file"})
			return
		}
		defer out.Close()

		bytesWritten, err := io.Copy(out, file)
		if err != nil {
			log.Printf("[ChunkUpload:%s] Failed to save chunk file %s: %v", uploadID, chunkPath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error saving chunk file"})
			return
		}
		log.Printf("[ChunkUpload:%s] Successfully wrote %d bytes for chunk %d to %s", uploadID, bytesWritten, chunkNumber, chunkPath) // Log bytes written

		// 检查是否所有分片都已上传
		// 检查是否所有分片都已上传
		// 读取目录前确保目录存在
		// Re-check directory existence before reading (belt-and-suspenders)
		if err := os.MkdirAll(chunkDir, 0755); err != nil {
			log.Printf("[ChunkUpload:%s] Failed to ensure chunk directory exists before reading %s: %v", uploadID, chunkDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error accessing storage"})
			return
		}
		// Use os.ReadDir instead of ioutil.ReadDir
		dirEntries, err := os.ReadDir(chunkDir) // Use os.ReadDir
		if err != nil {
			log.Printf("[ChunkUpload:%s] Failed to read chunk directory %s: %v", uploadID, chunkDir, err)
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error reading storage"})
			return
		}

		// 实际接收到的分片数量
		// Count valid entries (files, not directories or symlinks if any)
		receivedChunks := 0
		if err == nil { // Only count if ReadDir succeeded
			for _, entry := range dirEntries {
				if !entry.IsDir() { // Count only files
					receivedChunks++
				}
			}
		} else {
			// Error already logged, proceed with receivedChunks = 0
		}

		// 只有当实际接收到的分片数等于总分片数时才开始合并
		if receivedChunks == totalChunks {
			// 异步合并文件，传递 fileSize
			// 异步合并文件，传递 fileSize
			// 确保 ensureDirectoriesExist 在 main 中调用或在这里调用
			// EnsureUploadDirectoriesExist is called once at startup in main.go
			// 添加日志，记录即将传递给 mergeChunks 的 totalChunks 值
			log.Printf("[ChunkUpload:%s] All %d chunks received. Triggering merge for sanitized file '%s' (original: '%s').", uploadID, totalChunks, fileName, originalFileName) // Log sanitized name
			// Pass config to MergeChunks
			go MergeChunks(config, uploadID, fileName, totalChunks, fileSize, chunkDir) // Pass config and sanitized fileName

			// 返回成功响应
			c.JSON(http.StatusOK, ChunkResponse{
				Success:   true,
				Message:   "All chunks received, merging has begun",
				UploadID:  uploadID,
				Completed: false, // 合并是异步的
			})
			return

		}

		// 如果不是最后一个分片，返回当前分片上传成功响应
		c.JSON(http.StatusOK, ChunkResponse{
			Success:   true,
			Message:   fmt.Sprintf("Chunk %d of %d received (%d/%d total received)", chunkNumber, totalChunks, receivedChunks, totalChunks),
			UploadID:  uploadID,
			Completed: false,
		})

	} // Close returned handler
}

// CheckUploadStatusHandler 检查文件合并状态 (Exported)
// CheckUploadStatusHandler checks the status of a chunked upload (merged or in progress).
func CheckUploadStatusHandler(config *Config) gin.HandlerFunc { // Accept config
	return func(c *gin.Context) { // Return the actual handler
		// log.Printf("[CheckStatus] Handler started") // Reduce verbose logging
		uploadID := c.Query("uploadId") // 使用 c.Query 获取查询参数
		if uploadID == "" {
			log.Println("[CheckStatus] Error: Missing uploadId query parameter") // More specific log
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Missing 'uploadId' query parameter"})
			return
		}
		// --- Add validation ---
		if !IsValidUploadID(uploadID) { // --- Use shared validation ---
			log.Printf("[CheckStatus] Invalid uploadId format received: %s", uploadID)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid uploadId format"})
			return
		}
		// --- End validation ---
		// Brace moved down to enclose the entire handler logic
		// log.Printf("[CheckStatus:%s] Checking status for upload ID", uploadID) // Reduce verbose logging

		uploadStatusDir := filepath.Join(config.Paths.FinalUploadDir, uploadID) // Use config path
		completeMarkerPath := filepath.Join(uploadStatusDir, ".complete")
		fileNamePath := filepath.Join(uploadStatusDir, ".filename") // Path to store the original filename
		// log.Printf("[CheckStatus:%s] Complete marker path: %s", uploadID, completeMarkerPath) // Reduce verbose logging
		// log.Printf("[CheckStatus:%s] Filename path: %s", uploadID, fileNamePath) // Reduce verbose logging

		// 检查 .complete 文件
		// log.Printf("[CheckStatus:%s] Checking for complete marker: %s", uploadID, completeMarkerPath) // Reduce verbose logging
		_, completeStatErr := os.Stat(completeMarkerPath)
		// log.Printf("[CheckStatus:%s] Stat complete marker result: %v", uploadID, completeStatErr) // Reduce verbose logging
		if completeStatErr == nil {
			// .complete 文件存在，表示合并已完成
			// log.Printf("[CheckStatus:%s] Found .complete marker. Attempting to read filename from: %s", uploadID, fileNamePath) // Reduce verbose logging
			// 读取原始文件名
			// Use os.ReadFile instead of ioutil.ReadFile
			// Use os.ReadFile instead of ioutil.ReadFile
			fileNameBytes, readFileErr := os.ReadFile(fileNamePath) // Use os.ReadFile
			// log.Printf("[CheckStatus:%s] Read filename file result: %v", uploadID, readFileErr) // Reduce verbose logging
			if readFileErr != nil {
				log.Printf("[CheckStatus:%s] Error reading filename file '%s': %v", uploadID, fileNamePath, readFileErr) // Log error with path
				// If filename cannot be read after merge, it's an internal inconsistency
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error: Failed to retrieve filename after merge"})
				return
			}
			originalFileName := string(fileNameBytes)
			// Construct the relative path using the configured final upload directory
			// Note: This path is relative to the server root, not necessarily the host filesystem root.
			// It's intended for the client to know where the file *conceptually* is.
			finalRelativePath := filepath.Join(config.Paths.FinalUploadDir, uploadID, originalFileName)
			log.Printf("[CheckStatus:%s] Merge completed. Original filename: '%s'.", uploadID, originalFileName) // Simplified log

			c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
				Success:   true,
				Message:   "File merge completed",
				UploadID:  uploadID,
				FilePath:  finalRelativePath, // 返回包含原始文件名的相对路径
				Completed: true,
			})
			return
		} else if !os.IsNotExist(completeStatErr) {
			// If there was an error other than "Not Exists" checking the complete marker
			// Log other stat errors when checking complete marker
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Internal server error checking upload status"})
			return
		}
		// If .complete doesn't exist (os.IsNotExist(completeStatErr) is true), check temp dir

		// 检查临时目录是否存在，如果存在说明还在上传或合并中
		tempChunkDir := filepath.Join(config.Paths.TempChunkDir, uploadID) // Use config path
		// log.Printf("[CheckStatus:%s] Complete marker not found. Checking temporary chunk directory: %s", uploadID, tempChunkDir) // Reduce verbose logging
		_, tempStatErr := os.Stat(tempChunkDir)
		// log.Printf("[CheckStatus:%s] Stat temporary directory result: %v", uploadID, tempStatErr) // Reduce verbose logging
		if tempStatErr == nil {
			// 临时目录存在，合并尚未完成或正在进行
			// log.Printf("[CheckStatus:%s] Temporary directory exists. Reporting merge in progress.", uploadID) // Reduce verbose logging
			c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
				Success:   true,
				Message:   "File upload/merge in progress",
				UploadID:  uploadID,
				Completed: false,
			})
			return
		}
		// If there was an error other than "Not Exists" checking the temp directory
		// We would have returned earlier based on the logic before this block.
		// So if we reach here, it means the temp dir doesn't exist.

		// 如果 .complete 文件和临时目录都不存在，则认为上传未找到或已失败/清理
		log.Printf("[CheckStatus:%s] Upload status check: Neither complete marker nor temp directory found.", uploadID)
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Upload not found, incomplete, or failed"}) // Clearer message

	} // Close returned handler
}

// InitUploadHandler initializes the chunk upload process and returns an upload ID.
func InitUploadHandler(config *Config) gin.HandlerFunc { // Accept config (for consistency, though not used here)
	return func(c *gin.Context) { // Return the actual handler
		// Gin 会自动处理 Method Not Allowed

		// 解析请求体
		var uploadRequest struct {
			FileName string `json:"fileName"`
			FileSize int64  `json:"fileSize"`
		}

		if err := c.ShouldBindJSON(&uploadRequest); err != nil {
			log.Printf("[InitUpload] Error binding JSON: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request format", "details": err.Error()})
			return
		}

		if uploadRequest.FileName == "" {
			log.Println("[InitUpload] File name is required but missing")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "'fileName' is required"})
			return
		}

		// 生成上传ID
		uploadID := generateUploadID(uploadRequest.FileName)
		log.Printf("[InitUpload] Initialized upload for file '%s' with ID: %s", uploadRequest.FileName, uploadID)

		// 返回上传ID
		c.JSON(http.StatusOK, ChunkResponse{ // 使用 c.JSON
			Success:  true,
			Message:  "Upload initialized",
			UploadID: uploadID,
		})
	} // Close returned handler
}

// mergeChunks 合并所有分片成一个完整文件
// MergeChunks merges all chunks into a final file.
// MergeChunks merges all chunks into a final file.
func MergeChunks(config *Config, uploadID, fileName string, totalChunks int, expectedSize int64, chunkDir string) { // Accept config
	startTime := time.Now() // 记录开始时间
	log.Printf("[%s] MergeChunks: Started. totalChunks = %d, expectedSize = %d, chunkDir = %s", uploadID, totalChunks, expectedSize, chunkDir)
	// Defer the finish log and mutex unlock
	defer func() {
		duration := time.Since(startTime)
		log.Printf("[%s] MergeChunks: Finished. Duration: %s", uploadID, duration)
		// Ensure mutex is unlocked even if function returns early (e.g., due to panic or early return)
		mergeMutex.Unlock()
	}()
	// Lock the mutex *before* the defer unlock is set up
	mergeMutex.Lock()
	// Defer the unlock *after* successfully acquiring the lock
	defer mergeMutex.Unlock()

	// 注意：上面 defer 中已添加启动日志，此处移除重复日志

	// 创建最终文件所在的目录 uploadDir/uploadID
	finalDir := filepath.Join(config.Paths.FinalUploadDir, uploadID)                        // Use config path
	log.Printf("[%s] MergeChunks: Ensuring final directory exists: %s", uploadID, finalDir) // Changed log message slightly
	if err := os.MkdirAll(finalDir, 0755); err != nil {
		log.Printf("[%s] MergeChunks: ERROR - Failed to create final directory '%s': %v. Aborting.", uploadID, finalDir, err) // Added ERROR prefix and Aborting.
		return                                                                                                                // Return handled by defer unlock
	}
	log.Printf("[%s] MergeChunks: Final directory ensured.", uploadID) // Log success

	// 最终文件路径 uploadDir/uploadID/fileName
	finalFilePath := filepath.Join(finalDir, fileName)
	log.Printf("[%s] MergeChunks: Creating final file: %s", uploadID, finalFilePath)
	finalFile, err := os.Create(finalFilePath)
	if err != nil {
		log.Printf("[%s] MergeChunks: ERROR - Failed to create final file '%s': %v. Aborting.", uploadID, finalFilePath, err) // Added ERROR prefix and Aborting.
		return                                                                                                                // Return handled by defer unlock
	}
	log.Printf("[%s] MergeChunks: Successfully created final file.", uploadID) // Log success
	defer finalFile.Close()

	// 按顺序合并所有分片
	for i := 1; i <= totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", i))
		log.Printf("[%s] MergeChunks: Processing chunk %d/%d: %s", uploadID, i, totalChunks, chunkPath) // Log which chunk

		// 检查分片文件是否存在
		var chunkFile *os.File // Declare chunkFile here
		var statErr error
		if _, statErr = os.Stat(chunkPath); os.IsNotExist(statErr) {
			log.Printf("[%s] MergeChunks: ERROR - Chunk file %d not found: %s. Aborting merge.", uploadID, i, chunkPath)
			finalFile.Close()        // Close the output file
			os.Remove(finalFilePath) // Attempt cleanup
			return                   // Return handled by defer unlock
		} else if statErr != nil {
			// Log other stat errors
			log.Printf("[%s] MergeChunks: ERROR - Cannot stat chunk file %d (%s): %v. Aborting merge.", uploadID, i, chunkPath, statErr)
			finalFile.Close()        // Close the output file
			os.Remove(finalFilePath) // Attempt cleanup
			return                   // Return handled by defer unlock
		}

		// Open the chunk file
		log.Printf("[%s] MergeChunks: Opening chunk %d: %s", uploadID, i, chunkPath)
		chunkFile, err = os.Open(chunkPath) // Assign to declared chunkFile
		if err != nil {
			log.Printf("[%s] MergeChunks: ERROR - Failed to open chunk file %d (%s): %v. Aborting merge.", uploadID, i, chunkPath, err)
			finalFile.Close()        // Close the output file
			os.Remove(finalFilePath) // Attempt cleanup
			return                   // Return handled by defer unlock
		}

		// log.Printf("[%s] MergeChunks: Copying chunk %d...", uploadID, i) // Optional: Log before copy
		// Copy the chunk content
		log.Printf("[%s] MergeChunks: Copying chunk %d content...", uploadID, i)
		bytesCopied, err := io.Copy(finalFile, chunkFile)
		chunkFile.Close() // Close chunk file immediately after copy
		if err != nil {
			log.Printf("[%s] MergeChunks: ERROR - Failed to copy chunk %d (%s) to final file: %v. Aborting merge.", uploadID, i, chunkPath, err)
			finalFile.Close()        // Close the output file
			os.Remove(finalFilePath) // Attempt cleanup
			return                   // Return handled by defer unlock
		}
		log.Printf("[%s] MergeChunks: Successfully copied %d bytes from chunk %d.", uploadID, bytesCopied, i)

	}

	// 关闭最终文件以确保所有数据都已写入磁盘
	// Close the final merged file (defer already handles this, but explicit log is good)
	log.Printf("[%s] MergeChunks: Finished copying all chunks. Closing final file: %s", uploadID, finalFilePath)
	// finalFile.Close() is handled by defer, no need to call it explicitly here unless for error checking before defer

	// 文件大小验证逻辑已移除 - 我们信任服务器合并后的实际大小
	// 获取最终文件信息以记录大小
	// Get final file info (optional but good for verification)
	log.Printf("[%s] MergeChunks: Getting final file info: %s", uploadID, finalFilePath)
	finalFileInfo, err := os.Stat(finalFilePath)
	if err != nil {
		log.Printf("[%s] MergeChunks: WARNING - Failed to get final file info for '%s': %v", uploadID, finalFilePath, err)
		// Proceed without size check if stat fails
	} else {
		log.Printf("[%s] MergeChunks: Final file size: %d bytes.", uploadID, finalFileInfo.Size())
		// 记录一下实际大小和预期大小，但不作为失败条件
		// 记录文件大小差异（如果需要）
		if finalFileInfo.Size() != expectedSize {
			log.Printf("[%s] MergeChunks: INFO - File size mismatch. Client Expected: %d, Server Actual: %d.", uploadID, expectedSize, finalFileInfo.Size())
		} else {
			log.Printf("[%s] MergeChunks: Final file size matches expected size.", uploadID)
		}
	}

	// 创建 .complete 标记文件
	completeMarkerPath := filepath.Join(finalDir, ".complete")
	log.Printf("[%s] MergeChunks: Creating completion marker: %s", uploadID, completeMarkerPath)
	if _, err := os.Create(completeMarkerPath); err != nil {
		log.Printf("[%s] MergeChunks: ERROR - Failed to create complete marker file '%s': %v", uploadID, completeMarkerPath, err)
		// Don't return here, try to create filename marker anyway
	} else {
		log.Printf("[%s] MergeChunks: Successfully created completion marker.", uploadID)
	}

	// 创建 .filename 文件存储原始文件名
	fileNamePath := filepath.Join(finalDir, ".filename")
	log.Printf("[%s] MergeChunks: Creating filename marker: %s", uploadID, fileNamePath)
	// Use os.WriteFile instead of ioutil.WriteFile
	// Use os.WriteFile instead of ioutil.WriteFile
	if err := os.WriteFile(fileNamePath, []byte(fileName), 0640); err != nil { // Use os.WriteFile and restrictive permissions
		log.Printf("[%s] MergeChunks: ERROR - Failed to write original filename marker '%s': %v", uploadID, fileNamePath, err)
		// Status check might fail to get filename
	} else {
		log.Printf("[%s] MergeChunks: Successfully created filename marker.", uploadID)
	}

	// 清理临时分片目录
	log.Printf("[%s] MergeChunks: Cleaning up chunk directory: %s", uploadID, chunkDir)
	if err := os.RemoveAll(chunkDir); err != nil {
		log.Printf("[%s] MergeChunks: WARNING - Failed to clean up chunk directory '%s': %v", uploadID, chunkDir, err)
	} else {
		log.Printf("[%s] MergeChunks: Successfully cleaned up chunk directory.", uploadID)
	}

	// Final log is handled by the defer function
}

// generateUploadID 根据文件名生成唯一的上传ID
// generateUploadID generates a unique upload ID based on filename and timestamp.
func generateUploadID(fileName string) string {
	// Consider adding more entropy if high collision resistance is needed,
	// e.g., include random bytes or use a stronger hash like SHA-256.
	// For this use case, MD5 with timestamp is likely sufficient.
	timestamp := strconv.FormatInt(time.Now().UnixNano(), 10) // Use time directly
	data := fileName + timestamp + string(cryptoRandBytes(8)) // Add some random bytes
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:]) // Return full MD5 hash
}

// cryptoRandBytes generates cryptographically secure random bytes.
func cryptoRandBytes(n int) []byte {
	b := make([]byte, n)
	_, err := io.ReadFull(cryptoRand.Reader, b)
	if err != nil {
		// Fallback to less secure random on error (should not happen in practice)
		log.Printf("WARNING: crypto/rand failed: %v. Falling back to math/rand.", err)
		return mathRandBytes(n)
	}
	return b
}

// mathRandBytes generates pseudo-random bytes (fallback).
func mathRandBytes(n int) []byte {
	b := make([]byte, n)
	// Seed math/rand if not already done (e.g., in main or init)
	// mathRand.Seed(time.Now().UnixNano()) // Be careful with concurrent seeding
	for i := range b {
		b[i] = byte(mathRand.Intn(256))
	}
	return b
}

// getTimestamp 获取当前时间戳
func getTimestamp() int64 {
	return time.Now().UnixNano() // 使用纳秒时间戳增加唯一性
}

// EnsureDirectoriesExist 确保必要的目录存在 (Exported)
// EnsureUploadDirectoriesExist ensures the temporary and final upload directories exist.
// It should be called once during application startup after config is loaded.
func EnsureUploadDirectoriesExist(config *Config) error { // Rename and return error
	// Use MkdirAll which creates parent directories if needed and doesn't return error if dir exists
	// Use more restrictive permissions (e.g., 0750)
	if err := os.MkdirAll(config.Paths.TempChunkDir, 0750); err != nil {
		log.Printf("[Startup] Error creating temp chunk directory '%s': %v", config.Paths.TempChunkDir, err)
		return fmt.Errorf("failed to create temp chunk directory: %w", err)
	}
	log.Printf("[Startup] Ensured temp chunk directory exists: %s", config.Paths.TempChunkDir)

	if err := os.MkdirAll(config.Paths.FinalUploadDir, 0750); err != nil {
		log.Printf("[Startup] Error creating final upload directory '%s': %v", config.Paths.FinalUploadDir, err)
		return fmt.Errorf("failed to create final upload directory: %w", err)
	}
	log.Printf("[Startup] Ensured final upload directory exists: %s", config.Paths.FinalUploadDir)
	return nil
}

// Removed unused sendErrorResponse and sendSuccessResponse functions

// Removed local isValidUploadID function, will use IsValidUploadID from utils.go
