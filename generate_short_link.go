package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "log"
    "fmt"
    "crypto/md5"
    "encoding/hex"
    "time"
)

func generateShortLink(c *gin.Context) {
    longUrl := c.PostForm("longUrl")
    if longUrl == "" {
        log.Println("Long URL not provided")
        c.JSON(http.StatusBadRequest, gin.H{"error": "Long URL not provided"})
        return
    }

    log.Printf("Received long URL: %s", longUrl)

    shortCode := generateShortCode()
    log.Printf("Generated short code: %s", shortCode)

    if db == nil {
        log.Println("Database connection is nil")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection error"})
        return
    }

    _, err := db.Exec("INSERT INTO ShortLinks (ShortCode, OriginalUrl) VALUES (?, ?)", shortCode, longUrl)
    if err != nil {
        log.Printf("Error saving short link to database: %v", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to save short link: %v", err)})
        return
    }

    shortUrl := fmt.Sprintf("http://%s/s/%s", c.Request.Host, shortCode)
    log.Printf("Generated short URL: %s", shortUrl)

    c.JSON(http.StatusOK, gin.H{"shortUrl": shortUrl})
}

func generateShortCode() string {
    timestamp := time.Now().UnixNano()
    hash := md5.Sum([]byte(fmt.Sprintf("%d", timestamp)))
    return hex.EncodeToString(hash[:])[:6]
}

