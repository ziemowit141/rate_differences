package main

import (
	"flag"
	"log"

	"rate_differences/backend"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	flag.Parse()
	if err := backend.StartServer(*addr); err != nil {
		log.Fatal(err)
	}
}
