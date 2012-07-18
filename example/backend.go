package main

import (
	"flag"
	"fmt"
	"net/http"
)

var response = flag.String("saying", "Hi", "the response message")
var port = flag.String("port", "", "port to listen on")

func handler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(*response))
}

func main() {
	flag.Parse()
	http.HandleFunc("/", handler)
	fmt.Printf("Listening on port `%s` saying %q", *port, *response)
	http.ListenAndServe(":"+*port, nil)
}
