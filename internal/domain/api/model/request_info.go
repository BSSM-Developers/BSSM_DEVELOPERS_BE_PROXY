package model

import (
	"net/http"
	"strings"
)

const (
	BrowserBasePath = "/proxy-browser"
	ServerBasePath  = "/proxy-server"
)

// RequestInfo는 외부 API 호출에 필요한 요청 정보를 담는 값 객체다.
type RequestInfo struct {
	Headers  map[string]string
	Body     []byte
	Endpoint string
	Method   string
}

// NewRequestInfo는 HTTP 요청에서 프록시 전달용 RequestInfo를 생성한다.
// bssm-dev-token, bssm-dev-secret, host 헤더는 필터링된다.
func NewRequestInfo(r *http.Request, body []byte) *RequestInfo {
	return &RequestInfo{
		Headers:  extractHeaders(r),
		Body:     body,
		Endpoint: extractEndpoint(r),
		Method:   r.Method,
	}
}

func extractEndpoint(r *http.Request) string {
	path := r.URL.Path
	query := r.URL.RawQuery

	base := resolveBasePath(path)
	endpoint := path[len(base):]

	if strings.HasPrefix(endpoint, "//") {
		endpoint = "/" + strings.TrimLeft(endpoint, "/")
	}
	if query != "" {
		endpoint = endpoint + "?" + query
	}
	return endpoint
}

func resolveBasePath(path string) string {
	switch {
	case strings.HasPrefix(path, BrowserBasePath):
		return BrowserBasePath
	case strings.HasPrefix(path, ServerBasePath):
		return ServerBasePath
	default:
		return ""
	}
}

var filteredRequestHeaders = map[string]struct{}{
	"bssm-dev-token":  {},
	"bssm-dev-secret": {},
	"host":            {},
}

func extractHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string, len(r.Header))
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		if _, blocked := filteredRequestHeaders[lower]; blocked {
			continue
		}
		if len(values) > 0 {
			headers[name] = values[0]
		}
	}
	return headers
}
