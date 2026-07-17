package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

const (
	reconcileCleanExitCode = iota
	reconcileDriftExitCode
	reconcileUsageExitCode
)

type reconcileTotals struct {
	LegacyMissingFromDefault  int64 `json:"legacy_missing_from_default"`
	DefaultMissingFromLegacy  int64 `json:"default_missing_from_legacy"`
	DuplicateLogicalItems     int64 `json:"duplicate_logical_items"`
	MissingDefaultCollections int64 `json:"missing_default_collections"`
}

func (t reconcileTotals) HasDrift() bool {
	return t.LegacyMissingFromDefault > 0 ||
		t.DefaultMissingFromLegacy > 0 ||
		t.DuplicateLogicalItems > 0 ||
		t.MissingDefaultCollections > 0
}

type reconcileUserZone struct {
	UserID                    int64  `json:"user_id"`
	Zone                      string `json:"zone"`
	LegacyMissingFromDefault  int64  `json:"legacy_missing_from_default"`
	DefaultMissingFromLegacy  int64  `json:"default_missing_from_legacy"`
	DuplicateLogicalItems     int64  `json:"duplicate_logical_items"`
	MissingDefaultCollections int64  `json:"missing_default_collections"`
}

type reconcileRepairTotals struct {
	DefaultCollectionsCreated int64 `json:"default_collections_created"`
	CollectionItemsInserted   int64 `json:"collection_items_inserted"`
	FavoritesInserted         int64 `json:"favorites_inserted"`
}

type reconcileReport struct {
	Apply       bool                   `json:"apply"`
	Users       []reconcileUserZone    `json:"users"`
	Totals      reconcileTotals        `json:"totals"`
	BeforeApply *reconcileTotals       `json:"before_apply,omitempty"`
	Repairs     *reconcileRepairTotals `json:"repairs,omitempty"`
}

type reconcileOptions struct {
	Apply                      bool
	MaintenanceWindowConfirmed bool
}

func main() {
	db, err := gorm.Open(postgres.Open(loadCollectionReconcileDSN()), &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "collection reconciliation database connection failed")
		os.Exit(reconcileUsageExitCode)
	}
	os.Exit(executeCollectionReconcile(os.Args[1:], db, os.Stdout, os.Stderr))
}

func loadCollectionReconcileDSN() string {
	if explicitDSN, exists := os.LookupEnv("DB_DSN"); exists {
		return explicitDSN
	}
	return config.Load().Database.DSN
}

func executeCollectionReconcile(args []string, db *gorm.DB, stdout, stderr io.Writer) int {
	options, ok := parseReconcileArgs(args, stderr)
	if !ok {
		return reconcileUsageExitCode
	}
	if db == nil {
		fmt.Fprintln(stderr, "collection reconciliation requires a database")
		return reconcileUsageExitCode
	}

	report, err := inspectCollectionDrift(db)
	if err != nil {
		fmt.Fprintf(stderr, "collection reconciliation inspection failed: %v\n", err)
		return reconcileUsageExitCode
	}
	report.Apply = options.Apply

	if options.Apply {
		before := report.Totals
		repairs, repairErr := repairCollectionDrift(db)
		if repairErr != nil {
			fmt.Fprintf(stderr, "collection reconciliation apply failed: %v\n", repairErr)
			return reconcileUsageExitCode
		}
		report, err = inspectCollectionDrift(db)
		if err != nil {
			fmt.Fprintf(stderr, "collection reconciliation post-apply inspection failed: %v\n", err)
			return reconcileUsageExitCode
		}
		report.Apply = true
		report.BeforeApply = &before
		report.Repairs = &repairs
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(stderr, "collection reconciliation output failed: %v\n", err)
		return reconcileUsageExitCode
	}
	if report.Totals.HasDrift() {
		return reconcileDriftExitCode
	}
	return reconcileCleanExitCode
}

func parseReconcileArgs(args []string, stderr io.Writer) (reconcileOptions, bool) {
	options := reconcileOptions{}
	for _, arg := range args {
		switch arg {
		case "--apply":
			options.Apply = true
		case "--maintenance-window-confirmed":
			options.MaintenanceWindowConfirmed = true
		default:
			fmt.Fprintf(stderr, "unsupported argument: %s\n", arg)
			return reconcileOptions{}, false
		}
	}
	if options.Apply && !options.MaintenanceWindowConfirmed {
		fmt.Fprintln(stderr, "--apply requires maintenance window confirmation: stop all application writers, then pass --maintenance-window-confirmed")
		return reconcileOptions{}, false
	}
	if options.MaintenanceWindowConfirmed && !options.Apply {
		fmt.Fprintln(stderr, "--maintenance-window-confirmed is only valid with --apply")
		return reconcileOptions{}, false
	}
	return options, true
}

func inspectCollectionDrift(db *gorm.DB) (reconcileReport, error) {
	users, err := activeReconcileUserIDs(db)
	if err != nil {
		return reconcileReport{}, err
	}
	report := reconcileReport{Users: make([]reconcileUserZone, 0, len(users)*2)}
	for _, userID := range users {
		for _, zone := range []string{"original", "fanwork"} {
			row, inspectErr := inspectUserZone(db, userID, zone)
			if inspectErr != nil {
				return reconcileReport{}, inspectErr
			}
			report.Users = append(report.Users, row)
			report.Totals.LegacyMissingFromDefault += row.LegacyMissingFromDefault
			report.Totals.DefaultMissingFromLegacy += row.DefaultMissingFromLegacy
			report.Totals.DuplicateLogicalItems += row.DuplicateLogicalItems
			report.Totals.MissingDefaultCollections += row.MissingDefaultCollections
		}
	}
	return report, nil
}

