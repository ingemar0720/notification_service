package main

import (
	"fmt"
	"github/ingemar0720/xendit/database"
	"log"

	"github.com/pkg/errors"
)

func main() {
	db, err := database.New()
	if err != nil {
		log.Fatal(errors.Wrapf(err, "fail to init a DB instance"))
	}
	tx, err := db.Beginx()
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		_, err = tx.Exec("INSERT INTO customers (name) VALUES ($1)", fmt.Sprintf("customer %v", i))
		if err != nil {
			tx.Rollback()
			log.Fatal(err)
		}
	}
	tx.Commit()
}
