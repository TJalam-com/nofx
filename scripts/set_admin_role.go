//go:build ignore
// +build ignore

package main

import (
	"flag"
	"log"
	"nofx/config"
)

func main() {
	email := flag.String("email", "", "User email to set as admin")
	dbPath := flag.String("db", "config.db", "Database path")
	flag.Parse()

	if *email == "" {
		log.Fatal("Error: email is required. Use -email flag")
	}

	// Open database
	log.Printf("📋 Opening database: %s", *dbPath)
	database, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Get user by email
	user, err := database.GetUserByEmail(*email)
	if err != nil {
		log.Fatalf("Failed to find user with email %s: %v", *email, err)
	}

	log.Printf("Found user: ID=%s, Email=%s, Current Role=%s", user.ID, user.Email, user.Role)

	// Update role to admin
	err = database.UpdateUserRole(user.ID, "admin")
	if err != nil {
		log.Fatalf("Failed to update user role: %v", err)
	}

	log.Printf("✅ Successfully updated user %s (%s) to admin role", user.Email, user.ID)
}
