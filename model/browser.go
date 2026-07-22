package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/browseragentapi"

	"gorm.io/gorm"
)

const (
	BrowserAgentStatusOffline          = "offline"
	BrowserAgentStatusOnline           = "online"
	DefaultBrowserProxyChannelAccounts = 5
	MaxBrowserProxyChannelAccounts     = 100000

	CodexOAuthFlowStatusPending   = browseragentapi.FlowStatusPending
	CodexOAuthFlowStatusClaimed   = browseragentapi.FlowStatusClaimed
	CodexOAuthFlowStatusRunning   = browseragentapi.FlowStatusRunning
	CodexOAuthFlowStatusCompleted = browseragentapi.FlowStatusCompleted
	CodexOAuthFlowStatusFailed    = browseragentapi.FlowStatusFailed
	CodexOAuthFlowStatusCanceled  = browseragentapi.FlowStatusCanceled
	CodexOAuthFlowStatusExpired   = browseragentapi.FlowStatusExpired
)

var (
	ErrBrowserAgentBusy         = errors.New("browser agent already has an active browser operation")
	ErrBrowserProfileBound      = errors.New("browser profile is bound to another channel")
	ErrBrowserChannelBound      = errors.New("channel is bound to another browser profile")
	ErrBrowserProxyChannelLimit = errors.New("managed proxy channel account limit reached")
	ErrCodexOAuthFlowExpired    = errors.New("codex OAuth flow expired")
	ErrCodexOAuthFlowState      = errors.New("codex OAuth flow is not in the required state")
	ErrCodexOAuthFlowInstance   = errors.New("codex OAuth flow belongs to another agent instance")
	ErrBrowserLaunchExpired     = errors.New("browser launch expired")
	ErrBrowserLaunchState       = errors.New("browser launch is not in the required state")
	ErrBrowserLaunchInstance    = errors.New("browser launch belongs to another agent instance")
)

var codexOAuthBrowserActiveStatuses = []string{
	CodexOAuthFlowStatusPending,
	CodexOAuthFlowStatusClaimed,
	CodexOAuthFlowStatusRunning,
}

var browserLaunchActiveStatuses = []string{
	browseragentapi.FlowStatusPending,
	browseragentapi.FlowStatusClaimed,
	browseragentapi.FlowStatusRunning,
}

type BrowserAgent struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	TokenDigest string `json:"-" gorm:"type:char(64);uniqueIndex;not null"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status" gorm:"type:varchar(32);index"`
	LastSeenAt  int64  `json:"last_seen_at" gorm:"bigint;index"`
	Version     string `json:"version" gorm:"type:varchar(64)"`
	Platform    string `json:"platform" gorm:"type:varchar(32)"`
	Arch        string `json:"arch" gorm:"type:varchar(32)"`
	Runtimes    string `json:"runtimes" gorm:"type:text"`
	Metadata    string `json:"metadata" gorm:"type:text"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (BrowserAgent) TableName() string {
	return "browser_agents"
}

type BrowserProxy struct {
	Id                 int    `json:"id" gorm:"primaryKey"`
	Name               string `json:"name" gorm:"type:varchar(128);not null"`
	URLCiphertext      string `json:"-" gorm:"type:text;not null"`
	Scheme             string `json:"scheme" gorm:"type:varchar(16);index"`
	Enabled            bool   `json:"enabled"`
	MaxChannelAccounts int    `json:"max_channel_accounts"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint"`
}

func (BrowserProxy) TableName() string {
	return "browser_proxies"
}

type BrowserFingerprint struct {
	Id          int    `json:"id" gorm:"primaryKey"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	UserAgent   string `json:"user_agent" gorm:"type:text"`
	ViewportW   int    `json:"viewport_width"`
	ViewportH   int    `json:"viewport_height"`
	Payload     string `json:"payload" gorm:"type:text"`
	LaunchArgs  string `json:"launch_args" gorm:"type:text"`
	Environment string `json:"environment" gorm:"type:text"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt   int64  `json:"updated_at" gorm:"bigint"`
}

func (BrowserFingerprint) TableName() string {
	return "browser_fingerprints"
}

