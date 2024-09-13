package main

import (
    "encoding/base64"
    "github.com/gin-gonic/gin"
    "io/ioutil"
    "log"
    "database/sql"
    "net/http"
    "path/filepath"
    "os"
)

func getFile(c *gin.Context) {
    id := c.Query("id")
    log.Printf("Attempting to retrieve file with ID: %s", id)

    var fileName, fileType string
    var ivBytes, keyBytes []byte
    err := db.QueryRow("SELECT FileName, FileType, IV, EncryptionKey FROM Files WHERE Id = ?", id).Scan(&fileName, &fileType, &ivBytes, &keyBytes)
    if err != nil {
        log.Printf("Database query error for file ID %s: %v", id, err)
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "The file has been burned!"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve file info"})
        }
        return
    }
    log.Printf("File info retrieved for ID %s: %s, %s", id, fileName, fileType)

    // 文件路径应包含文件扩展名
    encryptedFileName := id + ".enc" // 确保文件扩展名为 .enc
    filePath := filepath.Join("temp-files", encryptedFileName)
    log.Printf("Constructed file path: %s", filePath)
    
    if _, err := os.Stat(filePath); os.IsNotExist(err) {
        log.Printf("File does not exist at path: %s", filePath)
        c.JSON(http.StatusNotFound, gin.H{"error": "The file has been burned!"})
        return
    }

    fileData, err := ioutil.ReadFile(filePath)
    if err != nil {
        log.Printf("Error reading file: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
        return
    }

    // 删除文件
    err = os.Remove(filePath)
    if err != nil {
        log.Printf("Error deleting file: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
        return
    }
    log.Printf("File deleted: %s", filePath)

    // 删除数据库记录
    _, err = db.Exec("DELETE FROM Files WHERE Id = ?", id)
    if err != nil {
        log.Printf("Error deleting database record for file ID %s: %v", id, err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file record from database"})
        return
    }
    log.Printf("Database record deleted for file ID %s", id)

    // 转换 IV 和 key 为 base64 编码字符串
    ivBase64 := base64.StdEncoding.EncodeToString(ivBytes)
    keyBase64 := base64.StdEncoding.EncodeToString(keyBytes)

    c.JSON(http.StatusOK, gin.H{
        "fileName":      fileName,
        "fileType":      fileType,
        "encryptedFile": base64.StdEncoding.EncodeToString(fileData),
        "iv":            ivBase64,
        "key":           keyBase64,
    })
}
