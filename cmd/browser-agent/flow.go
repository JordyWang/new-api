package main

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"
	"github.com/QuantumNous/new-api/pkg/browserproxy"
	"github.com/QuantumNous/new-api/pkg/codexoauth"

	"golang.org/x/text/language"
)

var errFlowTerminatedByServer = errors.New("OAuth flow terminated by control plane")

type callbackResult struct {
	Code  string
	Error string
}

type browserGeoReport struct {
	Locale    string   `json:"locale"`
	Languages []string `json:"languages"`
	Timezone  string   `json:"timezone"`
}

type oauthCallbackServer struct {
	server            *http.Server
	listener          net.Listener
	result            chan callbackResult
	completion        chan error
	preflight         chan error
	once              sync.Once
	preflightOnce     sync.Once
	preflightVerified atomic.Bool
}

func executeOAuthFlow(parent context.Context, client *agentClient, instanceId string, config agentConfig, claim *browseragentapi.CodexOAuthClaim) (flowErr error) {
	if claim == nil {
		return errors.New("OAuth claim is nil")
	}
	if err := validateOAuthClaim(claim, config.Runtimes); err != nil {
		return err
	}
	deadline := time.Unix(claim.ExpiresAt, 0)
	ctx, cancel := context.WithDeadline(parent, deadline)
	defer cancel()

	profileDir, cleanupProfile, err := prepareProfileDirectory(config.ProfileRoot, claim)
	if err != nil {
		return err
	}
	defer cleanupProfile()

	forwardProxy, err := startLocalForwardProxy(claim.Proxy.URL)
	if err != nil {
		return fmt.Errorf("start local proxy: %w", err)
	}
	defer forwardProxy.Close()
	proxyIdentity, err := verifyProxyGeoIdentity(ctx, forwardProxy.URL(), config.GeoIPURL)
	if err != nil {
		return fmt.Errorf("verify managed proxy geography: %w", err)
	}
	log.Printf(
		"verified managed proxy exit %s (%s, %s) for locale %s",
		proxyIdentity.IP,
		proxyIdentity.CountryCode,
		proxyIdentity.Timezone,
		proxyIdentity.Locale,
	)
	fingerprintFile, err := writeFingerprintPayload(profileDir, claim.Fingerprint.Payload, proxyIdentity)
	if err != nil {
		return err
	}

	callback, err := startOAuthCallbackServer(claim.State, claim.AuthorizeURL, proxyIdentity)
	if err != nil {
		return fmt.Errorf("listen on OAuth callback port 1455: %w", err)
	}
	defer callback.Close()
	preflightURL := "http://127.0.0.1:1455/browser/preflight?token=" + url.QueryEscape(claim.State)

	command, err := startManagedBrowser(
		ctx,
		config.Runtimes[claim.Profile.RuntimeKey],
		profileDir,
		fingerprintFile,
		forwardProxy.URL(),
		preflightURL,
		claim,
		proxyIdentity,
	)
	if err != nil {
		return err
	}
	defer stopBrowser(command)

	if err := client.markRunning(ctx, instanceId, claim.FlowId); err != nil {
		return fmt.Errorf("mark OAuth flow running: %w", err)
	}

	browserExit := make(chan error, 1)
	go func() {
		browserExit <- command.Wait()
	}()
	leaseTicker := time.NewTicker(10 * time.Second)
	defer leaseTicker.Stop()
	statusTicker := time.NewTicker(3 * time.Second)
	defer statusTicker.Stop()
	preflightTimer := time.NewTimer(30 * time.Second)
	defer preflightTimer.Stop()
	preflightResult := callback.preflight

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case preflightErr := <-preflightResult:
			if preflightErr != nil {
				return fmt.Errorf("browser geography preflight failed: %w", preflightErr)
			}
			if !preflightTimer.Stop() {
				select {
				case <-preflightTimer.C:
				default:
				}
			}
			preflightResult = nil
			log.Printf("browser geography preflight verified for flow %s", claim.FlowId)
		case <-preflightTimer.C:
			if !callback.preflightVerified.Load() {
				return errors.New("browser did not complete geography preflight within 30 seconds")
			}
			preflightResult = nil
		case result := <-callback.result:
			if result.Error != "" {
				callbackErr := errors.New(result.Error)
				callback.complete(callbackErr)
				return callbackErr
			}
			currentIdentity, geoErr := verifyProxyGeoIdentity(ctx, forwardProxy.URL(), config.GeoIPURL)
			if geoErr != nil {
				callback.complete(geoErr)
				return fmt.Errorf("reverify managed proxy geography before token exchange: %w", geoErr)
			}
			if currentIdentity.IP != proxyIdentity.IP || currentIdentity.CountryCode != proxyIdentity.CountryCode || currentIdentity.Locale != proxyIdentity.Locale || currentIdentity.Timezone != proxyIdentity.Timezone || currentIdentity.AcceptLanguage != proxyIdentity.AcceptLanguage {
				geoErr = errors.New("managed proxy exit identity changed during OAuth")
				callback.complete(geoErr)
				return geoErr
			}
			token, exchangeErr := exchangeCodexAuthorizationCode(ctx, result.Code, claim.Verifier, forwardProxy.URL())
			if exchangeErr != nil {
				callback.complete(exchangeErr)
				return fmt.Errorf("exchange OAuth authorization code: %w", exchangeErr)
			}
			if completeErr := client.completeFlow(ctx, instanceId, claim.FlowId, token); completeErr != nil {
				callback.complete(completeErr)
				return fmt.Errorf("save OAuth credential: %w", completeErr)
			}
			callback.complete(nil)
			return nil
		case exitErr := <-browserExit:
			browserExit = nil
			if exitErr != nil {
				return fmt.Errorf("Chromium exited before OAuth completed: %w", exitErr)
			}
			log.Printf("Chromium launcher exited; waiting for the OAuth callback")
		case <-leaseTicker.C:
			if err := client.markRunning(ctx, instanceId, claim.FlowId); err != nil {
				return fmt.Errorf("renew OAuth flow lease: %w", err)
			}
		case <-statusTicker.C:
			status, err := client.flowStatus(ctx, instanceId, claim.FlowId)
			if err != nil {
				log.Printf("check OAuth flow %s status: %v", claim.FlowId, err)
				continue
			}
			switch status {
			case browseragentapi.FlowStatusCanceled, browseragentapi.FlowStatusExpired, browseragentapi.FlowStatusFailed:
				return errFlowTerminatedByServer
			case browseragentapi.FlowStatusCompleted:
				return nil
			}
		}
	}
}

