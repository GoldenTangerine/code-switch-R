/**
 * @name: xAI OAuth 托管登录服务测试
 * @Descripttion: 用 httptest 模拟 xAI discovery / 设备码 / token 端点，验证设备码流程、刷新与重新授权标记
 * @version: 1.0.0
 * @Author: sm
 * @Date: 2026-08-17 05:45:00
 * @LastEditTime: 2026-08-17 05:45:00
 * @FilePath: services/xaioauthservice_test.go
 */

package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	xaiOAuthTestSub   = "user-1234"
	xaiOAuthTestEmail = "grok-user@example.com"
)

// xaiOAuthTestResponse 一次端点响应的模拟数据
type xaiOAuthTestResponse struct {
	Status int
	Body   string
}

// xaiOAuthTestAuthServer 模拟 xAI 授权服务器：discovery + 设备码 + token 三类端点
type xaiOAuthTestAuthServer struct {
	*httptest.Server

	mu              sync.Mutex
	pollScript      []xaiOAuthTestResponse
	pollIndex       int
	refreshResponse xaiOAuthTestResponse
	deviceCalls     int
	pollCalls       int
	refreshCalls    int
	lastDeviceForm  url.Values
	lastPollForm    url.Values
	lastRefreshForm url.Values
}

func newXaiOAuthTestAuthServer(t *testing.T) *xaiOAuthTestAuthServer {
	t.Helper()
	server := &xaiOAuthTestAuthServer{}
	server.Server = httptest.NewServer(http.HandlerFunc(server.handle))
	t.Cleanup(server.Close)
	return server
}

func (s *xaiOAuthTestAuthServer) handle(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/.well-known/openid-configuration":
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                         s.URL,
			"device_authorization_endpoint":  s.URL + "/oauth/device",
			"token_endpoint":                 s.URL + "/oauth/token",
		})
	case r.Method == http.MethodPost && r.URL.Path == "/oauth/device":
		_ = r.ParseForm()
		s.mu.Lock()
		s.deviceCalls++
		s.lastDeviceForm = r.PostForm
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, map[string]interface{}{
			"device_code":      "device-code-1",
			"user_code":        "ABCD-EFGH",
			"verification_uri": s.URL + "/device",
			"expires_in":       900,
			"interval":         5,
		})
	case r.Method == http.MethodPost && r.URL.Path == "/oauth/token":
		_ = r.ParseForm()
		if r.PostFormValue("grant_type") == "refresh_token" {
			s.mu.Lock()
			s.refreshCalls++
			s.lastRefreshForm = r.PostForm
			response := s.refreshResponse
			s.mu.Unlock()
			s.writeRaw(w, response)
			return
		}
		s.mu.Lock()
		s.pollCalls++
		s.lastPollForm = r.PostForm
		index := s.pollIndex
		if index >= len(s.pollScript) {
			index = len(s.pollScript) - 1
		}
		s.pollIndex++
		response := s.pollScript[index]
		s.mu.Unlock()
		s.writeRaw(w, response)
	default:
		http.NotFound(w, r)
	}
}

func (s *xaiOAuthTestAuthServer) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (s *xaiOAuthTestAuthServer) writeRaw(w http.ResponseWriter, response xaiOAuthTestResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.Status)
	_, _ = w.Write([]byte(response.Body))
}

func (s *xaiOAuthTestAuthServer) setPollScript(script ...xaiOAuthTestResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pollScript = script
}

func (s *xaiOAuthTestAuthServer) setRefreshResponse(response xaiOAuthTestResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshResponse = response
}

func (s *xaiOAuthTestAuthServer) snapshotCalls() (device int, poll int, refresh int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deviceCalls, s.pollCalls, s.refreshCalls
}

// newXaiOAuthTestService 构造指向模拟授权服务器的被测服务
func newXaiOAuthTestService(t *testing.T, server *xaiOAuthTestAuthServer) *XaiOAuthService {
	t.Helper()
	service := NewXaiOAuthService()
	service.discoveryURL = server.URL + "/.well-known/openid-configuration"
	return service
}

// xaiOAuthTestTokenJSON 构造 token 端点成功响应
func xaiOAuthTestTokenJSON(accessToken string, refreshToken string) string {
	return fmt.Sprintf(`{"access_token":%q,"refresh_token":%q,"id_token":%q,"token_type":"Bearer","expires_in":3600}`, accessToken, refreshToken, xaiOAuthTestJWT(xaiOAuthTestSub, xaiOAuthTestEmail))
}

