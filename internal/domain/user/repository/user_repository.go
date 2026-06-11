package repository

import (
	"context"

	usermodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/domain/user/model"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) FindEmailByUserID(ctx context.Context, userID int64) (string, error) {
	var user usermodel.User
	err := r.db.WithContext(ctx).
		Select("user_id, email").
		First(&user, "user_id = ?", userID).Error
	if err != nil {
		return "", err
	}
	return user.Email, nil
}
