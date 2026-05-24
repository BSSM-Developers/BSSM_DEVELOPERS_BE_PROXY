package model

// ApiUsage는 api_usage 테이블과 api 테이블 JOIN 결과를 담는 모델이다.
// domain, method는 api 테이블에서 조인하여 채워진다.
type ApiUsage struct {
	ApiTokenID     int64  `gorm:"column:api_token_id" json:"apiTokenId"`
	ApiID          string `gorm:"column:api_id"       json:"apiId"`
	ApiUseReasonID string `gorm:"column:api_use_reason_id" json:"apiUseReasonId"`
	Name           string `gorm:"column:name"         json:"name"`
	Endpoint       string `gorm:"column:endpoint"     json:"endpoint"`
	Domain         string `gorm:"column:domain"       json:"domain"`
	Method         string `gorm:"column:method"       json:"method"`
}
