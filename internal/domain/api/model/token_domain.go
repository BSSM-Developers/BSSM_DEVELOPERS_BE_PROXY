package model

import "strings"

// TokenDomain은 token_domain 테이블 엔티티다.
type TokenDomain struct {
	TokenDomainID int64  `gorm:"column:token_domain_id;primaryKey" json:"tokenDomainId"`
	ApiTokenID    int64  `gorm:"column:api_token_id"               json:"apiTokenId"`
	Origin        string `gorm:"column:domain"                     json:"origin"`
}

func (TokenDomain) TableName() string { return "token_domain" }

// MatchesOrigin은 요청 Origin이 등록된 도메인과 일치하는지 확인한다.
// 대소문자 무시, 후행 슬래시 정규화 후 비교한다.
func (d *TokenDomain) MatchesOrigin(requestOrigin string) bool {
	return normalize(d.Origin) == normalize(requestOrigin)
}

func normalize(origin string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(origin)), "/")
}
