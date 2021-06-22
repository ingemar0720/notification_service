package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github/ingemar0720/xendit/database"
	"log"
	"net/http"
	"net/url"
	"strings"

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
	IdempotencyKey string `json:"idempotency_key"`
}

type NotificationMsg struct {
	Token          string                  `json:"token"`
	IdempotencyKey string                  `json:"idempotency_key"`
	Details        database.PaymentDetails `json:"details"`
}

type NotificationSrv struct {
	DB  *sqlx.DB
	Ctx context.Context
}

type MockPaymentRequest struct {
	CustomerID uint64                  `json:"customer_id"`
	Details    database.PaymentDetails `json:"details"`
}

type ResendRequest struct {
	CustomerID     int64  `json:"customer_id"`
	Token          string `json:"token"`
	SecretKey      string `json:"secret_key"`
	IdempotencyKey string `json:"idempotency_key"`
}

func sendNotificationWithRetry(url string, body string, token string) error {
	err := backoff.Retry(func() error {
		req, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(body)))
		if err != nil {
			log.Println("compose notification request fail, error ", err)
			return errors.Wrapf(err, "compose notification request fail")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
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
	reqToken := r.Header.Get("Authorization")
	splitToken := strings.Split(reqToken, "Bearer ")
	reqToken = splitToken[1]
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
	if !snr.IsTest {
		if err := database.SetupNotification(srv.Ctx, reqToken, u.String(), uint64(snr.CustomerID), srv.DB); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		uuid := uuid.New()
		snresp := SetupNotificationResponse{
			IdempotencyKey: uuid.String(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(snresp)
		return
	} else {
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
		go sendNotificationWithRetry(u.String(), string(bodyBytes), reqToken)
	}
}

func (srv *NotificationSrv) NotifyCustomer(customerID uint64, details database.PaymentDetails) {
	url, token, err := database.GetNotificationURLAndToken(srv.Ctx, customerID, srv.DB)
	if err != nil {
		log.Println("fail to query notification url and token, error ", err)
	}
	idempotencyKey := uuid.New().String()
	database.SaveNotification(srv.Ctx, idempotencyKey, customerID, details, srv.DB)

	msg := NotificationMsg{
		IdempotencyKey: idempotencyKey,
		Token:          token,
		Details:        details,
	}
	bodyBytes, err := json.Marshal(msg)
	if err != nil {
		log.Println("fail to notify customer, error ", err)
	}
	go func() {
		if err := sendNotificationWithRetry(url, string(bodyBytes), token); err == nil {
			database.MarkUpdated(srv.Ctx, idempotencyKey, true, srv.DB)
		}
	}()
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

func (srv *NotificationSrv) ResendNotification(idempotencyKey string, customerID uint64) {
	details, err := database.GetNotification(srv.Ctx, idempotencyKey, srv.DB)
	if err != nil {
		log.Println("fail to query notificiation detail, error ", err)
	}
	url, token, err := database.GetNotificationURLAndToken(srv.Ctx, customerID, srv.DB)
	if err != nil {
		log.Println("fail to query notificiation url and token", err)
	}
	msg := NotificationMsg{
		IdempotencyKey: idempotencyKey,
		Token:          token,
		Details:        details,
	}
	bodyBytes, err := json.Marshal(msg)
	if err != nil {
		log.Println("fail to marshal notification msg, error ", err)
	}
	go func() {
		if err := sendNotificationWithRetry(url, string(bodyBytes), token); err == nil {
			database.MarkUpdated(srv.Ctx, idempotencyKey, true, srv.DB)
		}
	}()
}

func (srv *NotificationSrv) ResendHandler(w http.ResponseWriter, r *http.Request) {
	var request ResendRequest

	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusAccepted)
	srv.ResendNotification(request.IdempotencyKey, uint64(request.CustomerID))
}
