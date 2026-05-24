package logservice

import "strings"

// sensitiveHeaderKeywords는 로그에서 마스킹할 헤더 이름 키워드다.
var sensitiveHeaderKeywords = []string{
	"authorization", "cookie", "token", "secret", "credential",
}

// SanitizeHeaders는 민감한 헤더 값을 마스킹한다.
// Java의 LogHeaderSanitizer에 대응한다.
func SanitizeHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return map[string]string{}
	}
	safe := make(map[string]string, len(headers))
	for k, v := range headers {
		if isSensitive(k) {
			safe[k] = MaskToken(v)
		} else {
			safe[k] = v
		}
	}
	return safe
}

func isSensitive(headerName string) bool {
	lower := strings.ToLower(headerName)
	for _, kw := range sensitiveHeaderKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// MaskToken은 토큰 값을 앞 3자 + *** + 뒤 3자 형식으로 마스킹한다.
// Java의 TokenMasker에 대응한다.
func MaskToken(value string) string {
	if len(value) <= 6 {
		return "***"
	}
	return value[:3] + "***" + value[len(value)-3:]
}
