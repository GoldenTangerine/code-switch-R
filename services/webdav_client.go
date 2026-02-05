package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type webdavClient struct {
	httpClient *http.Client
	username   string
	password   string
}

type progressReadSeeker struct {
	rs         io.ReadSeeker
	total      int64
	sent       int64
	onProgress func(sent int64, total int64)
}

func (p *progressReadSeeker) Read(b []byte) (int, error) {
	n, err := p.rs.Read(b)
	if n > 0 {
		p.sent += int64(n)
		if p.onProgress != nil {
			p.onProgress(p.sent, p.total)
		}
	}
	return n, err
}

func (p *progressReadSeeker) Seek(offset int64, whence int) (int64, error) {
	pos, err := p.rs.Seek(offset, whence)
	if err != nil {
		return pos, err
	}
	// 重置进度（用于重试/重定向时重新上传）
	if whence == io.SeekStart && offset == 0 {
		p.sent = 0
		if p.onProgress != nil {
			p.onProgress(p.sent, p.total)
		}
	}
	return pos, nil
}

func newWebDAVClient(username, password string, timeout time.Duration) *webdavClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &webdavClient{
		httpClient: &http.Client{Timeout: timeout},
		username:   username,
		password:   password,
	}
}

func (c *webdavClient) propfind(ctx context.Context, targetURL string) error {
	body := []byte(`<?xml version="1.0" encoding="utf-8"?><propfind xmlns="DAV:"><prop><resourcetype/></prop></propfind>`)
	makeReq := func(u string) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "PROPFIND", u, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Depth", "0")
		req.Header.Set("Content-Type", "application/xml; charset=utf-8")
		return req, nil
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// 多数 WebDAV 返回 207 Multi-Status
	if resp.StatusCode == 207 {
		return nil
	}
	return fmt.Errorf("PROPFIND 失败: HTTP %d", resp.StatusCode)
}

func (c *webdavClient) mkcol(ctx context.Context, targetURL string) error {
	makeReq := func(u string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, "MKCOL", u, nil)
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK, http.StatusNoContent, http.StatusMethodNotAllowed:
		// 405 通常表示已存在
		return nil
	default:
		return fmt.Errorf("MKCOL 失败: HTTP %d", resp.StatusCode)
	}
}

func (c *webdavClient) put(ctx context.Context, targetURL string, contentType string, body io.ReadSeeker) error {
	return c.putWithProgress(ctx, targetURL, contentType, body, nil)
}

func (c *webdavClient) putWithProgress(ctx context.Context, targetURL string, contentType string, body io.ReadSeeker, onProgress func(sent int64, total int64)) error {
	if body == nil {
		return fmt.Errorf("nil body")
	}

	// 尽量补齐 Content-Length：部分 WebDAV/网盘网关不接受 chunked PUT
	total := int64(-1)
	if cur, err := body.Seek(0, io.SeekCurrent); err == nil {
		if end, err := body.Seek(0, io.SeekEnd); err == nil {
			total = end
		}
		_, _ = body.Seek(cur, io.SeekStart)
	}

	prs := &progressReadSeeker{rs: body, total: total, onProgress: onProgress}
	makeReq := func(u string) (*http.Request, error) {
		if _, err := prs.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, io.NopCloser(prs))
		if err != nil {
			return nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		if total >= 0 {
			req.ContentLength = total
		}
		return req, nil
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("PUT 失败: HTTP %d", resp.StatusCode)
}

func (c *webdavClient) get(ctx context.Context, targetURL string) ([]byte, error) {
	makeReq := func(u string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("GET 失败: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *webdavClient) getToWriter(ctx context.Context, targetURL string, w io.Writer) (int64, error) {
	if w == nil {
		return 0, fmt.Errorf("nil writer")
	}
	makeReq := func(u string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return 0, fmt.Errorf("GET 失败: HTTP %d", resp.StatusCode)
	}
	return io.Copy(w, resp.Body)
}

func (c *webdavClient) delete(ctx context.Context, targetURL string) error {
	makeReq := func(u string) (*http.Request, error) {
		return http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	}
	resp, err := c.doWithRedirects(makeReq, targetURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// 删除测试文件时，404 也算 OK
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	return fmt.Errorf("DELETE 失败: HTTP %d", resp.StatusCode)
}

type requestFactory func(url string) (*http.Request, error)

func (c *webdavClient) doWithRedirects(makeReq requestFactory, startURL string) (*http.Response, error) {
	current := strings.TrimSpace(startURL)
	if current == "" {
		return nil, fmt.Errorf("empty url")
	}

	for i := 0; i < 4; i++ {
		req, err := makeReq(current)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(c.username) != "" || strings.TrimSpace(c.password) != "" {
			req.SetBasicAuth(c.username, c.password)
		}
		req.Header.Set("User-Agent", "CodeSwitchR-WebDAV/1.0")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		if !isRedirectStatus(resp.StatusCode) {
			return resp, nil
		}

		location := strings.TrimSpace(resp.Header.Get("Location"))
		_ = resp.Body.Close()
		if location == "" {
			return nil, fmt.Errorf("redirect without location: HTTP %d", resp.StatusCode)
		}
		next, err := resolveRedirectURL(current, location)
		if err != nil {
			return nil, err
		}
		current = next
	}

	return nil, fmt.Errorf("too many redirects")
}

func isRedirectStatus(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func resolveRedirectURL(current string, location string) (string, error) {
	cur, err := url.Parse(current)
	if err != nil {
		return "", err
	}
	loc, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	return cur.ResolveReference(loc).String(), nil
}

func joinWebDAVURL(endpoint string, parts ...string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil {
		return "", err
	}

	basePath := u.Path
	if strings.TrimSpace(basePath) == "" {
		basePath = "/"
	}

	segments := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "/")
		p = strings.TrimSuffix(p, "/")
		if p == "" {
			continue
		}
		segments = append(segments, p)
	}

	if len(segments) == 0 {
		return u.String(), nil
	}

	u.Path = path.Join(append([]string{basePath}, segments...)...)
	return u.String(), nil
}

func ensureTrailingSlash(urlStr string) string {
	if strings.HasSuffix(urlStr, "/") {
		return urlStr
	}
	return urlStr + "/"
}
