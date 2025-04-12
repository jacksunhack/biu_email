package main

import (
	"log"
	mathRand "math/rand"
	"mime"          // Added for MIME type detection
	"path/filepath" // Added for getting file extension
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// 初始化随机数生成器
func init() {
	// 使用当前时间作为种子初始化 math/rand
	mathRand.Seed(time.Now().UnixNano())
	log.Println("Random number generator initialized")
}

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

// IsValidUploadID 验证上传ID是否符合预期格式（32位十六进制字符）
func IsValidUploadID(uploadID string) bool {
	// 检查长度是否为32位（MD5哈希的十六进制表示长度）
	if len(uploadID) != 32 {
		return false
	}
	// 检查是否只包含有效的十六进制字符
	match, _ := regexp.MatchString("^[0-9a-f]{32}$", uploadID)
	return match
}

// 更多工具函数可以根据需要添加在这里...

// GetContentTypeByFilename attempts to determine the MIME type based on the filename extension.
// It returns "application/octet-stream" if the type cannot be determined.
func GetContentTypeByFilename(filename string) string {
	ext := filepath.Ext(filename)
	// mime.TypeByExtension expects the extension starting with a dot (e.g., ".txt")
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		// Default to octet-stream if MIME type is unknown or extension is missing
		return "application/octet-stream"
	}
	// The mime package might return "type/subtype; charset=utf-8", we only want "type/subtype"
	return strings.Split(mimeType, ";")[0]
}
