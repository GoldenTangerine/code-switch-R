/**
 * @name: xAI OAuth 托管登录服务
 * @Descripttion: 基于 RFC 8628 设备码流管理 xAI 账号登录、凭据存储与 access token 内存缓存刷新
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 05:45:00
 * @LastEditTime: 2026-08-17 05:45:00
 * @FilePath: services/xaioauthservice.go
 */

package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	XaiOAuthProviderName = "xai_oauth"
	// xAI OAuth issuer，端点必须留在该 origin 内
	xaiOAuthIssuer = "https://auth.x.ai"
	// 与 Grok CLI 相同的公共 OAuth client
	xaiOAuthClientID = "b1a00492-073a-47ea-816f-4c329264a828"
	xaiOAuthScope    = "openid profile email offline_access grok-cli:access api:access"
	// 运行时 discovery：token / device 端点从发现文档解析
	xaiOAuthDiscoveryURL      = xaiOAuthIssuer + "/.well-known/openid-configuration"
	xaiOAuthFallbackVerifyURI = xaiOAuthIssuer + "/device"
	xaiOAuthDeviceGrantType   = "urn:ietf:params:oauth:grant-type:device_code"
	xaiOAuthUserAgent         = "code-switch-r-xai-oauth"
	xaiOAuthStoreFileName     = "xai_oauth_auth.json"
	xaiOAuthDefaultExpiresIn  = 3600
	xaiOAuthDeviceExpiresIn   = 900
	xaiOAuthDefaultInterval   = 5
	// 过期前 60 秒提前刷新，避免请求途中 token 失效
	xaiOAuthRefreshBuffer     = 60 * time.Second
	xaiOAuthHTTPClientTimeout = 30 * time.Second
)

var (
	// errXaiOAuthAuthorizationPending 用户尚未完成授权，前端应按 interval 继续轮询
	errXaiOAuthAuthorizationPending = errors.New("authorization_pending")
	// errXaiOAuthSlowDown 服务端要求降低轮询频率
	errXaiOAuthSlowDown = errors.New("slow_down")
	// errXaiOAuthRefreshTokenInvalid refresh_token 已失效，需要用户重新授权
	errXaiOAuthRefreshTokenInvalid = errors.New("refresh_token_invalid")
)

// XaiOAuthDeviceCodeResponse 是前端展示设备码登录时需要的数据。
type XaiOAuthDeviceCodeResponse struct {
	Provider        string `json:"provider"`
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int64  `json:"expiresIn"`
	Interval        int64  `json:"interval"`
}

// XaiOAuthAccount 是前端可见的 xAI 账号信息，不包含 token。
type XaiOAuthAccount struct {
	ID              string `json:"id"`
	Provider        string `json:"provider"`
	Login           string `json:"login"`
	AuthenticatedAt int64  `json:"authenticatedAt"`
	IsDefault       bool   `json:"isDefault"`
	RequiresReauth  bool   `json:"requiresReauth"`
}

// XaiOAuthStatus 汇总当前 xAI OAuth 登录状态。
type XaiOAuthStatus struct {
	Provider         string              `json:"provider"`
	Authenticated    bool                `json:"authenticated"`
	DefaultAccountID string              `json:"defaultAccountId,omitempty"`
	Accounts         []XaiOAuthAccount   `json:"accounts"`
}

// xaiOAuthDiscoveryDocument xAI OIDC 发现文档中本服务关心的字段
type xaiOAuthDiscoveryDocument struct {
	Issuer                      string `json:"issuer"`
	DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
	TokenEndpoint               string `json:"token_endpoint"`
}

// xaiOAuthDeviceCodeRaw 设备码端点的原始响应
type xaiOAuthDeviceCodeRaw struct {
	DeviceCode      string      `json:"device_code"`
	UserCode        string      `json:"user_code"`
	VerificationURI string      `json:"verification_uri"`
	ExpiresIn       int64       `json:"expires_in"`
	Interval        interface{} `json:"interval"`
}

// xaiOAuthTokenResponse token 端点的成功响应
type xaiOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

