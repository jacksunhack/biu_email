package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/jacksunhack/biu_email/storage"
)

var (
	shortLinks = make(map[string]string)
	mu         sync.RWMutex
)

func redirect(c *gin.Context) {
	shortCode := c.Param("shortCode")
	log.Printf("Attempting to redirect short code: %s", shortCode)

	url, burned, exists := storage.GetShortLink(shortCode)
	if !exists {
		log.Printf("Short link not found: %s", shortCode)
		c.JSON(http.StatusNotFound, gin.H{"error": "Short link not found"})
		return
	}

	if burned {
		log.Printf("Short link already accessed: %s", shortCode)
		c.JSON(http.StatusGone, gin.H{"error": "This message has been burned!"})
		return
	}

	log.Printf("Redirecting %s to %s", shortCode, url)
	c.Redirect(http.StatusFound, url)
}
