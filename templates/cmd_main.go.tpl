package main

// file is autogenerated, do not modify here, see
// generator and template: templates/cmd_main.go.tpl

import (
	"log"
	"context"

	"net/http"

	_ "github.com/go-sql-driver/mysql"

	"${MODULE}/rpc/${SERVICE}"
	server "${MODULE}/server/${SERVICE}"
)

func main() {
	ctx := context.TODO()

	srv, err := server.New(ctx)
	if err != nil {
		log.Fatalf("Error in service.New(): %+v", err)
	}

	twirpHandler := ${SERVICE}.New${SERVICE_CAMEL}ServiceServer(srv, nil)

	http.ListenAndServe(":3000", twirpHandler)
}
