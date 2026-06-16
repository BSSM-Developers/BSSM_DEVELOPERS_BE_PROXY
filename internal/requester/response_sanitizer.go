package requester

import (
	"io"
	"net/http"
	"strings"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
)

// allowedContentTypes는 프록시가 허용하는 Content-Type 목록이다.
// 이 외의 타입은 악성 바이너리 또는 스크립트 전달 가능성이 있어 차단한다.
var allowedContentTypes = []string{
	"application/json",
	"text/plain",
	"text/xml",
	"application/xml",
	"application/x-www-form-urlencoded",
}

// hopByHopHeaders는 프록시가 제거해야 하는 hop-by-hop 헤더다.
var hopByHopHeaders = map[string]struct{}{
	"connection":          {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

// excludedResponseHeaders는 CORS 처리 및 백엔드 인프라 정보 노출 방지를 위해 제거하는 응답 헤더다.
var excludedResponseHeaders = map[string]struct{}{
	// CORS — 프록시가 직접 관리
	"access-control-allow-origin":      {},
	"access-control-allow-credentials": {},
	"access-control-allow-headers":     {},
	"access-control-allow-methods":     {},
	"access-control-expose-headers":    {},
	"access-control-max-age":           {},
	"vary":                             {},
	"content-disposition":              {},
	// 백엔드 인프라 노출 차단
	"server":          {},
	"x-powered-by":    {},
	"location":        {},
	"x-forwarded-for": {},
	"x-real-ip":       {},
	"forwarded":       {},
	"via":             {},
}

// ProxyResponse는 버퍼링된 외부 API 응답을 담는 구조체다.
type ProxyResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// StreamResponse는 스트리밍 외부 API 응답을 담는 구조체다.
// Body는 io.ReadCloser로, 호출자가 반드시 닫아야 한다.
type StreamResponse struct {
	StatusCode int
	Headers    http.Header
	Body       io.ReadCloser
}

// sanitize는 응답 Content-Type을 검증하고 hop-by-hop 헤더를 제거한다.
func sanitize(resp *http.Response, body []byte) (*ProxyResponse, error) {
	if err := validateContentType(resp.Header); err != nil {
		return nil, err
	}
	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    filterHeaders(resp.Header),
		Body:       body,
	}, nil
}

// sanitizeStream은 응답이 text/event-stream인지 검증하고 hop-by-hop 헤더를 제거한다.
// 검증 성공 시 resp.Body 소유권이 StreamResponse로 이전된다.
func sanitizeStream(resp *http.Response) (*StreamResponse, error) {
	ct := resp.Header.Get("Content-Type")
	baseType := strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	if baseType != "text/event-stream" {
		return nil, apperrors.ErrStreamNotSupported
	}
	return &StreamResponse{
		StatusCode: resp.StatusCode,
		Headers:    filterHeaders(resp.Header),
		Body:       resp.Body,
	}, nil
}

// filterHeaders는 hop-by-hop 및 CORS 관련 헤더를 제거한 복사본을 반환한다.
func filterHeaders(header http.Header) http.Header {
	filtered := make(http.Header)
	for name, values := range header {
		lower := strings.ToLower(name)
		if _, blocked := hopByHopHeaders[lower]; blocked {
			continue
		}
		if _, blocked := excludedResponseHeaders[lower]; blocked {
			continue
		}
		filtered[name] = values
	}
	return filtered
}

func validateContentType(header http.Header) error {
	ct := header.Get("Content-Type")
	if ct == "" {
		return nil
	}

	// charset 등 파라미터 제거 후 기본 타입 비교
	baseType := strings.ToLower(strings.SplitN(ct, ";", 2)[0])
	baseType = strings.TrimSpace(baseType)

	for _, allowed := range allowedContentTypes {
		if strings.EqualFold(baseType, allowed) {
			return nil
		}
	}
	return apperrors.ErrBlockedContentType
}
