package main

import (
	"log"

	"github.com/titpetric/microservice/db"
)

func main() {
	log.Printf("Migration projects: %+v", db.List())
	log.Println("Migration statements for stats")
	if err := db.Print("stats"); err != nil {
		log.Printf("An error occured: %+v", err)
	}
}
