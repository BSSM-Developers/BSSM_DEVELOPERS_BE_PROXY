package repository

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"gorm.io/gorm"
)

type gormDomainRepository struct {
	db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) DomainRepository {
	return &gormDomainRepository{db: db}
}

func (r *gormDomainRepository) FindAllByTokenID(ctx context.Context, tokenID int64) ([]model.TokenDomain, error) {
	var domains []model.TokenDomain
	result := r.db.WithContext(ctx).Where("api_token_id = ?", tokenID).Find(&domains)
	return domains, result.Error
}
