package database

import "time"

type CustomerModel struct {
	ID              int       `json:"id" db:"id"`
	Name            string    `json:"name" db:"name"`
	NotificationURL string    `json:"notification_url" db:"notification_url"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}