func executeBrowserLaunch(parent context.Context, client *agentClient, instanceId string, config agentConfig, claim *browseragentapi.BrowserLaunchClaim) error {
	if claim == nil {
		return errors.New("browser launch claim is nil")
	}
	if err := validateBrowserLaunchClaim(claim, config.Runtimes); err != nil {
		return err
	}
	ctx, cancel := context.WithDeadline(parent, time.Unix(claim.ExpiresAt, 0))
	defer cancel()

	browserClaim := &browseragentapi.CodexOAuthClaim{
		FlowId:      claim.LaunchId,
		State:       claim.LaunchId,
		ExpiresAt:   claim.ExpiresAt,
		Profile:     claim.Profile,
		Proxy:       claim.Proxy,
		Fingerprint: claim.Fingerprint,
	}
	profileDir, cleanupProfile, err := prepareProfileDirectory(config.ProfileRoot, browserClaim)
	if err != nil {
		return err
	}
	defer cleanupProfile()

	forwardProxy, err := startLocalForwardProxy(claim.Proxy.URL)
	if err != nil {
		return fmt.Errorf("start local proxy: %w", err)
	}
	defer forwardProxy.Close()
	proxyIdentity, err := verifyProxyGeoIdentity(ctx, forwardProxy.URL(), config.GeoIPURL)
	if err != nil {
		return fmt.Errorf("verify managed proxy geography: %w", err)
	}
	log.Printf(
		"verified managed proxy exit %s (%s, %s) for browser launch %s",
		proxyIdentity.IP,
		proxyIdentity.CountryCode,
		proxyIdentity.Timezone,
		claim.LaunchId,
	)
	fingerprintFile, err := writeFingerprintPayload(profileDir, claim.Fingerprint.Payload, proxyIdentity)
	if err != nil {
		return err
	}

	callback, err := startOAuthCallbackServer(claim.LaunchId, claim.StartURL, proxyIdentity)
	if err != nil {
		return fmt.Errorf("listen on browser preflight port 1455: %w", err)
	}
	defer callback.Close()
	preflightURL := "http://127.0.0.1:1455/browser/preflight?token=" + url.QueryEscape(claim.LaunchId)

	command, err := startManagedBrowser(
		ctx,
		config.Runtimes[claim.Profile.RuntimeKey],
		profileDir,
		fingerprintFile,
		forwardProxy.URL(),
		preflightURL,
		browserClaim,
		proxyIdentity,
	)
	if err != nil {
		return err
	}
	defer stopBrowser(command)

	if err := client.markLaunchRunning(ctx, instanceId, claim.LaunchId); err != nil {
		return fmt.Errorf("mark browser launch running: %w", err)
	}

	browserExit := make(chan error, 1)
	go func() {
		browserExit <- command.Wait()
	}()
	leaseTicker := time.NewTicker(10 * time.Second)
	defer leaseTicker.Stop()
	statusTicker := time.NewTicker(3 * time.Second)
	defer statusTicker.Stop()
	preflightTimer := time.NewTimer(30 * time.Second)
	defer preflightTimer.Stop()
	preflightResult := callback.preflight

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case preflightErr := <-preflightResult:
			if preflightErr != nil {
				return fmt.Errorf("browser geography preflight failed: %w", preflightErr)
			}
			if !preflightTimer.Stop() {
				select {
				case <-preflightTimer.C:
				default:
				}
			}
			preflightResult = nil
			log.Printf("browser geography preflight verified for launch %s", claim.LaunchId)
		case <-preflightTimer.C:
			if !callback.preflightVerified.Load() {
				return errors.New("browser did not complete geography preflight within 30 seconds")
			}
			preflightResult = nil
		case exitErr := <-browserExit:
			if exitErr != nil {
				return fmt.Errorf("Chromium exited unexpectedly: %w", exitErr)
			}
			if err := client.completeLaunch(ctx, instanceId, claim.LaunchId); err != nil {
				return fmt.Errorf("complete browser launch: %w", err)
			}
			return nil
		case <-leaseTicker.C:
			if err := client.markLaunchRunning(ctx, instanceId, claim.LaunchId); err != nil {
				return fmt.Errorf("renew browser launch lease: %w", err)
			}
		case <-statusTicker.C:
			status, err := client.launchStatus(ctx, instanceId, claim.LaunchId)
			if err != nil {
				log.Printf("check browser launch %s status: %v", claim.LaunchId, err)
				continue
			}
			switch status {
			case browseragentapi.FlowStatusCanceled, browseragentapi.FlowStatusExpired, browseragentapi.FlowStatusFailed:
				return errFlowTerminatedByServer
			case browseragentapi.FlowStatusCompleted:
				return nil
			}
		}
	}
}

