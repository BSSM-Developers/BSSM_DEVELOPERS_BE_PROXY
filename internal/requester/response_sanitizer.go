package requester

import (
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

// excludedResponseHeaders는 CORS 처리를 위해 제거하는 응답 헤더다.
// 프록시 서버가 별도로 CORS를 관리하므로 업스트림 CORS 헤더는 제거한다.
var excludedResponseHeaders = map[string]struct{}{
	"access-control-allow-origin":      {},
	"access-control-allow-credentials": {},
	"access-control-allow-headers":     {},
	"access-control-allow-methods":     {},
	"access-control-expose-headers":    {},
	"access-control-max-age":           {},
	"vary":                             {},
	"content-disposition":              {},
}

// ProxyResponse는 외부 API 응답을 담는 구조체다.
type ProxyResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// sanitize는 응답 Content-Type을 검증하고 hop-by-hop 헤더를 제거한다.
func sanitize(resp *http.Response, body []byte) (*ProxyResponse, error) {
	if err := validateContentType(resp.Header); err != nil {
		return nil, err
	}

	filtered := make(http.Header)
	for name, values := range resp.Header {
		lower := strings.ToLower(name)
		if _, blocked := hopByHopHeaders[lower]; blocked {
			continue
		}
		if _, blocked := excludedResponseHeaders[lower]; blocked {
			continue
		}
		filtered[name] = values
	}

	return &ProxyResponse{
		StatusCode: resp.StatusCode,
		Headers:    filtered,
		Body:       body,
	}, nil
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
