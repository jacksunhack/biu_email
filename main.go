package main

import (
	"embed"
	"encoding/json" // Needed for cleanup task
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
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

//go:embed frontend/*
var embeddedFiles embed.FS

var (
	config     *Config
	configLock sync.RWMutex
	router     *gin.Engine
	routerOnce sync.Once
)

func main() {
	configFile := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	var err error
	configLock.Lock()
	config, err = LoadConfig(*configFile)
	configLock.Unlock()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// Ensure necessary directories exist
	if err := EnsureUploadDirectoriesExist(config); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}
	if err := ensureDataStorageDir(); err != nil { // Ensure data storage dir exists
		log.Fatalf("创建数据存储目录失败: %v", err)
	}

	// Initialize storage (e.g., load short links)
	if err := InitStorage(config); err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	// Initialize Gin router
	initRouter() // Call initRouter before starting the server

	// Start background cleanup task if expiration is enabled
	configLock.RLock()
	if config.Expiration.Enabled {
		// Use a reasonable interval, e.g., 1 hour. Adjust as needed.
		// Shorten interval for testing, e.g., every minute
		cleanupInterval := 1 * time.Minute
		log.Printf("Starting background cleanup task with interval %v", cleanupInterval)
		go startCleanupTask(config, cleanupInterval)
	}
	configLock.RUnlock()

	// Start the HTTP server
	host := "0.0.0.0" // Listen on all interfaces within the container
	port := strconv.Itoa(config.Server.Port)
	log.Printf("服务器运行在 %s:%s", host, port)
	if err := router.Run(fmt.Sprintf("%s:%s", host, port)); err != nil {
		log.Printf("启动服务器失败: %v", err)
		os.Exit(1)
	}
}

func initRouter() {
	routerOnce.Do(func() {
		gin.SetMode(gin.ReleaseMode)
		r := gin.New()
		r.Use(gin.Recovery())

		// Disable automatic redirection
		r.RedirectTrailingSlash = false
		r.RedirectFixedPath = false

		r.ForwardedByClientIP = true
		r.SetTrustedProxies([]string{"127.0.0.1"}) // Adjust if behind other proxies
		r.MaxMultipartMemory = 512 << 20           // 512 MiB

		// CORS Configuration
		configLock.RLock()
		corsConfig := cors.DefaultConfig()
		if len(config.Server.AllowedOrigins) > 0 && !(len(config.Server.AllowedOrigins) == 1 && config.Server.AllowedOrigins[0] == "*") {
			corsConfig.AllowOrigins = config.Server.AllowedOrigins
		} else if len(config.Server.AllowedOrigins) == 1 && config.Server.AllowedOrigins[0] == "*" {
			corsConfig.AllowAllOrigins = true // Allow all if explicitly set to "*"
		} else {
			// Default permissive for local dev if not specified
			corsConfig.AllowOrigins = []string{"http://localhost:3003", "http://127.0.0.1:3003"}
		}
		configLock.RUnlock()

		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		corsConfig.AllowCredentials = true
		r.Use(cors.New(corsConfig))

		// Static files from embedded filesystem
		staticFSBase, err := fs.Sub(embeddedFiles, "frontend")
		if err != nil {
			log.Fatalf("无法访问嵌入式前端目录: %v", err)
		}
		staticFS := http.FS(staticFSBase)

		// Serve static assets (CSS, JS, images) from /static path
		r.StaticFS("/static", http.FS(mustSubFS(staticFSBase, "static")))

		// Serve index.html for root and /index.html
		r.GET("/", func(c *gin.Context) {
			serveIndexHTMLWithSeeker(c, staticFS)
		})
		r.GET("/index.html", func(c *gin.Context) {
			serveIndexHTMLWithSeeker(c, staticFS)
		})

		// API Routes
		api := r.Group("/api")
		{
			configLock.RLock() // Lock for reading config within the group setup
			cfg := config      // Capture config for handlers
			configLock.RUnlock()

			api.POST("/store", StoreDataHandler(cfg))              // For text
			api.POST("/store/metadata", StoreMetadataHandler(cfg)) // For files after upload
			api.GET("/data/:id", GetDataHandler(cfg))
			api.POST("/burn/:id", BurnDataHandler(cfg))
			api.GET("/download/:id", DownloadHandler(cfg))

			// Chunk Upload API
			api.POST("/upload/init", InitUploadHandler(cfg))
			api.POST("/upload/chunk", ChunkUploadHandler(cfg))
			api.GET("/upload/status", CheckUploadStatusHandler(cfg))

			// Short Link API (if enabled/needed)
			api.POST("/shorten", generateShortLink(cfg))
		}

		// Short Link Redirect
		r.GET("/s/:shortCode", redirect(config)) // Pass config directly

		// Frontend Configuration Endpoint
		r.GET("/config", func(c *gin.Context) {
			configLock.RLock()
			defer configLock.RUnlock()
			if config == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "配置未加载"})
				return
			}
			// Prepare the config subset to send to the frontend
			frontendConfig := gin.H{
				"maxFileSizeMB": config.Server.MaxFileSizeMB,
				"expiration": gin.H{
					"enabled": config.Expiration.Enabled,
					"mode":    config.Expiration.Mode,
					// Only include available durations if relevant
					"availableDurations": config.Expiration.AvailableDurations,
					"default_duration":   config.Expiration.DefaultDuration, // Send default too
				},
			}
			// Only include available durations if expiration is enabled and mode is free
			// This check might be redundant if frontend handles it, but good for clarity
			// if !(config.Expiration.Enabled && config.Expiration.Mode == "free") {
			//     delete(frontendConfig["expiration"].(gin.H), "availableDurations")
			// }

			c.JSON(http.StatusOK, frontendConfig)
		})

		// Handle 404s - Serve index.html for potential client-side routing paths
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path

			// Don't serve index.html for API-like paths or static assets
			if strings.HasPrefix(path, "/api/") ||
				strings.HasPrefix(path, "/s/") ||
				strings.HasPrefix(path, "/static/") ||
				path == "/favicon.ico" {
				c.JSON(http.StatusNotFound, gin.H{"error": "资源未找到"})
				return
			}

			// Assume it's a client-side route, serve index.html
			serveIndexHTMLWithSeeker(c, staticFS)
		})

		router = r
	})
}