func validateBrowserLaunchClaim(claim *browseragentapi.BrowserLaunchClaim, runtimes map[string]string) error {
	if !safeIdentifier(claim.LaunchId) || !safeIdentifier(claim.Profile.DataKey) {
		return errors.New("control plane returned an invalid launch or profile key")
	}
	if _, ok := runtimes[claim.Profile.RuntimeKey]; !ok {
		return fmt.Errorf("runtime %q is not configured locally", claim.Profile.RuntimeKey)
	}
	startURL, err := url.Parse(claim.StartURL)
	if err != nil || startURL.Scheme != "https" || !strings.EqualFold(startURL.Host, "chatgpt.com") || startURL.Path != "/" || startURL.RawQuery != "" || startURL.Fragment != "" || startURL.User != nil {
		return errors.New("control plane returned an invalid standalone browser URL")
	}
	if claim.ExpiresAt <= time.Now().Unix() {
		return errors.New("browser launch is already expired")
	}
	_, _, err = browserproxy.ValidateURL(claim.Proxy.URL)
	return err
}

func validateOAuthClaim(claim *browseragentapi.CodexOAuthClaim, runtimes map[string]string) error {
	if !safeIdentifier(claim.FlowId) || !safeIdentifier(claim.Profile.DataKey) {
		return errors.New("control plane returned an invalid flow or profile key")
	}
	if _, ok := runtimes[claim.Profile.RuntimeKey]; !ok {
		return fmt.Errorf("runtime %q is not configured locally", claim.Profile.RuntimeKey)
	}
	authorizeURL, err := url.Parse(claim.AuthorizeURL)
	if err != nil || authorizeURL.Scheme != "https" || !strings.EqualFold(authorizeURL.Hostname(), "auth.openai.com") || authorizeURL.Path != "/oauth/authorize" {
		return errors.New("control plane returned an invalid Codex authorization URL")
	}
	if authorizeURL.Query().Get("state") != claim.State {
		return errors.New("Codex authorization URL state does not match the claimed flow")
	}
	if claim.ExpiresAt <= time.Now().Unix() {
		return errors.New("OAuth flow is already expired")
	}
	_, _, err = browserproxy.ValidateURL(claim.Proxy.URL)
	return err
}

