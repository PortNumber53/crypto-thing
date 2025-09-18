package main

import (
	"embed"
	"log"

	"cryptool/cmd/cryptool/root"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	if err := root.Execute(migrationsFS); err != nil {
		log.Fatal(err)
	}
}
