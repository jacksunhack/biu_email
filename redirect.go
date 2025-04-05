package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func redirect(c *gin.Context) {
	shortCode := c.Param("shortCode")
	log.Printf("Attempting to redirect short code: %s", shortCode)

	url, accessed, exists := GetShortLink(shortCode)
	if !exists {
		log.Printf("Short link not found: %s", shortCode)
		c.JSON(http.StatusNotFound, gin.H{"error": "Short link not found"})
		return
	}

	if accessed {
		log.Printf("Short link already accessed: %s", shortCode)
		c.JSON(http.StatusGone, gin.H{"error": "This message has been burned!"})
		return
	}

	log.Printf("Redirecting %s to %s", shortCode, url)
	c.Redirect(http.StatusFound, url)
}
