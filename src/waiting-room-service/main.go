package main

import (
	"log"

	"jeeves/waiting-room-service/internal/core/factory"
)

func main() {
	if err := factory.New().Run(); err != nil {
		log.Fatal(err)
	}
}
