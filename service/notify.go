package service

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	IdepotencyKey string `json:"idepotency_key"`
}

type NotificationMsg struct {
	Token         string                  `json:"token"`
	IdepotencyKey string                  `json:"idepotency_key"`
	Details       database.PaymentDetails `json:"details"`
}

type NotificationSrv struct {
	DB *sqlx.DB
}

type MockPaymentRequest struct {
	CustomerID uint64                  `json:"customer_id"`
	Details    database.PaymentDetails `json:"details"`
}

func sendNotificationWithRetry(url string, body string) error {
	err := backoff.Retry(func() error {
		resp, err := http.Post(url, "application/json", bytes.NewBuffer([]byte(body)))
		if err != nil {
			return errors.Wrapf(err, "fail to send notification, error %v, resp %v", err, resp)
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 5))
	if err != nil {
		log.Println("http post request fail for 5 times, error ", err)
		return fmt.Errorf("http post request fail for 5 times, error %v", err)
	}
	return nil
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
			IdepotencyKey: uuid.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(snresp)
		return
	} else {
		// send notification request
		mockDetails := database.PaymentDetails{
			ReferenceID: "test_reference_id",
			ChannelCode: "test_channel_code",
			Amount:      100000,
			Currency:    "SGD",
			Market:      "Singapore",
		}
		body := struct {
			IdempotencyKey string                  `json:"idepotency_key" db:"idepotency_key"`
			Details        database.PaymentDetails `json:"details" db:"details"`
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
		w.WriteHeader(http.StatusAccepted)
		sendNotificationWithRetry(u.String(), string(bodyBytes))
	}
}

func (srv *NotificationSrv) NotifyCustomer(customerID uint64, details database.PaymentDetails) {
	url, token, err := database.GetNotificationURLAndToken(customerID, srv.DB)
	if err != nil {
		log.Println("fail to query notification url and token, error ", err)
	}
	idempotencyKey := uuid.New().String()
	database.SaveNotification(idempotencyKey, customerID, details, srv.DB)

	msg := NotificationMsg{
		IdepotencyKey: idempotencyKey,
		Token:         token,
		Details:       details,
	}
	bodyBytes, err := json.Marshal(msg)
	if err != nil {
		log.Println("fail to notify customer, error ", err)
	}
	if err := sendNotificationWithRetry(url, string(bodyBytes)); err == nil {
		database.MarkUpdated(idempotencyKey, srv.DB)
	}
}

func (srv *NotificationSrv) MockPaymentHandler(w http.ResponseWriter, r *http.Request) {
	var request MockPaymentRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	srv.NotifyCustomer(request.CustomerID, request.Details)
}

// // MockPaymentLoop regularly query customers table, fetch customer whose's notification has been setup and send a mock notification to the customer.
// func (srv *NotificationSrv) MockPaymentLoop(ctx context.Context) error {
// 	for {
// 		log.Println("execuing payment loop")
// 		select {
// 		case <-ctx.Done():
// 			return nil
// 		default:
// 			ids, err := database.GetAllCustomerIDs(srv.DB)
// 			if err != nil {
// 				return errors.Wrapf(err, "fail to send payment notification")
// 			}
// 			for _, id := range ids {
// 				mockDetail := database.PaymentDetails{
// 					ReferenceID: fmt.Sprintf("mock_reference_id_%d_%s", id, time.Now().String()),
// 					ChannelCode: fmt.Sprintln("mock_channel"),
// 					Amount:      randFloats(5.0, 10000.0),
// 					Currency:    "SGD",
// 					Market:      "Singapore",
// 				}
// 				srv.NotifyCustomer(id, mockDetail)
// 			}
// 			time.Sleep(30 * time.Second)
// 		}
// 	}
// }

// func randFloats(min, max float64) float64 {
// 	rand.Seed(time.Now().UnixNano())
// 	return min + rand.Float64()*(max-min)
// }
