package services

import (
	"bytes"
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
	CodexOAuthProviderName       = "codex_oauth"
	codexOAuthClientID           = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexOAuthDeviceUserCodeURL  = "https://auth.openai.com/api/accounts/deviceauth/usercode"
	codexOAuthDeviceTokenURL     = "https://auth.openai.com/api/accounts/deviceauth/token"
	codexOAuthTokenURL           = "https://auth.openai.com/oauth/token"
	codexOAuthVerificationURL    = "https://auth.openai.com/codex/device"
	codexOAuthDeviceRedirectURI  = "https://auth.openai.com/deviceauth/callback"
	codexOAuthBackendAPIBaseURL  = "https://chatgpt.com/backend-api/codex"
	codexOAuthUserAgent          = "code-switch-r-codex-oauth"
	codexOAuthStoreFileName      = "codex_oauth_auth.json"
	codexOAuthDefaultExpiresIn   = 900
	codexOAuthRefreshBuffer      = 60 * time.Second
	codexOAuthHTTPClientTimeout  = 30 * time.Second
	codexOAuthDefaultProviderID  = int64(2000)
	codexOAuthDefaultProviderRef = "codex-oauth"
)

var errCodexOAuthAuthorizationPending = errors.New("authorization_pending")

// CodexOAuthDeviceCodeResponse 是前端展示设备码登录时需要的数据。
type CodexOAuthDeviceCodeResponse struct {
	Provider        string `json:"provider"`
	DeviceCode      string `json:"deviceCode"`
	UserCode        string `json:"userCode"`
	VerificationURI string `json:"verificationUri"`
	ExpiresIn       int64  `json:"expiresIn"`
	Interval        int64  `json:"interval"`
}

// CodexOAuthAccount 是前端可见的 ChatGPT 账号信息，不包含 token。
type CodexOAuthAccount struct {
	ID              string `json:"id"`
	Provider        string `json:"provider"`
	Login           string `json:"login"`
	AuthenticatedAt int64  `json:"authenticatedAt"`
	IsDefault       bool   `json:"isDefault"`
}

// CodexOAuthStatus 汇总当前 OAuth 登录状态。
type CodexOAuthStatus struct {
	Provider         string              `json:"provider"`
	Authenticated    bool                `json:"authenticated"`
	DefaultAccountID string              `json:"defaultAccountId,omitempty"`
	Accounts         []CodexOAuthAccount `json:"accounts"`
	ProviderCard     *Provider           `json:"providerCard,omitempty"`
}

type codexOAuthDeviceCodeRaw struct {
	DeviceAuthID string      `json:"device_auth_id"`
	UserCode     string      `json:"user_code"`
	Interval     interface{} `json:"interval"`
	ExpiresIn    int64       `json:"expires_in"`
}

type codexOAuthPollSuccess struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
}

type codexOAuthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type codexOAuthJWTClaims struct {
	ChatGPTAccountID string `json:"chatgpt_account_id"`
	Email            string `json:"email"`
	OpenAIAuth       struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
	} `json:"https://api.openai.com/auth"`
}

type codexOAuthStoredAccount struct {
	AccountID       string `json:"account_id"`
	Email           string `json:"email,omitempty"`
	RefreshToken    string `json:"refresh_token"`
	AuthenticatedAt int64  `json:"authenticated_at"`
}

type codexOAuthStore struct {
	Version          int                                `json:"version"`
	Accounts         map[string]codexOAuthStoredAccount `json:"accounts"`
	DefaultAccountID string                             `json:"default_account_id,omitempty"`
}

type codexOAuthCachedAccessToken struct {
	Token     string
	ExpiresAt time.Time
}

type codexOAuthPendingDeviceCode struct {
	UserCode  string
	ExpiresAt time.Time
}

// CodexOAuthService 管理 ChatGPT Codex OAuth 登录与 access token 刷新。
type CodexOAuthService struct {
	mu               sync.Mutex
	accounts         map[string]codexOAuthStoredAccount
	defaultAccountID string
	accessTokens     map[string]codexOAuthCachedAccessToken
	pendingDevices   map[string]codexOAuthPendingDeviceCode
	providerService  *ProviderService
	storePath        string
	httpClient       *http.Client
}

func NewCodexOAuthService(providerService *ProviderService) *CodexOAuthService {
	storePath := ""
	if home, err := getUserHomeDir(); err == nil {
		storePath = filepath.Join(home, ".code-switch", codexOAuthStoreFileName)
	}
	s := &CodexOAuthService{
		accounts:        make(map[string]codexOAuthStoredAccount),
		accessTokens:    make(map[string]codexOAuthCachedAccessToken),
		pendingDevices:  make(map[string]codexOAuthPendingDeviceCode),
		providerService: providerService,
		storePath:       storePath,
		httpClient:      &http.Client{Timeout: codexOAuthHTTPClientTimeout},
	}
	if err := s.loadFromDisk(); err != nil {
		fmt.Printf("[CodexOAuth] 加载认证存储失败: %v\n", err)
	}
	return s
}

