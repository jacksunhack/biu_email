package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings" // Added for string manipulation
	"time"    // Added for time duration parsing

	"gopkg.in/yaml.v3"
)

// AccessWindowRule defines a rule for dynamic access window duration.
type AccessWindowRule struct {
	Type      []string `yaml:"type"`        // List of file extensions (lowercase)
	MinSizeMB int      `yaml:"min_size_mb"` // Optional: Minimum file size in MB for this rule
	MaxSizeMB int      `yaml:"max_size_mb"` // Optional: Maximum file size in MB for this rule
	Duration  string   `yaml:"duration"`    // Access window duration (e.g., "5m", "1h")
}

// AccessWindowConfig holds settings for the short-lived access window.
type AccessWindowConfig struct {
	Enabled         bool               `yaml:"enabled"`          // Whether the access window feature is enabled
	DefaultDuration string             `yaml:"default_duration"` // Default window duration if no rule matches
	Rules           []AccessWindowRule `yaml:"rules"`            // List of rules for dynamic duration
}

// ExpirationConfig holds all expiration-related settings.
type ExpirationConfig struct {
	Enabled            bool               `yaml:"enabled"`             // Whether expiration feature is enabled at all
	Mode               string             `yaml:"mode"`                // "forced" or "free"
	DefaultDuration    string             `yaml:"default_duration"`    // Default primary expiration (e.g., "24h")
	AvailableDurations []string           `yaml:"available_durations"` // Options for "free" mode
	AccessWindow       AccessWindowConfig `yaml:"access_window"`       // Access window settings
}