// serveIndexHTMLWithSeeker serves the index.html file using http.ServeContent
// which handles Range requests and caching headers appropriately.
func serveIndexHTMLWithSeeker(c *gin.Context, staticFS http.FileSystem) {
	// Set cache control headers to prevent caching of index.html
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	file, err := staticFS.Open("index.html") // Open relative to the staticFS root
	if err != nil {
		log.Printf("Error opening index.html from embedded FS: %v", err)
		c.String(http.StatusInternalServerError, "无法打开主页")
		return
	}
	defer file.Close()

	// Check if the file implements io.ReadSeeker, required by http.ServeContent
	seeker, ok := file.(io.ReadSeeker)
	if !ok {
		log.Printf("Error: embedded index.html does not implement io.ReadSeeker")
		c.String(http.StatusInternalServerError, "无法提供主页")
		return
	}

	stat, err := file.Stat()
	if err != nil {
		log.Printf("Error stating index.html from embedded FS: %v", err)
		c.String(http.StatusInternalServerError, "无法获取主页信息")
		return
	}

	// Serve the content using http.ServeContent
	http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), seeker)
}

// ensureDataStorageDir ensures the primary directory for storing .json metadata files exists.
func ensureDataStorageDir() error {
	configLock.RLock()
	dataDir := config.Paths.DataStorageDir // Use the already absolute path from config validation
	configLock.RUnlock()

	// No need for default logic here as config validation handles it
	if dataDir == "" {
		// This should not happen if LoadConfig worked correctly
		return fmt.Errorf("数据存储目录未在配置中正确设置")
	}

	if err := os.MkdirAll(dataDir, 0750); err != nil { // Use restrictive permissions
		return fmt.Errorf("创建数据存储目录 '%s' 失败: %w", dataDir, err)
	}

	log.Printf("数据存储目录已确保存在: %s", dataDir)
	return nil
}

// startCleanupTask starts a background goroutine to periodically clean up expired data.
func startCleanupTask(config *Config, interval time.Duration) {
	log.Printf("[CleanupTask] Starting background cleanup task with interval %v", interval)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately at startup, then tick
	log.Println("[CleanupTask] Running initial cleanup cycle...")
	cleanupExpiredData(config)

	for range ticker.C {
		log.Println("[CleanupTask] Running cleanup cycle...")
		cleanupExpiredData(config)
	}
}

