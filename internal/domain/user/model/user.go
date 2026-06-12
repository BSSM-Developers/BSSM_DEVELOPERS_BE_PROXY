package model

// User holds only the fields needed for email lookup.
type User struct {
	UserID int64  `gorm:"column:user_id;primaryKey"`
	Email  string `gorm:"column:email"`
}

func (User) TableName() string { return "user" }
