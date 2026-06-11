package repository

import (
	"context"

	"github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/api/model"
	"gorm.io/gorm"
)

type gormUsageRepository struct {
	db *gorm.DB
}

func NewUsageRepository(db *gorm.DB) UsageRepository {
	return &gormUsageRepository{db: db}
}

func (r *gormUsageRepository) FindAllByTokenID(ctx context.Context, tokenID int64) ([]model.ApiUsage, error) {
	var usages []model.ApiUsage
	result := r.db.WithContext(ctx).Raw(`
		SELECT au.api_token_id, au.api_group_id, au.api_use_reason_id, au.name, au.endpoint,
		       a.domain, a.method
		FROM api_usage au
		INNER JOIN api a ON au.api_group_id = a.api_group_id AND a.is_current = TRUE
		WHERE au.api_token_id = ?
	`, tokenID).Scan(&usages)
	return usages, result.Error
}