func (s *CodexOAuthService) Start() error { return nil }
func (s *CodexOAuthService) Stop() error  { return nil }

func (s *CodexOAuthService) StartLogin() (*CodexOAuthDeviceCodeResponse, error) {
	payload, _ := json.Marshal(map[string]string{"client_id": codexOAuthClientID})
	var raw codexOAuthDeviceCodeRaw
	if err := s.doJSON(http.MethodPost, codexOAuthDeviceUserCodeURL, "application/json", bytes.NewReader(payload), &raw); err != nil {
		return nil, fmt.Errorf("Device Code 请求失败: %w", err)
	}
	if raw.DeviceAuthID == "" || raw.UserCode == "" {
		return nil, fmt.Errorf("Device Code 响应缺少必要字段")
	}
	expiresIn := raw.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = codexOAuthDefaultExpiresIn
	}
	interval := parseCodexOAuthInterval(raw.Interval)
	if interval <= 0 {
		interval = 5
	}

	s.mu.Lock()
	now := time.Now()
	for key, pending := range s.pendingDevices {
		if !pending.ExpiresAt.After(now) {
			delete(s.pendingDevices, key)
		}
	}
	s.pendingDevices[raw.DeviceAuthID] = codexOAuthPendingDeviceCode{UserCode: raw.UserCode, ExpiresAt: now.Add(time.Duration(expiresIn) * time.Second)}
	s.mu.Unlock()

	return &CodexOAuthDeviceCodeResponse{
		Provider:        CodexOAuthProviderName,
		DeviceCode:      raw.DeviceAuthID,
		UserCode:        raw.UserCode,
		VerificationURI: codexOAuthVerificationURL,
		ExpiresIn:       expiresIn,
		Interval:        interval,
	}, nil
}

func (s *CodexOAuthService) PollLogin(deviceCode string) (*CodexOAuthAccount, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil, fmt.Errorf("deviceCode 不能为空")
	}

	s.mu.Lock()
	pending, ok := s.pendingDevices[deviceCode]
	if ok && !pending.ExpiresAt.After(time.Now()) {
		delete(s.pendingDevices, deviceCode)
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("登录码已过期，请重新登录")
	}

	pollPayload, _ := json.Marshal(map[string]string{
		"device_auth_id": deviceCode,
		"user_code":      pending.UserCode,
	})
	var poll codexOAuthPollSuccess
	status, body, err := s.doRaw(http.MethodPost, codexOAuthDeviceTokenURL, "application/json", bytes.NewReader(pollPayload))
	if err != nil {
		return nil, err
	}
	if status == http.StatusForbidden || status == http.StatusNotFound {
		return nil, errCodexOAuthAuthorizationPending
	}
	if status == http.StatusGone {
		return nil, fmt.Errorf("登录码已过期，请重新登录")
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Token 轮询失败: HTTP %d - %s", status, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, &poll); err != nil {
		return nil, fmt.Errorf("解析 Token 轮询响应失败: %w", err)
	}
	if poll.AuthorizationCode == "" || poll.CodeVerifier == "" {
		return nil, fmt.Errorf("Token 轮询响应缺少必要字段")
	}

	tokens, err := s.exchangeCodeForTokens(poll.AuthorizationCode, poll.CodeVerifier)
	if err != nil {
		return nil, err
	}
	if tokens.RefreshToken == "" {
		return nil, fmt.Errorf("OAuth 响应缺少 refresh_token")
	}
	accountID, email := extractCodexOAuthIdentity(tokens)
	if accountID == "" {
		return nil, fmt.Errorf("无法从 token 中提取 ChatGPT 账号 ID")
	}

	nowSec := time.Now().Unix()
	stored := codexOAuthStoredAccount{AccountID: accountID, Email: email, RefreshToken: tokens.RefreshToken, AuthenticatedAt: nowSec}
	shouldSelectProvider := false
	defaultAccountID := ""
	s.mu.Lock()
	s.accounts[accountID] = stored
	if s.defaultAccountID == "" {
		s.defaultAccountID = accountID
		shouldSelectProvider = true
	} else if s.defaultAccountID == accountID {
		shouldSelectProvider = true
	}
	defaultAccountID = s.defaultAccountID
	s.accessTokens[accountID] = codexOAuthCachedAccessToken{Token: tokens.AccessToken, ExpiresAt: computeCodexOAuthExpiresAt(tokens.ExpiresIn)}
	delete(s.pendingDevices, deviceCode)
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		return nil, err
	}
	if s.providerService != nil {
		var err error
		if shouldSelectProvider {
			_, err = s.providerService.SelectCodexOAuthProvider(accountID, email)
		} else {
			_, err = s.providerService.EnsureCodexOAuthProvider(accountID, email)
		}
		if err != nil {
			return nil, fmt.Errorf("创建 Codex OAuth provider 失败: %w", err)
		}
	}

	account := mapCodexOAuthAccount(stored, defaultAccountID)
	return &account, nil
}

