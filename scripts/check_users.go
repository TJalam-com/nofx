//go:build ignore
// +build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"nofx/config"
)

func main() {
	dbPath := flag.String("db", "data/data.db", "Database path")
	flag.Parse()

	// Open database
	log.Printf("📋 Opening database: %s", *dbPath)
	database, err := config.NewDatabase(*dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Get all users
	users, err := database.GetAllUsersWithRoles()
	if err != nil {
		log.Fatalf("Failed to get users: %v", err)
	}

	// Display results
	if len(users) == 0 {
		log.Println("❌ No users found in the database")
		return
	}

	log.Printf("✅ Found %d user(s) in the database:\n", len(users))
	separator := "=================================================================================="
	fmt.Println("\n" + separator)
	fmt.Printf("%-20s %-30s %-15s %-10s %-20s\n", "ID", "Email", "Role", "OTP Verified", "Created At")
	fmt.Println(separator)

	for _, user := range users {
		otpStatus := "No"
		if user.OTPVerified {
			otpStatus = "Yes"
		}
		fmt.Printf("%-20s %-30s %-15s %-10s %-20s\n",
			user.ID,
			user.Email,
			user.Role,
			otpStatus,
			user.CreatedAt.Format("2006-01-02 15:04:05"),
		)
	}
	fmt.Println(separator)
}

