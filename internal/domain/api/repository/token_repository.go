package repository

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"gorm.io/gorm"
)

type gormTokenRepository struct {
	db *gorm.DB
}

func NewTokenRepository(db *gorm.DB) TokenRepository {
	return &gormTokenRepository{db: db}
}

func (r *gormTokenRepository) FindByClientID(ctx context.Context, clientID string) (*model.ApiToken, error) {
	var token model.ApiToken
	result := r.db.WithContext(ctx).
		Where("api_token_uuid = ?", clientID).
		First(&token)
	if result.Error != nil {
		return nil, result.Error
	}
	return &token, nil
}

func (r *gormTokenRepository) FindByID(ctx context.Context, id int64) (*model.ApiToken, error) {
	var token model.ApiToken
	result := r.db.WithContext(ctx).First(&token, id)
	if result.Error != nil {
		return nil, result.Error
	}
	return &token, nil
}

func (r *gormTokenRepository) Save(ctx context.Context, token *model.ApiToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}
