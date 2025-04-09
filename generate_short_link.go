package main

import (
	"log"
	"math/rand"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	shortCodeLength = 6
	shortCodeChars  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func generateShortLink(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			URL string `json:"url" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			log.Printf("[ShortLink] JSON绑定失败: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求格式"})
			return
		}

		shortCode := generateUniqueShortCode()
		longUrl := strings.TrimSpace(request.URL)

		err := SetShortLink(shortCode, longUrl)
		if err != nil {
			log.Printf("[ShortLink] 保存短链接失败: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "保存短链接失败"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"shortCode": shortCode,
			"shortUrl":  "/s/" + shortCode,
		})
	}
}

func generateUniqueShortCode() string {
	chars := []rune(shortCodeChars)
	length := len(chars)
	result := make([]rune, shortCodeLength)

	for i := range result {
		result[i] = chars[rand.Intn(length)]
	}

	return string(result)
}
