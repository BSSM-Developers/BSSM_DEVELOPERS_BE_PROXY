package validator

import (
	"net"
	"net/url"
	"strings"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
)

// DomainValidator는 SSRF 방어를 위한 도메인 URL 검증기다.
// Java의 DomainValidator와 동일한 규칙을 적용한다:
//  1. scheme이 https:// 여야 한다
//  2. IP 직접 입력(IPv4/IPv6) 차단
//  3. 내부 hostname 패턴 차단 (localhost, *.local 등)
//  4. DNS 해석 후 resolved IP가 내부 대역이면 차단
type DomainValidator struct{}

func New() *DomainValidator {
	return &DomainValidator{}
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
	return validateResolvedIP(hostname)
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

	// IPv4 직접 입력 차단
	if net.ParseIP(hostname) != nil {
		return apperrors.ErrBlockedInternalDomain
	}

	// IPv6 직접 입력 (브래킷 포함) 차단
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
		// DNS 해석 실패 = 존재하지 않는 도메인 → 차단
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

// isPrivateIP는 IP가 내부 대역(RFC1918, 루프백, 링크로컬 등)인지 확인한다.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	private := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"100.64.0.0/10",   // CGNAT
		"169.254.0.0/16",  // Link-local (AWS metadata 등)
		"192.0.0.0/24",    // IETF Protocol
		"198.51.100.0/24", // TEST-NET-2
		"203.0.113.0/24",  // TEST-NET-3
		"240.0.0.0/4",     // Reserved
		"0.0.0.0/8",       // 현재 네트워크
		"fc00::/7",        // IPv6 Unique local
		"fe80::/10",       // IPv6 Link-local
		"::1/128",         // IPv6 Loopback
	}

	for _, cidr := range private {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
