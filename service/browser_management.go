package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/browserproxy"
)

const browserProxySecretContext = "browser-proxy-url"

type cachedBrowserProxy struct {
	URL       string
	ExpiresAt time.Time
}

var browserProxyURLCache = struct {
	sync.RWMutex
	items map[int]cachedBrowserProxy
}{items: make(map[int]cachedBrowserProxy)}

func ValidateBrowserProxyURL(rawURL string) (string, string, error) {
	return browserproxy.ValidateURL(rawURL)
}

func EncryptBrowserProxyURL(proxyURL string) (string, error) {
	if strings.TrimSpace(os.Getenv("CRYPTO_SECRET")) == "" && strings.TrimSpace(os.Getenv("SESSION_SECRET")) == "" {
		return "", errors.New("CRYPTO_SECRET or SESSION_SECRET must be configured before storing managed browser proxies")
	}
	return common.EncryptSecret(proxyURL, browserProxySecretContext)
}

func DecryptBrowserProxyURL(proxy *model.BrowserProxy) (string, error) {
	if proxy == nil {
		return "", errors.New("browser proxy is nil")
	}
	decrypted, err := common.DecryptSecret(proxy.URLCiphertext, browserProxySecretContext)
	if err != nil {
		return "", fmt.Errorf("decrypt browser proxy %d: %w", proxy.Id, err)
	}
	return decrypted, nil
}

func MaskBrowserProxyURL(proxyURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(proxyURL))
	if err != nil || parsed.Host == "" {
		return "***"
	}
	if parsed.User != nil {
		parsed.User = url.User("***")
	}
	if parsed.RawQuery != "" {
		query := parsed.Query()
		for key := range query {
			query.Set(key, "***")
		}
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func ResolveBrowserProxyURL(proxyId int) (string, error) {
	if proxyId <= 0 {
		return "", errors.New("browser proxy ID is required")
	}
	now := time.Now()
	browserProxyURLCache.RLock()
	cached, ok := browserProxyURLCache.items[proxyId]
	browserProxyURLCache.RUnlock()
	if ok && now.Before(cached.ExpiresAt) {
		return cached.URL, nil
	}

	proxy, err := model.GetBrowserProxyById(proxyId)
	if err != nil {
		return "", err
	}
	if !proxy.Enabled {
		return "", errors.New("managed browser proxy is disabled")
	}
	proxyURL, err := DecryptBrowserProxyURL(proxy)
	if err != nil {
		return "", err
	}

	browserProxyURLCache.Lock()
	browserProxyURLCache.items[proxyId] = cachedBrowserProxy{
		URL:       proxyURL,
		ExpiresAt: now.Add(30 * time.Second),
	}
	browserProxyURLCache.Unlock()
	return proxyURL, nil
}

func ResolveChannelProxyURL(channel *model.Channel) (string, error) {
	if channel == nil {
		return "", errors.New("channel is nil")
	}
	setting := channel.GetSetting()
	if setting.BrowserProxyId > 0 {
		return ResolveBrowserProxyURL(setting.BrowserProxyId)
	}
	return strings.TrimSpace(setting.Proxy), nil
}

func ResetBrowserProxyURLCache() {
	browserProxyURLCache.Lock()
	browserProxyURLCache.items = make(map[int]cachedBrowserProxy)
	browserProxyURLCache.Unlock()
}

func BrowserAgentTokenDigest(token string) string {
	digest := sha256.Sum256([]byte("browser-agent-token:" + strings.TrimSpace(token)))
	return hex.EncodeToString(digest[:])
}
