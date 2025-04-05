package main

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
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

	// 读取加密文件
	filePath := filepath.Join("temp-files", id+".enc")
	fileData, err := ioutil.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "The file has been burned!"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		}
		return
	}

	// 删除文件
	if err := os.Remove(filePath); err != nil {
		log.Printf("Error deleting file: %v", err)
	}

	// 删除元数据文件
	if err := os.Remove(metaFile); err != nil {
		log.Printf("Error deleting metadata file: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"fileName":      fileMeta.FileName,
		"fileType":      fileMeta.FileType,
		"encryptedFile": base64.StdEncoding.EncodeToString(fileData),
		"iv":            base64.StdEncoding.EncodeToString(fileMeta.IV),
		"key":           base64.StdEncoding.EncodeToString(fileMeta.EncryptionKey),
	})
}