func exchangeCodexAuthorizationCode(ctx context.Context, code string, verifier string, proxyURL string) (*codexoauth.TokenResult, error) {
	parsedProxy, err := url.Parse(proxyURL)
	if err != nil || parsedProxy.Scheme != "http" || parsedProxy.Host == "" {
		return nil, errors.New("local OAuth proxy URL is invalid")
	}
	transport := &http.Transport{
		Proxy:             http.ProxyURL(parsedProxy),
		ForceAttemptHTTP2: true,
		IdleConnTimeout:   30 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   20 * time.Second,
	}
	return codexoauth.ExchangeAuthorizationCode(ctx, client, code, verifier)
}

func prepareProfileDirectory(root string, claim *browseragentapi.CodexOAuthClaim) (string, func(), error) {
	if !claim.Profile.Persistent {
		directory, err := os.MkdirTemp(root, "ephemeral-"+claim.Profile.DataKey+"-")
		if err != nil {
			return "", func() {}, err
		}
		return directory, func() {
			if err := os.RemoveAll(directory); err != nil {
				log.Printf("remove ephemeral profile %s: %v", claim.Profile.Name, err)
			}
		}, nil
	}

	directory := filepath.Join(root, claim.Profile.DataKey)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", func() {}, err
	}
	return directory, func() {}, nil
}

