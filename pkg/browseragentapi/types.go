package browseragentapi

const (
	FlowStatusPending   = "pending"
	FlowStatusClaimed   = "claimed"
	FlowStatusRunning   = "running"
	FlowStatusCompleted = "completed"
	FlowStatusFailed    = "failed"
	FlowStatusCanceled  = "canceled"
	FlowStatusExpired   = "expired"

	CapabilityStrictProxyGeoV1  = "strict_proxy_geo_v1"
	CapabilityProxyGeoOverlayV1 = "proxy_geo_overlay_v1"
)

type CodexOAuthClaim struct {
	FlowId       string                `json:"flow_id"`
	AuthorizeURL string                `json:"authorize_url"`
	State        string                `json:"state"`
	Verifier     string                `json:"verifier"`
	ExpiresAt    int64                 `json:"expires_at"`
	Profile      CodexOAuthProfile     `json:"profile"`
	Proxy        CodexOAuthProxy       `json:"proxy"`
	Fingerprint  CodexOAuthFingerprint `json:"fingerprint"`
}

type CodexOAuthProfile struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	RuntimeKey string `json:"runtime_key"`
	DataKey    string `json:"data_key"`
	Persistent bool   `json:"persistent"`
}

type CodexOAuthProxy struct {
	Id  int    `json:"id"`
	URL string `json:"url"`
}

type CodexOAuthFingerprint struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	UserAgent   string `json:"user_agent"`
	ViewportW   int    `json:"viewport_width"`
	ViewportH   int    `json:"viewport_height"`
	Payload     string `json:"payload"`
	LaunchArgs  string `json:"launch_args"`
	Environment string `json:"environment"`
}
