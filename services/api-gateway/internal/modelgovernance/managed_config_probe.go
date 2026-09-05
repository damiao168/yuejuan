package modelgovernance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPManagedAPIProber struct {
	timeout time.Duration
}

func NewHTTPManagedAPIProber(timeout time.Duration) *HTTPManagedAPIProber {
	if timeout <= 0 || timeout > 15*time.Second {
		timeout = 8 * time.Second
	}
	return &HTTPManagedAPIProber{timeout: timeout}
}

func (p *HTTPManagedAPIProber) Probe(ctx context.Context, connection ManagedAPIConnection) (result ManagedAPIProbeResult) {
	started := time.Now()
	result = ManagedAPIProbeResult{Message: "连接失败"}
	defer func() { result.LatencyMS = time.Since(started).Milliseconds() }()
	endpoint, err := managedModelsEndpoint(connection.Config.BaseURL)
	if err != nil {
		result.Message = "接口地址无效"
		return result
	}
	transport := &http.Transport{
		DialContext:           safePublicDialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: p.timeout,
		IdleConnTimeout:       10 * time.Second,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   p.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	method := http.MethodGet
	var body []byte
	if connection.Config.AdapterType == "dashscope_native" {
		if connection.Config.BaseURL != "https://dashscope.aliyuncs.com/api/v1" {
			result.Message = "DashScope 原生接口地址无效"
			return result
		}
		endpoint = connection.Config.BaseURL + "/services/aigc/text-generation/generation"
		method = http.MethodPost
		body, _ = json.Marshal(map[string]any{
			"model":      connection.Config.ModelName,
			"input":      map[string]any{"messages": []map[string]string{{"role": "user", "content": "Reply OK"}}},
			"parameters": map[string]any{"max_tokens": 1, "result_format": "message"},
		})
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		result.Message = "接口地址无效"
		return result
	}
	req.Header.Set("Authorization", "Bearer "+connection.APIKey)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "EduGrade-API-Probe/1.0")
	response, err := client.Do(req)
	if err != nil {
		result.Message = "无法连接供应商接口"
		return result
	}
	defer response.Body.Close()
	result.StatusCode = response.StatusCode
	switch {
	case response.StatusCode >= 200 && response.StatusCode < 300:
		result.OK = true
		result.Message = "连接成功，密钥可用"
	case response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden:
		result.Message = "密钥无效或没有模型访问权限"
	case response.StatusCode == http.StatusNotFound:
		result.Message = "接口地址未提供 models 能力，请检查 Base URL"
	case response.StatusCode == http.StatusTooManyRequests:
		result.Message = "供应商限流，请稍后再试"
	default:
		result.Message = fmt.Sprintf("供应商返回 HTTP %d", response.StatusCode)
	}
	return result
}

func managedModelsEndpoint(baseURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", ErrInvalidManagedConfig
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/models"
	return parsed.String(), nil
}

func safePublicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, ErrInvalidManagedConfig
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errorsOrInvalid(err)
	}
	for _, address := range addresses {
		if !publicProviderIP(address.IP) {
			return nil, ErrInvalidManagedConfig
		}
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 10 * time.Second}
	return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func publicProviderIP(ip net.IP) bool {
	return ip != nil && !ip.IsLoopback() && !ip.IsPrivate() && !ip.IsUnspecified() &&
		!ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsMulticast()
}

func errorsOrInvalid(err error) error {
	if err != nil {
		return err
	}
	return ErrInvalidManagedConfig
}

var _ ManagedAPIProber = (*HTTPManagedAPIProber)(nil)
