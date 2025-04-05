package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// 处理文件上传的Handler
// 全局配置变量 (假设在 main.go 中加载并赋值)
// var config *Config

func SaveFileHandler(c *gin.Context, config *Config) { // 添加 config 参数
	log.Println("Received file upload request")

	// 确保临时文件目录存在
	uploadDir := "temp-files"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		log.Printf("Error creating directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}
	log.Println("Upload directory checked/created successfully")

	// 从表单数据中检索文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		log.Printf("Error retrieving file from form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// --- 新增：使用配置检查文件大小 ---
	maxSize := int64(config.Server.MaxFileSizeMB) * 1024 * 1024 // 转换为字节
	if header.Size > maxSize {
		log.Printf("File size exceeds limit: %d > %d bytes", header.Size, maxSize)
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": fmt.Sprintf("File size exceeds the limit of %d MB", config.Server.MaxFileSizeMB),
		})
		return
	}
	// --- 结束新增 ---

	log.Printf("File retrieved from form: %s, Size: %d bytes", header.Filename, header.Size)

	// 生成唯一的文件名
	filename := uuid.New().String()
	filePath := filepath.Join(uploadDir, filename+filepath.Ext(header.Filename))

	// 创建文件
	out, err := os.Create(filePath)
	if err != nil {
		log.Printf("Error creating file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file on server"})
		return
	}
	defer out.Close()
	log.Printf("File created: %s", filePath)

	// 写入文件
	_, err = io.Copy(out, file)
	if err != nil {
		log.Printf("Error writing file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file on server"})
		return
	}
	log.Println("File written successfully")

	// 从表单数据中获取其他必要信息
	fileName := c.PostForm("fileName")
	fileType := c.PostForm("fileType")

	// 获取 iv 和 encryptionKey 的内容
	ivFile, _, err := c.Request.FormFile("iv")
	if err != nil {
		log.Printf("Error retrieving iv from form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get IV"})
		return
	}
	defer ivFile.Close()
	iv, err := ioutil.ReadAll(ivFile)
	if err != nil {
		log.Printf("Error reading iv file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read IV"})
		return
	}

	encryptionKeyFile, _, err := c.Request.FormFile("key")
	if err != nil {
		log.Printf("Error retrieving encryption key from form: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get encryption key"})
		return
	}
	defer encryptionKeyFile.Close()
	encryptionKey, err := ioutil.ReadAll(encryptionKeyFile)
	if err != nil {
		log.Printf("Error reading encryption key file: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read encryption key"})
		return
	}

	if fileName == "" || fileType == "" || len(iv) == 0 || len(encryptionKey) == 0 {
		log.Printf("Error: Missing required parameters")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing required parameters"})
		return
	}

	log.Printf("Received file: %s, type: %s", fileName, fileType)
	log.Printf("IV length: %d, Key length: %d", len(iv), len(encryptionKey))
	log.Printf("Filename: %s", filename)

	// 保存文件元数据
	metaData := map[string]interface{}{
		"FileName":      fileName,
		"FileType":      fileType,
		"IV":            iv,
		"EncryptionKey": encryptionKey,
	}
	metaBytes, err := json.Marshal(metaData)
	if err != nil {
		log.Printf("Error marshaling metadata: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	metaPath := filepath.Join("temp-files", filename+".meta")
	if err := ioutil.WriteFile(metaPath, metaBytes, 0644); err != nil {
		log.Printf("Error saving metadata file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file metadata"})
		return
	}

	log.Printf("Successfully saved file and metadata: %s", filename)
	c.JSON(http.StatusOK, gin.H{
		"message":  fmt.Sprintf("File uploaded and info saved successfully: %s", fileName),
		"filename": filename,
	})
}
