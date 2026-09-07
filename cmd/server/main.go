package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("TaiSang-KB starting...")
	if err := http.ListenAndServe("127.0.0.1:8080", nil); err != nil {
		log.Fatal(err)
	}
}