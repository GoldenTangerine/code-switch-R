package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"
)

// acwChallengeTimeout 是 JS 挑战脚本执行的最大时长
const acwChallengeTimeout = 5 * time.Second

// acwSetTimeoutMaxDepth 是 setTimeout 同步执行的最大递归深度，防止栈溢出
const acwSetTimeoutMaxDepth int32 = 10

// scriptTagRe 匹配 <script>...</script> 标签中的 JS 代码
var scriptTagRe = regexp.MustCompile(`(?is)<script[^>]*>\s*(.*?)\s*</script>`)

// acwCookieRe 从 document.cookie 值中提取 acw_sc__v2=xxx
var acwCookieRe = regexp.MustCompile(`acw_sc__v2=([^;]+)`)

// solveAcwChallenge 接收 WAF 挑战页的 HTML body，
// 使用 goja JS 引擎执行脚本，捕获 document.cookie 中的 acw_sc__v2 值
func solveAcwChallenge(htmlBody string) (cookieValue string, err error) {
	// 提取所有 <script> 标签中的 JS 代码
	matches := scriptTagRe.FindAllStringSubmatch(htmlBody, -1)
	if len(matches) == 0 {
		return "", fmt.Errorf("未找到 <script> 标签")
	}

	// 拼接所有脚本块
	var scripts []string
	for _, m := range matches {
		code := strings.TrimSpace(m[1])
		if code != "" {
			scripts = append(scripts, code)
		}
	}
	if len(scripts) == 0 {
		return "", fmt.Errorf("所有 <script> 标签均为空")
	}

	// 创建 goja VM
	vm := goja.New()

	// 用于捕获 cookie 写入
	var capturedCookie string

	// 构建模拟的 document 对象
	docObj := vm.NewObject()

	// document.cookie: getter/setter
	// WAF 脚本最终会执行 document.cookie = 'acw_sc__v2=xxx'
	_ = docObj.DefineAccessorProperty("cookie", vm.ToValue(func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(capturedCookie)
	}), vm.ToValue(func(call goja.FunctionCall) goja.Value {
		capturedCookie = call.Argument(0).String()
		return goja.Undefined()
	}), goja.FLAG_FALSE, goja.FLAG_TRUE)

	// document.location 模拟
	locObj := vm.NewObject()
	_ = locObj.Set("reload", func() {
		// 空操作 —— WAF 脚本在写入 cookie 后会调用 location.reload()
	})
	_ = locObj.Set("href", "")
	_ = docObj.Set("location", locObj)

	// document.createElement — 某些变种脚本可能调用
	_ = docObj.Set("createElement", func(tag string) *goja.Object {
		elem := vm.NewObject()
		_ = elem.Set("innerHTML", "")
		_ = elem.Set("style", vm.NewObject())
		return elem
	})

	// document.head / document.body 模拟
	headObj := vm.NewObject()
	_ = headObj.Set("appendChild", func(child *goja.Object) {})
	_ = docObj.Set("head", headObj)

	bodyObj := vm.NewObject()
	_ = bodyObj.Set("appendChild", func(child *goja.Object) {})
	_ = docObj.Set("body", bodyObj)

	_ = docObj.Set("getElementById", func(id string) goja.Value { return goja.Null() })
	_ = docObj.Set("getElementsByTagName", func(tag string) []interface{} { return []interface{}{} })

	_ = vm.Set("document", docObj)

	// window 对象模拟
	winObj := vm.NewObject()
	_ = winObj.Set("location", locObj)
	_ = vm.Set("window", winObj)

	// navigator 模拟
	navObj := vm.NewObject()
	_ = navObj.Set("userAgent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	_ = vm.Set("navigator", navObj)

	// setTimeout/setInterval 模拟 —— 同步执行，限制递归深度防止栈溢出
	var setTimeoutDepth int32
	_ = vm.Set("setTimeout", func(call goja.FunctionCall) goja.Value {
		if atomic.AddInt32(&setTimeoutDepth, 1) > acwSetTimeoutMaxDepth {
			atomic.AddInt32(&setTimeoutDepth, -1)
			return vm.ToValue(0)
		}
		defer atomic.AddInt32(&setTimeoutDepth, -1)
		if fn, ok := goja.AssertFunction(call.Argument(0)); ok {
			_, _ = fn(goja.Undefined())
		}
		return vm.ToValue(1)
	})
	_ = vm.Set("setInterval", func(call goja.FunctionCall) goja.Value {
		return vm.ToValue(1)
	})
	_ = vm.Set("clearTimeout", func(id int) {})
	_ = vm.Set("clearInterval", func(id int) {})

	// 超时保护：防止恶意脚本无限循环阻塞 goroutine
	timer := time.AfterFunc(acwChallengeTimeout, func() {
		vm.Interrupt("ACW challenge execution timeout")
	})
	defer timer.Stop()

	// 执行所有脚本
	for i, script := range scripts {
		_, execErr := vm.RunString(script)
		if execErr != nil {
			// 超时中断直接返回错误
			if strings.Contains(execErr.Error(), "timeout") {
				return "", fmt.Errorf("JS 执行超时 (>%s): %w", acwChallengeTimeout, execErr)
			}
			// 某些脚本可能不是挑战脚本，忽略非关键错误
			// 但如果已经拿到 cookie 值就不再继续
			if capturedCookie != "" {
				break
			}
			// 如果是最后一个脚本且都执行失败
			if i == len(scripts)-1 && capturedCookie == "" {
				return "", fmt.Errorf("JS 执行失败: %w", execErr)
			}
		}
		if capturedCookie != "" {
			break
		}
	}

	if capturedCookie == "" {
		return "", fmt.Errorf("脚本执行完毕但未设置 document.cookie")
	}

	// 解析 acw_sc__v2 值
	m := acwCookieRe.FindStringSubmatch(capturedCookie)
	if len(m) < 2 {
		return "", fmt.Errorf("cookie 中未找到 acw_sc__v2 值: %s", capturedCookie)
	}

	return m[1], nil
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

	// 设置 acw_sc__v2 cookie
	jar.SetCookies(parsedURL, []*http.Cookie{
		{
			Name:  "acw_sc__v2",
			Value: cookieValue,
		},
	})

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