// xaiOAuthTestJWT 构造仅含 sub / email 的测试 id_token
func xaiOAuthTestJWT(sub string, email string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payloadJSON, _ := json.Marshal(map[string]string{"sub": sub, "email": email})
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	return header + "." + payload + ".test-signature"
}

// loginXaiOAuthTestAccount 走一次完整设备码登录并返回账号 ID
func loginXaiOAuthTestAccount(t *testing.T, service *XaiOAuthService) string {
	t.Helper()
	started, err := service.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin 失败: %v", err)
	}
	account, err := service.PollLogin(started.DeviceCode)
	if err != nil {
		t.Fatalf("PollLogin 失败: %v", err)
	}
	return account.ID
}

// expireXaiOAuthTestCache 将指定账号的内存 token 缓存置为过期，触发刷新路径
func expireXaiOAuthTestCache(t *testing.T, service *XaiOAuthService, accountID string) {
	t.Helper()
	service.mu.Lock()
	service.accessTokens[accountID] = xaiOAuthCachedAccessToken{Token: "stale-token", ExpiresAt: time.Now().Add(-time.Minute)}
	service.mu.Unlock()
}

func TestXaiOAuthDeviceFlowSuccess(t *testing.T) {
	useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")})
	service := newXaiOAuthTestService(t, server)

	started, err := service.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin 失败: %v", err)
	}
	if started.Provider != XaiOAuthProviderName {
		t.Fatalf("provider = %s", started.Provider)
	}
	if started.DeviceCode != "device-code-1" || started.UserCode != "ABCD-EFGH" {
		t.Fatalf("设备码响应字段错误: %#v", started)
	}
	if started.VerificationURI != server.URL+"/device" {
		t.Fatalf("verificationUri = %s", started.VerificationURI)
	}
	if started.Interval != 5 || started.ExpiresIn != 900 {
		t.Fatalf("interval/expiresIn = %d/%d", started.Interval, started.ExpiresIn)
	}

	// 校验设备码请求携带 client_id 与 scope
	server.mu.Lock()
	deviceForm := server.lastDeviceForm
	server.mu.Unlock()
	if deviceForm.Get("client_id") != xaiOAuthClientID {
		t.Fatalf("设备码请求 client_id = %s", deviceForm.Get("client_id"))
	}
	if deviceForm.Get("scope") != xaiOAuthScope {
		t.Fatalf("设备码请求 scope = %s", deviceForm.Get("scope"))
	}

	account, err := service.PollLogin(started.DeviceCode)
	if err != nil {
		t.Fatalf("PollLogin 失败: %v", err)
	}
	if account.ID != xaiOAuthTestSub {
		t.Fatalf("账号 ID = %s, 期望 id_token 的 sub", account.ID)
	}
	if account.Login != xaiOAuthTestEmail {
		t.Fatalf("账号 login = %s", account.Login)
	}
	if !account.IsDefault || account.RequiresReauth {
		t.Fatalf("账号默认/重授权标记错误: %#v", account)
	}

	accounts, err := service.ListAccounts()
	if err != nil || len(accounts) != 1 {
		t.Fatalf("ListAccounts = %#v, err=%v", accounts, err)
	}
	status, err := service.GetStatus()
	if err != nil || !status.Authenticated || status.DefaultAccountID != xaiOAuthTestSub {
		t.Fatalf("GetStatus = %#v, err=%v", status, err)
	}

	// 存储文件以 0600 权限原子写入
	storePath := service.storePath
	info, err := os.Stat(storePath)
	if err != nil {
		t.Fatalf("存储文件不存在: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("存储文件权限 = %o, 期望 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"refresh_token": "rt-initial"`) {
		t.Fatalf("存储文件缺少 refresh_token: %s", string(data))
	}

	// 设备码流程返回的 access_token 已进内存缓存，GetValidToken 无需刷新
	token, tokenAccountID, err := service.GetValidToken("")
	if err != nil {
		t.Fatalf("GetValidToken 失败: %v", err)
	}
	if token != "at-initial" || tokenAccountID != xaiOAuthTestSub {
		t.Fatalf("GetValidToken = %s/%s", token, tokenAccountID)
	}
	if _, _, refreshCalls := server.snapshotCalls(); refreshCalls != 0 {
		t.Fatalf("缓存有效时不应刷新, refreshCalls = %d", refreshCalls)
	}

	// 移除账号后状态与 token 均不可用
	if err := service.RemoveAccount(xaiOAuthTestSub); err != nil {
		t.Fatalf("RemoveAccount 失败: %v", err)
	}
	if accounts, _ = service.ListAccounts(); len(accounts) != 0 {
		t.Fatalf("移除后账号未清空: %#v", accounts)
	}
	if _, _, err := service.GetValidToken(""); err == nil {
		t.Fatal("移除全部账号后 GetValidToken 应报错")
	}
}

func TestXaiOAuthPollPendingThenSuccess(t *testing.T) {
	useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(
		xaiOAuthTestResponse{Status: http.StatusBadRequest, Body: `{"error":"slow_down","interval":10}`},
		xaiOAuthTestResponse{Status: http.StatusBadRequest, Body: `{"error":"authorization_pending"}`},
		xaiOAuthTestResponse{Status: http.StatusBadRequest, Body: `{"error":"expired_token"}`},
		xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")},
	)
	service := newXaiOAuthTestService(t, server)

	started, err := service.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin 失败: %v", err)
	}

	if _, err := service.PollLogin(started.DeviceCode); !errors.Is(err, errXaiOAuthSlowDown) {
		t.Fatalf("首次轮询期望 slow_down, got %v", err)
	}
	if _, err := service.PollLogin(started.DeviceCode); !errors.Is(err, errXaiOAuthAuthorizationPending) {
		t.Fatalf("第二次轮询期望 authorization_pending, got %v", err)
	}
	if _, err := service.PollLogin(started.DeviceCode); err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("第三次轮询期望过期错误, got %v", err)
	}
	account, err := service.PollLogin(started.DeviceCode)
	if err != nil {
		t.Fatalf("第四次轮询应成功: %v", err)
	}
	if account.ID != xaiOAuthTestSub || !account.IsDefault {
		t.Fatalf("轮询成功账号错误: %#v", account)
	}

	// 设备码轮询必须使用 device_code grant 并携带 client_id
	server.mu.Lock()
	pollForm := server.lastPollForm
	server.mu.Unlock()
	if pollForm.Get("grant_type") != xaiOAuthDeviceGrantType {
		t.Fatalf("轮询 grant_type = %s", pollForm.Get("grant_type"))
	}
	if pollForm.Get("device_code") != "device-code-1" || pollForm.Get("client_id") != xaiOAuthClientID {
		t.Fatalf("轮询表单字段错误: %s", pollForm.Encode())
	}
}

func TestXaiOAuthDiscoveryRejectsForeignOrigin(t *testing.T) {
	useIsolatedHomeDir(t)
	// 发现文档返回指向其他 origin 的端点，必须拒绝
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                        "https://auth.x.ai",
			"device_authorization_endpoint": "https://evil.example/oauth/device",
			"token_endpoint":                "https://evil.example/oauth/token",
		})
	}))
	t.Cleanup(foreign.Close)

	service := NewXaiOAuthService()
	service.discoveryURL = foreign.URL + "/.well-known/openid-configuration"
	if _, err := service.StartLogin(); err == nil || !strings.Contains(err.Error(), "端点校验失败") {
		t.Fatalf("期望端点 origin 校验失败, got %v", err)
	}
}

func TestXaiOAuthRefreshFlow(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")})
	server.setRefreshResponse(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-refreshed", "rt-rotated")})
	service := newXaiOAuthTestService(t, server)

	accountID := loginXaiOAuthTestAccount(t, service)
	if _, _, err := service.GetValidToken(accountID); err != nil {
		t.Fatalf("登录后 GetValidToken 失败: %v", err)
	}

	// 缓存过期后刷新，并轮换 refresh_token
	expireXaiOAuthTestCache(t, service, accountID)
	token, tokenAccountID, err := service.GetValidToken(accountID)
	if err != nil {
		t.Fatalf("刷新后 GetValidToken 失败: %v", err)
	}
	if token != "at-refreshed" || tokenAccountID != accountID {
		t.Fatalf("刷新结果 = %s/%s", token, tokenAccountID)
	}
	if _, _, refreshCalls := server.snapshotCalls(); refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, 期望 1", refreshCalls)
	}

	// 校验刷新请求表单
	server.mu.Lock()
	refreshForm := server.lastRefreshForm
	server.mu.Unlock()
	if refreshForm.Get("grant_type") != "refresh_token" || refreshForm.Get("refresh_token") != "rt-initial" || refreshForm.Get("client_id") != xaiOAuthClientID {
		t.Fatalf("刷新表单字段错误: %s", refreshForm.Encode())
	}

	// 轮换后的 refresh_token 已持久化到磁盘
	data, err := os.ReadFile(filepath.Join(homeDir, ".code-switch", xaiOAuthStoreFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"refresh_token": "rt-rotated"`) {
		t.Fatalf("存储文件未更新轮换后的 refresh_token: %s", string(data))
	}
}

