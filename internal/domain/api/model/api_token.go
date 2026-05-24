package model

import (
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"golang.org/x/crypto/bcrypt"
)

type ApiTokenState string

const (
	StateNormal  ApiTokenState = "NORMAL"
	StateWarning ApiTokenState = "WARNING"
	StateBlocked ApiTokenState = "BLOCKED"
)

// ApiToken은 api_token 테이블 엔티티이자 도메인 모델이다.
// 접근 검증 로직을 직접 보유한다 (Information Expert).
type ApiToken struct {
	ApiTokenID   int64         `gorm:"column:api_token_id;primaryKey" json:"apiTokenId"`
	UserID       int64         `gorm:"column:user_id"                 json:"userId"`
	ApiTokenName string        `gorm:"column:api_token_name"          json:"apiTokenName"`
	ApiTokenUUID string        `gorm:"column:api_token_uuid"          json:"apiTokenUUID"`
	SecretKey    string        `gorm:"column:secret_key"              json:"secretKey"`
	State        ApiTokenState `gorm:"column:state"                   json:"state"`
}

func (ApiToken) TableName() string { return "api_token" }

// ValidateNotBlocked는 토큰이 차단 상태이면 에러를 반환한다.
func (t *ApiToken) ValidateNotBlocked() error {
	if t.State == StateBlocked {
		return apperrors.ErrApiTokenBlocked
	}
	return nil
}

// ValidateServerAccess는 서버-투-서버 요청의 시크릿 키를 검증한다.
func (t *ApiToken) ValidateServerAccess(plainSecretKey string) error {
	if plainSecretKey == "" {
		return apperrors.ErrInvalidSecretKey
	}
	if err := bcrypt.CompareHashAndPassword([]byte(t.SecretKey), []byte(plainSecretKey)); err != nil {
		return apperrors.ErrInvalidSecretKey
	}
	return nil
}

// ValidateBrowserAccess는 브라우저 요청의 Origin을 허용 도메인 목록과 비교한다.
func (t *ApiToken) ValidateBrowserAccess(requestOrigin string, domains []TokenDomain) error {
	if len(domains) == 0 || requestOrigin == "" {
		return apperrors.ErrUnauthorizedDomain
	}
	for _, d := range domains {
		if d.MatchesOrigin(requestOrigin) {
			return nil
		}
	}
	return apperrors.ErrUnauthorizedDomain
}

// TransitionToNextState는 상태를 NORMAL→WARNING→BLOCKED 순으로 전환한다.
// 변경이 없으면 빈 문자열을 반환한다.
func (t *ApiToken) TransitionToNextState() ApiTokenState {
	switch t.State {
	case StateNormal, "":
		t.State = StateWarning
		return StateWarning
	case StateWarning:
		t.State = StateBlocked
		return StateBlocked
	default:
		return ""
	}
}
