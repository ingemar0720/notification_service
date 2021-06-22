package database

import (
	"context"
	"encoding/json"
	"fmt"

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

func New() (*sqlx.DB, error) {
	db, err := sqlx.Connect("postgres", "host=db user=user dbname=postgres password=mysecretpassword sslmode=disable")
	if err != nil {
		return &sqlx.DB{}, fmt.Errorf("fail to connect to db, error: %v", err)
	}
	return db, nil
}

func SetupNotification(ctx context.Context, Token, URL string, customerID uint64, db *sqlx.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "fail to setup notification")
	}
	// assume customerID has exist
	_, err = tx.ExecContext(ctx, "Update customers SET token=$1, notification_url=$2 WHERE id=$3", Token, URL, customerID)
	if err != nil {
		err = fmt.Errorf("fail to setup notification, error %v", err)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback setupNotification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}

func SaveNotification(ctx context.Context, idempotencyKey string, customerID uint64, details PaymentDetails, db *sqlx.DB) error {
	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return errors.Wrapf(err, "fail to marshal notification detail")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "fail to save notifications")
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO notifications (customer_id, idempotency_key, details) VALUES ($1, $2, $3)", customerID, idempotencyKey, types.JSONText(detailsBytes))
	if err != nil {
		err = fmt.Errorf("fail to save notifications, error %v, customerID %v, idempotency_key %v, detailsBytes %v", err, customerID, idempotencyKey, details)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to rollback saveNotification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}

func GetNotificationURLAndToken(ctx context.Context, customerID uint64, db *sqlx.DB) (string, string, error) {
	var token string
	var url string
	rows, err := db.QueryContext(ctx, "SELECT notification_url, token from customers where id=$1", customerID)
	if err != nil {
		return "", "", errors.Wrapf(err, "fail to query notification url and user token")
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&url, &token)
		if err != nil {
			return "", "", errors.Wrapf(err, "fail to query notification url and user token")
		}
	}
	return url, token, nil
}

func GetNotification(ctx context.Context, idempotencyKey string, db *sqlx.DB) (PaymentDetails, error) {
	var detailBytes []byte
	var detail PaymentDetails
	rows, err := db.QueryContext(ctx, "SELECT details from notifications where idempotency_key=$1", idempotencyKey)
	if err != nil {
		return PaymentDetails{}, errors.Wrapf(err, "fail to query notification detail")
	}
	defer rows.Close()
	for rows.Next() {
		err := rows.Scan(&detailBytes)
		if err != nil {
			return PaymentDetails{}, errors.Wrapf(err, "fail to query notification detail")
		}
	}
	json.Unmarshal(detailBytes, &detail)
	return detail, nil
}

func MarkUpdated(ctx context.Context, idempotencyKey string, notified bool, db *sqlx.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "fail to mark notified notification")
	}
	// assume customerID has exist
	_, err = tx.ExecContext(ctx, "Update notifications SET notified=$1 WHERE idempotency_key=$2", notified, idempotencyKey)
	if err != nil {
		err = fmt.Errorf("fail to mark notified notification, error %v", err)
		if err1 := tx.Rollback(); err1 != nil {
			return errors.Wrapf(err1, "fail to mark notified notification, error %v", err1)
		}
		return err
	}
	return tx.Commit()
}
