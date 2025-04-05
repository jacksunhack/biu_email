package main

import (
    "github.com/gin-gonic/gin"
    "io/ioutil"
    "os"
    "path/filepath"
    "fmt"
    "time"
)

func saveMessage(c *gin.Context) {
    var data struct {
        Message string `json:"message"`
    }
    
    if err := c.BindJSON(&data); err != nil {
        c.JSON(400, gin.H{"error": "Invalid JSON"})
        return
    }
    
    if data.Message == "" {
        c.JSON(400, gin.H{"error": "Message not provided"})
        return
    }
    
    id := generateUniqueID()
    filename := filepath.Join("messages", id+".txt")
    
    err := os.MkdirAll("messages", 0755)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to create directory"})
        return
    }
    
    err = ioutil.WriteFile(filename, []byte(data.Message), 0644)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to save message"})
        return
    }
    
    c.JSON(200, gin.H{"id": id})
}

func generateUniqueID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