func writeFingerprintPayload(profileDir string, payload string, geo *proxyGeoIdentity) (string, error) {
	if geo == nil || geo.Locale == "" || geo.Timezone == "" || geo.AcceptLanguage == "" {
		return "", errors.New("proxy geography overlay is incomplete")
	}
	var parsed map[string]any
	if err := common.UnmarshalJsonStr(payload, &parsed); err != nil {
		return "", errors.New("fingerprint payload is invalid")
	}
	if parsed == nil {
		return "", errors.New("fingerprint payload must be a JSON object")
	}
	for _, key := range []string{"accept_language", "accept_languages", "country_code", "geo_overlay", "languages", "locale", "timezone", "timezone_id"} {
		delete(parsed, key)
	}
	fingerprint := parsed
	if fingerprintValue, exists := parsed["fingerprint"]; exists {
		nestedFingerprint, ok := fingerprintValue.(map[string]any)
		if !ok {
			return "", errors.New("fingerprint payload fingerprint field is invalid")
		}
		fingerprint = nestedFingerprint
	}
	for _, key := range []string{"accept_language", "accept_languages", "country_code", "geo_overlay", "languages", "locale", "timezone", "timezone_id"} {
		delete(fingerprint, key)
	}
	if navigator, ok := fingerprint["navigator"].(map[string]any); ok {
		delete(navigator, "language")
		delete(navigator, "languages")
	}
	fingerprint["accept_language"] = geo.AcceptLanguage
	fingerprint["timezone"] = geo.Timezone
	parsed["geo_overlay"] = map[string]any{
		"country_code": geo.CountryCode,
		"locale":       geo.Locale,
		"timezone":     geo.Timezone,
		"languages":    geo.Languages,
	}
	encoded, err := common.Marshal(parsed)
	if err != nil {
		return "", err
	}
	directory := filepath.Join(profileDir, ".new-api")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(directory, "fingerprint.json")
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func startManagedBrowser(
	ctx context.Context,
	executable string,
	profileDir string,
	fingerprintFile string,
	proxyURL string,
	startURL string,
	claim *browseragentapi.CodexOAuthClaim,
	geo *proxyGeoIdentity,
) (*exec.Cmd, error) {
	fingerprintConfig, err := os.ReadFile(fingerprintFile)
	if err != nil {
		return nil, fmt.Errorf("read managed fingerprint config: %w", err)
	}
	fingerprintConfigJSON := base64.StdEncoding.EncodeToString(fingerprintConfig)
	launchArgs, err := buildBrowserLaunchArgs(profileDir, fingerprintFile, fingerprintConfigJSON, proxyURL, startURL, claim, geo)
	if err != nil {
		return nil, err
	}
	environment, err := buildBrowserEnvironment(profileDir, fingerprintFile, fingerprintConfigJSON, proxyURL, startURL, claim, geo)
	if err != nil {
		return nil, err
	}

	command := exec.CommandContext(ctx, executable, launchArgs...)
	command.Env = environment
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start Chromium runtime %q: %w", claim.Profile.RuntimeKey, err)
	}
	return command, nil
}

func buildBrowserLaunchArgs(profileDir string, fingerprintFile string, fingerprintConfigJSON string, proxyURL string, startURL string, claim *browseragentapi.CodexOAuthClaim, geo *proxyGeoIdentity) ([]string, error) {
	var configured []string
	if err := common.UnmarshalJsonStr(claim.Fingerprint.LaunchArgs, &configured); err != nil {
		return nil, errors.New("fingerprint launch arguments are invalid")
	}
	replacements := browserTemplateReplacements(profileDir, fingerprintFile, proxyURL, startURL, claim, geo)
	args := make([]string, 0, len(configured)+12)
	for _, argument := range configured {
		argument = replaceBrowserTemplate(argument, replacements)
		if managedFingerprintArgument(argument) {
			continue
		}
		if forbiddenBrowserArgument(argument) {
			return nil, fmt.Errorf("fingerprint launch argument %q conflicts with an agent-enforced setting", argument)
		}
		args = append(args, argument)
	}

	args = append(args,
		"--user-data-dir="+profileDir,
		"--transfigure-fingerprint-config="+fingerprintFile,
		"--transfigure-fingerprint-config-json="+fingerprintConfigJSON,
		"--proxy-server="+proxyURL,
		"--proxy-bypass-list=<-loopback>;localhost;127.0.0.1;[::1]",
		"--disable-quic",
		"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		"--force-time-zone-for-testing="+geo.Timezone,
		"--no-first-run",
		"--no-default-browser-check",
	)
	if claim.Fingerprint.UserAgent != "" {
		args = append(args, "--user-agent="+claim.Fingerprint.UserAgent)
	}
	if geo.Locale != "" {
		args = append(args, "--lang="+geo.Locale)
	}
	if claim.Fingerprint.ViewportW > 0 && claim.Fingerprint.ViewportH > 0 {
		args = append(args, "--window-size="+strconv.Itoa(claim.Fingerprint.ViewportW)+","+strconv.Itoa(claim.Fingerprint.ViewportH))
	}
	args = append(args, "--new-window", startURL)
	return args, nil
}

