package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func redirect(config *Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		shortCode := c.Param("shortCode")
		if shortCode == "" {
			log.Printf("[Redirect] 短代码为空")
			c.JSON(http.StatusBadRequest, gin.H{"error": "无效的短链接"})
			return
		}

		url, exists := GetShortLink(shortCode)
		if !exists {
			log.Printf("[Redirect] 未找到短链接: %s", shortCode)
			c.JSON(http.StatusNotFound, gin.H{"error": "短链接不存在或已过期"})
			return
		}

		log.Printf("[Redirect] 重定向 %s 到 %s", shortCode, url)
		c.Redirect(http.StatusMovedPermanently, url)
	}
}
