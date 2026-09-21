// seed_admin is a LOCAL-ONLY bootstrap tool (SP-25 FR-12 低-38): it resets the
// demo admin credentials on a fresh local database. It must never run against
// a shared environment — the whole point of the env-gated password is that no
// well-known credential ever ships with the repo.
package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
)

func main() {
	// 低-38：众所周知种子口令改 env 必填——缺省直接拒绝运行，杜绝
	// "Admin123456 字面量口令被部署到任何环境"的残余面。
	adminPassword := os.Getenv("OMNICRAFT_SEED_ADMIN_PASSWORD")
	secondPassword := os.Getenv("OMNICRAFT_SEED_ADMIN2_PASSWORD")
	if adminPassword == "" {
		log.Fatal("OMNICRAFT_SEED_ADMIN_PASSWORD is required (local bootstrap only; pass a one-off value, never a shared production credential)")
	}
	if secondPassword == "" {
		secondPassword = adminPassword
	}

	dsn := "host=localhost port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
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

	hash2, err := bcrypt.GenerateFromPassword([]byte(secondPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash second password: %v", err)
	}
	// Also create a second local admin test user for safety
	admin2 := model.User{
		Username:        "admintest",
		Email:           "admintest@omnicraft.com",
		PasswordHash:    string(hash2),
		Role:            "admin",
		Reputation:      999,
		PreferredLocale: "zh-CN",
	}
	result2 := db.Where("email = ?", admin2.Email).Assign(admin2).FirstOrCreate(&admin2)
	if result2.Error != nil {
		log.Fatalf("Failed to create admin2: %v", result2.Error)
	}
	fmt.Printf("Admin2: id=%d, rows=%d\n", admin2.ID, result2.RowsAffected)
}