func buildBrowserEnvironment(profileDir string, fingerprintFile string, fingerprintConfigJSON string, proxyURL string, startURL string, claim *browseragentapi.CodexOAuthClaim, geo *proxyGeoIdentity) ([]string, error) {
	var configured map[string]string
	if err := common.UnmarshalJsonStr(claim.Fingerprint.Environment, &configured); err != nil {
		return nil, errors.New("fingerprint environment is invalid")
	}
	replacements := browserTemplateReplacements(profileDir, fingerprintFile, proxyURL, startURL, claim, geo)
	environment := make([]string, 0, len(os.Environ())+len(configured)+8)
	for _, item := range os.Environ() {
		key, _, found := strings.Cut(item, "=")
		if found && managedBrowserEnvironmentKey(key) {
			continue
		}
		environment = append(environment, item)
	}
	for key, value := range configured {
		if forbiddenBrowserEnvironmentKey(key) {
			return nil, fmt.Errorf("fingerprint environment variable %q is not allowed", key)
		}
		environment = append(environment, key+"="+replaceBrowserTemplate(value, replacements))
	}
	posixLocale := strings.ReplaceAll(geo.Locale, "-", "_") + ".UTF-8"
	environment = append(environment,
		"TRANSFIGURE_FINGERPRINT_CONFIG="+fingerprintFile,
		"TRANSFIGURE_FINGERPRINT_CONFIG_JSON="+fingerprintConfigJSON,
		"TZ="+geo.Timezone,
		"LANG="+posixLocale,
		"LANGUAGE="+geo.Locale,
		"LC_ALL="+posixLocale,
		"HTTP_PROXY="+proxyURL,
		"HTTPS_PROXY="+proxyURL,
		"ALL_PROXY="+proxyURL,
		"NO_PROXY=localhost,127.0.0.1,::1",
	)
	return environment, nil
}

func browserTemplateReplacements(profileDir string, fingerprintFile string, proxyURL string, startURL string, claim *browseragentapi.CodexOAuthClaim, geo *proxyGeoIdentity) map[string]string {
	return map[string]string{
		"{profile_dir}":         profileDir,
		"{fingerprint_file}":    fingerprintFile,
		"{proxy_server}":        proxyURL,
		"{authorize_url}":       startURL,
		"{oauth_preflight_url}": startURL,
		"{locale}":              geo.Locale,
		"{timezone}":            geo.Timezone,
		"{user_agent}":          claim.Fingerprint.UserAgent,
		"{viewport_width}":      strconv.Itoa(claim.Fingerprint.ViewportW),
		"{viewport_height}":     strconv.Itoa(claim.Fingerprint.ViewportH),
	}
}

func replaceBrowserTemplate(value string, replacements map[string]string) string {
	for placeholder, replacement := range replacements {
		value = strings.ReplaceAll(value, placeholder, replacement)
	}
	return value
}

func forbiddenBrowserArgument(argument string) bool {
	lower := strings.ToLower(strings.TrimSpace(argument))
	for _, prefix := range []string{
		"--user-data-dir",
		"--proxy-server",
		"--proxy-pac-url",
		"--proxy-bypass-list",
		"--proxy-auto-detect",
		"--no-proxy-server",
		"--user-agent",
		"--lang",
		"--window-size",
		"--force-time-zone-for-testing",
		"--force-webrtc-ip-handling-policy",
		"--transfigure-fingerprint-config",
		"--transfigure-fingerprint-config-json",
		"--enable-quic",
		"--disable-quic",
		"--origin-to-force-quic-on",
		"--host-resolver-rules",
		"--remote-debugging-address",
		"--remote-debugging-port",
		"--remote-debugging-pipe",
	} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"=") {
			return true
		}
	}
	return false
}

