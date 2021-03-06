package main

import (
	"context"
	"log"
	"net/http"

	"github/ingemar0720/xendit/database"
	"github/ingemar0720/xendit/service"

	"github.com/go-chi/chi/v5"
	"github.com/pkg/errors"
)

func main() {
	db, err := database.New()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "fail to init a DB instance"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notificationSvc := service.NotificationSrv{DB: db, Ctx: ctx}
	r := chi.NewRouter()
	r.Post("/notifications", notificationSvc.NotificationHandler)
	r.Post("/notifications/resend", notificationSvc.ResendHandler)
	r.Post("/payments", notificationSvc.MockPaymentHandler)
	log.Fatal(http.ListenAndServe(":5000", r))

}
