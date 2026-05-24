package cache

import (
	"context"
	"fmt"
)

// Service는 2레벨 캐시의 공개 계약이다.
// 구현은 TwoLevelCache가 담당한다.
type Service interface {
	// GetOrFetch는 키로 캐시를 조회하고 없으면 fetch를 호출하여 저장한다.
	// dest는 반드시 포인터여야 한다.
	GetOrFetch(ctx context.Context, key string, fetch func() (any, error), dest any) error

	// GetOrFetchSlice는 슬라이스 타입을 위한 캐시 조회/저장이다.
	// dest는 슬라이스 포인터여야 한다 (e.g. *[]model.ApiUsage).
	GetOrFetchSlice(ctx context.Context, key string, fetch func() (any, error), dest any) error

	// Evict는 특정 키의 캐시를 무효화한다.
	Evict(ctx context.Context, key string) error
}

// Keys는 캐시 키 생성 함수 모음이다.
// Java의 CacheKeys 클래스에 대응한다.
type Keys struct{}

const (
	prefixApiToken    = "proxy:api_token:"
	prefixTokenDomain = "proxy:token_domain:"
	prefixApiUsage    = "proxy:api_usage:"
)

func ApiTokenKey(clientID string) string        { return prefixApiToken + clientID }
func TokenDomainKey(tokenID int64) string        { return prefixTokenDomain + fmt.Sprint(tokenID) }
func ApiUsageListKey(tokenID int64) string       { return prefixApiUsage + fmt.Sprint(tokenID) }
