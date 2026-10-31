package main

import (
	"log"

	"jeeves/card-provider-mock/internal/core/factory"
)

func main() {
	if err := factory.New().Run(); err != nil {
		log.Fatal(err)
	}
}
