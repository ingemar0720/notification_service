package database

import (
	"context"
	"encoding/json"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/jmoiron/sqlx/types"
)

func TestSetupNotification(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	fixtureToken := "test_token"
	fixtureURL := "test_url"
	fixtureCustomerID := 1

	tests := []struct {
		name            string
		givenToken      string
		givenURL        string
		givenCustomerID uint64
		wantErr         bool
	}{
		{
			name:            "update successfully",
			givenToken:      fixtureToken,
			givenURL:        fixtureURL,
			givenCustomerID: uint64(fixtureCustomerID),
			wantErr:         false,
		},
		{
			name:            "update fail and rollback",
			givenToken:      "not exist",
			givenURL:        fixtureURL,
			givenCustomerID: uint64(fixtureCustomerID),
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.ExpectBegin()
			mock.ExpectExec("Update customers").WithArgs(fixtureToken, fixtureURL, fixtureCustomerID).WillReturnResult(sqlmock.NewResult(1, 1))
			if !tt.wantErr {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}

			if err := SetupNotification(context.Background(), tt.givenToken, tt.givenURL, tt.givenCustomerID, sqlx.NewDb(db, "sqlmock")); (err != nil) != tt.wantErr {
				t.Errorf("SetupNotification() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveNotification(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()
	fixtureDetail := PaymentDetails{
		ReferenceID: "mock_reference_id",
		ChannelCode: "mock_channel_code",
		Amount:      100000.00,
		Currency:    "SGD",
		Market:      "Singapore",
	}
	fixtureidempotencyKey := "mock_idempotency_key"
	fixtureCustomerID := 1
	tests := []struct {
		name                string
		givenCustomerID     uint64
		givenIdempotencyKey string
		givenDetails        PaymentDetails
		wantErr             bool
	}{
		// TODO: Add test cases.
		{
			name:                "insert notification successfully",
			givenCustomerID:     uint64(fixtureCustomerID),
			givenIdempotencyKey: fixtureidempotencyKey,
			givenDetails:        fixtureDetail,
			wantErr:             false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock.ExpectBegin()
			detailsBytes, err := json.Marshal(tt.givenDetails)
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectExec("INSERT INTO notifications").WithArgs(tt.givenCustomerID, tt.givenIdempotencyKey, types.JSONText(detailsBytes)).WillReturnResult(sqlmock.NewResult(1, 1))
			if !tt.wantErr {
				mock.ExpectCommit()
			} else {
				mock.ExpectRollback()
			}
			if err := SaveNotification(context.Background(), tt.givenIdempotencyKey, tt.givenCustomerID, tt.givenDetails, sqlx.NewDb(db, "sqlmock")); (err != nil) != tt.wantErr {
				t.Errorf("SaveNotification() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