func managedFingerprintArgument(argument string) bool {
	lower := strings.ToLower(strings.TrimSpace(argument))
	for _, prefix := range []string{
		"--transfigure-fingerprint-config",
		"--transfigure-fingerprint-config-json",
	} {
		if lower == prefix || strings.HasPrefix(lower, prefix+"=") {
			return true
		}
	}
	return false
}

func forbiddenBrowserEnvironmentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if strings.HasPrefix(upper, "DYLD_") {
		return true
	}
	if managedBrowserEnvironmentKey(upper) {
		return true
	}
	_, denied := map[string]struct{}{
		"LD_PRELOAD": {}, "LD_LIBRARY_PATH": {}, "PATH": {}, "HOME": {},
		"SHELL": {}, "BASH_ENV": {}, "ENV": {}, "GCONV_PATH": {},
		"PYTHONPATH": {}, "PYTHONHOME": {}, "NODE_OPTIONS": {},
		"RUBYOPT": {}, "PERL5OPT": {},
	}[upper]
	return denied
}

func managedBrowserEnvironmentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if strings.HasPrefix(upper, "LC_") {
		return true
	}
	_, managed := map[string]struct{}{
		"TZ": {}, "LANG": {}, "LANGUAGE": {},
		"HTTP_PROXY": {}, "HTTPS_PROXY": {}, "ALL_PROXY": {}, "NO_PROXY": {},
		"TRANSFIGURE_FINGERPRINT_CONFIG": {}, "TRANSFIGURE_FINGERPRINT_CONFIG_JSON": {},
	}[upper]
	return managed
}

func validateBrowserReportedGeo(report browserGeoReport, geo *proxyGeoIdentity) error {
	if geo == nil {
		return errors.New("proxy geography overlay is missing")
	}
	reportedLocale, err := language.Parse(strings.TrimSpace(report.Locale))
	if err != nil {
		return errors.New("browser reported an invalid locale")
	}
	expectedLocale, err := language.Parse(strings.TrimSpace(geo.Locale))
	if err != nil {
		return errors.New("proxy geography locale is invalid")
	}
	if reportedLocale != expectedLocale {
		return fmt.Errorf("browser locale %q does not match proxy geography locale %q", report.Locale, geo.Locale)
	}
	languageFound := false
	for _, reportedLanguage := range report.Languages {
		parsedLanguage, err := language.Parse(strings.TrimSpace(reportedLanguage))
		if err == nil && parsedLanguage == expectedLocale {
			languageFound = true
			break
		}
	}
	if !languageFound {
		return fmt.Errorf("browser languages do not contain proxy geography locale %q", geo.Locale)
	}
	if strings.TrimSpace(report.Timezone) != strings.TrimSpace(geo.Timezone) {
		return fmt.Errorf("browser timezone %q does not match proxy geography timezone %q", report.Timezone, geo.Timezone)
	}
	return nil
}

const browserFingerprintPreflightPage = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Browser verification</title>
</head>
<body>
  <p id="status">Verifying managed browser geography…</p>
  <script>
    (async () => {
      const response = await fetch(window.location.pathname + window.location.search, {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
          locale: navigator.language || '',
          languages: Array.from(navigator.languages || []),
          timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || ''
        })
      });
      if (!response.ok) throw new Error(await response.text());
      const result = await response.json();
      window.location.replace(result.authorize_url);
    })().catch((error) => {
      document.getElementById('status').textContent = 'Browser geography verification failed: ' + error.message;
    });
  </script>
