package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=localhost port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	queries := []string{
		"ALTER TABLE comments ADD COLUMN IF NOT EXISTS target_type VARCHAR(20) DEFAULT ''",
		"ALTER TABLE comments ADD COLUMN IF NOT EXISTS target_id BIGINT DEFAULT 0",
		"ALTER TABLE comments ADD COLUMN IF NOT EXISTS content TEXT DEFAULT ''",
		"CREATE INDEX IF NOT EXISTS idx_comments_target ON comments(target_type, target_id)",
	}

	for _, q := range queries {
		result := db.Exec(q)
		if result.Error != nil {
			fmt.Printf("ERR: %s => %v\n", q, result.Error)
		} else {
			fmt.Printf("OK: %s\n", q)
		}
	}

	rows, _ := db.Raw("SELECT column_name FROM information_schema.columns WHERE table_name = 'comments' ORDER BY ordinal_position").Rows()
	defer rows.Close()
	fmt.Println("\nComments columns:")
	for rows.Next() {
		var col string
		rows.Scan(&col)
		fmt.Printf("  %s\n", col)
	}
}