func inspectUserZone(db *gorm.DB, userID int64, zone string) (reconcileUserZone, error) {
	row := reconcileUserZone{UserID: userID, Zone: zone}
	defaults, err := loadDefaultCollections(db, userID, zone)
	if err != nil {
		return row, err
	}
	if len(defaults) == 0 {
		row.MissingDefaultCollections = 1
	}
	legacy, err := loadLegacyFavoriteSet(db, userID, zone)
	if err != nil {
		return row, err
	}
	defaultCounts, err := loadDefaultContentCounts(db, defaults)
	if err != nil {
		return row, err
	}
	defaultSet := make(map[int64]struct{}, len(defaultCounts))
	for contentID, count := range defaultCounts {
		defaultSet[contentID] = struct{}{}
		if count > 1 {
			row.DuplicateLogicalItems += count - 1
		}
	}
	row.LegacyMissingFromDefault = countSetDifference(legacy, defaultSet)
	row.DefaultMissingFromLegacy = countSetDifference(defaultSet, legacy)
	return row, nil
}

func repairCollectionDrift(db *gorm.DB) (reconcileRepairTotals, error) {
	repairs := reconcileRepairTotals{}
	err := db.Transaction(func(tx *gorm.DB) error {
		users, err := activeReconcileUserIDs(tx)
		if err != nil {
			return err
		}
		repo := repository.NewCollectionRepository(tx)
		for _, userID := range users {
			for _, zone := range []string{"original", "fanwork"} {
				defaults, loadErr := loadDefaultCollections(tx, userID, zone)
				if loadErr != nil {
					return loadErr
				}
				if len(defaults) == 0 {
					created, ensureErr := repo.EnsureDefaultCollection(tx.Statement.Context, userID, zone)
					if ensureErr != nil {
						return ensureErr
					}
					defaults = []model.Collection{*created}
					repairs.DefaultCollectionsCreated++
				}

				legacy, legacyErr := loadLegacyFavoriteSet(tx, userID, zone)
				if legacyErr != nil {
					return legacyErr
				}
				defaultCounts, defaultErr := loadDefaultContentCounts(tx, defaults)
				if defaultErr != nil {
					return defaultErr
				}
				defaultSet := make(map[int64]struct{}, len(defaultCounts))
				for contentID := range defaultCounts {
					defaultSet[contentID] = struct{}{}
				}

				for contentID := range legacy {
					if _, exists := defaultSet[contentID]; exists {
						continue
					}
					result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CollectionItem{
						CollectionID:  defaults[0].ID,
						ContentItemID: contentID,
					})
					if result.Error != nil {
						return result.Error
					}
					repairs.CollectionItemsInserted += result.RowsAffected
				}
				for contentID := range defaultSet {
					if _, exists := legacy[contentID]; exists {
						continue
					}
					result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.Favorite{
						UserID:        userID,
						ContentItemID: contentID,
					})
					if result.Error != nil {
						return result.Error
					}
					repairs.FavoritesInserted += result.RowsAffected
				}
			}
		}
		return nil
	})
	return repairs, err
}

func activeReconcileUserIDs(db *gorm.DB) ([]int64, error) {
	var userIDs []int64
	err := db.Model(&model.User{}).
		Where("deleted_at IS NULL").
		Order("id ASC").
		Pluck("id", &userIDs).Error
	return userIDs, err
}

func loadDefaultCollections(db *gorm.DB, userID int64, zone string) ([]model.Collection, error) {
	var collections []model.Collection
	err := db.Where("user_id = ? AND zone = ? AND is_default = ? AND deleted_at IS NULL", userID, zone, true).
		Order("id ASC").
		Find(&collections).Error
	return collections, err
}

func loadLegacyFavoriteSet(db *gorm.DB, userID int64, zone string) (map[int64]struct{}, error) {
	var contentIDs []int64
	err := db.Model(&model.Favorite{}).
		Joins("JOIN content_items ON content_items.id = favorites.content_item_id").
		Where("favorites.user_id = ? AND content_items.zone = ?", userID, zone).
		Pluck("favorites.content_item_id", &contentIDs).Error
	return idSet(contentIDs), err
}

func loadDefaultContentCounts(db *gorm.DB, defaults []model.Collection) (map[int64]int64, error) {
	counts := map[int64]int64{}
	if len(defaults) == 0 {
		return counts, nil
	}
	collectionIDs := make([]int64, 0, len(defaults))
	for _, collection := range defaults {
		collectionIDs = append(collectionIDs, collection.ID)
	}
	var contentIDs []int64
	if err := db.Model(&model.CollectionItem{}).
		Where("collection_id IN ?", collectionIDs).
		Pluck("content_item_id", &contentIDs).Error; err != nil {
		return nil, err
	}
	for _, contentID := range contentIDs {
		counts[contentID]++
	}
	return counts, nil
}

func idSet(ids []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

func countSetDifference(left, right map[int64]struct{}) int64 {
	var count int64
	for id := range left {
		if _, exists := right[id]; !exists {
			count++
		}
	}
	return count
}
