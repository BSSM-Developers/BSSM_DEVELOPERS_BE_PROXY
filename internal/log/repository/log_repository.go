package logrepository

import (
	"context"

	logmodel "github.com/BSSM-Developers/BSSM_DEVELOPERS_BE_PROXY/internal/log/model"
	"go.mongodb.org/mongo-driver/mongo"
)

// LogRepository는 프록시 로그 저장 계약이다.
type LogRepository interface {
	Save(ctx context.Context, log *logmodel.ProxyLog) error
}

type mongoLogRepository struct {
	collection *mongo.Collection
}

func NewLogRepository(db *mongo.Database) LogRepository {
	return &mongoLogRepository{
		collection: db.Collection("proxy_logs"),
	}
}

func (r *mongoLogRepository) Save(ctx context.Context, log *logmodel.ProxyLog) error {
	_, err := r.collection.InsertOne(ctx, log)
	return err
}
