package database

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
	_ "github.com/lib/pq"
	"github.com/pkg/errors"
)

type PaymentDetails struct {
	ReferenceID string  `json:"reference_id" db:"reference_id"`
	ChannelCode string  `json:"channel_code" db:"channel_code"`
	Amount      float64 `json:"amount" db:"amount"`
	Currency    string  `json:"currency" db:"currency"`
	Market      string  `json:"market" db:"market"`
}

type NotificationModel struct {
	ID             int            `json:"id" db:"id"`
	CustomerID     uint64         `json:"customer_id" db:"customer_id"`
	IdempotencyKey string         `json:"idepotency_key" db:"idepotency_key"`
	Details        PaymentDetails `json:"details" db:"details"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
}

func New() (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", "host=db user=user dbname=postgres password=mysecretpassword sslmode=disable")
	if err != nil {
		return &sqlx.DB{}, fmt.Errorf("fail to connect to db, error: %v", err)
	}
	return db, nil
}

func SetupNotification(Token, URL string, customerID uint64, db *sqlx.DB) error {
	tx, err := db.Beginx()
	if err != nil {
		return errors.Wrapf(err, "fail to setup notification")
	}
	// assume customerID has exist
	_, err = tx.Exec("Update customers SET token=$1, notification_url=$2 WHERE id=$3", Token, URL, customerID)
	if err != nil {
		err = fmt.Errorf("fail to setup notification, error %v", err)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback setupNotification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}

func SaveNotification(idempotencyKey string, customerID uint64, details PaymentDetails, db *sqlx.DB) error {
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return errors.Wrapf(err, "fail to marshal notification detail")
	}
	tx, err := db.Beginx()
	if err != nil {
		return errors.Wrapf(err, "fail to save notifications")
	}
	_, err = tx.Exec("INSERT INTO notifications (customer_id, idempotency_key, details) VALUES ($1, $2, $3)", customerID, idempotencyKey, types.JSONText(detailsBytes))
	if err != nil {
		log.Printf("fail to save notifications, error %v, customerID %v, idempotency_key %v, detailsBytes %v", err, customerID, idempotencyKey, details)
		err = fmt.Errorf("fail to save notifications, error %v, customerID %v, idempotency_key %v, detailsBytes %v", err, customerID, idempotencyKey, details)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback saveNotification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}

func GetNotificationURLAndToken(customerID uint64, db *sqlx.DB) (string, string, error) {
	var token string
	var url string
	rows, err := db.Query("SELECT notification_url, token from customers where id=$1", customerID)
	if err != nil {
		return "", "", errors.Wrapf(err, "fail to query customer_id")
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&url, &token)
		if err != nil {
			return "", "", errors.Wrapf(err, "fail to query customer id, error")
		}
	}
	return url, token, nil
}

func GetNotification(idempotencyKey string, db *sqlx.DB) (PaymentDetails, error) {
	var detailBytes []byte
	var detail PaymentDetails
	rows, err := db.Query("SELECT details from notifications where idempotency_key=$1", idempotencyKey)
	if err != nil {
		return PaymentDetails{}, errors.Wrapf(err, "fail to query notification detail")
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&detailBytes)
		if err != nil {
			return PaymentDetails{}, errors.Wrapf(err, "fail to query customer id, error")
		}
	}
	json.Unmarshal(detailBytes, &detail)
	return detail, nil
}

func MarkUpdated(idempotencyKey string, notified bool, db *sqlx.DB) error {
	tx, err := db.Beginx()
	if err != nil {
		return errors.Wrapf(err, "fail to mark notified notification")
	}
	// assume customerID has exist
	_, err = tx.Exec("Update notifications SET notified=$1 WHERE idempotency_key=$2", notified, idempotencyKey)
	if err != nil {
		err = fmt.Errorf("fail to mark notified notification, error %v", err)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to mark notified notification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}

// func GetAllCustomerIDs(db *sqlx.DB) ([]uint64, error) {
// 	var ids []uint64
// 	rows, err := db.Query("SELECT id from customers")
// 	if err != nil {
// 		return []uint64{}, errors.Wrapf(err, "fail to query all customer id")
// 	}
// 	defer rows.Close()
// 	for rows.Next() {
// 		var id uint64
// 		err := rows.Scan(&id)
// 		if err != nil {
// 			return []uint64{}, errors.Wrapf(err, "fail to query all customer id")
// 		}
// 		ids = append(ids, id)
// 	}
// 	return ids, nil
// }
