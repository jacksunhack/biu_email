package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"       // Needed for fmt.Sprintf
	"io"        // Needed for io.Copy
	"io/ioutil" // Needed for ReadFile (metadata)
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func getFile(c *gin.Context) {
	id := c.Query("id")
	log.Printf("Attempting to retrieve file with ID: %s", id)

	// 读取文件元数据
	metaFile := filepath.Join("temp-files", id+".meta")
	metaData, err := ioutil.ReadFile(metaFile)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "The file has been burned!"})
		return
	}

	var fileMeta struct {
		FileName      string
		FileType      string
		IV            []byte
		EncryptionKey []byte
	}
	if err := json.Unmarshal(metaData, &fileMeta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file metadata"})
		return
	}

	// 准备流式传输加密文件
	filePath := filepath.Join("temp-files", id+".enc")
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Encrypted file not found (already burned?): %s", filePath)
			c.JSON(http.StatusNotFound, gin.H{"error": "The file has been burned or does not exist!"})
		} else {
			log.Printf("Failed to open encrypted file %s: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		}
		return
	}
	defer file.Close() // 确保文件句柄关闭

	// 获取文件大小用于 Content-Length (可选但推荐)
	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Failed to get file info for %s: %v", filePath, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file info"})
		return
	}

	// 设置响应头，特别是 Content-Type 和 Content-Disposition
	// Content-Type 设为 application/octet-stream 通常用于强制下载
	// 或者可以保留 fileMeta.FileType
	// Send metadata via custom headers
	// Encode filename using Base64 URL encoding to handle potential special characters safely in headers
	encodedFileName := base64.URLEncoding.EncodeToString([]byte(fileMeta.FileName))
	c.Header("X-File-Name-Base64", encodedFileName)
	c.Header("X-File-Type", fileMeta.FileType)
	c.Header("X-File-IV", base64.StdEncoding.EncodeToString(fileMeta.IV)) // Standard Base64 is fine for IV/Key
	c.Header("X-File-Key", base64.StdEncoding.EncodeToString(fileMeta.EncryptionKey))

	// Set headers for file download
	// Use the original filename in Content-Disposition for a better user experience
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.enc\"", fileMeta.FileName)) // Quote filename
	c.Header("Content-Type", "application/octet-stream")                                               // Indicate binary data
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// Stream the file content directly to the response body
	_, err = io.Copy(c.Writer, file) // Use io.Copy to stream from file to response writer
	if err != nil {
		// Log error, but might be too late to send a JSON error if headers are already sent
		log.Printf("Error streaming file %s: %v", filePath, err)
		// Attempt to send an error status if possible, though client might already be processing
		// c.Status(http.StatusInternalServerError) // Avoid sending JSON body here
	}
	// Note: No c.JSON call here, the body is the raw file content.
} // getFile 函数结束

// burnFileHandler 处理文件销毁请求
func burnFileHandler(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		log.Println("Burn request missing file ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID not provided"})
		return
	}

	log.Printf("Attempting to burn file with ID: %s", id)

	// 构建文件路径
	// 注意：这里假设文件名中不包含路径遍历字符，实际应用中应进行更严格的校验
	filePath := filepath.Join("temp-files", id+".enc")
	metaFile := filepath.Join("temp-files", id+".meta")

	// 尝试删除加密文件
	errEnc := os.Remove(filePath)
	if errEnc != nil && !os.IsNotExist(errEnc) {
		// If the error is something other than "not exist", log it.
		log.Printf("Error deleting encrypted file %s: %v", filePath, errEnc)
		// Continue to attempt deleting the meta file regardless.
	} else if errEnc == nil {
		log.Printf("Successfully deleted encrypted file: %s", filePath)
	} else {
		// File was already gone.
		log.Printf("Encrypted file %s already deleted or not found.", filePath)
	}

	// 尝试删除元数据文件
	errMeta := os.Remove(metaFile)
	if errMeta != nil && !os.IsNotExist(errMeta) {
		log.Printf("Error deleting metadata file %s: %v", metaFile, errMeta)
		// If deleting metadata fails (and it wasn't already gone), this is more critical.
		// Return an error to the client.
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete metadata file"})
		return
	} else if errMeta == nil {
		log.Printf("Successfully deleted metadata file: %s", metaFile)
	} else {
		log.Printf("Metadata file %s already deleted or not found.", metaFile)
	}

	// If we reached here without returning an error, the burn attempt was made.
	c.JSON(http.StatusOK, gin.H{"message": "File burn process initiated/completed."})
} // burnFileHandler 函数结束
