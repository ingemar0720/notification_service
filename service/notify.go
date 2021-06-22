package service

import (
	"bytes"
	"encoding/json"
	"github/ingemar0720/xendit/database"
	"log"
	"net/http"
	"net/url"

	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

type SetupNotificationRequest struct {
	IsTest     bool   `json:"is_test"`
	SecretKey  string `json:"secret_key"`
	CustomerID int64  `json:"customer_id"`
	URL        string `json:"url"`
	Token      string `json:"token"`
}

type SetupNotificationResponse struct {
	Token string `json:"idepotency_key"`
}

// type NotificationResponse struct {
// 	IdempotencyKey string  `json:"idempotency_key"`
// 	ReferenceID    string  `json:"reference_id"`
// 	ChannelCode    string  `json:"channel_code"`
// 	CustomerName   string  `json:"customer_name"`
// 	Amount         float64 `json:"amount"`
// 	Currency       string  `json:"currency"`
// 	Market         string  `json:"market"`
// }

type NotificationSrv struct {
	DB *sqlx.DB
}

func sendNotificationWithRetry(url string, body string) {
	err := backoff.Retry(func() error {
		resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(body)))
		if err != nil {
			return errors.Wrapf(err, "fail to send notification, error %v, resp %v", err, resp)
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
	if err != nil {
		log.Println("http post request fail for 5 times, error ", err)
	}
}

func (srv *NotificationSrv) NotificationHandler(w http.ResponseWriter, r *http.Request) {
	var snr SetupNotificationRequest
	err := json.NewDecoder(r.Body).Decode(&snr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	u, err := url.ParseRequestURI(snr.URL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// setup notification table
	if !snr.IsTest {
		// setupNotification
		uuid := uuid.New()
		if err := database.SetupNotification(uuid.String(), u.String(), uint64(snr.CustomerID), srv.DB); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		snresp := SetupNotificationResponse{
			Token: uuid.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(snresp)
		return
	} else {
		// send notification request
		mockDetails := database.NotificationDetails{
			ReferenceID: "test_reference_id",
			ChannelCode: "test_channel_code",
			Amount:      100000,
			Currency:    "SGD",
			Market:      "Singapore",
		}
		body := struct {
			IdempotencyKey string                       `json:"idepotency_key" db:"idepotency_key"`
			Details        database.NotificationDetails `json:"details" db:"details"`
		}{
			IdempotencyKey: "test_idempotency_key",
			Details:        mockDetails,
		}
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusAccepted)
		sendNotificationWithRetry(u.String(), string(bodyBytes))
	}
}

func (srv *NotificationSrv) NotifyCustomer() {
	// generate key and form the request
	// save into notification db
	// send request with retry
}
