package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatalf("Usage: go run main.go <content_id> <author_id> <body_text>")
	}

	contentID, _ := strconv.ParseInt(os.Args[1], 10, 64)
	authorID, _ := strconv.ParseInt(os.Args[2], 10, 64)
	bodyText := os.Args[3]

	dsn := "host=localhost port=5432 user=omnicraft password=omnicraft dbname=omnicraft sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Check if version already exists
	var count int64
	db.Model(&model.ContentVersion{}).Where("content_item_id = ?", contentID).Count(&count)
	if count > 0 {
		fmt.Printf("Versions already exist for content %d (count: %d)\n", contentID, count)
		return
	}

	version := model.ContentVersion{
		ContentItemID: contentID,
		AuthorID:      authorID,
		VersionNumber: 1,
		StorageType:   "full",
		StorageKey:    bodyText,
		Status:        "active",
		IsLatest:      true,
	}

	if err := db.Create(&version).Error; err != nil {
		log.Fatalf("Failed to create version: %v", err)
	}

	fmt.Printf("Created initial version: id=%d, content_id=%d, ver_num=%d\n",
		version.ID, version.ContentItemID, version.VersionNumber)
}
