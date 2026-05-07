package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
)

func main() {
	dsn := "host=localhost port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("Admin123456"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// Force update the admin user
	result := db.Model(&model.User{}).Where("email = ?", "admin@omnicraft.com").
		Updates(map[string]interface{}{
			"password_hash": string(hash),
			"role":          "admin",
		})
	if result.Error != nil {
		log.Fatalf("Failed to update admin user: %v", result.Error)
	}

	fmt.Printf("Admin user updated: rows=%d\n", result.RowsAffected)

	// Also create a second admin test user for safety
	admin2 := model.User{
		Username:       "admintest",
		Email:          "admintest@omnicraft.com",
		PasswordHash:   string(hash),
		Role:           "admin",
		Reputation:     999,
		PreferredLocale: "zh-CN",
	}
	result2 := db.Where("email = ?", admin2.Email).Assign(admin2).FirstOrCreate(&admin2)
	if result2.Error != nil {
		log.Fatalf("Failed to create admin2: %v", result2.Error)
	}
	fmt.Printf("Admin2: id=%d, rows=%d\n", admin2.ID, result2.RowsAffected)
}
