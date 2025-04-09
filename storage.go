package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// StorageManager 管理数据存储的结构体
type StorageManager struct {
	config    *Config
	linksLock sync.RWMutex
	links     map[string]string
	dataDir   string
}

var (
	storageManager *StorageManager
	managerLock    sync.RWMutex
)

// InitStorage 初始化存储管理器
func InitStorage(config *Config) error {
	managerLock.Lock()
	defer managerLock.Unlock()

	if storageManager != nil {
		return nil // 已经初始化
	}

	dataDir := filepath.Join(config.Paths.DataStorageDir, "data")
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	storageManager = &StorageManager{
		config:  config,
		links:   make(map[string]string),
		dataDir: dataDir,
	}

	// 加载现有短链接
	return storageManager.loadLinks()
}

// GetStorageManager 获取存储管理器实例
func GetStorageManager() *StorageManager {
	managerLock.RLock()
	defer managerLock.RUnlock()
	return storageManager
}

// loadLinks 从文件加载短链接映射
func (sm *StorageManager) loadLinks() error {
	sm.linksLock.Lock()
	defer sm.linksLock.Unlock()

	linksFile := filepath.Join(sm.dataDir, "shortlinks.json")
	log.Printf("[Storage] 尝试从文件加载链接: %s", linksFile)

	data, err := os.ReadFile(linksFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("[Storage] 链接文件未找到，从空映射开始。")
			return nil
		}
		return fmt.Errorf("读取链接文件失败: %w", err)
	}

	if len(data) == 0 {
		log.Println("[Storage] 链接文件为空，从空映射开始。")
		return nil
	}

	if err := json.Unmarshal(data, &sm.links); err != nil {
		return fmt.Errorf("解析链接文件失败: %w", err)
	}

	log.Printf("[Storage] 成功加载了 %d 个短链接", len(sm.links))
	return nil
}

// SaveLinks 保存短链接映射到文件
func (sm *StorageManager) SaveLinks() error {
	sm.linksLock.RLock()
	defer sm.linksLock.RUnlock()

	linksFile := filepath.Join(sm.dataDir, "shortlinks.json")
	tempFile := linksFile + ".tmp"

	data, err := json.MarshalIndent(sm.links, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化链接数据失败: %w", err)
	}

	// 先写入临时文件
	if err := os.WriteFile(tempFile, data, 0640); err != nil {
		return fmt.Errorf("写入临时文件失败: %w", err)
	}

	// 重命名临时文件为正式文件
	if err := os.Rename(tempFile, linksFile); err != nil {
		// 清理临时文件
		os.Remove(tempFile)
		return fmt.Errorf("重命名临时文件失败: %w", err)
	}

	return nil
}

// StoreShortLink 存储短链接映射
func (sm *StorageManager) StoreShortLink(shortCode, longURL string) error {
	sm.linksLock.Lock()
	sm.links[shortCode] = longURL
	sm.linksLock.Unlock()

	return sm.SaveLinks()
}

// GetLongURL 获取短链接对应的原始URL
func (sm *StorageManager) GetLongURL(shortCode string) (string, bool) {
	sm.linksLock.RLock()
	longURL, exists := sm.links[shortCode]
	sm.linksLock.RUnlock()
	return longURL, exists
}

// DeleteShortLink 删除短链接
func (sm *StorageManager) DeleteShortLink(shortCode string) error {
	sm.linksLock.Lock()
	delete(sm.links, shortCode)
	sm.linksLock.Unlock()

	return sm.SaveLinks()
}

// GetConfig 获取存储管理器的配置
func (sm *StorageManager) GetConfig() *Config {
	return sm.config
}

// SetShortLink 设置短链接到存储系统中
func SetShortLink(shortCode, longURL string) error {
	manager := GetStorageManager()
	if manager == nil {
		return fmt.Errorf("存储管理器未初始化")
	}
	return manager.StoreShortLink(shortCode, longURL)
}

// GetShortLink 从存储系统中获取短链接对应的原始URL
func GetShortLink(shortCode string) (string, bool) {
	manager := GetStorageManager()
	if manager == nil {
		return "", false
	}
	return manager.GetLongURL(shortCode)
}

// StoreMetadata 存储元数据，包括可选的密码保护
func StoreMetadata(metadata *StoredMetadata) error {
	manager := GetStorageManager()
	if manager == nil {
		return fmt.Errorf("存储管理器未初始化")
	}

	dataDir := filepath.Join(manager.dataDir, "data")
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}

	filePath := filepath.Join(dataDir, metadata.ID+".json")
	jsonData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化元数据失败: %w", err)
	}

	if err := os.WriteFile(filePath, jsonData, 0640); err != nil {
		return fmt.Errorf("写入元数据文件失败: %w", err)
	}

	return nil
}

// GetMetadata 获取元数据，包括密码保护信息
func GetMetadata(id string) (*StoredMetadata, error) {
	manager := GetStorageManager()
	if manager == nil {
		return nil, fmt.Errorf("存储管理器未初始化")
	}

	filePath := filepath.Join(manager.dataDir, "data", id+".json")
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("元数据不存在")
		}
		return nil, fmt.Errorf("读取元数据文件失败: %w", err)
	}

	var metadata StoredMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("解析元数据失败: %w", err)
	}

	return &metadata, nil
}
