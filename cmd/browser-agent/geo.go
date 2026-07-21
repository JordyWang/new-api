package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"golang.org/x/text/language"
)

const (
	defaultBrowserGeoIPURL = "https://ipapi.co/json/"
	maxGeoIPResponseBytes  = 64 * 1024
)

type proxyGeoIdentity struct {
	IP          string
	CountryCode string
	Timezone    string
	Languages   []string
}

func validateGeoIPURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", errors.New("GeoIP URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || strings.TrimSpace(parsed.Hostname()) == "" {
		return "", errors.New("GeoIP URL is invalid")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("GeoIP URL must not contain credentials or a fragment")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "https" {
		hostname := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
		loopback := hostname == "localhost"
		if ip := net.ParseIP(hostname); ip != nil {
			loopback = ip.IsLoopback()
		}
		if parsed.Scheme != "http" || !loopback {
			return "", errors.New("GeoIP URL must use HTTPS; HTTP is allowed only for loopback testing")
		}
	}
	return parsed.String(), nil
}

func verifyProxyGeoIdentity(
	ctx context.Context,
	localProxyURL string,
	geoIPURL string,
	fingerprint browseragentapi.CodexOAuthFingerprint,
) (*proxyGeoIdentity, error) {
	proxyURL, err := url.Parse(strings.TrimSpace(localProxyURL))
	if err != nil || proxyURL.Scheme != "http" || proxyURL.Host == "" || proxyURL.User != nil {
		return nil, errors.New("local managed proxy URL is invalid")
	}
	proxyIP := net.ParseIP(strings.TrimSuffix(proxyURL.Hostname(), "."))
	if proxyIP == nil || !proxyIP.IsLoopback() || (proxyURL.Path != "" && proxyURL.Path != "/") || proxyURL.RawQuery != "" || proxyURL.Fragment != "" {
		return nil, errors.New("GeoIP verification requires the agent local managed proxy")
	}
	normalizedGeoIPURL, err := validateGeoIPURL(geoIPURL)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		ForceAttemptHTTP2: true,
		IdleConnTimeout:   15 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("GeoIP service redirected too many times")
			}
			_, err := validateGeoIPURL(request.URL.String())
			return err
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedGeoIPURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create GeoIP request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "new-api-browser-agent/"+browserAgentVersion)

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("GeoIP request through managed proxy failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("GeoIP service returned HTTP %d", response.StatusCode)
	}
	responseBytes, err := io.ReadAll(io.LimitReader(response.Body, maxGeoIPResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read GeoIP response: %w", err)
	}
	if len(responseBytes) > maxGeoIPResponseBytes {
		return nil, errors.New("GeoIP response is too large")
	}
	var payload struct {
		IP          string `json:"ip"`
		CountryCode string `json:"country_code"`
		Timezone    string `json:"timezone"`
		Languages   string `json:"languages"`
	}
	if err := common.Unmarshal(responseBytes, &payload); err != nil {
		return nil, fmt.Errorf("decode GeoIP response: %w", err)
	}

	ipText := strings.TrimSpace(payload.IP)
	exitIP := net.ParseIP(ipText)
	if exitIP == nil || !exitIP.IsGlobalUnicast() || exitIP.IsPrivate() {
		return nil, errors.New("GeoIP response does not contain a public proxy exit IP")
	}
	countryCode := strings.ToUpper(strings.TrimSpace(payload.CountryCode))
	if len(countryCode) != 2 || countryCode[0] < 'A' || countryCode[0] > 'Z' || countryCode[1] < 'A' || countryCode[1] > 'Z' {
		return nil, errors.New("GeoIP response country_code is invalid")
	}
	timezone := strings.TrimSpace(payload.Timezone)
	if timezone == "" {
		return nil, errors.New("GeoIP response timezone is missing")
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil, errors.New("GeoIP response timezone is invalid")
	}
	fingerprintTimezone := strings.TrimSpace(fingerprint.Timezone)
	if _, err := time.LoadLocation(fingerprintTimezone); err != nil {
		return nil, errors.New("browser fingerprint timezone is invalid")
	}
	if fingerprintTimezone != timezone {
		return nil, fmt.Errorf("browser fingerprint timezone %q does not match proxy exit timezone %q", fingerprintTimezone, timezone)
	}

	fingerprintLocale, err := language.Parse(strings.TrimSpace(fingerprint.Locale))
	if err != nil {
		return nil, errors.New("browser fingerprint locale is invalid")
	}
	languageValues := strings.Split(payload.Languages, ",")
	languages := make([]string, 0, len(languageValues))
	localeMatches := false
	for _, value := range languageValues {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsedLanguage, err := language.Parse(value)
		if err != nil {
			return nil, errors.New("GeoIP response languages are invalid")
		}
		languages = append(languages, parsedLanguage.String())
		if parsedLanguage == fingerprintLocale {
			localeMatches = true
		}
	}
	if len(languages) == 0 {
		return nil, errors.New("GeoIP response languages are missing")
	}
	if !localeMatches {
		return nil, fmt.Errorf("browser fingerprint locale %q does not match proxy exit languages %q", fingerprint.Locale, payload.Languages)
	}

	return &proxyGeoIdentity{
		IP:          ipText,
		CountryCode: countryCode,
		Timezone:    timezone,
		Languages:   languages,
	}, nil
}
