package requester

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/validator"
)

const maxBodySize = 16 * 1024 * 1024 // 16MB

// Requester는 버퍼링 방식 외부 API 호출 계약이다.
type Requester interface {
	Request(ctx context.Context, domain string, info *model.RequestInfo) (*ProxyResponse, error)
}

// StreamRequester는 스트리밍 방식 외부 API 호출 계약이다.
// 반환된 StreamResponse.Body는 호출자가 반드시 닫아야 한다.
type StreamRequester interface {
	RequestStream(ctx context.Context, domain string, info *model.RequestInfo) (*StreamResponse, error)
}

// clientPool은 도메인별 http.Client를 재사용하는 풀이다.
// Java의 RestRequester.POOL ConcurrentHashMap에 대응한다.
var clientPool sync.Map // map[string]*http.Client

// streamClientPool은 스트리밍 전용 도메인별 http.Client 풀이다.
// 일반 클라이언트와 달리 전체 타임아웃이 없다 (context로 수명을 제어한다).
var streamClientPool sync.Map // map[string]*http.Client

// HTTPRequester는 net/http 기반 외부 API 호출 구현체다.
type HTTPRequester struct {
	validator *validator.DomainValidator
}

// HTTPStreamRequester는 응답 바디를 버퍼링하지 않고 스트림으로 반환하는 구현체다.
type HTTPStreamRequester struct {
	validator *validator.DomainValidator
}

func NewHTTPStreamRequester(v *validator.DomainValidator) StreamRequester {
	return &HTTPStreamRequester{validator: v}
}

func (r *HTTPStreamRequester) RequestStream(ctx context.Context, domain string, info *model.RequestInfo) (*StreamResponse, error) {
	if err := r.validator.Validate(domain); err != nil {
		return nil, err
	}

	targetURL := domain + info.Endpoint
	req, err := http.NewRequestWithContext(ctx, info.Method, targetURL, bytes.NewReader(info.Body))
	if err != nil {
		return nil, apperrors.NewExternalAPIError(0, err.Error())
	}

	for k, v := range info.Headers {
		req.Header.Set(k, v)
	}

	client := getOrCreateStreamClient(domain)
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.NewExternalAPIError(0, err.Error())
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		return nil, apperrors.NewExternalAPIError(resp.StatusCode, string(body))
	}

	result, err := sanitizeStream(resp)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	return result, nil
	// resp.Body는 닫지 않는다 — StreamResponse.Body가 소유권을 가진다
}

func NewHTTPRequester(v *validator.DomainValidator) Requester {
	return &HTTPRequester{validator: v}
}

func (r *HTTPRequester) Request(ctx context.Context, domain string, info *model.RequestInfo) (*ProxyResponse, error) {
	if err := r.validator.Validate(domain); err != nil {
		return nil, err
	}

	targetURL := domain + info.Endpoint
	req, err := http.NewRequestWithContext(ctx, info.Method, targetURL, bytes.NewReader(info.Body))
	if err != nil {
		return nil, apperrors.NewExternalAPIError(0, err.Error())
	}

	for k, v := range info.Headers {
		req.Header.Set(k, v)
	}

	client := getOrCreateClient(domain)
	resp, err := client.Do(req)
	if err != nil {
		return nil, apperrors.NewExternalAPIError(0, err.Error())
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, apperrors.NewExternalAPIError(resp.StatusCode, err.Error())
	}

	if resp.StatusCode >= 400 {
		return nil, apperrors.NewExternalAPIError(resp.StatusCode, string(body))
	}

	return sanitize(resp, body)
}

func getOrCreateClient(domain string) *http.Client {
	if v, ok := clientPool.Load(domain); ok {
		return v.(*http.Client)
	}
	actual, _ := clientPool.LoadOrStore(domain, newClient())
	return actual.(*http.Client)
}

func getOrCreateStreamClient(domain string) *http.Client {
	if v, ok := streamClientPool.Load(domain); ok {
		return v.(*http.Client)
	}
	actual, _ := streamClientPool.LoadOrStore(domain, newStreamClient())
	return actual.(*http.Client)
}

func newClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
		// 리다이렉트 비활성화: 악성 서버의 내부 주소 리다이렉트를 통한 SSRF 우회 방어
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func newStreamClient() *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		MaxIdleConns:          20,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   0, // context가 연결 수명을 제어한다
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}
