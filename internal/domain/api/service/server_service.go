package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/apperrors"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/requester"
	gocache "github.com/patrickmn/go-cache"
	"golang.org/x/crypto/bcrypt"
)

const bcryptCacheTTL = 60 * time.Second

// ServerService는 서버-투-서버 프록시 요청을 처리한다.
// bssm-dev-secret 헤더의 bcrypt 시크릿 키를 검증하여 접근을 허가한다.
// Java의 ServerUseApiService에 대응한다.
type ServerService struct {
	pipeline     *Pipeline
	bcryptCache  *gocache.Cache
}

func NewServerService(pipeline *Pipeline) *ServerService {
	return &ServerService{
		pipeline:    pipeline,
		bcryptCache: gocache.New(bcryptCacheTTL, 5*time.Minute),
	}
}

func (s *ServerService) Handle(
	ctx context.Context,
	secretKey string,
	token string,
	r *http.Request,
	body []byte,
) (*requester.ProxyResponse, error) {
	info := model.NewRequestInfo(r, body)

	return s.pipeline.Execute(
		ctx, token, info, r,
		logmodel.DirectionServerToServer,
		func(_ context.Context, apiToken *model.ApiToken) error {
			return s.validateSecretKey(apiToken, secretKey)
		},
	)
}

// validateSecretKey는 bcrypt 검증 결과를 60초간 캐시한다.
// key: apiTokenID + SHA-256(plainSecretKey) — 평문 노출 없이 식별
func (s *ServerService) validateSecretKey(apiToken *model.ApiToken, plain string) error {
	if plain == "" {
		return apperrors.ErrInvalidSecretKey
	}

	sum := sha256.Sum256([]byte(plain))
	key := fmt.Sprintf("bcrypt:%d:%x", apiToken.ApiTokenID, sum)

	if cached, ok := s.bcryptCache.Get(key); ok {
		if cached.(bool) {
			return nil
		}
		return apperrors.ErrInvalidSecretKey
	}

	err := bcrypt.CompareHashAndPassword([]byte(apiToken.SecretKey), []byte(plain))
	valid := err == nil
	s.bcryptCache.Set(key, valid, bcryptCacheTTL)

	if !valid {
		return apperrors.ErrInvalidSecretKey
	}
	return nil
}
