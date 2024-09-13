package main

import (
    "github.com/gin-gonic/gin"
    "log"
    "net/http"
    "os"
    "path/filepath"
)

func getMessage(c *gin.Context) {
    id := c.Query("id")
    if id == "" {
        log.Println("Message ID not provided")
        c.JSON(http.StatusBadRequest, gin.H{"error": "Message ID not provided"})
        return
    }

    log.Printf("Attempting to retrieve message with ID: %s", id)

    filename := filepath.Join("messages", id+".txt")
    log.Printf("Full file path: %s", filename)

    content, err := os.ReadFile(filename)
    if err != nil {
        if os.IsNotExist(err) {
            log.Printf("Message file not found: %s", filename)
            c.JSON(http.StatusOK, gin.H{"message": "The message has been burned!"})
        } else {
            log.Printf("Failed to read message file: %v", err)
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read message"})
        }
        return
    }

    log.Printf("Successfully read message file: %s", filename)

    // 读取后立即删除文件
    err = os.Remove(filename)
    if err != nil {
        log.Printf("Failed to delete message file: %v", err)
    } else {
        log.Printf("Successfully deleted message file: %s", filename)
    }

    c.JSON(http.StatusOK, gin.H{"message": string(content)})
}