</body>
</html>`

func startOAuthCallbackServer(expectedState string, authorizeURL string, geo *proxyGeoIdentity) (*oauthCallbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		return nil, err
	}
	return serveOAuthCallbackServer(listener, expectedState, authorizeURL, geo), nil
}

func serveOAuthCallbackServer(listener net.Listener, expectedState string, authorizeURL string, geo *proxyGeoIdentity) *oauthCallbackServer {
	callback := &oauthCallbackServer{
		listener:   listener,
		result:     make(chan callbackResult, 1),
		completion: make(chan error, 1),
		preflight:  make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/browser/preflight", func(response http.ResponseWriter, request *http.Request) {
		token := request.URL.Query().Get("token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(expectedState)) != 1 {
			http.Error(response, "browser preflight token mismatch", http.StatusBadRequest)
			return
		}
		switch request.Method {
		case http.MethodGet:
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			response.Header().Set("Cache-Control", "no-store")
			response.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; connect-src 'self'")
			_, _ = response.Write([]byte(browserFingerprintPreflightPage))
		case http.MethodPost:
			request.Body = http.MaxBytesReader(response, request.Body, 16*1024)
			var report browserGeoReport
			if err := common.DecodeJson(request.Body, &report); err != nil {
				preflightErr := errors.New("browser geography report is invalid")
				callback.preflightOnce.Do(func() { callback.preflight <- preflightErr })
				http.Error(response, preflightErr.Error(), http.StatusBadRequest)
				return
			}
			if err := validateBrowserReportedGeo(report, geo); err != nil {
				callback.preflightOnce.Do(func() { callback.preflight <- err })
				http.Error(response, err.Error(), http.StatusConflict)
				return
			}
			callback.preflightVerified.Store(true)
			callback.preflightOnce.Do(func() { callback.preflight <- nil })
			encoded, err := common.Marshal(map[string]string{"authorize_url": authorizeURL})
			if err != nil {
				http.Error(response, "failed to create browser preflight response", http.StatusInternalServerError)
				return
			}
			response.Header().Set("Content-Type", "application/json")
			response.Header().Set("Cache-Control", "no-store")
			_, _ = response.Write(encoded)
		default:
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/auth/callback", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !callback.preflightVerified.Load() {
			http.Error(response, "browser geography preflight is incomplete", http.StatusConflict)
			return
		}
		state := request.URL.Query().Get("state")
		if subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
			http.Error(response, "OAuth state mismatch", http.StatusBadRequest)
			return
		}
		if oauthError := strings.TrimSpace(request.URL.Query().Get("error")); oauthError != "" {
			http.Error(response, "OAuth authorization was not completed", http.StatusBadRequest)
			callback.once.Do(func() {
				callback.result <- callbackResult{Error: "OAuth authorization was not completed"}
			})
			return
		}
		code := strings.TrimSpace(request.URL.Query().Get("code"))
		if code == "" {
			http.Error(response, "authorization code is missing", http.StatusBadRequest)
			callback.once.Do(func() {
				callback.result <- callbackResult{Error: "authorization code is missing"}
			})
			return
		}
		accepted := false
		callback.once.Do(func() {
			accepted = true
			callback.result <- callbackResult{Code: code}
		})
		if !accepted {
			http.Error(response, "OAuth callback was already processed", http.StatusConflict)
			return
		}

		select {
		case completionErr := <-callback.completion:
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			response.Header().Set("Cache-Control", "no-store")
			if completionErr != nil {
				response.WriteHeader(http.StatusBadGateway)
				_, _ = response.Write([]byte("<!doctype html><title>OAuth failed</title><h1>OAuth could not be completed</h1><p>Return to new-api and review the flow error.</p>"))
				return
			}
			_, _ = response.Write([]byte("<!doctype html><title>OAuth complete</title><h1>OAuth completed</h1><p>You can close this browser window and return to new-api.</p>"))
		case <-request.Context().Done():
		}
	})
	callback.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := callback.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("OAuth callback server: %v", err)
		}
	}()
	return callback
}

func (callback *oauthCallbackServer) complete(err error) {
	select {
	case callback.completion <- err:
	default:
	}
}

func (callback *oauthCallbackServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = callback.server.Shutdown(ctx)
	_ = callback.listener.Close()
}

func stopBrowser(command *exec.Cmd) {
	if command == nil || command.Process == nil {
		return
	}
	_ = command.Process.Signal(os.Interrupt)
	time.AfterFunc(3*time.Second, func() {
		_ = command.Process.Kill()
	})
}
