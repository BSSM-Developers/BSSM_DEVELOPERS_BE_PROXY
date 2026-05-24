package validator

import (
	"net"
	"net/url"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
)

// privateNetworks는 프로세스 시작 시 한 번만 파싱된다.
var privateNetworks []*net.IPNet

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",
		"169.254.0.0/16",
		"192.0.0.0/24",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"240.0.0.0/4",
		"0.0.0.0/8",
		"fc00::/7",
		"fe80::/10",
		"::1/128",
	}
	for _, cidr := range cidrs {
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			privateNetworks = append(privateNetworks, network)
		}
	}
}

// DomainValidator는 SSRF 방어를 위한 도메인 URL 검증기다.
// DNS 해석 결과를 인메모리 캐시(5분 TTL)에 저장해 반복 DNS I/O를 제거한다.
type DomainValidator struct {
	cache *gocache.Cache
}

func New() *DomainValidator {
	return &DomainValidator{
		cache: gocache.New(5*time.Minute, 10*time.Minute),
	}
}

// Validate는 domainURL을 검증하고 위반 시 ProxyError를 반환한다.
func (v *DomainValidator) Validate(domainURL string) error {
	if strings.TrimSpace(domainURL) == "" {
		return apperrors.ErrInvalidDomainURL
	}

	parsed, err := url.Parse(domainURL)
	if err != nil || parsed.Host == "" {
		return apperrors.ErrInvalidDomainURL
	}

	if !strings.EqualFold(parsed.Scheme, "https") {
		return apperrors.ErrInvalidDomainURL
	}

	hostname := parsed.Hostname()
	if err := validateHostname(hostname); err != nil {
		return err
	}

	// DNS 검증 결과 캐시 조회
	if cached, ok := v.cache.Get(hostname); ok {
		if cached == nil {
			return nil
		}
		return cached.(error)
	}

	result := validateResolvedIP(hostname)
	v.cache.Set(hostname, result, gocache.DefaultExpiration)
	return result
}

var blockedHostnamePatterns = []string{
	"localhost",
}

var blockedHostnameSuffixes = []string{
	".local", ".internal", ".intranet", ".corp", ".lan",
}

func validateHostname(hostname string) error {
	if hostname == "" {
		return apperrors.ErrInvalidDomainURL
	}

	if net.ParseIP(hostname) != nil {
		return apperrors.ErrBlockedInternalDomain
	}

	if strings.HasPrefix(hostname, "[") {
		return apperrors.ErrBlockedInternalDomain
	}

	lower := strings.ToLower(hostname)
	for _, pattern := range blockedHostnamePatterns {
		if lower == pattern {
			return apperrors.ErrBlockedInternalDomain
		}
	}
	for _, suffix := range blockedHostnameSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return apperrors.ErrBlockedInternalDomain
		}
	}
	return nil
}

func validateResolvedIP(hostname string) error {
	addrs, err := net.LookupHost(hostname)
	if err != nil {
		return apperrors.ErrInvalidDomainURL
	}

	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if isPrivateIP(ip) {
			return apperrors.ErrBlockedInternalDomain
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, network := range privateNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
