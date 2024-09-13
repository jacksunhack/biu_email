package main

import (
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "io/ioutil"
)

// 处理文件上传的Handler
func SaveFileHandler(c *gin.Context) {
    log.Println("Received file upload request")

    // 检查数据库连接
    err := db.Ping()
    if err != nil {
        log.Printf("Database connection error: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
        return
    }
    log.Println("Database connection successful")

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
    log.Printf("File retrieved from form: %s", header.Filename)

    // 生成唯一的文件名
    filename := uuid.New().String()
    filePath := filepath.Join(uploadDir, filename + filepath.Ext(header.Filename))

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

    // 将文件信息插入数据库
    stmt, err := db.Prepare(`
        INSERT INTO Files (Id, FileName, FileType, IV, EncryptionKey, CreatedAt)
        VALUES (?, ?, ?, ?, ?, NOW())
    `)
    if err != nil {
        log.Printf("Error preparing SQL statement: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare database statement"})
        return
    }
    defer stmt.Close()
    log.Println("SQL statement prepared successfully")

    _, err = stmt.Exec(filename, fileName, fileType, iv, encryptionKey)
    if err != nil {
        log.Printf("Detailed error inserting file info into database: %v", err)
        log.Printf("Attempted to insert: Id='%s', FileName='%s', FileType='%s', IV length=%d, EncryptionKey length=%d", 
            filename, fileName, fileType, len(iv), len(encryptionKey))
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file information"})
        return
    }

    log.Printf("Successfully saved file info to database: %s", filename)
    c.JSON(http.StatusOK, gin.H{
        "message": fmt.Sprintf("File uploaded and info saved successfully: %s", fileName),
        "filename": filename,
    })
}

