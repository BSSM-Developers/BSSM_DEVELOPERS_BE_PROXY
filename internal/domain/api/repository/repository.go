package repository

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
)

// TokenRepository는 api_token 테이블 접근 계약을 정의한다.
type TokenRepository interface {
	FindByClientID(ctx context.Context, clientID string) (*model.ApiToken, error)
	FindByID(ctx context.Context, id int64) (*model.ApiToken, error)
	Save(ctx context.Context, token *model.ApiToken) error
}

// UsageRepository는 api_usage + api JOIN 조회 계약을 정의한다.
type UsageRepository interface {
	FindAllByTokenID(ctx context.Context, tokenID int64) ([]model.ApiUsage, error)
}

// DomainRepository는 token_domain 테이블 접근 계약을 정의한다.
type DomainRepository interface {
	FindAllByTokenID(ctx context.Context, tokenID int64) ([]model.TokenDomain, error)
}
