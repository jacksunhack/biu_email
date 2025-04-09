package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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
	Frontend struct {
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
