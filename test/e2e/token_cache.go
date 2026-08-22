package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// TokenCacheEntry 落盘的 token 缓存。
type TokenCacheEntry struct {
	Username string    `json:"username"`
	SSOBase  string    `json:"ssoBase"`
	Token    string    `json:"token"`
	Exp      time.Time `json:"exp"`
	SavedAt  time.Time `json:"savedAt"`
}

func tokenCachePath() string {
	if p := os.Getenv("NAZHI_E2E_TOKEN_CACHE"); p != "" {
		return p
	}
	return filepath.Join(".e2e_token")
}

func loadTokenCache(path string) (*TokenCacheEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var e TokenCacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return nil, err
	}
	return &e, nil
}

func saveTokenCache(path string, e *TokenCacheEntry) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	// 0600 仅本机可读
	return os.WriteFile(path, data, 0600)
}

func isTokenCacheValid(e *TokenCacheEntry, username, ssoBase string) bool {
	if e == nil || e.Token == "" {
		return false
	}
	if e.Username != username || e.SSOBase != ssoBase {
		return false
	}
	// exp 前 10 分钟视为过期，提前刷新避免用时刚好过期
	if time.Now().Add(10 * time.Minute).After(e.Exp) {
		return false
	}
	return true
}
