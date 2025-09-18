package main

import (
	"log"

	"cryptool/cmd/cryptool/root"
)

func main() {
	if err := root.Execute(); err != nil {
		log.Fatal(err)
	}
}
