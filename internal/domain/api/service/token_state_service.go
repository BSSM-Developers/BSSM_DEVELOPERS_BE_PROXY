package service

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/cache"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/repository"
	userrepo "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/user/repository"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/mailclient"
	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/notifier"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TokenStateService는 API 토큰 상태를 NORMAL→WARNING 자동, BLOCKED는 관리자 수동으로 전환한다.
// Java의 ApiTokenStateUpdateService에 대응한다.
type TokenStateService struct {
	repo     repository.TokenRepository
	cache    cache.Service
	notifier notifier.Notifier
	userRepo *userrepo.UserRepository
	mail     *mailclient.MailClient
	logger   *zap.Logger
}

func NewTokenStateService(
	repo repository.TokenRepository,
	cache cache.Service,
	n notifier.Notifier,
	userRepo *userrepo.UserRepository,
	mail *mailclient.MailClient,
	logger *zap.Logger,
) *TokenStateService {
	return &TokenStateService{repo: repo, cache: cache, notifier: n, userRepo: userRepo, mail: mail, logger: logger}
}

// TransitionState는 현재 상태에서 다음 단계로 전환한다.
// 상태가 변경된 경우 새 상태를 반환하고, 이미 WARNING이면 빈 문자열을 반환한다.
// requestCount는 WARNING 알림 본문에 사용된다.
func (s *TokenStateService) TransitionState(ctx context.Context, apiTokenID int64, requestCount int64) (model.ApiTokenState, error) {
	token, err := s.repo.FindByID(ctx, apiTokenID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}

	prev := token.State
	next := token.TransitionToNextState()
	if next == "" {
		return "", nil
	}

	if err := s.repo.Save(ctx, token); err != nil {
		return "", err
	}

	// BLOCKED 전환 시 캐시 즉시 무효화 — TTL 내 차단 우회 방지
	if next == model.StateBlocked {
		if err := s.cache.Evict(ctx, cache.ApiTokenKey(token.ApiTokenUUID)); err != nil {
			s.logger.Warn("BLOCKED 토큰 캐시 evict 실패",
				zap.Int64("apiTokenId", apiTokenID),
				zap.Error(err),
			)
		}
	}

	s.logger.Warn("API 토큰 상태 변경",
		zap.Int64("apiTokenId", apiTokenID),
		zap.String("from", string(prev)),
		zap.String("to", string(next)),
	)

	// WARNING 전환 시 비동기 알림 발송 (차단 액션 버튼 포함)
	if next == model.StateWarning {
		go func() {
			if err := s.notifier.NotifyWarning(
				context.Background(),
				token.ApiTokenID,
				token.ApiTokenName,
				token.ApiTokenUUID,
				requestCount,
			); err != nil {
				s.logger.Warn("WARNING 알림 발송 실패",
					zap.Int64("apiTokenId", apiTokenID),
					zap.Error(err),
				)
			}
		}()
		go s.sendTokenStateEmail(context.Background(), token, "WARNING")
	}

	if next == model.StateBlocked {
		go s.sendTokenStateEmail(context.Background(), token, "BLOCKED")
	}

	return next, nil
}

// ForceBlock은 관리자 웹훅 요청으로 토큰을 즉시 BLOCKED 처리한다.
func (s *TokenStateService) ForceBlock(ctx context.Context, apiTokenID int64) error {
	token, err := s.repo.FindByID(ctx, apiTokenID)
	if err != nil {
		return err
	}
	token.State = model.StateBlocked
	if err := s.repo.Save(ctx, token); err != nil {
		return err
	}
	if err := s.cache.Evict(ctx, cache.ApiTokenKey(token.ApiTokenUUID)); err != nil {
		s.logger.Warn("ForceBlock 캐시 evict 실패",
			zap.Int64("apiTokenId", apiTokenID),
			zap.Error(err),
		)
	}
	s.logger.Warn("API 토큰 강제 차단",
		zap.Int64("apiTokenId", apiTokenID),
		zap.String("tokenName", token.ApiTokenName),
	)
	go s.sendTokenStateEmail(context.Background(), token, "BLOCKED")
	return nil
}

// RecoverToNormal은 WARNING 상태 토큰을 NORMAL로 복구한다.
// WARNING TTL 만료 후 rateLimitTracker가 호출한다.
func (s *TokenStateService) RecoverToNormal(ctx context.Context, token *model.ApiToken) error {
	token.State = model.StateNormal
	if err := s.repo.Save(ctx, token); err != nil {
		return err
	}
	return s.cache.Evict(ctx, cache.ApiTokenKey(token.ApiTokenUUID))
}

func (s *TokenStateService) sendTokenStateEmail(ctx context.Context, token *model.ApiToken, state string) {
	email, err := s.userRepo.FindEmailByUserID(ctx, token.UserID)
	if err != nil {
		s.logger.Warn("토큰 소유자 이메일 조회 실패",
			zap.Int64("apiTokenId", token.ApiTokenID),
			zap.Error(err),
		)
		return
	}

	var subject, body string
	switch state {
	case "WARNING":
		subject = "[BSSM Developers] API 토큰 과다 사용 경고"
		body = "안녕하세요.\n\nAPI 토큰 '" + token.ApiTokenName + "'이 과도한 요청으로 WARNING 상태로 전환되었습니다.\n\n요청량을 줄이지 않으면 토큰이 차단될 수 있습니다.\n\n감사합니다."
	case "BLOCKED":
		subject = "[BSSM Developers] API 토큰 차단 안내"
		body = "안녕하세요.\n\nAPI 토큰 '" + token.ApiTokenName + "'이 차단되었습니다.\n\n차단 해제를 원하시면 관리자에게 해제 요청을 제출해 주세요.\n\n감사합니다."
	}

	s.mail.Send(ctx, email, subject, body)
}
