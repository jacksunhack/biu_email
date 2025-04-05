package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
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

	log.Printf("Storing short link: %s -> %s", shortCode, longUrl)
	SetShortLink(shortCode, longUrl)
	log.Printf("Successfully stored short link in memory")

	shortUrl := fmt.Sprintf("http://%s/s/%s", c.Request.Host, shortCode)
	log.Printf("Generated short URL: %s", shortUrl)

	c.JSON(http.StatusOK, gin.H{"shortUrl": shortUrl})
}

func generateShortCode() string {
	timestamp := time.Now().UnixNano()
	hash := md5.Sum([]byte(fmt.Sprintf("%d", timestamp)))
	return hex.EncodeToString(hash[:])[:6]
}
