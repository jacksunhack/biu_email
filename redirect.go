package main

import (
    "github.com/gin-gonic/gin"
    "net/http"
    "log"
    "database/sql"
)

func redirect(c *gin.Context) {
    shortCode := c.Param("shortCode")

    var originalUrl string
    err := db.QueryRow("SELECT OriginalUrl FROM ShortLinks WHERE ShortCode = ?", shortCode).Scan(&originalUrl)
    if err != nil {
        log.Printf("Error retrieving original URL: %v", err)
        if err == sql.ErrNoRows {
            c.JSON(http.StatusNotFound, gin.H{"error": "Short link not found"})
        } else {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve original URL"})
        }
        return
    }

    c.Redirect(http.StatusFound, originalUrl)
}

