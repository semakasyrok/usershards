package models

import "time"

type User struct {
	ID        int64     `json:"id"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Balance   int64     `json:"balance"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
