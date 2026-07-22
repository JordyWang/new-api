package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"
	"github.com/QuantumNous/new-api/pkg/codexoauth"
)

var browserAgentVersion = "dev"

type runtimeMapFlag map[string]string

func (values runtimeMapFlag) String() string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func (values runtimeMapFlag) Set(value string) error {
	key, executable, ok := strings.Cut(value, "=")
	key = strings.TrimSpace(key)
	executable = strings.TrimSpace(executable)
	if !ok || !safeIdentifier(key) || executable == "" {
		return errors.New("runtime must use key=/absolute/path/to/chrome format")
	}
	absolute, err := filepath.Abs(executable)
	if err != nil {
		return err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return fmt.Errorf("inspect runtime %q: %w", key, err)
	}
	if info.IsDir() {
		return fmt.Errorf("runtime %q points to a directory", key)
	}
	values[key] = absolute
	return nil
}

type agentConfig struct {
	ServerURL    string
	Token        string
	ProfileRoot  string
	GeoIPURL     string
	PollInterval time.Duration
	Runtimes     map[string]string
}

type agentClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type apiEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func (envelope *apiEnvelope[T]) apiStatus() (string, bool) {
	return envelope.Message, envelope.Success
}

type heartbeatRequest struct {
	InstanceId string         `json:"instance_id"`
	Version    string         `json:"version"`
	Platform   string         `json:"platform"`
	Arch       string         `json:"arch"`
	Runtimes   []string       `json:"runtimes"`
	Metadata   map[string]any `json:"metadata"`
}

type flowInstanceRequest struct {
	InstanceId string `json:"instance_id"`
}

