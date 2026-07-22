package browserproxy

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

func ValidateURL(rawURL string) (string, string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", "", errors.New("proxy URL is required")
	}
	if len(trimmed) > 4096 {
		return "", "", errors.New("proxy URL is too long")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", "", errors.New("proxy URL is invalid")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	switch parsed.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", "", errors.New("proxy URL must use http, https, socks5, or socks5h")
	}
	if parsed.Host == "" || strings.TrimSpace(parsed.Hostname()) == "" {
		return "", "", errors.New("proxy URL host is required")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", "", errors.New("proxy URL must not contain a path")
	}
	if parsed.Fragment != "" {
		return "", "", errors.New("proxy URL must not contain a fragment")
	}

	hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return "", "", errors.New("proxy URL must use a publicly reachable host")
	}
	if ip := net.ParseIP(hostname); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() {
			return "", "", errors.New("proxy URL must use a public IP address")
		}
	}

	parsed.Path = ""
	return parsed.String(), parsed.Scheme, nil
}
