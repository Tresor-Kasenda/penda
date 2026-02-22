package main

import (
	"log"

	"penda/framework/orm"
)

func main() {
	db, err := orm.Open(orm.Config{
		Dialector: "sqlite",
		DSN:       "file:rest-api.db?_foreign_keys=on",
	})
	if err != nil {
		log.Fatal(err)
	}

	server, err := BuildApp(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("rest-api example listening on :8083")
	log.Fatal(server.Run(":8083"))
}