func (s *CodexOAuthService) GetStatus() (*CodexOAuthStatus, error) {
	accounts, defaultID := s.snapshotAccounts()
	status := &CodexOAuthStatus{
		Provider:         CodexOAuthProviderName,
		Authenticated:    len(accounts) > 0,
		DefaultAccountID: defaultID,
		Accounts:         accounts,
	}
	if s.providerService != nil {
		if providers, err := s.providerService.LoadProviders("codex"); err == nil {
			for i := range providers {
				if isCodexOAuthProvider(providers[i]) {
					provider := providers[i]
					status.ProviderCard = &provider
					break
				}
			}
		}
	}
	return status, nil
}

func (s *CodexOAuthService) ListAccounts() ([]CodexOAuthAccount, error) {
	accounts, _ := s.snapshotAccounts()
	return accounts, nil
}

func (s *CodexOAuthService) SetDefaultAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	s.mu.Lock()
	if _, ok := s.accounts[accountID]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("账号不存在: %s", accountID)
	}
	s.defaultAccountID = accountID
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		return err
	}
	if s.providerService != nil {
		account := s.accountSnapshot(accountID)
		_, err := s.providerService.SelectCodexOAuthProvider(accountID, account.Email)
		return err
	}
	return nil
}

func (s *CodexOAuthService) RemoveAccount(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	s.mu.Lock()
	if _, ok := s.accounts[accountID]; !ok {
		s.mu.Unlock()
		return fmt.Errorf("账号不存在: %s", accountID)
	}
	delete(s.accounts, accountID)
	delete(s.accessTokens, accountID)
	nextDefaultAccountID := s.defaultAccountID
	if s.defaultAccountID == accountID {
		s.defaultAccountID = pickCodexOAuthDefaultAccountID(s.accounts)
		nextDefaultAccountID = s.defaultAccountID
	}
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		return err
	}
	if s.providerService != nil {
		if err := s.providerService.DisableCodexOAuthProviders(accountID); err != nil {
			return err
		}
		if nextDefaultAccountID != "" && nextDefaultAccountID != accountID {
			account := s.accountSnapshot(nextDefaultAccountID)
			_, err := s.providerService.SelectCodexOAuthProvider(nextDefaultAccountID, account.Email)
			return err
		}
	}
	return nil
}

func (s *CodexOAuthService) Logout() error {
	s.mu.Lock()
	s.accounts = make(map[string]codexOAuthStoredAccount)
	s.accessTokens = make(map[string]codexOAuthCachedAccessToken)
	s.pendingDevices = make(map[string]codexOAuthPendingDeviceCode)
	s.defaultAccountID = ""
	s.mu.Unlock()
	if err := s.saveToDisk(); err != nil {
		return err
	}
	if s.providerService != nil {
		return s.providerService.DisableCodexOAuthProviders("")
	}
	return nil
}

func (s *CodexOAuthService) GetValidToken(accountID string) (string, string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		accountID = s.defaultAccountSnapshot()
	}
	if accountID == "" {
		return "", "", fmt.Errorf("无可用的 ChatGPT 账号，请先登录")
	}

	s.mu.Lock()
	if cached, ok := s.accessTokens[accountID]; ok && time.Until(cached.ExpiresAt) > codexOAuthRefreshBuffer {
		s.mu.Unlock()
		return cached.Token, accountID, nil
	}
	account, ok := s.accounts[accountID]
	s.mu.Unlock()
	if !ok {
		return "", "", fmt.Errorf("账号不存在: %s", accountID)
	}

	tokens, err := s.refreshWithToken(account.RefreshToken)
	if err != nil {
		return "", "", err
	}
	if tokens.AccessToken == "" {
		return "", "", fmt.Errorf("刷新响应缺少 access_token")
	}

	s.mu.Lock()
	if tokens.RefreshToken != "" {
		account.RefreshToken = tokens.RefreshToken
		s.accounts[accountID] = account
	}
	s.accessTokens[accountID] = codexOAuthCachedAccessToken{Token: tokens.AccessToken, ExpiresAt: computeCodexOAuthExpiresAt(tokens.ExpiresIn)}
	s.mu.Unlock()
	if tokens.RefreshToken != "" {
		if err := s.saveToDisk(); err != nil {
			return "", "", err
		}
	}
	return tokens.AccessToken, accountID, nil
}