type BrowserProfile struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	Name          string `json:"name" gorm:"type:varchar(128);not null"`
	ChannelId     *int   `json:"channel_id" gorm:"uniqueIndex"`
	AgentId       int    `json:"agent_id" gorm:"index;not null"`
	ProxyId       int    `json:"proxy_id" gorm:"index;not null"`
	FingerprintId int    `json:"fingerprint_id" gorm:"index;not null"`
	RuntimeKey    string `json:"runtime_key" gorm:"type:varchar(128);not null"`
	DataKey       string `json:"-" gorm:"type:varchar(64);uniqueIndex;not null"`
	Persistent    bool   `json:"persistent"`
	Enabled       bool   `json:"enabled"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (BrowserProfile) TableName() string {
	return "browser_profiles"
}

type BrowserLaunch struct {
	Id              string `json:"id" gorm:"type:varchar(40);primaryKey"`
	ProfileId       int    `json:"profile_id" gorm:"index;not null"`
	ChannelId       int    `json:"channel_id" gorm:"index;not null"`
	AgentId         int    `json:"agent_id" gorm:"index;not null"`
	ProxyId         int    `json:"proxy_id" gorm:"index;not null"`
	FingerprintId   int    `json:"fingerprint_id" gorm:"index;not null"`
	Status          string `json:"status" gorm:"type:varchar(32);index;not null"`
	AgentInstanceId string `json:"agent_instance_id" gorm:"type:varchar(64);index"`
	StartURL        string `json:"-" gorm:"type:text;not null"`
	ExpiresAt       int64  `json:"expires_at" gorm:"bigint;index"`
	LeaseExpiresAt  int64  `json:"lease_expires_at" gorm:"bigint;index"`
	CreatedAt       int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt       int64  `json:"updated_at" gorm:"bigint"`
	ClaimedAt       int64  `json:"claimed_at" gorm:"bigint"`
	RunningAt       int64  `json:"running_at" gorm:"bigint"`
	CompletedAt     int64  `json:"completed_at" gorm:"bigint"`
	ErrorMessage    string `json:"error_message" gorm:"type:varchar(512)"`
}

func (BrowserLaunch) TableName() string {
	return "browser_launches"
}

type CodexOAuthFlow struct {
	Id                    string `json:"id" gorm:"type:varchar(40);primaryKey"`
	UserId                int    `json:"user_id" gorm:"index;not null"`
	ChannelId             int    `json:"channel_id" gorm:"index"`
	ProfileId             int    `json:"profile_id" gorm:"index;not null"`
	AgentId               int    `json:"agent_id" gorm:"index;not null"`
	ProxyId               int    `json:"proxy_id" gorm:"index;not null"`
	FingerprintId         int    `json:"fingerprint_id" gorm:"index;not null"`
	Status                string `json:"status" gorm:"type:varchar(32);index;not null"`
	AgentInstanceId       string `json:"agent_instance_id" gorm:"type:varchar(64);index"`
	StateDigest           string `json:"-" gorm:"type:char(64);not null"`
	StateCiphertext       string `json:"-" gorm:"type:text;not null"`
	VerifierCiphertext    string `json:"-" gorm:"type:text;not null"`
	CredentialsCiphertext string `json:"-" gorm:"type:text"`
	AuthorizeURL          string `json:"-" gorm:"type:text;not null"`
	AccountId             string `json:"account_id" gorm:"type:varchar(128)"`
	Email                 string `json:"email" gorm:"type:varchar(256)"`
	PlanType              string `json:"plan_type" gorm:"type:varchar(64)"`
	CredentialExpiresAt   int64  `json:"credential_expires_at" gorm:"bigint"`
	ExpiresAt             int64  `json:"expires_at" gorm:"bigint;index"`
	LeaseExpiresAt        int64  `json:"lease_expires_at" gorm:"bigint;index"`
	CreatedAt             int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt             int64  `json:"updated_at" gorm:"bigint"`
	ClaimedAt             int64  `json:"claimed_at" gorm:"bigint"`
	CompletedAt           int64  `json:"completed_at" gorm:"bigint"`
	ConsumedAt            int64  `json:"consumed_at" gorm:"bigint"`
	ErrorMessage          string `json:"error_message" gorm:"type:varchar(512)"`
}

func (CodexOAuthFlow) TableName() string {
	return "codex_oauth_flows"
}

func ListBrowserAgents() ([]BrowserAgent, error) {
	var agents []BrowserAgent
	err := DB.Order("id desc").Find(&agents).Error
	return agents, err
}

func GetBrowserAgentById(id int) (*BrowserAgent, error) {
	var agent BrowserAgent
	if err := DB.First(&agent, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func GetBrowserAgentByTokenDigest(digest string) (*BrowserAgent, error) {
	var agent BrowserAgent
	if err := DB.Where("token_digest = ?", digest).First(&agent).Error; err != nil {
		return nil, err
	}
	return &agent, nil
}

func CreateBrowserAgent(agent *BrowserAgent) error {
	return DB.Create(agent).Error
}

func UpdateBrowserAgent(agent *BrowserAgent) error {
	return DB.Model(&BrowserAgent{}).Where("id = ?", agent.Id).Updates(map[string]any{
		"name":       agent.Name,
		"enabled":    agent.Enabled,
		"updated_at": agent.UpdatedAt,
	}).Error
}

func DeleteBrowserAgent(id int) error {
	return DB.Delete(&BrowserAgent{}, id).Error
}

func TouchBrowserAgent(id int, version string, platform string, arch string, runtimes string, metadata string, now int64) error {
	return DB.Model(&BrowserAgent{}).Where("id = ?", id).Updates(map[string]any{
		"status":       BrowserAgentStatusOnline,
		"last_seen_at": now,
		"version":      version,
		"platform":     platform,
		"arch":         arch,
		"runtimes":     runtimes,
		"metadata":     metadata,
		"updated_at":   now,
	}).Error
}

func RotateBrowserAgentToken(id int, digest string, now int64) error {
	return DB.Model(&BrowserAgent{}).Where("id = ?", id).Updates(map[string]any{
		"token_digest": digest,
		"updated_at":   now,
	}).Error
}

func ListBrowserProxies() ([]BrowserProxy, error) {
	var proxies []BrowserProxy
	err := DB.Order("id desc").Find(&proxies).Error
	return proxies, err
}

func GetBrowserProxyById(id int) (*BrowserProxy, error) {
	var proxy BrowserProxy
	if err := DB.First(&proxy, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &proxy, nil
}

func CreateBrowserProxy(proxy *BrowserProxy) error {
	if proxy.MaxChannelAccounts < 0 || proxy.MaxChannelAccounts > MaxBrowserProxyChannelAccounts {
		return fmt.Errorf("managed proxy channel account limit must be between 0 and %d", MaxBrowserProxyChannelAccounts)
	}
	return DB.Create(proxy).Error
}

func UpdateBrowserProxy(proxy *BrowserProxy, updateURL bool) error {
	if proxy.MaxChannelAccounts < 0 || proxy.MaxChannelAccounts > MaxBrowserProxyChannelAccounts {
		return fmt.Errorf("managed proxy channel account limit must be between 0 and %d", MaxBrowserProxyChannelAccounts)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var stored BrowserProxy
		if err := lockForUpdate(tx).First(&stored, "id = ?", proxy.Id).Error; err != nil {
			return err
		}
		if proxy.MaxChannelAccounts > 0 {
			counts, err := countChannelsByBrowserProxyIds(tx, []int{proxy.Id})
			if err != nil {
				return err
			}
			if counts[proxy.Id] > int64(proxy.MaxChannelAccounts) {
				return browserProxyChannelLimitError(&stored, proxy.MaxChannelAccounts, counts[proxy.Id], 0)
			}
		}

		updates := map[string]any{
			"name":                 proxy.Name,
			"enabled":              proxy.Enabled,
			"max_channel_accounts": proxy.MaxChannelAccounts,
			"updated_at":           proxy.UpdatedAt,
		}
		if updateURL {
			updates["url_ciphertext"] = proxy.URLCiphertext
			updates["scheme"] = proxy.Scheme
		}
		return tx.Model(&BrowserProxy{}).Where("id = ?", proxy.Id).Updates(updates).Error
	})
}

func DeleteBrowserProxy(id int) error {
	return DB.Delete(&BrowserProxy{}, id).Error
}

func normalizeBrowserProxyChannelLimits() error {
	return DB.Model(&BrowserProxy{}).
		Where("max_channel_accounts IS NULL").
		Update("max_channel_accounts", DefaultBrowserProxyChannelAccounts).Error
}

func ListBrowserFingerprints() ([]BrowserFingerprint, error) {
	var fingerprints []BrowserFingerprint
	err := DB.Order("id desc").Find(&fingerprints).Error
	return fingerprints, err
}

func GetBrowserFingerprintById(id int) (*BrowserFingerprint, error) {
	var fingerprint BrowserFingerprint
	if err := DB.First(&fingerprint, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &fingerprint, nil
}

func CreateBrowserFingerprint(fingerprint *BrowserFingerprint) error {
	return DB.Create(fingerprint).Error
}

func UpdateBrowserFingerprint(fingerprint *BrowserFingerprint) error {
	return DB.Model(&BrowserFingerprint{}).Where("id = ?", fingerprint.Id).Updates(map[string]any{
		"name":        fingerprint.Name,
		"user_agent":  fingerprint.UserAgent,
		"viewport_w":  fingerprint.ViewportW,
		"viewport_h":  fingerprint.ViewportH,
		"payload":     fingerprint.Payload,
		"launch_args": fingerprint.LaunchArgs,
		"environment": fingerprint.Environment,
		"enabled":     fingerprint.Enabled,
		"updated_at":  fingerprint.UpdatedAt,
	}).Error
}

func DeleteBrowserFingerprint(id int) error {
	return DB.Delete(&BrowserFingerprint{}, id).Error
}

func ListBrowserProfiles() ([]BrowserProfile, error) {
	var profiles []BrowserProfile
	err := DB.Order("id desc").Find(&profiles).Error
	return profiles, err
}

func GetBrowserProfileById(id int) (*BrowserProfile, error) {
	var profile BrowserProfile
	if err := DB.First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func GetBrowserProfileByChannelId(channelId int) (*BrowserProfile, error) {
	var profile BrowserProfile
	if err := DB.First(&profile, "channel_id = ?", channelId).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func ValidateBrowserProfileChannelBinding(channel *Channel) error {
	return validateBrowserProfileChannelBinding(DB, channel)
}

func validateBrowserProfileChannelBinding(tx *gorm.DB, channel *Channel) error {
	if !tx.Migrator().HasTable(&BrowserProfile{}) {
		return nil
	}
	var profile BrowserProfile
	err := tx.First(&profile, "channel_id = ?", channel.Id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if channel.Type != constant.ChannelTypeCodex {
		return errors.New("a channel bound to a browser profile must remain a Codex channel")
	}
	setting, err := decodeChannelSettings(channel)
	if err != nil {
		return err
	}
	if setting.BrowserProxyId != profile.ProxyId {
		return fmt.Errorf("channel must use browser profile managed proxy %d", profile.ProxyId)
	}
	var credentialBinding struct {
		ManagedProxyId int `json:"managed_proxy_id"`
	}
	if strings.HasPrefix(strings.TrimSpace(channel.Key), "{") {
		if err := common.UnmarshalJsonStr(channel.Key, &credentialBinding); err == nil && credentialBinding.ManagedProxyId > 0 && credentialBinding.ManagedProxyId != profile.ProxyId {
			return fmt.Errorf("channel credential requires managed proxy %d, but browser profile uses proxy %d", credentialBinding.ManagedProxyId, profile.ProxyId)
		}
	}
	return nil
}

func CreateBrowserProfile(profile *BrowserProfile) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := syncBrowserProfileChannel(tx, profile); err != nil {
			return err
		}
		return tx.Create(profile).Error
	})
}

func CreateBrowserProfileWithFingerprint(profile *BrowserProfile, fingerprint *BrowserFingerprint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(fingerprint).Error; err != nil {
			return err
		}
		profile.FingerprintId = fingerprint.Id
		if err := syncBrowserProfileChannel(tx, profile); err != nil {
			return err
		}
		return tx.Create(profile).Error
	})
}

func UpdateBrowserProfile(profile *BrowserProfile) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := syncBrowserProfileChannel(tx, profile); err != nil {
			return err
		}
		return tx.Model(&BrowserProfile{}).Where("id = ?", profile.Id).Updates(map[string]any{
			"name":           profile.Name,
			"channel_id":     profile.ChannelId,
			"agent_id":       profile.AgentId,
			"proxy_id":       profile.ProxyId,
			"fingerprint_id": profile.FingerprintId,
			"runtime_key":    profile.RuntimeKey,
			"persistent":     profile.Persistent,
			"enabled":        profile.Enabled,
			"updated_at":     profile.UpdatedAt,
		}).Error
	})
}

func syncBrowserProfileChannel(tx *gorm.DB, profile *BrowserProfile) error {
	if profile.ChannelId == nil {
		return nil
	}
	if *profile.ChannelId <= 0 {
		return errors.New("browser profile channel ID is invalid")
	}

	var channel Channel
	if err := lockForUpdate(tx).First(&channel, "id = ?", *profile.ChannelId).Error; err != nil {
		return fmt.Errorf("browser profile channel not found: %w", err)
	}
	if channel.Type != constant.ChannelTypeCodex {
		return errors.New("browser profiles can only bind to Codex channels")
	}

	var count int64
	query := tx.Model(&BrowserProfile{}).Where("channel_id = ?", *profile.ChannelId)
	if profile.Id > 0 {
		query = query.Where("id != ?", profile.Id)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrBrowserChannelBound
	}

	setting, err := decodeChannelSettings(&channel)
	if err != nil {
		return err
	}
	if setting.BrowserProxyId > 0 && setting.BrowserProxyId != profile.ProxyId {
		return fmt.Errorf("channel uses managed proxy %d, but browser profile uses proxy %d", setting.BrowserProxyId, profile.ProxyId)
	}
	var credentialBinding struct {
		ManagedProxyId int `json:"managed_proxy_id"`
	}
	if strings.HasPrefix(strings.TrimSpace(channel.Key), "{") {
		if err := common.UnmarshalJsonStr(channel.Key, &credentialBinding); err == nil && credentialBinding.ManagedProxyId > 0 && credentialBinding.ManagedProxyId != profile.ProxyId {
			return fmt.Errorf("channel credential requires managed proxy %d", credentialBinding.ManagedProxyId)
		}
	}
	if setting.BrowserProxyId != profile.ProxyId {
		if err := ensureBrowserProxyChannelCapacity(tx, map[int]int64{profile.ProxyId: 1}); err != nil {
			return err
		}
	}
	if setting.BrowserProxyId == profile.ProxyId && strings.TrimSpace(setting.Proxy) == "" {
		return nil
	}
	setting.BrowserProxyId = profile.ProxyId
	setting.Proxy = ""
	settingBytes, err := common.Marshal(setting)
	if err != nil {
		return err
	}
	return tx.Model(&Channel{}).Where("id = ?", channel.Id).Update("setting", string(settingBytes)).Error
}

func BindBrowserProfileToChannel(tx *gorm.DB, profileId int, channelId int, proxyId int, now int64) error {
	var profile BrowserProfile
	if err := lockForUpdate(tx).First(&profile, "id = ?", profileId).Error; err != nil {
		return err
	}
	if profile.ProxyId != proxyId {
		return fmt.Errorf("browser profile proxy changed from %d to %d during browser operation", proxyId, profile.ProxyId)
	}
	if profile.ChannelId != nil && *profile.ChannelId != channelId {
		return ErrBrowserProfileBound
	}
	profile.ChannelId = &channelId
	if err := syncBrowserProfileChannel(tx, &profile); err != nil {
		return err
	}
	return tx.Model(&BrowserProfile{}).Where("id = ?", profile.Id).Updates(map[string]any{
		"channel_id": channelId,
		"updated_at": now,
	}).Error
}

func RotateBrowserProfileDataKey(id int, dataKey string, now int64) error {
	return DB.Model(&BrowserProfile{}).Where("id = ?", id).Updates(map[string]any{
		"data_key":   dataKey,
		"updated_at": now,
	}).Error
}

func DeleteBrowserProfile(id int) error {
	return DB.Delete(&BrowserProfile{}, id).Error
}

func CountBrowserProfilesByAgent(agentId int) (int64, error) {
	var count int64
	err := DB.Model(&BrowserProfile{}).Where("agent_id = ?", agentId).Count(&count).Error
	return count, err
}

func CountBrowserProfilesByAgents(agentIds []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(agentIds))
	uniqueIds := make([]int, 0, len(agentIds))
	for _, agentId := range agentIds {
		if agentId <= 0 {
			continue
		}
		if _, exists := counts[agentId]; exists {
			continue
		}
		counts[agentId] = 0
		uniqueIds = append(uniqueIds, agentId)
	}
	if len(uniqueIds) == 0 {
		return counts, nil
	}

	var rows []struct {
		AgentId int
		Total   int64
	}
	if err := DB.Model(&BrowserProfile{}).
		Select("agent_id, COUNT(*) AS total").
		Where("agent_id IN ?", uniqueIds).
		Group("agent_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.AgentId] = row.Total
	}
	return counts, nil
}

func CountBrowserProfilesByProxy(proxyId int) (int64, error) {
	var count int64
	err := DB.Model(&BrowserProfile{}).Where("proxy_id = ?", proxyId).Count(&count).Error
	return count, err
}

func CountBrowserProfilesByProxies(proxyIds []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(proxyIds))
	uniqueIds := make([]int, 0, len(proxyIds))
	for _, proxyId := range proxyIds {
		if proxyId <= 0 {
			continue
		}
		if _, exists := counts[proxyId]; exists {
			continue
		}
		counts[proxyId] = 0
		uniqueIds = append(uniqueIds, proxyId)
	}
	if len(uniqueIds) == 0 {
		return counts, nil
	}

	var rows []struct {
		ProxyId int
		Total   int64
	}
	if err := DB.Model(&BrowserProfile{}).
		Select("proxy_id, COUNT(*) AS total").
		Where("proxy_id IN ?", uniqueIds).
		Group("proxy_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.ProxyId] = row.Total
	}
	return counts, nil
}

func CountChannelsByBrowserProxy(proxyId int) (int64, error) {
	if proxyId <= 0 {
		return 0, nil
	}
	counts, err := countChannelsByBrowserProxyIds(DB, []int{proxyId})
	return counts[proxyId], err
}

func CountChannelsByBrowserProxies(proxyIds []int) (map[int]int64, error) {
	return countChannelsByBrowserProxyIds(DB, proxyIds)
}

func CheckBrowserProxyChannelCapacity(proxyId int, additionalAccounts int64) error {
	if proxyId <= 0 || additionalAccounts <= 0 {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		return ensureBrowserProxyChannelCapacity(tx, map[int]int64{proxyId: additionalAccounts})
	})
}

func decodeChannelSettings(channel *Channel) (dto.ChannelSettings, error) {
	setting := dto.ChannelSettings{}
	if channel.Setting == nil || strings.TrimSpace(*channel.Setting) == "" {
		return setting, nil
	}
	if err := common.Unmarshal([]byte(*channel.Setting), &setting); err != nil {
		return dto.ChannelSettings{}, fmt.Errorf("decode channel %d setting: %w", channel.Id, err)
	}
	return setting, nil
}

func browserProxyChannelAdditions(channels []Channel) (map[int]int64, error) {
	additions := make(map[int]int64)
	for index := range channels {
		setting, err := decodeChannelSettings(&channels[index])
		if err != nil {
			return nil, err
		}
		if setting.BrowserProxyId > 0 {
			additions[setting.BrowserProxyId]++
		}
	}
	return additions, nil
}

func countChannelsByBrowserProxyIds(tx *gorm.DB, proxyIds []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(proxyIds))
	wanted := make(map[int]struct{}, len(proxyIds))
	for _, proxyId := range proxyIds {
		if proxyId <= 0 {
			continue
		}
		counts[proxyId] = 0
		wanted[proxyId] = struct{}{}
	}
	if len(wanted) == 0 {
		return counts, nil
	}

	var channels []Channel
	if err := tx.Model(&Channel{}).
		Select("id", "setting").
		Where("setting IS NOT NULL AND setting != ''").
		Find(&channels).Error; err != nil {
		return nil, err
	}
	for index := range channels {
		setting, err := decodeChannelSettings(&channels[index])
		if err != nil {
			return nil, err
		}
		if _, ok := wanted[setting.BrowserProxyId]; ok {
			counts[setting.BrowserProxyId]++
		}
	}
	return counts, nil
}

func ensureBrowserProxyChannelCapacity(tx *gorm.DB, additions map[int]int64) error {
	proxyIds := make([]int, 0, len(additions))
	for proxyId, additionalAccounts := range additions {
		if proxyId > 0 && additionalAccounts > 0 {
			proxyIds = append(proxyIds, proxyId)
		}
	}
	if len(proxyIds) == 0 {
		return nil
	}
	sort.Ints(proxyIds)

	proxies := make(map[int]BrowserProxy, len(proxyIds))
	for _, proxyId := range proxyIds {
		var proxy BrowserProxy
		if err := lockForUpdate(tx).First(&proxy, "id = ?", proxyId).Error; err != nil {
			return fmt.Errorf("managed proxy %d not found: %w", proxyId, err)
		}
		proxies[proxyId] = proxy
	}
	counts, err := countChannelsByBrowserProxyIds(tx, proxyIds)
	if err != nil {
		return err
	}
	for _, proxyId := range proxyIds {
		proxy := proxies[proxyId]
		if proxy.MaxChannelAccounts == 0 {
			continue
		}
		additionalAccounts := additions[proxyId]
		if counts[proxyId]+additionalAccounts > int64(proxy.MaxChannelAccounts) {
			return browserProxyChannelLimitError(&proxy, proxy.MaxChannelAccounts, counts[proxyId], additionalAccounts)
		}
	}
	return nil
}

func browserProxyChannelLimitError(proxy *BrowserProxy, limit int, currentAccounts int64, additionalAccounts int64) error {
	return fmt.Errorf(
		"%w: managed proxy %q (id %d) has %d of %d channel accounts assigned; requested %d additional",
		ErrBrowserProxyChannelLimit,
		proxy.Name,
		proxy.Id,
		currentAccounts,
		limit,
		additionalAccounts,
	)
}

func CountBrowserProfilesByFingerprint(fingerprintId int) (int64, error) {
	var count int64
	err := DB.Model(&BrowserProfile{}).Where("fingerprint_id = ?", fingerprintId).Count(&count).Error
	return count, err
}

func CountBrowserProfilesByChannelIds(channelIds []int) (int64, error) {
	if len(channelIds) == 0 {
		return 0, nil
	}
	var count int64
	err := DB.Model(&BrowserProfile{}).Where("channel_id IN ?", channelIds).Count(&count).Error
	return count, err
}

func HasUnfinishedCodexOAuthFlowForProfile(profileId int, now int64) (bool, error) {
	unfinishedStatuses := append([]string{}, codexOAuthBrowserActiveStatuses...)
	unfinishedStatuses = append(unfinishedStatuses, CodexOAuthFlowStatusCompleted)
	var count int64
	err := DB.Model(&CodexOAuthFlow{}).
		Where("profile_id = ? AND consumed_at = 0 AND expires_at > ? AND status IN ?", profileId, now, unfinishedStatuses).
		Count(&count).Error
	return count > 0, err
}

func HasActiveBrowserLaunchForProfile(profileId int, now int64) (bool, error) {
	if err := expireBrowserLaunches(DB, now); err != nil {
		return false, err
	}
	var count int64
	err := DB.Model(&BrowserLaunch{}).
		Where("profile_id = ? AND expires_at > ? AND status IN ?", profileId, now, browserLaunchActiveStatuses).
		Count(&count).Error
	return count > 0, err
}

func ListActiveBrowserLaunches(now int64) ([]BrowserLaunch, error) {
	if err := expireBrowserLaunches(DB, now); err != nil {
		return nil, err
	}
	var launches []BrowserLaunch
	err := DB.Where("expires_at > ? AND status IN ?", now, browserLaunchActiveStatuses).
		Order("created_at desc").
		Find(&launches).Error
	return launches, err
}

func CreateExclusiveBrowserLaunch(launch *BrowserLaunch) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var agent BrowserAgent
		if err := lockForUpdate(tx).First(&agent, "id = ?", launch.AgentId).Error; err != nil {
			return err
		}
		if err := expireCodexOAuthFlows(tx, launch.CreatedAt); err != nil {
			return err
		}
		if err := expireBrowserLaunches(tx, launch.CreatedAt); err != nil {
			return err
		}
		busy, err := browserAgentHasActiveOperation(tx, launch.AgentId, launch.CreatedAt)
		if err != nil {
			return err
		}
		if busy {
			return ErrBrowserAgentBusy
		}
		return tx.Create(launch).Error
	})
}

func GetBrowserLaunchById(id string, now int64) (*BrowserLaunch, error) {
	if err := expireBrowserLaunches(DB, now); err != nil {
		return nil, err
	}
	var launch BrowserLaunch
	if err := DB.First(&launch, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &launch, nil
}

func ClaimBrowserLaunch(agentId int, instanceId string, now int64, leaseExpiresAt int64) (*BrowserLaunch, error) {
	var claimed BrowserLaunch
	err := DB.Transaction(func(tx *gorm.DB) error {
		var agent BrowserAgent
		if err := lockForUpdate(tx).First(&agent, "id = ?", agentId).Error; err != nil {
			return err
		}
		if err := expireBrowserLaunches(tx, now); err != nil {
			return err
		}
		if err := lockForUpdate(tx).
			Where("agent_id = ? AND expires_at > ?", agentId, now).
			Where("status = ? OR (status IN ? AND lease_expires_at < ?)", browseragentapi.FlowStatusPending, []string{browseragentapi.FlowStatusClaimed, browseragentapi.FlowStatusRunning}, now).
			Order("created_at asc").
			First(&claimed).Error; err != nil {
			return err
		}
		claimedAt := claimed.ClaimedAt
		if claimedAt == 0 {
			claimedAt = now
		}
		return tx.Model(&BrowserLaunch{}).Where("id = ?", claimed.Id).Updates(map[string]any{
			"status":            browseragentapi.FlowStatusClaimed,
			"agent_instance_id": instanceId,
			"claimed_at":        claimedAt,
			"lease_expires_at":  leaseExpiresAt,
			"updated_at":        now,
			"error_message":     "",
		}).Error
	})
	if err != nil {
		return nil, err
	}
	claimed.Status = browseragentapi.FlowStatusClaimed
	claimed.AgentInstanceId = instanceId
	claimed.LeaseExpiresAt = leaseExpiresAt
	if claimed.ClaimedAt == 0 {
		claimed.ClaimedAt = now
	}
	return &claimed, nil
}

func MarkBrowserLaunchRunning(agentId int, instanceId string, launchId string, now int64, leaseExpiresAt int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		launch, err := getLockedBrowserLaunch(tx, launchId)
		if err != nil {
			return err
		}
		if err := validateBrowserLaunchInstance(launch, agentId, instanceId, now); err != nil {
			return err
		}
		if launch.Status != browseragentapi.FlowStatusClaimed && launch.Status != browseragentapi.FlowStatusRunning {
			return ErrBrowserLaunchState
		}
		runningAt := launch.RunningAt
		if runningAt == 0 {
			runningAt = now
		}
		return tx.Model(&BrowserLaunch{}).Where("id = ?", launch.Id).Updates(map[string]any{
			"status":           browseragentapi.FlowStatusRunning,
			"running_at":       runningAt,
			"lease_expires_at": leaseExpiresAt,
			"updated_at":       now,
		}).Error
	})
}

func CompleteBrowserLaunch(agentId int, instanceId string, launchId string, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		launch, err := getLockedBrowserLaunch(tx, launchId)
		if err != nil {
			return err
		}
		if launch.Status == browseragentapi.FlowStatusCompleted && launch.AgentId == agentId && launch.AgentInstanceId == instanceId {
			return nil
		}
		if err := validateBrowserLaunchInstance(launch, agentId, instanceId, now); err != nil {
			return err
		}
		if launch.Status != browseragentapi.FlowStatusClaimed && launch.Status != browseragentapi.FlowStatusRunning {
			return ErrBrowserLaunchState
		}
		return tx.Model(&BrowserLaunch{}).Where("id = ?", launch.Id).Updates(map[string]any{
			"status":           browseragentapi.FlowStatusCompleted,
			"completed_at":     now,
			"lease_expires_at": 0,
			"updated_at":       now,
			"error_message":    "",
		}).Error
	})
}

func FailBrowserLaunch(agentId int, instanceId string, launchId string, message string, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		launch, err := getLockedBrowserLaunch(tx, launchId)
		if err != nil {
			return err
		}
		if launch.Status == browseragentapi.FlowStatusFailed {
			return nil
		}
		if launch.AgentId != agentId || launch.AgentInstanceId != instanceId {
			return ErrBrowserLaunchInstance
		}
		if !isBrowserLaunchActiveStatus(launch.Status) {
			return ErrBrowserLaunchState
		}
		return tx.Model(&BrowserLaunch{}).Where("id = ?", launch.Id).Updates(map[string]any{
			"status":           browseragentapi.FlowStatusFailed,
			"lease_expires_at": 0,
			"updated_at":       now,
			"error_message":    message,
		}).Error
	})
}

func CancelBrowserLaunch(launchId string, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		launch, err := getLockedBrowserLaunch(tx, launchId)
		if err != nil {
			return err
		}
		if launch.Status == browseragentapi.FlowStatusCanceled {
			return nil
		}
		if !isBrowserLaunchActiveStatus(launch.Status) {
			return ErrBrowserLaunchState
		}
		return tx.Model(&BrowserLaunch{}).Where("id = ?", launch.Id).Updates(map[string]any{
			"status":           browseragentapi.FlowStatusCanceled,
			"lease_expires_at": 0,
			"updated_at":       now,
			"error_message":    "",
		}).Error
	})
}

func getLockedBrowserLaunch(tx *gorm.DB, launchId string) (*BrowserLaunch, error) {
	var launch BrowserLaunch
	if err := lockForUpdate(tx).First(&launch, "id = ?", launchId).Error; err != nil {
		return nil, err
	}
	return &launch, nil
}

func validateBrowserLaunchInstance(launch *BrowserLaunch, agentId int, instanceId string, now int64) error {
	if launch.AgentId != agentId || launch.AgentInstanceId != instanceId {
		return ErrBrowserLaunchInstance
	}
	if launch.ExpiresAt <= now {
		return ErrBrowserLaunchExpired
	}
	return nil
}

func expireBrowserLaunches(tx *gorm.DB, now int64) error {
	return tx.Model(&BrowserLaunch{}).
		Where("expires_at <= ? AND status IN ?", now, browserLaunchActiveStatuses).
		Updates(map[string]any{
			"status":           browseragentapi.FlowStatusExpired,
			"lease_expires_at": 0,
			"updated_at":       now,
			"error_message":    "",
		}).Error
}

func browserAgentHasActiveOperation(tx *gorm.DB, agentId int, now int64) (bool, error) {
	var count int64
	if err := tx.Model(&CodexOAuthFlow{}).
		Where("agent_id = ? AND status IN ? AND expires_at > ?", agentId, codexOAuthBrowserActiveStatuses, now).
		Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	if err := tx.Model(&BrowserLaunch{}).
		Where("agent_id = ? AND status IN ? AND expires_at > ?", agentId, browserLaunchActiveStatuses, now).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func isBrowserLaunchActiveStatus(status string) bool {
	for _, activeStatus := range browserLaunchActiveStatuses {
		if status == activeStatus {
			return true
		}
	}
	return false
}

func CreateExclusiveCodexOAuthFlow(flow *CodexOAuthFlow) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var agent BrowserAgent
		if err := lockForUpdate(tx).First(&agent, "id = ?", flow.AgentId).Error; err != nil {
			return err
		}

		if err := expireCodexOAuthFlows(tx, flow.CreatedAt); err != nil {
			return err
		}
		if err := expireBrowserLaunches(tx, flow.CreatedAt); err != nil {
			return err
		}
		busy, err := browserAgentHasActiveOperation(tx, flow.AgentId, flow.CreatedAt)
		if err != nil {
			return err
		}
		if busy {
			return ErrBrowserAgentBusy
		}
		return tx.Create(flow).Error
	})
}

func GetCodexOAuthFlowById(id string) (*CodexOAuthFlow, error) {
	var flow CodexOAuthFlow
	if err := DB.First(&flow, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &flow, nil
}

func GetCodexOAuthFlowForUser(id string, userId int, isRoot bool, now int64) (*CodexOAuthFlow, error) {
	if err := DB.Transaction(func(tx *gorm.DB) error {
		if err := expireCodexOAuthFlows(tx, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	var flow CodexOAuthFlow
	query := DB.Where("id = ?", id)
	if !isRoot {
		query = query.Where("user_id = ?", userId)
	}
	if err := query.First(&flow).Error; err != nil {
		return nil, err
	}
	return &flow, nil
}

func ClaimCodexOAuthFlow(agentId int, instanceId string, now int64, leaseExpiresAt int64) (*CodexOAuthFlow, error) {
	var claimed CodexOAuthFlow
	err := DB.Transaction(func(tx *gorm.DB) error {
		var agent BrowserAgent
		if err := lockForUpdate(tx).First(&agent, "id = ?", agentId).Error; err != nil {
			return err
		}
		if err := expireCodexOAuthFlows(tx, now); err != nil {
			return err
		}

		query := lockForUpdate(tx).
			Where("agent_id = ? AND expires_at > ?", agentId, now).
			Where("status = ? OR (status IN ? AND lease_expires_at < ?)", CodexOAuthFlowStatusPending, []string{CodexOAuthFlowStatusClaimed, CodexOAuthFlowStatusRunning}, now).
			Order("created_at asc")
		if err := query.First(&claimed).Error; err != nil {
			return err
		}

		claimedAt := claimed.ClaimedAt
		if claimedAt == 0 {
			claimedAt = now
		}
		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", claimed.Id).Updates(map[string]any{
			"status":            CodexOAuthFlowStatusClaimed,
			"agent_instance_id": instanceId,
			"claimed_at":        claimedAt,
			"lease_expires_at":  leaseExpiresAt,
			"updated_at":        now,
			"error_message":     "",
		}).Error
	})
	if err != nil {
		return nil, err
	}
	claimed.Status = CodexOAuthFlowStatusClaimed
	claimed.AgentInstanceId = instanceId
	claimed.LeaseExpiresAt = leaseExpiresAt
	if claimed.ClaimedAt == 0 {
		claimed.ClaimedAt = now
	}
	return &claimed, nil
}

func MarkCodexOAuthFlowRunning(agentId int, instanceId string, flowId string, now int64, leaseExpiresAt int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		flow, err := getLockedCodexOAuthFlow(tx, flowId)
		if err != nil {
			return err
		}
		if err := validateAgentFlowLease(flow, agentId, instanceId, now); err != nil {
			return err
		}
		if flow.Status != CodexOAuthFlowStatusClaimed && flow.Status != CodexOAuthFlowStatusRunning {
			return ErrCodexOAuthFlowState
		}
		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", flow.Id).Updates(map[string]any{
			"status":           CodexOAuthFlowStatusRunning,
			"lease_expires_at": leaseExpiresAt,
			"updated_at":       now,
		}).Error
	})
}

type CodexOAuthFlowCompletion struct {
	CredentialsCiphertext string
	ChannelKey            string
	AccountId             string
	Email                 string
	PlanType              string
	CredentialExpiresAt   int64
	CompletedAt           int64
}

func CompleteCodexOAuthFlow(agentId int, instanceId string, flowId string, completion CodexOAuthFlowCompletion) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		flow, err := getLockedCodexOAuthFlow(tx, flowId)
		if err != nil {
			return err
		}
		if flow.Status == CodexOAuthFlowStatusCompleted && flow.AgentId == agentId && flow.AgentInstanceId == instanceId {
			return nil
		}
		if err := validateAgentFlowLease(flow, agentId, instanceId, completion.CompletedAt); err != nil {
			return err
		}
		if flow.Status != CodexOAuthFlowStatusClaimed && flow.Status != CodexOAuthFlowStatusRunning {
			return ErrCodexOAuthFlowState
		}

		credentialsCiphertext := completion.CredentialsCiphertext
		consumedAt := int64(0)
		if flow.ChannelId > 0 {
			var channel Channel
			if err := lockForUpdate(tx).First(&channel, "id = ?", flow.ChannelId).Error; err != nil {
				return err
			}
			if channel.Type != constant.ChannelTypeCodex {
				return errors.New("channel type is not Codex")
			}

			channelSetting, err := decodeChannelSettings(&channel)
			if err != nil {
				return err
			}
			if channelSetting.BrowserProxyId > 0 && channelSetting.BrowserProxyId != flow.ProxyId {
				return fmt.Errorf("channel uses managed proxy %d, but OAuth flow uses proxy %d", channelSetting.BrowserProxyId, flow.ProxyId)
			}
			if channelSetting.BrowserProxyId != flow.ProxyId {
				if err := ensureBrowserProxyChannelCapacity(tx, map[int]int64{flow.ProxyId: 1}); err != nil {
					return err
				}
			}
			channelSetting.BrowserProxyId = flow.ProxyId
			channelSetting.Proxy = ""
			settingBytes, err := common.Marshal(channelSetting)
			if err != nil {
				return err
			}
			if err := tx.Model(&Channel{}).Where("id = ?", channel.Id).Updates(map[string]any{
				"key":     completion.ChannelKey,
				"setting": string(settingBytes),
			}).Error; err != nil {
				return err
			}
			if err := BindBrowserProfileToChannel(tx, flow.ProfileId, channel.Id, flow.ProxyId, completion.CompletedAt); err != nil {
				return err
			}
			credentialsCiphertext = ""
			consumedAt = completion.CompletedAt
		}

		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", flow.Id).Updates(map[string]any{
			"status":                 CodexOAuthFlowStatusCompleted,
			"state_ciphertext":       "",
			"verifier_ciphertext":    "",
			"credentials_ciphertext": credentialsCiphertext,
			"account_id":             completion.AccountId,
			"email":                  completion.Email,
			"plan_type":              completion.PlanType,
			"credential_expires_at":  completion.CredentialExpiresAt,
			"completed_at":           completion.CompletedAt,
			"consumed_at":            consumedAt,
			"lease_expires_at":       0,
			"updated_at":             completion.CompletedAt,
			"error_message":          "",
		}).Error
	})
}

func FailCodexOAuthFlow(agentId int, instanceId string, flowId string, message string, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		flow, err := getLockedCodexOAuthFlow(tx, flowId)
		if err != nil {
			return err
		}
		if flow.Status == CodexOAuthFlowStatusFailed {
			return nil
		}
		if flow.AgentId != agentId || flow.AgentInstanceId != instanceId {
			return ErrCodexOAuthFlowInstance
		}
		if !isCodexOAuthBrowserActiveStatus(flow.Status) {
			return ErrCodexOAuthFlowState
		}
		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", flow.Id).Updates(map[string]any{
			"status":                 CodexOAuthFlowStatusFailed,
			"state_ciphertext":       "",
			"verifier_ciphertext":    "",
			"credentials_ciphertext": "",
			"lease_expires_at":       0,
			"updated_at":             now,
			"error_message":          message,
		}).Error
	})
}

func CancelCodexOAuthFlow(flowId string, userId int, isRoot bool, now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		flow, err := getLockedCodexOAuthFlow(tx, flowId)
		if err != nil {
			return err
		}
		if !isRoot && flow.UserId != userId {
			return gorm.ErrRecordNotFound
		}
		if flow.Status == CodexOAuthFlowStatusCanceled {
			return nil
		}
		canDiscardCompleted := flow.Status == CodexOAuthFlowStatusCompleted && flow.ConsumedAt == 0
		if !isCodexOAuthBrowserActiveStatus(flow.Status) && !canDiscardCompleted {
			return ErrCodexOAuthFlowState
		}
		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", flow.Id).Updates(map[string]any{
			"status":                 CodexOAuthFlowStatusCanceled,
			"state_ciphertext":       "",
			"verifier_ciphertext":    "",
			"credentials_ciphertext": "",
			"lease_expires_at":       0,
			"updated_at":             now,
			"error_message":          "",
		}).Error
	})
}

func InsertChannelsWithCodexOAuthFlow(channels []Channel, flowId string, userId int, now int64) error {
	if len(channels) != 1 {
		return errors.New("Codex OAuth flow can only create one channel")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		flow, err := getLockedCodexOAuthFlow(tx, flowId)
		if err != nil {
			return err
		}
		if flow.UserId != userId {
			return gorm.ErrRecordNotFound
		}
		if flow.Status != CodexOAuthFlowStatusCompleted || flow.ConsumedAt != 0 || strings.TrimSpace(flow.CredentialsCiphertext) == "" {
			return ErrCodexOAuthFlowState
		}
		if flow.ExpiresAt <= now {
			return ErrCodexOAuthFlowExpired
		}
		if err := ensureBrowserProxyChannelCapacity(tx, map[int]int64{flow.ProxyId: 1}); err != nil {
			return err
		}

		if err := tx.Create(&channels).Error; err != nil {
			return err
		}
		for index := range channels {
			if err := channels[index].AddAbilities(tx); err != nil {
				return err
			}
		}
		channelId := channels[0].Id
		if err := BindBrowserProfileToChannel(tx, flow.ProfileId, channelId, flow.ProxyId, now); err != nil {
			return err
		}
		return tx.Model(&CodexOAuthFlow{}).Where("id = ?", flow.Id).Updates(map[string]any{
			"channel_id":             channelId,
			"consumed_at":            now,
			"credentials_ciphertext": "",
			"state_ciphertext":       "",
			"verifier_ciphertext":    "",
			"updated_at":             now,
		}).Error
	})
}

func getLockedCodexOAuthFlow(tx *gorm.DB, flowId string) (*CodexOAuthFlow, error) {
	var flow CodexOAuthFlow
	if err := lockForUpdate(tx).First(&flow, "id = ?", flowId).Error; err != nil {
		return nil, err
	}
	return &flow, nil
}

func validateAgentFlowLease(flow *CodexOAuthFlow, agentId int, instanceId string, now int64) error {
	if flow.AgentId != agentId || flow.AgentInstanceId != instanceId {
		return ErrCodexOAuthFlowInstance
	}
	if flow.ExpiresAt <= now {
		return ErrCodexOAuthFlowExpired
	}
	return nil
}

func expireCodexOAuthFlows(tx *gorm.DB, now int64) error {
	expirableStatuses := append([]string{}, codexOAuthBrowserActiveStatuses...)
	expirableStatuses = append(expirableStatuses, CodexOAuthFlowStatusCompleted)
	return tx.Model(&CodexOAuthFlow{}).
		Where("consumed_at = 0 AND expires_at <= ? AND status IN ?", now, expirableStatuses).
		Updates(map[string]any{
			"status":                 CodexOAuthFlowStatusExpired,
			"state_ciphertext":       "",
			"verifier_ciphertext":    "",
			"credentials_ciphertext": "",
			"lease_expires_at":       0,
			"updated_at":             now,
			"error_message":          "",
		}).Error
}

func isCodexOAuthBrowserActiveStatus(status string) bool {
	for _, activeStatus := range codexOAuthBrowserActiveStatuses {
		if status == activeStatus {
			return true
		}
	}
	return false
}

func ValidateBrowserModelReferences(agentId int, proxyId int, fingerprintId int) error {
	if err := ValidateBrowserAgentProxyReferences(agentId, proxyId); err != nil {
		return err
	}
	if _, err := GetBrowserFingerprintById(fingerprintId); err != nil {
		return fmt.Errorf("browser fingerprint not found: %w", err)
	}
	return nil
}

func ValidateBrowserAgentProxyReferences(agentId int, proxyId int) error {
	if _, err := GetBrowserAgentById(agentId); err != nil {
		return fmt.Errorf("browser agent not found: %w", err)
	}
	if _, err := GetBrowserProxyById(proxyId); err != nil {
		return fmt.Errorf("browser proxy not found: %w", err)
	}
	return nil
}