func TestXaiOAuthRequiresReauth(t *testing.T) {
	homeDir := useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")})
	server.setRefreshResponse(xaiOAuthTestResponse{Status: http.StatusBadRequest, Body: `{"error":"invalid_grant","error_description":"refresh token expired"}`})
	service := newXaiOAuthTestService(t, server)

	accountID := loginXaiOAuthTestAccount(t, service)
	accounts, _ := service.ListAccounts()
	if accounts[0].RequiresReauth {
		t.Fatal("初始状态不应要求重新授权")
	}

	// 刷新失败（invalid_grant）→ 标记 requires_reauth 并持久化
	expireXaiOAuthTestCache(t, service, accountID)
	_, _, err := service.GetValidToken(accountID)
	if err == nil || !strings.Contains(err.Error(), "重新授权") {
		t.Fatalf("期望重新授权错误, got %v", err)
	}
	accounts, _ = service.ListAccounts()
	if len(accounts) != 1 || !accounts[0].RequiresReauth {
		t.Fatalf("账号未标记 requires_reauth: %#v", accounts)
	}

	// 标记后不再发起网络刷新
	_, _, err = service.GetValidToken(accountID)
	if err == nil || !strings.Contains(err.Error(), "重新授权") {
		t.Fatalf("标记后 GetValidToken 应直接报错, got %v", err)
	}
	if _, _, refreshCalls := server.snapshotCalls(); refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, 期望 1", refreshCalls)
	}

	data, err := os.ReadFile(filepath.Join(homeDir, ".code-switch", xaiOAuthStoreFileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"requires_reauth": true`) {
		t.Fatalf("存储文件未持久化 requires_reauth: %s", string(data))
	}

	// 重新登录同一账号后标记应被清除
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-relogin", "rt-relogin")})
	started, err := service.StartLogin()
	if err != nil {
		t.Fatalf("重新 StartLogin 失败: %v", err)
	}
	if _, err := service.PollLogin(started.DeviceCode); err != nil {
		t.Fatalf("重新 PollLogin 失败: %v", err)
	}
	accounts, _ = service.ListAccounts()
	if len(accounts) != 1 || accounts[0].RequiresReauth {
		t.Fatalf("重新登录后 requires_reauth 未清除: %#v", accounts)
	}
}

func TestXaiOAuthRefreshSingleFlightPerAccount(t *testing.T) {
	useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")})
	server.setRefreshResponse(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-refreshed", "rt-refreshed")})
	service := newXaiOAuthTestService(t, server)

	accountID := loginXaiOAuthTestAccount(t, service)
	accounts, _ := service.ListAccounts()
	if len(accounts) != 1 {
		t.Fatalf("账号数 = %d", len(accounts))
	}
	expireXaiOAuthTestCache(t, service, accountID)

	// 并发请求同一账号：按账号互斥 + 双重检查，只应触发一次刷新
	var wg sync.WaitGroup
	results := make([]string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			token, _, err := service.GetValidToken(accountID)
			if err != nil {
				t.Errorf("并发 GetValidToken 失败: %v", err)
				return
			}
			results[index] = token
		}(i)
	}
	wg.Wait()
	for _, token := range results {
		if token != "at-refreshed" {
			t.Fatalf("并发刷新结果 = %s", token)
		}
	}
	if _, _, refreshCalls := server.snapshotCalls(); refreshCalls != 1 {
		t.Fatalf("并发下 refreshCalls = %d, 期望 1", refreshCalls)
	}
}

func TestXaiOAuthLogoutAndDefaultAccount(t *testing.T) {
	useIsolatedHomeDir(t)
	server := newXaiOAuthTestAuthServer(t)
	server.setPollScript(xaiOAuthTestResponse{Status: http.StatusOK, Body: xaiOAuthTestTokenJSON("at-initial", "rt-initial")})
	service := newXaiOAuthTestService(t, server)

	accountID := loginXaiOAuthTestAccount(t, service)

	if err := service.SetDefaultAccount("not-exist"); err == nil {
		t.Fatal("设置不存在的默认账号应报错")
	}
	if err := service.SetDefaultAccount(accountID); err != nil {
		t.Fatalf("SetDefaultAccount 失败: %v", err)
	}
	status, _ := service.GetStatus()
	if status.DefaultAccountID != accountID {
		t.Fatalf("默认账号 = %s", status.DefaultAccountID)
	}

	if err := service.Logout(); err != nil {
		t.Fatalf("Logout 失败: %v", err)
	}
	accounts, _ := service.ListAccounts()
	if len(accounts) != 0 {
		t.Fatalf("Logout 后账号未清空: %#v", accounts)
	}
	if _, _, err := service.GetValidToken(""); err == nil {
		t.Fatal("Logout 后 GetValidToken 应报错")
	}
}