type Config struct {
	Application struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"application"`
	Paths struct {
		// DataStorageDir: Directory to store metadata JSON files.
		DataStorageDir string `yaml:"data_storage_dir"`
		// FinalUploadDir: Directory to store the final merged uploaded files.
		FinalUploadDir string `yaml:"final_upload_dir"`
		// TempChunkDir: Directory to store temporary file chunks during upload.
		TempChunkDir string `yaml:"temp_chunk_dir"`
		// LogFilePath: Path for the application log file (defined under Logging).
		// LogFilePath string `yaml:"log_file_path"` // Keep this under Logging section
	} `yaml:"paths"`
	Server struct {
		Host           string   `yaml:"host"`
		Port           int      `yaml:"port"`
		MaxFileSizeMB  int      `yaml:"max_file_size_mb"`          // 新增：最大文件上传大小 (MB)
		AllowedOrigins []string `yaml:"allowed_origins,omitempty"` // 新增：允许的 CORS 来源
		TLS            struct {
			Enabled  bool   `yaml:"enabled"`
			Domain   string `yaml:"domain"`
			Email    string `yaml:"email"`
			CacheDir string `yaml:"cache_dir"`
		} `yaml:"tls"`
	} `yaml:"server"`
	Security struct {
		EncryptionKeyLength int    `yaml:"encryption_key_length"`
		EncryptionAlgorithm string `yaml:"encryption_algorithm"`
	} `yaml:"security"`
	Expiration ExpirationConfig `yaml:"expiration"` // Added expiration settings
	Frontend   struct {
		Theme        string `yaml:"theme"`
		MatrixEffect bool   `yaml:"matrix_effect"`
		Styles       struct {
			BackgroundColor string `yaml:"background_color"`
			TextColor       string `yaml:"text_color"`
			FontFamily      string `yaml:"font_family"`
			MaxWidth        string `yaml:"max_width"`
			Margin          string `yaml:"margin"`
			Padding         string `yaml:"padding"`
			Border          string `yaml:"border"`
			BoxShadow       string `yaml:"box_shadow"`
			BorderRadius    string `yaml:"border_radius"`
		} `yaml:"styles"`
		Fonts struct {
			Primary   string `yaml:"primary"`
			Secondary string `yaml:"secondary"`
		} `yaml:"fonts"`
		AdSpace struct {
			Width   string `yaml:"width"`
			Padding string `yaml:"padding"`
		} `yaml:"ad_space"`
		Buttons struct {
			Default struct {
				BackgroundColor string `yaml:"background_color"`
				TextColor       string `yaml:"text_color"`
				Border          string `yaml:"border"`
				Padding         string `yaml:"padding"`
				BorderRadius    string `yaml:"border_radius"`
				LetterSpacing   string `yaml:"letter_spacing"`
				FontWeight      string `yaml:"font_weight"`
				BoxShadow       string `yaml:"box_shadow"`
			} `yaml:"default"`
			Hover struct {
				BackgroundColor string `yaml:"background_color"`
				TextColor       string `yaml:"text_color"`
				BoxShadow       string `yaml:"box_shadow"`
			} `yaml:"hover"`
			Active struct {
				Transform string `yaml:"transform"`
			} `yaml:"active"`
		} `yaml:"buttons"`
	} `yaml:"frontend"`
	APIEndpoints struct {
		SaveFile          string `yaml:"save_file"`
		GetFile           string `yaml:"get_file"`
		SaveMessage       string `yaml:"save_message"`
		GetMessage        string `yaml:"get_message"`
		GenerateShortLink string `yaml:"generate_short_link"`
	} `yaml:"api_endpoints"`
	Logging struct {
		Level    string `yaml:"level"`
		Format   string `yaml:"format"`
		Handlers struct {
			Console struct{} `yaml:"console"`
			File    struct {
				Path string `yaml:"path"`
			} `yaml:"file"`
		} `yaml:"handlers"`
	} `yaml:"logging"`
	Messages struct {
		EncryptionSuccess string `yaml:"encryption_success"`
		EncryptionError   string `yaml:"encryption_error"`
		UploadSuccess     string `yaml:"upload_success"`
		UploadError       string `yaml:"upload_error"`
		DownloadSuccess   string `yaml:"download_success"`
		DownloadError     string `yaml:"download_error"`
		InvalidParameters string `yaml:"invalid_parameters"`
	} `yaml:"messages"`
	UIText struct {
		EncryptAndTransmit string `yaml:"encrypt_and_transmit"`
		SwitchMode         string `yaml:"switch_mode"`
		EncryptedMessage   string `yaml:"encrypted_message"`
		EncryptedFile      string `yaml:"encrypted_file"`
		Loading            string `yaml:"loading"`
		Error              string `yaml:"error"`
		Success            string `yaml:"success"`
	} `yaml:"ui_text"`
}

func LoadConfig(configFile string) (*Config, error) {
	// 规范化配置文件路径
	absPath, err := filepath.Abs(configFile)
	if err != nil {
		return nil, fmt.Errorf("无法获取配置文件的绝对路径: %w", err)
	}

	// 读取配置文件
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 验证和补充配置
	if err := validateAndNormalizeConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	return &config, nil
}

// validateAndNormalizeConfig 验证并规范化配置
func validateAndNormalizeConfig(config *Config) error {
	// 验证并设置默认路径
	if config.Paths.DataStorageDir == "" {
		config.Paths.DataStorageDir = "storage"
		log.Println("警告: 未指定数据存储目录，使用默认值: storage")
	}

	if config.Paths.TempChunkDir == "" {
		config.Paths.TempChunkDir = "temp-files"
		log.Println("警告: 未指定临时分片目录，使用默认值: temp-files")
	}

	if config.Paths.FinalUploadDir == "" {
		config.Paths.FinalUploadDir = "uploads"
		log.Println("警告: 未指定最终上传目录，使用默认值: uploads")
	}

	// 验证并设置服务器配置
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		config.Server.Port = 3003
		log.Println("警告: 端口号无效，使用默认端口: 3003")
	}

	if config.Server.MaxFileSizeMB <= 0 {
		config.Server.MaxFileSizeMB = 100
		log.Println("警告: 未指定最大文件大小，使用默认值: 100MB")
	}

	// 验证并设置安全配置
	if config.Security.EncryptionKeyLength <= 0 {
		config.Security.EncryptionKeyLength = 256
		log.Println("警告: 未指定加密密钥长度，使用默认值: 256位")
	}

	if config.Security.EncryptionAlgorithm == "" {
		config.Security.EncryptionAlgorithm = "AES-GCM"
		log.Println("警告: 未指定加密算法，使用默认值: AES-GCM")
	}

	// Validate and set default expiration settings
	if config.Expiration.Enabled {
		if config.Expiration.Mode == "" {
			config.Expiration.Mode = "free" // Default to free mode
			log.Println("警告: 未指定有效期模式 (expiration.mode)，使用默认值: free")
		}
		if config.Expiration.Mode != "forced" && config.Expiration.Mode != "free" {
			return fmt.Errorf("无效的有效期模式 (expiration.mode): %s，必须是 'forced' 或 'free'", config.Expiration.Mode)
		}
		if config.Expiration.DefaultDuration == "" {
			config.Expiration.DefaultDuration = "24h" // Default to 24 hours
			log.Println("警告: 未指定默认有效期 (expiration.default_duration)，使用默认值: 24h")
		}
		// Validate DefaultDuration format
		if _, err := time.ParseDuration(config.Expiration.DefaultDuration); err != nil {
			return fmt.Errorf("无效的默认有效期格式 (expiration.default_duration: %s): %w", config.Expiration.DefaultDuration, err)
		}

		// Validate AvailableDurations if in free mode
		if config.Expiration.Mode == "free" {
			if len(config.Expiration.AvailableDurations) == 0 {
				config.Expiration.AvailableDurations = []string{"1h", "24h", "168h"} // Default options
				log.Println("警告: 未指定可用有效期选项 (expiration.available_durations)，使用默认值: [1h, 24h, 168h]")
			}
			for _, dur := range config.Expiration.AvailableDurations {
				if _, err := time.ParseDuration(dur); err != nil {
					return fmt.Errorf("无效的可用有效期格式 (expiration.available_durations: %s): %w", dur, err)
				}
			}
		}

		// Validate Access Window settings if enabled
		if config.Expiration.AccessWindow.Enabled {
			if config.Expiration.AccessWindow.DefaultDuration == "" {
				config.Expiration.AccessWindow.DefaultDuration = "10m" // Default access window
				log.Println("警告: 未指定默认访问窗口期 (expiration.access_window.default_duration)，使用默认值: 10m")
			}
			if _, err := time.ParseDuration(config.Expiration.AccessWindow.DefaultDuration); err != nil {
				return fmt.Errorf("无效的默认访问窗口期格式 (expiration.access_window.default_duration: %s): %w", config.Expiration.AccessWindow.DefaultDuration, err)
			}
			// Validate rules
			for i, rule := range config.Expiration.AccessWindow.Rules {
				if len(rule.Type) == 0 {
					return fmt.Errorf("访问窗口规则 %d 缺少 'type' 字段", i)
				}
				if rule.Duration == "" {
					return fmt.Errorf("访问窗口规则 %d (类型: %v) 缺少 'duration' 字段", i, rule.Type)
				}
				if _, err := time.ParseDuration(rule.Duration); err != nil {
					return fmt.Errorf("无效的访问窗口规则 %d (类型: %v) 持续时间格式 (duration: %s): %w", i, rule.Type, rule.Duration, err)
				}
				// Normalize file types to lowercase
				for j, ext := range rule.Type {
					config.Expiration.AccessWindow.Rules[i].Type[j] = strings.ToLower(ext)
				}
			}
		}
	} else {
		log.Println("信息: 有效期功能未启用 (expiration.enabled is false or not set)")
	}
	if config.Expiration.Enabled {
		if config.Expiration.Mode == "" {
			config.Expiration.Mode = "free" // Default to free mode
			log.Println("警告: 未指定有效期模式 (expiration.mode)，使用默认值: free")
		}
		if config.Expiration.Mode != "forced" && config.Expiration.Mode != "free" {
			return fmt.Errorf("无效的有效期模式 (expiration.mode): %s，必须是 'forced' 或 'free'", config.Expiration.Mode)
		}
		if config.Expiration.DefaultDuration == "" {
			config.Expiration.DefaultDuration = "24h" // Default to 24 hours
			log.Println("警告: 未指定默认有效期 (expiration.default_duration)，使用默认值: 24h")
		}
		// Validate DefaultDuration format
		if _, err := time.ParseDuration(config.Expiration.DefaultDuration); err != nil {
			return fmt.Errorf("无效的默认有效期格式 (expiration.default_duration: %s): %w", config.Expiration.DefaultDuration, err)
		}

		// Validate AvailableDurations if in free mode
		if config.Expiration.Mode == "free" {
			if len(config.Expiration.AvailableDurations) == 0 {
				config.Expiration.AvailableDurations = []string{"1h", "24h", "168h"} // Default options
				log.Println("警告: 未指定可用有效期选项 (expiration.available_durations)，使用默认值: [1h, 24h, 168h]")
			}
			for _, dur := range config.Expiration.AvailableDurations {
				if _, err := time.ParseDuration(dur); err != nil {
					return fmt.Errorf("无效的可用有效期格式 (expiration.available_durations: %s): %w", dur, err)
				}
			}
		}

		// Validate Access Window settings if enabled
		if config.Expiration.AccessWindow.Enabled {
			if config.Expiration.AccessWindow.DefaultDuration == "" {
				config.Expiration.AccessWindow.DefaultDuration = "10m" // Default access window
				log.Println("警告: 未指定默认访问窗口期 (expiration.access_window.default_duration)，使用默认值: 10m")
			}
			if _, err := time.ParseDuration(config.Expiration.AccessWindow.DefaultDuration); err != nil {
				return fmt.Errorf("无效的默认访问窗口期格式 (expiration.access_window.default_duration: %s): %w", config.Expiration.AccessWindow.DefaultDuration, err)
			}
			// Validate rules
			for i, rule := range config.Expiration.AccessWindow.Rules {
				if len(rule.Type) == 0 {
					return fmt.Errorf("访问窗口规则 %d 缺少 'type' 字段", i)
				}
				if rule.Duration == "" {
					return fmt.Errorf("访问窗口规则 %d (类型: %v) 缺少 'duration' 字段", i, rule.Type)
				}
				if _, err := time.ParseDuration(rule.Duration); err != nil {
					return fmt.Errorf("无效的访问窗口规则 %d (类型: %v) 持续时间格式 (duration: %s): %w", i, rule.Type, rule.Duration, err)
				}
				// Normalize file types to lowercase
				for j, ext := range rule.Type {
					config.Expiration.AccessWindow.Rules[i].Type[j] = strings.ToLower(ext)
				}
			}
		}
	} else {
		log.Println("信息: 有效期功能未启用 (expiration.enabled is false or not set)")
	}

	// 规范化路径（确保所有路径都是绝对路径）
	var err error
	config.Paths.DataStorageDir, err = filepath.Abs(config.Paths.DataStorageDir)
	if err != nil {
		return fmt.Errorf("无法获取数据存储目录的绝对路径: %w", err)
	}

	config.Paths.TempChunkDir, err = filepath.Abs(config.Paths.TempChunkDir)
	if err != nil {
		return fmt.Errorf("无法获取临时分片目录的绝对路径: %w", err)
	}

	config.Paths.FinalUploadDir, err = filepath.Abs(config.Paths.FinalUploadDir)
	if err != nil {
		return fmt.Errorf("无法获取最终上传目录的绝对路径: %w", err)
	}

	return nil
}