// xaiOAuthJWTClaims id_token 的 payload 字段（仅解析需要的部分）
type xaiOAuthJWTClaims struct {
	Sub               string `json:"sub"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
}

// xaiOAuthStoredAccount 持久化的账号数据，账号 ID 作为 accounts map 的 key
type xaiOAuthStoredAccount struct {
	Login           string `json:"login,omitempty"`
	RefreshToken    string `json:"refresh_token"`
	RequiresReauth  bool   `json:"requires_reauth,omitempty"`
	AuthenticatedAt int64  `json:"authenticated_at"`
}

type xaiOAuthStore struct {
	Version          int                              `json:"version"`
	Accounts         map[string]xaiOAuthStoredAccount `json:"accounts"`
	DefaultAccountID string                           `json:"default_account_id,omitempty"`
}

// xaiOAuthCachedAccessToken 仅存内存的 access token 缓存
type xaiOAuthCachedAccessToken struct {
	Token     string
	ExpiresAt time.Time
}

// xaiOAuthEndpoints discovery 解析出的端点
type xaiOAuthEndpoints struct {
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
}

// XaiOAuthService 管理 xAI 设备码登录与 access token 刷新。
type XaiOAuthService struct {
	mu               sync.Mutex
	accounts         map[string]xaiOAuthStoredAccount
	defaultAccountID string
	accessTokens     map[string]xaiOAuthCachedAccessToken
	// refreshLocks 按账号互斥刷新，避免并发请求重复刷新同一账号
	refreshLocks map[string]*sync.Mutex
	endpoints    *xaiOAuthEndpoints
	storePath    string
	discoveryURL string
	httpClient   *http.Client
}

func NewXaiOAuthService() *XaiOAuthService {
	storePath := ""
	if home, err := getUserHomeDir(); err == nil {
		storePath = filepath.Join(home, ".code-switch", xaiOAuthStoreFileName)
	}
	s := &XaiOAuthService{
		accounts:     make(map[string]xaiOAuthStoredAccount),
		accessTokens: make(map[string]xaiOAuthCachedAccessToken),
		refreshLocks: make(map[string]*sync.Mutex),
		storePath:    storePath,
		discoveryURL: xaiOAuthDiscoveryURL,
		httpClient:   &http.Client{Timeout: xaiOAuthHTTPClientTimeout},
	}
	if err := s.loadFromDisk(); err != nil {
		fmt.Printf("[XaiOAuth] 加载认证存储失败: %v\n", err)
	}
	return s
}

func (s *XaiOAuthService) Start() error { return nil }
func (s *XaiOAuthService) Stop() error  { return nil }

// StartLogin 发起设备码授权，返回前端展示用的用户码与校验地址
func (s *XaiOAuthService) StartLogin() (*XaiOAuthDeviceCodeResponse, error) {
	endpoints, err := s.resolveEndpoints()
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("client_id", xaiOAuthClientID)
	values.Set("scope", xaiOAuthScope)
	var raw xaiOAuthDeviceCodeRaw
	if err := s.doJSON(http.MethodPost, endpoints.DeviceAuthorizationEndpoint, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), &raw); err != nil {
		return nil, fmt.Errorf("Device Code 请求失败: %w", err)
	}
	if strings.TrimSpace(raw.DeviceCode) == "" || strings.TrimSpace(raw.UserCode) == "" {
		return nil, fmt.Errorf("Device Code 响应缺少必要字段")
	}
	expiresIn := raw.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = xaiOAuthDeviceExpiresIn
	}
	interval := parseXaiOAuthInterval(raw.Interval)
	if interval <= 0 {
		interval = xaiOAuthDefaultInterval
	}
	verificationURI := strings.TrimSpace(raw.VerificationURI)
	if verificationURI == "" {
		verificationURI = xaiOAuthFallbackVerifyURI
	}

	return &XaiOAuthDeviceCodeResponse{
		Provider:        XaiOAuthProviderName,
		DeviceCode:      strings.TrimSpace(raw.DeviceCode),
		UserCode:        strings.TrimSpace(raw.UserCode),
		VerificationURI: verificationURI,
		ExpiresIn:       expiresIn,
		Interval:        interval,
	}, nil
}

// PollLogin 轮询 token 端点换取凭据；授权未完成时返回可重试的哨兵错误
func (s *XaiOAuthService) PollLogin(deviceCode string) (*XaiOAuthAccount, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil, fmt.Errorf("deviceCode 不能为空")
	}
	endpoints, err := s.resolveEndpoints()
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("grant_type", xaiOAuthDeviceGrantType)
	values.Set("device_code", deviceCode)
	values.Set("client_id", xaiOAuthClientID)
	status, body, err := s.doRaw(http.MethodPost, endpoints.TokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, translateXaiOAuthTokenError(status, body, "Token 轮询")
	}
	var tokens xaiOAuthTokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("解析 Token 响应失败: %w", err)
	}
	if tokens.AccessToken == "" {
		return nil, fmt.Errorf("Token 响应缺少 access_token")
	}
	if tokens.RefreshToken == "" {
		return nil, fmt.Errorf("Token 响应缺少 refresh_token")
	}
	accountID, login := extractXaiOAuthIdentity(&tokens)
	if accountID == "" {
		return nil, fmt.Errorf("无法从 id_token 中提取 xAI 账号标识")
	}

	nowSec := time.Now().Unix()
	stored := xaiOAuthStoredAccount{Login: login, RefreshToken: tokens.RefreshToken, AuthenticatedAt: nowSec}
	s.mu.Lock()
	// 保留旧的认证时间，重复登录同一账号视为刷新会话
	if previous, ok := s.accounts[accountID]; ok && previous.AuthenticatedAt > 0 {
		stored.AuthenticatedAt = previous.AuthenticatedAt
	}
	s.accounts[accountID] = stored
	if s.defaultAccountID == "" {
		s.defaultAccountID = accountID
	}
	defaultAccountID := s.defaultAccountID
	s.accessTokens[accountID] = xaiOAuthCachedAccessToken{Token: tokens.AccessToken, ExpiresAt: computeXaiOAuthExpiresAt(tokens.ExpiresIn)}
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		return nil, err
	}

	account := mapXaiOAuthAccount(accountID, stored, defaultAccountID)
	return &account, nil
}

// GetStatus 汇总当前登录状态
func (s *XaiOAuthService) GetStatus() (*XaiOAuthStatus, error) {
	accounts, defaultID := s.snapshotAccounts()
	return &XaiOAuthStatus{
		Provider:         XaiOAuthProviderName,
		Authenticated:    len(accounts) > 0,
		DefaultAccountID: defaultID,
		Accounts:         accounts,
	}, nil
}

// ListAccounts 返回全部已登录账号
func (s *XaiOAuthService) ListAccounts() ([]XaiOAuthAccount, error) {
	accounts, _ := s.snapshotAccounts()
	return accounts, nil
}

// SetDefaultAccount 设置默认账号
func (s *XaiOAuthService) SetDefaultAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	s.mu.Lock()
	if _, ok := s.accounts[accountID]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("账号不存在: %s", accountID)
	}
	s.defaultAccountID = accountID
	s.mu.Unlock()
	return s.saveToDisk()
}

// RemoveAccount 移除指定账号并清理其内存缓存
func (s *XaiOAuthService) RemoveAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	s.mu.Lock()
	if _, ok := s.accounts[accountID]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("账号不存在: %s", accountID)
	}
	delete(s.accounts, accountID)
	delete(s.accessTokens, accountID)
	delete(s.refreshLocks, accountID)
	if s.defaultAccountID == accountID {
		s.defaultAccountID = pickXaiOAuthDefaultAccountID(s.accounts)
	}
	s.mu.Unlock()
	return s.saveToDisk()
}

// Logout 清空全部账号与内存缓存
func (s *XaiOAuthService) Logout() error {
	s.mu.Lock()
	s.accounts = make(map[string]xaiOAuthStoredAccount)
	s.accessTokens = make(map[string]xaiOAuthCachedAccessToken)
	s.refreshLocks = make(map[string]*sync.Mutex)
	s.defaultAccountID = ""
	s.mu.Unlock()
	return s.saveToDisk()
}

// GetValidToken 返回有效 access token：缓存可用直接返回，否则按账号互斥刷新
func (s *XaiOAuthService) GetValidToken(accountID string) (string, string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		accountID = s.defaultAccountSnapshot()
	}
	if accountID == "" {
		return "", "", fmt.Errorf("无可用的 xAI 账号，请先登录")
	}

	s.mu.Lock()
	if cached, ok := s.accessTokens[accountID]; ok && time.Until(cached.ExpiresAt) > xaiOAuthRefreshBuffer {
		s.mu.Unlock()
		return cached.Token, accountID, nil
	}
	account, ok := s.accounts[accountID]
	accountLock := s.refreshLocks[accountID]
	if accountLock == nil {
		accountLock = &sync.Mutex{}
		s.refreshLocks[accountID] = accountLock
	}
	s.mu.Unlock()
	if !ok {
		return "", "", fmt.Errorf("账号不存在: %s", accountID)
	}

	// 按账号互斥：并发请求同一账号时只触发一次刷新
	accountLock.Lock()
	defer accountLock.Unlock()

	// 双重检查：等待锁期间其他请求可能已完成刷新
	s.mu.Lock()
	if cached, ok := s.accessTokens[accountID]; ok && time.Until(cached.ExpiresAt) > xaiOAuthRefreshBuffer {
		s.mu.Unlock()
		return cached.Token, accountID, nil
	}
	account = s.accounts[accountID]
	s.mu.Unlock()
	if account.RequiresReauth {
		return "", "", fmt.Errorf("xAI 账号需要重新授权: %s", mapXaiOAuthLogin(accountID, account))
	}
	if strings.TrimSpace(account.RefreshToken) == "" {
		s.markRequiresReauth(accountID)
		return "", "", fmt.Errorf("xAI 登录凭证缺失，请重新授权")
	}

	tokens, err := s.refreshAccessToken(account.RefreshToken)
	if err != nil {
		if errors.Is(err, errXaiOAuthRefreshTokenInvalid) {
			s.markRequiresReauth(accountID)
			return "", "", fmt.Errorf("xAI 登录凭证已失效，请重新授权")
		}
		return "", "", err
	}
	if tokens.AccessToken == "" {
		return "", "", fmt.Errorf("刷新响应缺少 access_token")
	}

	s.mu.Lock()
	if tokens.RefreshToken != "" {
		account.RefreshToken = tokens.RefreshToken
	}
	s.accounts[accountID] = account
	s.accessTokens[accountID] = xaiOAuthCachedAccessToken{Token: tokens.AccessToken, ExpiresAt: computeXaiOAuthExpiresAt(tokens.ExpiresIn)}
	s.mu.Unlock()
	if tokens.RefreshToken != "" {
		if err := s.saveToDisk(); err != nil {
			return "", "", err
		}
	}
	return tokens.AccessToken, accountID, nil
}

// resolveEndpoints 获取（并缓存）discovery 解析出的端点，校验端点必须留在 discovery 同一 origin
func (s *XaiOAuthService) resolveEndpoints() (*xaiOAuthEndpoints, error) {
	s.mu.Lock()
	if s.endpoints != nil {
		endpoints := s.endpoints
		s.mu.Unlock()
		return endpoints, nil
	}
	discoveryURL := s.discoveryURL
	s.mu.Unlock()

	var doc xaiOAuthDiscoveryDocument
	if err := s.doJSON(http.MethodGet, discoveryURL, "", nil, &doc); err != nil {
		return nil, fmt.Errorf("获取 xAI OIDC 发现文档失败: %w", err)
	}
	allowedOrigin := xaiOAuthURLOrigin(discoveryURL)
	deviceEndpoint := strings.TrimSpace(doc.DeviceAuthorizationEndpoint)
	tokenEndpoint := strings.TrimSpace(doc.TokenEndpoint)
	if deviceEndpoint == "" || tokenEndpoint == "" || xaiOAuthURLOrigin(deviceEndpoint) != allowedOrigin || xaiOAuthURLOrigin(tokenEndpoint) != allowedOrigin {
		return nil, fmt.Errorf("xAI 发现文档端点校验失败: 端点必须位于 %s", allowedOrigin)
	}
	endpoints := &xaiOAuthEndpoints{DeviceAuthorizationEndpoint: deviceEndpoint, TokenEndpoint: tokenEndpoint}
	s.mu.Lock()
	s.endpoints = endpoints
	s.mu.Unlock()
	return endpoints, nil
}

// refreshAccessToken 用 refresh_token 换取新的 access token
func (s *XaiOAuthService) refreshAccessToken(refreshToken string) (*xaiOAuthTokenResponse, error) {
	endpoints, err := s.resolveEndpoints()
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("client_id", xaiOAuthClientID)
	status, body, err := s.doRaw(http.MethodPost, endpoints.TokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("Refresh 请求失败: %w", err)
	}
	if status != http.StatusOK {
		// refresh_token 失效（invalid_grant / invalid_client）需要用户重新授权
		if errorCode := xaiOAuthErrorFromBody(body); errorCode == "invalid_grant" || errorCode == "invalid_client" {
			return nil, errXaiOAuthRefreshTokenInvalid
		}
		return nil, translateXaiOAuthTokenError(status, body, "Refresh")
	}
	var tokens xaiOAuthTokenResponse
	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("解析 Refresh 响应失败: %w", err)
	}
	return &tokens, nil
}

// markRequiresReauth 标记账号需要重新授权并持久化
func (s *XaiOAuthService) markRequiresReauth(accountID string) {
	s.mu.Lock()
	account, ok := s.accounts[accountID]
	if !ok {
		s.mu.Unlock()
		return
	}
	account.RequiresReauth = true
	s.accounts[accountID] = account
	delete(s.accessTokens, accountID)
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		fmt.Printf("[XaiOAuth] 持久化 requires_reauth 失败: %v\n", err)
	}
}

func (s *XaiOAuthService) doJSON(method string, rawURL string, contentType string, body io.Reader, target interface{}) error {
	status, data, err := s.doRaw(method, rawURL, contentType, body)
	if err != nil {
		return err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d - %s", status, strings.TrimSpace(string(data)))
	}
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	return nil
}

func (s *XaiOAuthService) doRaw(method string, rawURL string, contentType string, body io.Reader) (int, []byte, error) {
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: xaiOAuthHTTPClientTimeout}
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", xaiOAuthUserAgent)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

func (s *XaiOAuthService) snapshotAccounts() ([]XaiOAuthAccount, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts := make([]XaiOAuthAccount, 0, len(s.accounts))
	for id, account := range s.accounts {
		accounts = append(accounts, mapXaiOAuthAccount(id, account, s.defaultAccountID))
	}
	return accounts, s.defaultAccountID
}

func (s *XaiOAuthService) defaultAccountSnapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.defaultAccountID != "" {
		return s.defaultAccountID
	}
	for id := range s.accounts {
		return id
	}
	return ""
}

func (s *XaiOAuthService) loadFromDisk() error {
	if strings.TrimSpace(s.storePath) == "" {
		return nil
	}
	data, err := os.ReadFile(s.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	var store xaiOAuthStore
	if err := json.Unmarshal(data, &store); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if store.Accounts != nil {
		s.accounts = store.Accounts
	}
	s.defaultAccountID = store.DefaultAccountID
	if s.defaultAccountID != "" {
		if _, ok := s.accounts[s.defaultAccountID]; !ok {
			s.defaultAccountID = ""
		}
	}
	return nil
}

func (s *XaiOAuthService) saveToDisk() error {
	if strings.TrimSpace(s.storePath) == "" {
		return nil
	}
	s.mu.Lock()
	store := xaiOAuthStore{Version: 1, Accounts: make(map[string]xaiOAuthStoredAccount, len(s.accounts)), DefaultAccountID: s.defaultAccountID}
	for id, account := range s.accounts {
		store.Accounts[id] = account
	}
	s.mu.Unlock()
	return AtomicWriteJSON(s.storePath, store)
}

// translateXaiOAuthTokenError 将 token 端点的错误响应映射为可识别错误
func translateXaiOAuthTokenError(status int, body []byte, action string) error {
	var errResp struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &errResp)
	switch strings.TrimSpace(errResp.Error) {
	case "authorization_pending":
		return errXaiOAuthAuthorizationPending
	case "slow_down":
		return errXaiOAuthSlowDown
	case "access_denied":
		return fmt.Errorf("用户拒绝了授权，请重新登录")
	case "expired_token":
		return fmt.Errorf("登录码已过期，请重新登录")
	}
	if errResp.Description != "" {
		return fmt.Errorf("%s失败: HTTP %d - %s", action, status, strings.TrimSpace(errResp.Description))
	}
	return fmt.Errorf("%s失败: HTTP %d - %s", action, status, strings.TrimSpace(string(body)))
}

// xaiOAuthErrorFromBody 提取 OAuth 错误响应中的 error 码
func xaiOAuthErrorFromBody(body []byte) string {
	var errResp struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &errResp)
	return strings.TrimSpace(errResp.Error)
}

// xaiOAuthURLOrigin 提取 URL 的 scheme://host 部分，非法 URL 返回空串
func xaiOAuthURLOrigin(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func mapXaiOAuthAccount(accountID string, account xaiOAuthStoredAccount, defaultID string) XaiOAuthAccount {
	return XaiOAuthAccount{
		ID:              accountID,
		Provider:        XaiOAuthProviderName,
		Login:           mapXaiOAuthLogin(accountID, account),
		AuthenticatedAt: account.AuthenticatedAt,
		IsDefault:       defaultID == accountID,
		RequiresReauth:  account.RequiresReauth,
	}
}

func mapXaiOAuthLogin(accountID string, account xaiOAuthStoredAccount) string {
	login := strings.TrimSpace(account.Login)
	if login == "" {
		return "xAI " + accountID
	}
	return login
}

// pickXaiOAuthDefaultAccountID 默认账号缺失时选择最近认证的账号
func pickXaiOAuthDefaultAccountID(accounts map[string]xaiOAuthStoredAccount) string {
	selectedID := ""
	var selectedAuthenticatedAt int64
	for id, account := range accounts {
		if selectedID == "" || account.AuthenticatedAt > selectedAuthenticatedAt || (account.AuthenticatedAt == selectedAuthenticatedAt && id < selectedID) {
			selectedID = id
			selectedAuthenticatedAt = account.AuthenticatedAt
		}
	}
	return selectedID
}

// extractXaiOAuthIdentity 从 id_token 中提取账号标识（sub）与登录名
func extractXaiOAuthIdentity(tokens *xaiOAuthTokenResponse) (string, string) {
	if tokens == nil {
		return "", ""
	}
	claims, ok := parseXaiOAuthJWTClaims(tokens.IDToken)
	if !ok {
		return "", ""
	}
	accountID := strings.TrimSpace(claims.Sub)
	login := strings.TrimSpace(claims.Email)
	if login == "" {
		login = strings.TrimSpace(claims.PreferredUsername)
	}
	return accountID, login
}

func parseXaiOAuthJWTClaims(token string) (xaiOAuthJWTClaims, bool) {
	var claims xaiOAuthJWTClaims
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return claims, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
	}
	if err != nil {
		return claims, false
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return claims, false
	}
	return claims, true
}

func parseXaiOAuthInterval(value interface{}) int64 {
	switch v := value.(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		var parsed int64
		_, _ = fmt.Sscanf(v, "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func computeXaiOAuthExpiresAt(expiresIn int64) time.Time {
	if expiresIn <= 0 {
		expiresIn = xaiOAuthDefaultExpiresIn
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}
