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
	"sync"

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

	if err := EnsureUploadDirectoriesExist(config); err != nil {
		log.Fatalf("创建上传目录失败: %v", err)
	}

	if err := ensureDataStorageDir(); err != nil {
		log.Fatalf("创建数据存储目录失败: %v", err)
	}

	if err := InitStorage(config); err != nil {
		log.Fatalf("初始化存储失败: %v", err)
	}

	host := "0.0.0.0"
	port := strconv.Itoa(config.Server.Port)
	log.Printf("服务器运行在 %s:%s", host, port)

	// 初始化路由器
	initRouter()

	log.Printf("HTTP 服务器运行在 %s:%s", host, port)
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

		// 禁用自动重定向
		r.RedirectTrailingSlash = false
		r.RedirectFixedPath = false

		r.ForwardedByClientIP = true
		r.SetTrustedProxies([]string{"127.0.0.1"})
		r.MaxMultipartMemory = 512 << 20

		configLock.RLock()
		corsConfig := cors.DefaultConfig()
		if len(config.Server.AllowedOrigins) > 0 {
			corsConfig.AllowOrigins = config.Server.AllowedOrigins
		} else {
			corsConfig.AllowOrigins = []string{"http://localhost:3003", "http://127.0.0.1:3003"}
		}
		configLock.RUnlock()

		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Accept", "Authorization"}
		corsConfig.AllowCredentials = true
		r.Use(cors.New(corsConfig))

		// 静态文件处理
		staticFS := http.FS(embeddedFiles)
		staticFiles, err := fs.Sub(embeddedFiles, "frontend/static")
		if err != nil {
			log.Fatalf("无法设置静态文件服务: %v", err)
		}
		r.StaticFS("/static", http.FS(staticFiles))

		// 主页路由处理
		r.GET("/", func(c *gin.Context) {
			serveIndexHTMLWithSeeker(c, staticFS)
		})
		r.GET("/index.html", func(c *gin.Context) {
			serveIndexHTMLWithSeeker(c, staticFS)
		})

		// API 路由组
		api := r.Group("/api")
		{
			configLock.RLock()
			cfg := config
			configLock.RUnlock()

			api.POST("/store", StoreDataHandler(cfg))
			api.GET("/data/:id", GetDataHandler(cfg))
			api.POST("/burn/:id", BurnDataHandler(cfg))
			api.POST("/store/metadata", StoreMetadataHandler(cfg))
			api.GET("/download/:id", DownloadHandler(cfg))

			api.POST("/upload/init", InitUploadHandler(cfg))
			api.POST("/upload/chunk", ChunkUploadHandler(cfg))
			api.GET("/upload/status", CheckUploadStatusHandler(cfg))

			api.POST("/shorten", generateShortLink(cfg))
		}

		r.GET("/s/:shortCode", redirect(config))

		r.GET("/config", func(c *gin.Context) {
			configLock.RLock()
			defer configLock.RUnlock()
			if config == nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "配置未加载"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"maxFileSizeMB": config.Server.MaxFileSizeMB,
			})
		})

		// NoRoute 处理
		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path

			if strings.HasPrefix(path, "/api/") ||
				strings.HasPrefix(path, "/s/") ||
				strings.HasPrefix(path, "/static/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "未找到"})
				return
			}

			if path == "/favicon.ico" {
				c.Status(http.StatusNotFound)
				return
			}

			serveIndexHTMLWithSeeker(c, staticFS)
		})

		router = r
	})
}

func serveIndexHTMLWithSeeker(c *gin.Context, staticFS http.FileSystem) {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	file, err := staticFS.Open("frontend/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "无法打开 index.html")
		return
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		c.String(http.StatusInternalServerError, "无法获取 index.html 状态")
		return
	}

	http.ServeContent(c.Writer, c.Request, "index.html", stat.ModTime(), file.(io.ReadSeeker))
}

func ensureDataStorageDir() error {
	configLock.RLock()
	dataDir := config.Paths.DataStorageDir
	configLock.RUnlock()

	if dataDir == "" {
		dataDir = filepath.Join("storage", "data")
		log.Printf("警告: 数据存储目录未配置，使用默认值: %s", dataDir)
	}

	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("创建数据存储目录失败: %w", err)
	}

	log.Printf("数据存储目录已确保存在: %s", dataDir)
	return nil
}
