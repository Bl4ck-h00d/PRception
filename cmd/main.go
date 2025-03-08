package main

import (
	"fmt"
	"log"
	"net/http"
	"prception/api"
	"prception/core/github"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	installationID, err := github.GetInstallationID()
	if err != nil {
		log.Fatalf("Failed to get installation ID: %v", err)
	}

	fmt.Printf("Installation ID: %s\n", installationID)

	token, err := github.GetInstallationToken(installationID)
	if err != nil {
		log.Fatalf("Failed to get installation token: %v", err)
	}

	http.HandleFunc("/webhook", api.HandleWebhook(token))

	port := "8080"
	log.Println("server listening on port: ", port)
	if error := http.ListenAndServe(port, nil); error != nil {
		log.Fatalf("failed to start server: %v", error)
	}
}
