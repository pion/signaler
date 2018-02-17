package main

import (
	"log"
	"net/http"
)

const httpPort = "80"

func EchoHandler(writer http.ResponseWriter, request *http.Request) {
	log.Println("Echoing back request made to " + request.URL.Path + " to client (" + request.RemoteAddr + ")")
	request.Write(writer)
}

func main() {
	log.Println("starting server, listening on port " + httpPort)

	http.HandleFunc("/", EchoHandler)
	http.ListenAndServe(":"+httpPort, nil)
}
