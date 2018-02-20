package main

import (
	"log"
	"net/http"
	"os"

	"gitlab.com/pions/pion/signaler/api"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		panic("PORT is a requred enviroment variable")
	}

	http.HandleFunc("/", api.HandleRoot)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, nil))
}
