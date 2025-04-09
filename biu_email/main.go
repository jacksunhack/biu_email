package main

import (
	"embed"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed frontend/index.html frontend/static/*
var embeddedFiles embed.FS

var config *Config // Global config variable

func main() {
	// Parse command line flags
	configFile := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	var err error
	config, err = LoadConfig(*configFile) // Assign to global config
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Ensure directories for chunk uploads exist and handle potential errors
	if err := EnsureUploadDirectoriesExist(config); err != nil { // Call renamed function and check error
		log.Fatalf("Failed to ensure upload directories exist: %v", err)
	}

	// Ensure data storage directory exists (moved from api_handlers.go for clarity)
	if err := ensureDataStorageDir(); err != nil {
		log.Fatalf("Failed to ensure data storage directory exists: %v", err)
	}

	// Initialize storage after loading config
	if err := InitStorage(config); err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Use 0.0.0.0 to bind to all interfaces inside the container, or use config value if needed
	host := "0.0.0.0" // Or use config.Server.Host if you want it configurable
	port := strconv.Itoa(config.Server.Port)
	log.Printf("Server running on %s:%s", host, port)

	// 配置 gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// 禁用自动重定向行为
	router.RedirectTrailingSlash = false
	router.RedirectFixedPath = false

	// 设置信任所有代理
	router.ForwardedByClientIP = true
	router.SetTrustedProxies([]string{"127.0.0.1"})

	router.MaxMultipartMemory = 512 << 20 // 512 MiB

	// CORS Configuration
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"http://localhost:3003", "http://127.0.0.1:3003"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
	corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
	corsConfig.AllowCredentials = true
	router.Use(cors.New(corsConfig))

	// 静态文件处理
	staticFS := http.FS(embeddedFiles)
	staticFiles, err := fs.Sub(embeddedFiles, "frontend/static")
	if err != nil {
		log.Fatalf("Failed to get static files: %v", err)
	}

	// 静态文件路由
	router.StaticFS("/static", http.FS(staticFiles))

	// 处理主页和其他路由
	router.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		file, err := staticFS.Open("frontend/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Could not open index.html")
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			c.String(http.StatusInternalServerError, "Could not stat index.html")
			return
		}
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
	})

	// NoRoute handler
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// API 路径返回 404
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}

		// 短链接路径返回 404
		if strings.HasPrefix(path, "/s/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Short link not found"})
			return
		}

		// 静态文件路径返回 404
		if strings.HasPrefix(path, "/static/") {
			c.String(http.StatusNotFound, "Static file not found")
			return
		}

		// favicon.ico 请求返回 404
		if path == "/favicon.ico" {
			c.Status(http.StatusNotFound)
			return
		}

		// 其他所有路径都返回 index.html
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("Expires", "0")
		file, err := staticFS.Open("frontend/index.html")
		if err != nil {
			c.String(http.StatusInternalServerError, "Could not open index.html")
			return
		}
		defer file.Close()
		stat, err := file.Stat()
		if err != nil {
			c.String(http.StatusInternalServerError, "Could not stat index.html")
			return
		}
		http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
	})

	// --- Short Link Endpoints ---
	router.POST("/api/shorten", generateShortLink(config)) // Assuming this is the endpoint for creating short links
	router.GET("/s/:shortCode", redirect(config))          // Endpoint for redirecting short links

	// --- Data/File Handling Endpoints ---
	// Pass the config object to the handler functions to get the actual gin.HandlerFunc
	router.POST("/api/store", StoreDataHandler(config))              // Stores encrypted TEXT data, returns ID
	router.GET("/api/data/:id", GetDataHandler(config))              // Gets metadata (or text data) by ID
	router.POST("/api/burn/:id", BurnDataHandler(config))            // Burns data (metadata and potentially file) by ID
	router.POST("/api/store/metadata", StoreMetadataHandler(config)) // Stores file metadata after chunk upload
	router.GET("/api/download/:id", DownloadHandler(config))         // Downloads the merged encrypted file

	// --- Chunk Upload Endpoints ---
	uploadGroup := router.Group("/api/upload")
	{
		// Pass the config object to the handler functions
		uploadGroup.POST("/init", InitUploadHandler(config))
		uploadGroup.POST("/chunk", ChunkUploadHandler(config))
		uploadGroup.GET("/status", CheckUploadStatusHandler(config))
	}

	// --- 返回配置信息的端点 (保持) ---
	router.GET("/config", func(c *gin.Context) {
		if config == nil {
			// 确保 config 已加载
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Server configuration not loaded"})
			return
		}
		// 只返回前端需要的信息
		c.JSON(http.StatusOK, gin.H{
			"maxFileSizeMB": config.Server.MaxFileSizeMB,
		})
	})
	// --- 结束新增 ---

	// Start the server
	log.Printf("Server running HTTP on %s:%s", host, port)
	err = router.Run(fmt.Sprintf("%s:%s", host, port))
	if err != nil {
		log.Printf("Error starting HTTP server: %v", err) // Print error instead of Fatalf
		// os.Exit(1) // Optionally exit explicitly after logging
	}
}

// ensureDataStorageDir ensures the directory for storing data files exists.
// It's defined here to be accessible by main.
func ensureDataStorageDir() error {
	// Use config path instead of hardcoded path
	dataDir := config.Paths.DataStorageDir
	if dataDir == "" {
		// Fallback or default if not set in config, though config loading should handle defaults
		dataDir = filepath.Join("storage", "data")
		log.Printf("Warning: config.Paths.DataStorageDir is empty, using default: %s", dataDir)
	}
	// Use MkdirAll which creates parent directories if needed and doesn't return error if dir exists
	if err := os.MkdirAll(dataDir, 0750); err != nil { // Use 0750 for better permissions
		log.Printf("Error creating data storage directory '%s': %v", dataDir, err)
		return fmt.Errorf("failed to create data storage directory: %w", err)
	}
	log.Printf("Data storage directory ensured: %s", dataDir)
	return nil
}

// serveIndexHTML 是一个辅助函数，用于统一处理 index.html 的服务
func serveIndexHTML(c *gin.Context) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.FileFromFS("frontend/index.html", http.FS(embeddedFiles))
}
