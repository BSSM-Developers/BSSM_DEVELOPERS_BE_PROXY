package apperrors

import "fmt"

// ProxyError는 HTTP 상태 코드를 포함하는 프록시 도메인 에러다.
// 미들웨어에서 errors.As로 감지하여 JSON 응답으로 변환한다.
type ProxyError struct {
	Code       string
	StatusCode int
	Message    string
}

func (e *ProxyError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ExternalAPIError는 업스트림 API 호출 실패 시 발생하는 에러다.
// 업스트림 상태 코드와 응답 바디를 포함한다.
type ExternalAPIError struct {
	ProxyError
	UpstreamStatusCode int
	UpstreamBody       string
}

func NewExternalAPIError(upstreamStatus int, upstreamBody string) *ExternalAPIError {
	return &ExternalAPIError{
		ProxyError: ProxyError{
			Code:       "EXTERNAL_API_ERROR",
			StatusCode: 502,
			Message:    "API 호출 중 오류가 발생했습니다.",
		},
		UpstreamStatusCode: upstreamStatus,
		UpstreamBody:       upstreamBody,
	}
}

// 프록시 도메인 센티넬 에러 목록.
// 각 에러는 Java ErrorCode enum의 대응 항목과 동일한 상태 코드 및 메시지를 갖는다.
var (
	ErrApiTokenNotFound = &ProxyError{
		Code: "API_TOKEN_NOT_FOUND", StatusCode: 404, Message: "API 토큰을 찾을 수 없습니다.",
	}
	ErrApiTokenBlocked = &ProxyError{
		Code: "API_TOKEN_BLOCKED", StatusCode: 403,
		Message: "요청 제한 정책 위반으로 API 토큰이 차단되었습니다. 차단 해제는 BSSM Developers 공식 홈페이지를 통해 요청할 수 있습니다.",
	}
	ErrInvalidSecretKey = &ProxyError{
		Code: "INVALID_SECRET_KEY", StatusCode: 401, Message: "유효하지 않은 시크릿 키입니다.",
	}
	ErrUnauthorizedDomain = &ProxyError{
		Code: "UNAUTHORIZED_DOMAIN", StatusCode: 403, Message: "허용되지 않은 도메인에서의 요청입니다.",
	}
	ErrApiUsageNotFound = &ProxyError{
		Code: "API_USAGE_NOT_FOUND", StatusCode: 404, Message: "API 사용을 찾을 수 없습니다.",
	}
	ErrTooManyRequests = &ProxyError{
		Code: "TOO_MANY_REQUESTS", StatusCode: 429, Message: "요청이 너무 많습니다. 잠시 후 다시 시도해주세요.",
	}
	ErrInvalidDomainURL = &ProxyError{
		Code: "INVALID_DOMAIN_URL", StatusCode: 400,
		Message: "유효하지 않은 도메인 URL입니다. https:// 로 시작하는 공개 도메인만 허용됩니다.",
	}
	ErrBlockedInternalDomain = &ProxyError{
		Code: "BLOCKED_INTERNAL_DOMAIN", StatusCode: 400, Message: "내부 네트워크 주소로의 요청은 허용되지 않습니다.",
	}
	ErrBlockedContentType = &ProxyError{
		Code: "BLOCKED_CONTENT_TYPE", StatusCode: 502, Message: "허용되지 않는 Content-Type의 응답입니다.",
	}
	ErrUnsupportedBasePath = &ProxyError{
		Code: "UNSUPPORTED_PROXY_BASE_PATH", StatusCode: 400, Message: "지원하지 않는 프록시 경로입니다.",
	}
)
