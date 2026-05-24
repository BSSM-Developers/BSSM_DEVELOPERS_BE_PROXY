package logservice

import (
	"strings"
	"unicode/utf8"
)

const maxBodyBytes = 2048

// BodyTruncateResult는 본문 잘라내기 결과다.
type BodyTruncateResult struct {
	Body          string
	Truncated     bool
	ContentLength int64
}

// TruncateBody는 본문을 최대 2048바이트로 잘라낸다.
// 바이너리 데이터는 "[binary data]"로 대체한다.
// Java의 BodyTruncator에 대응한다.
func TruncateBody(body []byte) BodyTruncateResult {
	if len(body) == 0 {
		return BodyTruncateResult{}
	}

	if !utf8.Valid(body) {
		return BodyTruncateResult{Body: "[binary data]", ContentLength: int64(len(body))}
	}

	text := string(body)
	if len(body) <= maxBodyBytes {
		return BodyTruncateResult{Body: text, ContentLength: int64(len(body))}
	}

	// utf8 경계에서 잘라낸다
	truncated := truncateAtRuneBoundary(body, maxBodyBytes)
	return BodyTruncateResult{
		Body:          truncated,
		Truncated:     true,
		ContentLength: int64(len(body)),
	}
}

// TruncateString은 문자열 타입 본문을 잘라낸다.
func TruncateString(text string) BodyTruncateResult {
	return TruncateBody([]byte(text))
}

func truncateAtRuneBoundary(b []byte, maxLen int) string {
	if len(b) <= maxLen {
		return string(b)
	}
	s := string(b[:maxLen])
	return strings.ToValidUTF8(s, "")
}
