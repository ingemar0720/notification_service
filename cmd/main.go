package main

import (
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
	notificationSvc := service.NotificationSrv{DB: db}
	r := chi.NewRouter()
	r.Post("/notifications", notificationSvc.NotificationHandler)
	log.Fatal(http.ListenAndServe(":5000", r))

}