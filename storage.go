package storage

import "sync"

var (
	shortLinks = make(map[string]string)
	burned     = make(map[string]bool)
	mu         sync.RWMutex
)

func SetShortLink(shortCode, longUrl string) {
	mu.Lock()
	defer mu.Unlock()
	shortLinks[shortCode] = longUrl
	burned[shortCode] = false
}

func GetShortLink(shortCode string) (string, bool, bool) {
	mu.RLock()
	defer mu.RUnlock()
	longUrl, exists := shortLinks[shortCode]
	if !exists {
		return "", false, false
	}
	return longUrl, burned[shortCode], true
}

func MarkAsBurned(shortCode string) {
	mu.Lock()
	defer mu.Unlock()
	burned[shortCode] = true
}

func CheckStoragePermissions() error {
	return nil
}
