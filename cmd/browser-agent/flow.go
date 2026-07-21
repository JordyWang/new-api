package main

import (
	"context"
	"crypto/subtle"
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
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"
	"github.com/QuantumNous/new-api/pkg/browserproxy"
	"github.com/QuantumNous/new-api/pkg/codexoauth"
)

var errFlowTerminatedByServer = errors.New("OAuth flow terminated by control plane")

type callbackResult struct {
	Code  string
	Error string
}

type oauthCallbackServer struct {
	server     *http.Server
	listener   net.Listener
	result     chan callbackResult
	completion chan error
	once       sync.Once
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

	fingerprintFile, err := writeFingerprintPayload(profileDir, claim.Fingerprint.Payload)
	if err != nil {
		return err
	}

	forwardProxy, err := startLocalForwardProxy(claim.Proxy.URL)
	if err != nil {
		return fmt.Errorf("start local proxy: %w", err)
	}
	defer forwardProxy.Close()

	callback, err := startOAuthCallbackServer(claim.State)
	if err != nil {
		return fmt.Errorf("listen on OAuth callback port 1455: %w", err)
	}
	defer callback.Close()

	command, err := startManagedBrowser(
		ctx,
		config.Runtimes[claim.Profile.RuntimeKey],
		profileDir,
		fingerprintFile,
		forwardProxy.URL(),
		claim,
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

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-callback.result:
			if result.Error != "" {
				callbackErr := errors.New(result.Error)
				callback.complete(callbackErr)
				return callbackErr
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

func writeFingerprintPayload(profileDir string, payload string) (string, error) {
	var parsed map[string]any
	if err := common.UnmarshalJsonStr(payload, &parsed); err != nil {
		return "", errors.New("fingerprint payload is invalid")
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
	claim *browseragentapi.CodexOAuthClaim,
) (*exec.Cmd, error) {
	launchArgs, err := buildBrowserLaunchArgs(profileDir, fingerprintFile, proxyURL, claim)
	if err != nil {
		return nil, err
	}
	environment, err := buildBrowserEnvironment(profileDir, fingerprintFile, proxyURL, claim)
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

func buildBrowserLaunchArgs(profileDir string, fingerprintFile string, proxyURL string, claim *browseragentapi.CodexOAuthClaim) ([]string, error) {
	var configured []string
	if err := common.UnmarshalJsonStr(claim.Fingerprint.LaunchArgs, &configured); err != nil {
		return nil, errors.New("fingerprint launch arguments are invalid")
	}
	replacements := browserTemplateReplacements(profileDir, fingerprintFile, proxyURL, claim)
	args := make([]string, 0, len(configured)+10)
	for _, argument := range configured {
		argument = replaceBrowserTemplate(argument, replacements)
		if forbiddenBrowserArgument(argument) {
			return nil, fmt.Errorf("fingerprint launch argument %q conflicts with an agent-enforced setting", argument)
		}
		args = append(args, argument)
	}

	args = append(args,
		"--user-data-dir="+profileDir,
		"--proxy-server="+proxyURL,
		"--no-first-run",
		"--no-default-browser-check",
	)
	if claim.Fingerprint.UserAgent != "" {
		args = append(args, "--user-agent="+claim.Fingerprint.UserAgent)
	}
	if claim.Fingerprint.Locale != "" {
		args = append(args, "--lang="+claim.Fingerprint.Locale)
	}
	if claim.Fingerprint.ViewportW > 0 && claim.Fingerprint.ViewportH > 0 {
		args = append(args, "--window-size="+strconv.Itoa(claim.Fingerprint.ViewportW)+","+strconv.Itoa(claim.Fingerprint.ViewportH))
	}
	args = append(args, "--new-window", claim.AuthorizeURL)
	return args, nil
}

func buildBrowserEnvironment(profileDir string, fingerprintFile string, proxyURL string, claim *browseragentapi.CodexOAuthClaim) ([]string, error) {
	var configured map[string]string
	if err := common.UnmarshalJsonStr(claim.Fingerprint.Environment, &configured); err != nil {
		return nil, errors.New("fingerprint environment is invalid")
	}
	replacements := browserTemplateReplacements(profileDir, fingerprintFile, proxyURL, claim)
	environment := os.Environ()
	for key, value := range configured {
		if forbiddenBrowserEnvironmentKey(key) {
			return nil, fmt.Errorf("fingerprint environment variable %q is not allowed", key)
		}
		environment = append(environment, key+"="+replaceBrowserTemplate(value, replacements))
	}
	return environment, nil
}

func browserTemplateReplacements(profileDir string, fingerprintFile string, proxyURL string, claim *browseragentapi.CodexOAuthClaim) map[string]string {
	return map[string]string{
		"{profile_dir}":      profileDir,
		"{fingerprint_file}": fingerprintFile,
		"{proxy_server}":     proxyURL,
		"{authorize_url}":    claim.AuthorizeURL,
		"{locale}":           claim.Fingerprint.Locale,
		"{timezone}":         claim.Fingerprint.Timezone,
		"{user_agent}":       claim.Fingerprint.UserAgent,
		"{viewport_width}":   strconv.Itoa(claim.Fingerprint.ViewportW),
		"{viewport_height}":  strconv.Itoa(claim.Fingerprint.ViewportH),
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
		"--no-proxy-server",
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

func forbiddenBrowserEnvironmentKey(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	if strings.HasPrefix(upper, "DYLD_") {
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

func startOAuthCallbackServer(expectedState string) (*oauthCallbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:1455")
	if err != nil {
		return nil, err
	}
	callback := &oauthCallbackServer{
		listener:   listener,
		result:     make(chan callbackResult, 1),
		completion: make(chan error, 1),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/callback", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
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
	return callback, nil
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
