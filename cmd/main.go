package main

import (
	"log"
	"tenders/internal/app"
)

func main() {
	app, err := app.NewApp()
	if err != nil {
		log.Fatal(err)
	}

	app.Run()
}