func (s *CodexOAuthService) exchangeCodeForTokens(code string, verifier string) (*codexOAuthTokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "authorization_code")
	values.Set("code", code)
	values.Set("redirect_uri", codexOAuthDeviceRedirectURI)
	values.Set("client_id", codexOAuthClientID)
	values.Set("code_verifier", verifier)
	var tokens codexOAuthTokenResponse
	if err := s.doJSON(http.MethodPost, codexOAuthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), &tokens); err != nil {
		return nil, fmt.Errorf("Token 交换失败: %w", err)
	}
	return &tokens, nil
}

func (s *CodexOAuthService) refreshWithToken(refreshToken string) (*codexOAuthTokenResponse, error) {
	values := url.Values{}
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("client_id", codexOAuthClientID)
	values.Set("scope", "openid profile email")
	var tokens codexOAuthTokenResponse
	if err := s.doJSON(http.MethodPost, codexOAuthTokenURL, "application/x-www-form-urlencoded", strings.NewReader(values.Encode()), &tokens); err != nil {
		return nil, fmt.Errorf("Refresh 失败: %w", err)
	}
	return &tokens, nil
}

func (s *CodexOAuthService) doJSON(method string, rawURL string, contentType string, body io.Reader, target interface{}) error {
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

func (s *CodexOAuthService) doRaw(method string, rawURL string, contentType string, body io.Reader) (int, []byte, error) {
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: codexOAuthHTTPClientTimeout}
	}
	req, err := http.NewRequest(method, rawURL, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("User-Agent", codexOAuthUserAgent)
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

func (s *CodexOAuthService) snapshotAccounts() ([]CodexOAuthAccount, string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	accounts := make([]CodexOAuthAccount, 0, len(s.accounts))
	for _, account := range s.accounts {
		accounts = append(accounts, mapCodexOAuthAccount(account, s.defaultAccountID))
	}
	return accounts, s.defaultAccountID
}

func (s *CodexOAuthService) accountSnapshot(accountID string) codexOAuthStoredAccount {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accounts[accountID]
}

func (s *CodexOAuthService) defaultAccountSnapshot() string {
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

func (s *CodexOAuthService) loadFromDisk() error {
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
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	var store codexOAuthStore
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

func (s *CodexOAuthService) saveToDisk() error {
	if strings.TrimSpace(s.storePath) == "" {
		return nil
	}
	s.mu.Lock()
	store := codexOAuthStore{Version: 1, Accounts: make(map[string]codexOAuthStoredAccount, len(s.accounts)), DefaultAccountID: s.defaultAccountID}
	for id, account := range s.accounts {
		store.Accounts[id] = account
	}
	s.mu.Unlock()
	return AtomicWriteJSON(s.storePath, store)
}

func mapCodexOAuthAccount(account codexOAuthStoredAccount, defaultID string) CodexOAuthAccount {
	login := strings.TrimSpace(account.Email)
	if login == "" {
		login = "ChatGPT " + account.AccountID
	}
	return CodexOAuthAccount{ID: account.AccountID, Provider: CodexOAuthProviderName, Login: login, AuthenticatedAt: account.AuthenticatedAt, IsDefault: defaultID == account.AccountID}
}

func pickCodexOAuthDefaultAccountID(accounts map[string]codexOAuthStoredAccount) string {
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

func parseCodexOAuthInterval(value interface{}) int64 {
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

func computeCodexOAuthExpiresAt(expiresIn int64) time.Time {
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}

func extractCodexOAuthIdentity(tokens *codexOAuthTokenResponse) (string, string) {
	if tokens == nil {
		return "", ""
	}
	for _, jwt := range []string{tokens.IDToken, tokens.AccessToken} {
		claims, ok := parseCodexOAuthJWTClaims(jwt)
		if !ok {
			continue
		}
		accountID := strings.TrimSpace(claims.ChatGPTAccountID)
		if accountID == "" {
			accountID = strings.TrimSpace(claims.OpenAIAuth.ChatGPTAccountID)
		}
		if accountID != "" {
			return accountID, strings.TrimSpace(claims.Email)
		}
	}
	return "", ""
}

func parseCodexOAuthJWTClaims(token string) (codexOAuthJWTClaims, bool) {
	var claims codexOAuthJWTClaims
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

func isCodexOAuthProvider(provider Provider) bool {
	return strings.EqualFold(strings.TrimSpace(provider.AuthProvider), CodexOAuthProviderName)
}

func codexOAuthProviderDisplayName(login string) string {
	login = strings.TrimSpace(login)
	if login == "" {
		return "ChatGPT Codex OAuth"
	}
	return "ChatGPT Codex OAuth - " + login
}