type flowCompleteRequest struct {
	InstanceId   string `json:"instance_id"`
	IDToken      string `json:"id_token"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type flowFailRequest struct {
	InstanceId string `json:"instance_id"`
	Message    string `json:"message"`
}

type flowStatusResponse struct {
	Status string `json:"status"`
}

func main() {
	config, err := parseAgentConfig()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(config.ProfileRoot, 0o700); err != nil {
		log.Fatalf("create profile root: %v", err)
	}

	instanceId := common.GetUUID()
	client := &agentClient{
		baseURL: strings.TrimRight(config.ServerURL, "/"),
		token:   config.Token,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("browser agent %s starting with runtimes: %s", browserAgentVersion, runtimeMapFlag(config.Runtimes).String())
	go heartbeatLoop(ctx, client, instanceId, config.Runtimes)
	if err := claimLoop(ctx, client, instanceId, config); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func parseAgentConfig() (agentConfig, error) {
	runtimes := runtimeMapFlag{}
	geoIPDefault := strings.TrimSpace(os.Getenv("NEW_API_BROWSER_GEOIP_URL"))
	if geoIPDefault == "" {
		geoIPDefault = defaultBrowserGeoIPURL
	}
	serverURL := flag.String("server", strings.TrimSpace(os.Getenv("NEW_API_BROWSER_AGENT_SERVER")), "new-api server URL")
	token := flag.String("token", strings.TrimSpace(os.Getenv("NEW_API_BROWSER_AGENT_TOKEN")), "browser agent token")
	profileRoot := flag.String("profile-root", strings.TrimSpace(os.Getenv("NEW_API_BROWSER_PROFILE_ROOT")), "managed Chromium profile root")
	geoIPURL := flag.String("geoip-url", geoIPDefault, "HTTPS GeoIP endpoint queried through the managed proxy")
	pollInterval := flag.Duration("poll-interval", 2*time.Second, "OAuth claim polling interval")
	flag.Var(runtimes, "runtime", "local runtime mapping: key=/absolute/path/to/chrome (repeatable)")
	flag.Parse()

	if environmentRuntimes := strings.TrimSpace(os.Getenv("NEW_API_BROWSER_RUNTIMES")); environmentRuntimes != "" {
		for _, item := range strings.Split(environmentRuntimes, ";") {
			if strings.TrimSpace(item) == "" {
				continue
			}
			if err := runtimes.Set(item); err != nil {
				return agentConfig{}, err
			}
		}
	}
	if len(runtimes) == 0 {
		return agentConfig{}, errors.New("at least one --runtime mapping is required")
	}

	parsedServer, err := url.Parse(strings.TrimSpace(*serverURL))
	if err != nil || parsedServer.Host == "" {
		return agentConfig{}, errors.New("a valid --server URL is required")
	}
	parsedServer.Scheme = strings.ToLower(parsedServer.Scheme)
	if parsedServer.User != nil || parsedServer.RawQuery != "" || parsedServer.Fragment != "" {
		return agentConfig{}, errors.New("browser agent server URL must not contain credentials, a query, or a fragment")
	}
	hostname := strings.ToLower(parsedServer.Hostname())
	if parsedServer.Scheme != "https" && !(parsedServer.Scheme == "http" && (hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1")) {
		return agentConfig{}, errors.New("remote browser agents require an HTTPS server URL")
	}
	if len(strings.TrimSpace(*token)) < 32 {
		return agentConfig{}, errors.New("a valid --token is required")
	}
	if *pollInterval < 500*time.Millisecond || *pollInterval > 30*time.Second {
		return agentConfig{}, errors.New("poll interval must be between 500ms and 30s")
	}
	normalizedGeoIPURL, err := validateGeoIPURL(*geoIPURL)
	if err != nil {
		return agentConfig{}, err
	}

	root := strings.TrimSpace(*profileRoot)
	if root == "" {
		configRoot, err := os.UserConfigDir()
		if err != nil {
			return agentConfig{}, fmt.Errorf("resolve user config directory: %w", err)
		}
		root = filepath.Join(configRoot, "new-api", "browser-profiles")
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return agentConfig{}, err
	}
	return agentConfig{
		ServerURL:    parsedServer.String(),
		Token:        strings.TrimSpace(*token),
		ProfileRoot:  root,
		GeoIPURL:     normalizedGeoIPURL,
		PollInterval: *pollInterval,
		Runtimes:     map[string]string(runtimes),
	}, nil
}

func heartbeatLoop(ctx context.Context, client *agentClient, instanceId string, runtimes map[string]string) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		if err := client.heartbeat(ctx, instanceId, runtimes); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("heartbeat failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func claimLoop(ctx context.Context, client *agentClient, instanceId string, config agentConfig) error {
	ticker := time.NewTicker(config.PollInterval)
	defer ticker.Stop()

	for {
		claim, err := client.claim(ctx, instanceId)
		if err != nil {
			log.Printf("claim failed: %v", err)
		} else if claim != nil {
			log.Printf("claimed OAuth flow %s for profile %s", claim.FlowId, claim.Profile.Name)
			if err := executeOAuthFlow(ctx, client, instanceId, config, claim); err != nil {
				if !errors.Is(err, errFlowTerminatedByServer) && !errors.Is(err, context.Canceled) {
					log.Printf("OAuth flow %s failed: %v", claim.FlowId, err)
					if failErr := client.failFlow(context.Background(), instanceId, claim.FlowId, err.Error()); failErr != nil {
						log.Printf("report OAuth failure %s: %v", claim.FlowId, failErr)
					}
				}
			} else {
				log.Printf("OAuth flow %s completed", claim.FlowId)
			}
		} else {
			launch, launchErr := client.claimLaunch(ctx, instanceId)
			if launchErr != nil {
				log.Printf("browser launch claim failed: %v", launchErr)
			} else if launch != nil {
				log.Printf("claimed browser launch %s for profile %s", launch.LaunchId, launch.Profile.Name)
				if executeErr := executeBrowserLaunch(ctx, client, instanceId, config, launch); executeErr != nil {
					if !errors.Is(executeErr, errFlowTerminatedByServer) && !errors.Is(executeErr, context.Canceled) {
						log.Printf("browser launch %s failed: %v", launch.LaunchId, executeErr)
						if failErr := client.failLaunch(context.Background(), instanceId, launch.LaunchId, executeErr.Error()); failErr != nil {
							log.Printf("report browser launch failure %s: %v", launch.LaunchId, failErr)
						}
					}
				} else {
					log.Printf("browser launch %s completed", launch.LaunchId)
				}
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (client *agentClient) heartbeat(ctx context.Context, instanceId string, runtimes map[string]string) error {
	keys := make([]string, 0, len(runtimes))
	for key := range runtimes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	hostname, _ := os.Hostname()
	request := heartbeatRequest{
		InstanceId: instanceId,
		Version:    browserAgentVersion,
		Platform:   runtime.GOOS,
		Arch:       runtime.GOARCH,
		Runtimes:   keys,
		Metadata: map[string]any{
			"hostname": hostname,
			"capabilities": []string{
				browseragentapi.CapabilityStrictProxyGeoV1,
				browseragentapi.CapabilityProxyGeoOverlayV1,
			},
		},
	}
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/heartbeat", request, &response)
}

func (client *agentClient) claim(ctx context.Context, instanceId string) (*browseragentapi.CodexOAuthClaim, error) {
	var response apiEnvelope[*browseragentapi.CodexOAuthClaim]
	if err := client.post(ctx, "/api/browser-agent/codex/claim", flowInstanceRequest{InstanceId: instanceId}, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *agentClient) claimLaunch(ctx context.Context, instanceId string) (*browseragentapi.BrowserLaunchClaim, error) {
	var response apiEnvelope[*browseragentapi.BrowserLaunchClaim]
	if err := client.post(ctx, "/api/browser-agent/launches/claim", flowInstanceRequest{InstanceId: instanceId}, &response); err != nil {
		return nil, err
	}
	return response.Data, nil
}

func (client *agentClient) markRunning(ctx context.Context, instanceId string, flowId string) error {
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/codex/"+url.PathEscape(flowId)+"/running", flowInstanceRequest{InstanceId: instanceId}, &response)
}

func (client *agentClient) completeFlow(ctx context.Context, instanceId string, flowId string, token *codexoauth.TokenResult) error {
	expiresIn := int(time.Until(token.ExpiresAt).Seconds())
	if expiresIn <= 0 {
		return errors.New("OAuth token is already expired")
	}
	request := flowCompleteRequest{
		InstanceId:   instanceId,
		IDToken:      token.IDToken,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    expiresIn,
	}
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/codex/"+url.PathEscape(flowId)+"/complete", request, &response)
}

func (client *agentClient) failFlow(ctx context.Context, instanceId string, flowId string, message string) error {
	request := flowFailRequest{InstanceId: instanceId, Message: message}
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/codex/"+url.PathEscape(flowId)+"/fail", request, &response)
}

func (client *agentClient) flowStatus(ctx context.Context, instanceId string, flowId string) (string, error) {
	requestURL := "/api/browser-agent/codex/" + url.PathEscape(flowId) + "/status?instance_id=" + url.QueryEscape(instanceId)
	var response apiEnvelope[flowStatusResponse]
	if err := client.get(ctx, requestURL, &response); err != nil {
		return "", err
	}
	return response.Data.Status, nil
}

func (client *agentClient) markLaunchRunning(ctx context.Context, instanceId string, launchId string) error {
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/launches/"+url.PathEscape(launchId)+"/running", flowInstanceRequest{InstanceId: instanceId}, &response)
}

func (client *agentClient) completeLaunch(ctx context.Context, instanceId string, launchId string) error {
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/launches/"+url.PathEscape(launchId)+"/complete", flowInstanceRequest{InstanceId: instanceId}, &response)
}

func (client *agentClient) failLaunch(ctx context.Context, instanceId string, launchId string, message string) error {
	request := flowFailRequest{InstanceId: instanceId, Message: message}
	var response apiEnvelope[map[string]any]
	return client.post(ctx, "/api/browser-agent/launches/"+url.PathEscape(launchId)+"/fail", request, &response)
}

func (client *agentClient) launchStatus(ctx context.Context, instanceId string, launchId string) (string, error) {
	requestURL := "/api/browser-agent/launches/" + url.PathEscape(launchId) + "/status?instance_id=" + url.QueryEscape(instanceId)
	var response apiEnvelope[flowStatusResponse]
	if err := client.get(ctx, requestURL, &response); err != nil {
		return "", err
	}
	return response.Data.Status, nil
}

func (client *agentClient) post(ctx context.Context, path string, requestBody any, responseBody any) error {
	encoded, err := common.Marshal(requestBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, strings.NewReader(string(encoded)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-browser-agent/"+browserAgentVersion)
	return client.do(req, responseBody)
}

func (client *agentClient) get(ctx context.Context, path string, responseBody any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "new-api-browser-agent/"+browserAgentVersion)
	return client.do(req, responseBody)
}

func (client *agentClient) do(request *http.Request, responseBody any) error {
	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := common.DecodeJson(response.Body, responseBody); err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("control plane returned HTTP %d", response.StatusCode)
	}
	status, ok := responseBody.(interface {
		apiStatus() (string, bool)
	})
	if !ok {
		return errors.New("control plane response type is invalid")
	}
	message, success := status.apiStatus()
	if !success {
		if message == "" {
			message = "control plane rejected the request"
		}
		return errors.New(message)
	}
	return nil
}

func safeIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') {
			continue
		}
		if index > 0 && (character == '.' || character == '_' || character == '-') {
			continue
		}
		return false
	}
	return true
}
