package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

func saveMessage(c *gin.Context) {
	var data struct {
		Message string `json:"message"`
	}

	if err := c.BindJSON(&data); err != nil {
		log.Printf("Failed to bind JSON: %v", err)
		c.JSON(400, gin.H{"error": "Invalid JSON", "details": err.Error()})
		return
	}

	if data.Message == "" {
		log.Println("Empty message received")
		c.JSON(400, gin.H{"error": "Message not provided"})
		return
	}

	id := generateUniqueID()
	filename := filepath.Join("messages", id+".txt")
	log.Printf("Attempting to save message to: %s", filename)

	// Create messages directory with wider permissions
	err := os.MkdirAll("messages", 0777)
	if err != nil {
		log.Printf("Failed to create messages directory: %v", err)
		c.JSON(500, gin.H{
			"error":   "Failed to create directory",
			"details": err.Error(),
		})
		return
	}

	// Write message file with wider permissions
	err = ioutil.WriteFile(filename, []byte(data.Message), 0666)
	if err != nil {
		log.Printf("Failed to write message file: %v", err)
		c.JSON(500, gin.H{
			"error":   "Failed to save message",
			"details": err.Error(),
			"path":    filename,
		})
		return
	}

	log.Printf("Successfully saved message with ID: %s", id)
	c.JSON(200, gin.H{"id": id})
}

func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