// cleanupExpiredData scans the data directory and removes expired entries.
func cleanupExpiredData(config *Config) {
	log.Println("[CleanupTask] Starting cleanup cycle...") // Log start of cycle
	dataDir := config.Paths.DataStorageDir
	log.Printf("[CleanupTask] Scanning directory: %s", dataDir)
	// 使用 os.ReadDir 替换 ioutil.ReadDir
	files, err := os.ReadDir(dataDir) // <--- 使用 os.ReadDir
	if err != nil {
		log.Printf("[CleanupTask] Error reading data directory %s: %v", dataDir, err)
		return
	}
	log.Printf("[CleanupTask] Found %d entries in data directory.", len(files))

	now := time.Now()
	cleanedCount := 0
	for _, file := range files {
		// file 现在是 fs.DirEntry 类型
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue // Skip directories and non-json files
		}

		filePath := filepath.Join(dataDir, file.Name())
		id := strings.TrimSuffix(file.Name(), ".json")

		// Basic validation of ID format before reading file
		if !IsValidUUID(id) {
			log.Printf("[CleanupTask] Skipping file with invalid ID format: %s", file.Name())
			continue
		}

		jsonData, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // File might have been deleted by another process/request
			}
			log.Printf("[CleanupTask:%s] Error reading metadata file %s: %v", id, filePath, err)
			continue
		}

		// Unmarshal only necessary fields for expiration check
		var metadata struct {
			ExpiresAt          *time.Time `json:"expiresAt"`
			AccessWindowEndsAt *time.Time `json:"accessWindowEndsAt"`
		}
		if err := json.Unmarshal(jsonData, &metadata); err != nil {
			log.Printf("[CleanupTask:%s] Error unmarshaling metadata from %s: %v. Skipping.", id, filePath, err)
			continue // Skip potentially corrupt file
		}
		log.Printf("[CleanupTask:%s] Read ExpiresAt: %v, AccessWindowEndsAt: %v", id, metadata.ExpiresAt, metadata.AccessWindowEndsAt) // Log timestamps

		expired := false
		// Check primary expiration
		if metadata.ExpiresAt != nil && now.After(*metadata.ExpiresAt) {
			log.Printf("[CleanupTask:%s] Primary expiration time (%s) passed. Current time: %s", id, (*metadata.ExpiresAt).Format(time.RFC3339), now.Format(time.RFC3339))
			expired = true
		}
		// Check access window expiration (if applicable and enabled)
		// No need to check config.Expiration.AccessWindow.Enabled here,
		// as AccessWindowEndsAt should only be set if it was enabled at creation time.
		if !expired && metadata.AccessWindowEndsAt != nil && now.After(*metadata.AccessWindowEndsAt) {
			log.Printf("[CleanupTask:%s] Access window expired at %s. Current time: %s", id, (*metadata.AccessWindowEndsAt).Format(time.RFC3339), now.Format(time.RFC3339))
			expired = true
		}

		if expired {
			log.Printf("[CleanupTask:%s] Data expired. Initiating burn.", id)
			// Call burnData asynchronously to avoid blocking the cleanup loop for long burns
			go func(cfg *Config, dataID string) {
				err := burnData(cfg, dataID) // <--- 检查 burnData 的错误
				if err != nil {
					// 记录更详细的错误信息
					log.Printf("[CleanupTask:Burn:%s] Error during background burn: %v", dataID, err)
				} else {
					log.Printf("[CleanupTask:Burn:%s] Background burn completed successfully.", dataID)
				}
			}(config, id) // Pass config and id to the goroutine
			cleanedCount++ // Increment count when burn is initiated
		}
	}
	log.Printf("[CleanupTask] Finished cleanup cycle. Initiated burn for %d entries.", cleanedCount)
}

// mustSubFS is a helper to handle errors from fs.Sub, panicking on error
// as this indicates a programming error with the embedded filesystem structure.
func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(fmt.Sprintf("failed to get sub FS for %s: %v", dir, err))
	}
	return sub
}
