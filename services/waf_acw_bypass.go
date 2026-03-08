package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"time"
)

// acwArg1Re 从挑战页 HTML 中提取 arg1 值: var arg1='HEX40CHARS'
var acwArg1Re = regexp.MustCompile(`var\s+arg1\s*=\s*'([0-9A-Fa-f]{40})'`)

// acw_sc__v2 挑战的固定参数（阿里云 CDN WAF）
// 排列表：将 arg1 的 40 个字符按此顺序重排
var acwUnsboxOrder = [40]int{
	15, 35, 29, 24, 33, 16, 1, 38, 10, 9,
	19, 31, 40, 27, 22, 23, 25, 13, 6, 11,
	39, 18, 20, 8, 14, 21, 32, 26, 2, 30,
	7, 4, 17, 5, 3, 28, 34, 37, 12, 36,
}

// 固定 XOR 密钥
const acwXorKey = "3000176000856006061501533003690027800375"

// solveAcwChallenge 从挑战页 HTML 中提取 arg1，
// 通过固定排列 + XOR 算法计算 acw_sc__v2 cookie 值。
// 算法：acw_sc__v2 = hexXor(unsbox(arg1), acwXorKey)
func solveAcwChallenge(htmlBody string) (string, error) {
	// 提取 arg1
	m := acwArg1Re.FindStringSubmatch(htmlBody)
	if len(m) < 2 {
		return "", fmt.Errorf("未找到 var arg1='...' 声明")
	}
	arg1 := m[1]
	if len(arg1) != 40 {
		return "", fmt.Errorf("arg1 长度异常: %d (期望 40)", len(arg1))
	}

	// 步骤 1: unsbox — 按排列表重排 arg1 字符
	reordered := acwUnsbox(arg1)

	// 步骤 2: hexXor — 逐两字符与固定密钥做 XOR
	cookie, err := acwHexXor(reordered, acwXorKey)
	if err != nil {
		return "", fmt.Errorf("XOR 计算失败: %w", err)
	}

	return cookie, nil
}

// acwUnsbox 按固定排列表重排 arg1 的字符
func acwUnsbox(arg1 string) string {
	result := make([]byte, len(acwUnsboxOrder))
	for i := 0; i < len(arg1) && i < len(acwUnsboxOrder); i++ {
		for j := 0; j < len(acwUnsboxOrder); j++ {
			if acwUnsboxOrder[j] == i+1 {
				result[j] = arg1[i]
			}
		}
	}
	return string(result)
}

// acwHexXor 将两个等长十六进制字符串逐两字符做 XOR
func acwHexXor(a, b string) (string, error) {
	if len(a) != len(b) {
		return "", fmt.Errorf("长度不匹配: %d vs %d", len(a), len(b))
	}
	result := make([]byte, 0, len(a))
	for i := 0; i+1 < len(a) && i+1 < len(b); i += 2 {
		va := hexPairValue(a[i], a[i+1])
		vb := hexPairValue(b[i], b[i+1])
		if va < 0 || vb < 0 {
			return "", fmt.Errorf("无效的十六进制字符位置 %d", i)
		}
		xored := va ^ vb
		result = append(result, hexChar(xored>>4), hexChar(xored&0x0f))
	}
	return string(result), nil
}

// hexPairValue 将两个十六进制字符转为 byte 值，无效返回 -1
func hexPairValue(hi, lo byte) int {
	h := hexDigit(hi)
	l := hexDigit(lo)
	if h < 0 || l < 0 {
		return -1
	}
	return h<<4 | l
}

func hexDigit(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	default:
		return -1
	}
}

func hexChar(v int) byte {
	if v < 10 {
		return byte('0' + v)
	}
	return byte('a' + v - 10)
}

// tryAcwBypass 在检测到 acw_sc__v2 挑战后被调用：
// 1. 用 solveAcwChallenge 计算 cookie
// 2. 创建带 cookie jar 的 http.Client
// 3. 重新发起 /api/pricing 请求
// 4. 返回解析好的 pricing 数据或错误（降级到原有提示）
func tryAcwBypass(client *http.Client, targetURL, apiKey, authType, challengeBody string,
	debug *ProviderModelPricingDebug) (*providerPricingResponse, error) {

	// 第一步：计算 cookie 值
	cookieValue, err := solveAcwChallenge(challengeBody)
	if err != nil {
		return nil, fmt.Errorf("ACW 挑战求解失败: %w", err)
	}

	// 第二步：解析 targetURL 获取域名
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("解析 URL 失败: %w", err)
	}

	// 第三步：创建带 cookie jar 的 client
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("创建 cookie jar 失败: %w", err)
	}

	// 设置 acw_sc__v2 cookie，同时回传服务端下发的 acw_tc / cdn_sec_tc cookie
	cookies := []*http.Cookie{
		{Name: "acw_sc__v2", Value: cookieValue},
	}
	jar.SetCookies(parsedURL, cookies)

	retryClient := &http.Client{
		Timeout:   client.Timeout,
		Transport: client.Transport,
		Jar:       jar,
	}

	// 第四步：重新发起请求
	retryAttempt := newProviderModelPricingDebugAttempt(
		providerModelPricingSourceCommon,
		"/api/pricing (acw-bypass-retry)",
		"GET",
		targetURL,
		authType,
	)

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		retryAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, err
	}
	attachAuthHeader(req, apiKey, authType)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", targetURL)
	retryAttempt.RequestHeaders = buildProviderPricingHeaderSnapshot(req.Header)

	startedAt := time.Now()
	resp, err := retryClient.Do(req)
	if err != nil {
		retryAttempt.DurationMs = time.Since(startedAt).Milliseconds()
		retryAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	setProviderPricingDebugResponse(retryAttempt, resp, body, time.Since(startedAt))
	if err != nil {
		retryAttempt.Error = err.Error()
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		httpErr := fmt.Errorf("ACW 绕过重试返回 HTTP %d", resp.StatusCode)
		retryAttempt.Error = httpErr.Error()
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, httpErr
	}

	var pricing providerPricingResponse
	if err := json.Unmarshal(body, &pricing); err != nil {
		retryAttempt.Error = fmt.Sprintf("ACW 绕过重试 JSON 解析失败: %v", err)
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, fmt.Errorf("ACW 绕过重试 JSON 解析失败: %w", err)
	}

	if !pricing.Success {
		retryAttempt.Error = "ACW 绕过重试返回 success=false"
		appendProviderModelPricingDebugAttempt(debug, retryAttempt)
		return nil, fmt.Errorf("ACW 绕过重试返回 success=false")
	}

	appendProviderModelPricingDebugAttempt(debug, retryAttempt)
	return &pricing, nil
}
