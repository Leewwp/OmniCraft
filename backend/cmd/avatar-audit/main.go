// avatar-audit audits and downgrades legacy user avatars that are not
// platform OSS objects (issue #111). Before #111 every avatar_url was
// accepted as-is, so existing users may carry arbitrary external URLs that
// would now be rejected at update time and are out of moderation reach. The
// audit lists them; --apply downgrades them to the default avatar (empty
// avatar_url), reusing the runtime platform-object gate.
//
// Usage:
//
//	go run ./cmd/avatar-audit                       # read-only audit
//	go run ./cmd/avatar-audit --apply --maintenance-window-confirmed
//
// The domain check reuses the runtime contract via aliyun.IsPlatformObjectURL:
// only URLs prefixed with the configured oss.domain are platform-verified
// objects. Requires DB_DSN or config.Load().Database.DSN.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/pkg/aliyun"
)

const (
	avatarAuditCleanExitCode = iota
	avatarAuditDriftExitCode
	avatarAuditUsageExitCode
)

type avatarAuditOptions struct {
	Apply                      bool
	MaintenanceWindowConfirmed bool
}

type avatarAuditRow struct {
	UserID    int64  `json:"user_id"`
	AvatarURL string `json:"avatar_url"`
}

type avatarAuditTotals struct {
	ExternalAvatarUsers int64 `json:"external_avatar_users"`
}

func (t avatarAuditTotals) HasDrift() bool {
	return t.ExternalAvatarUsers > 0
}

type avatarAuditRepairTotals struct {
	DowngradedToDefault int64 `json:"downgraded_to_default"`
}

type avatarAuditReport struct {
	Apply       bool                     `json:"apply"`
	Users       []avatarAuditRow         `json:"users"`
	Totals      avatarAuditTotals        `json:"totals"`
	BeforeApply *avatarAuditTotals       `json:"before_apply,omitempty"`
	Repairs     *avatarAuditRepairTotals `json:"repairs,omitempty"`
}

func main() {
	db, err := gorm.Open(postgres.Open(loadAvatarAuditDSN()), &gorm.Config{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "avatar audit database connection failed")
		os.Exit(avatarAuditUsageExitCode)
	}
	os.Exit(executeAvatarAudit(os.Args[1:], db, config.Load().OSS.Domain, os.Stdout, os.Stderr))
}

func loadAvatarAuditDSN() string {
	if explicitDSN, exists := os.LookupEnv("DB_DSN"); exists {
		return explicitDSN
	}
	return config.Load().Database.DSN
}

func executeAvatarAudit(args []string, db *gorm.DB, ossDomain string, stdout, stderr io.Writer) int {
	options, ok := parseAvatarAuditArgs(args, stderr)
	if !ok {
		return avatarAuditUsageExitCode
	}
	if db == nil {
		fmt.Fprintln(stderr, "avatar audit requires a database")
		return avatarAuditUsageExitCode
	}

	report, err := inspectAvatarAudit(db, ossDomain)
	if err != nil {
		fmt.Fprintf(stderr, "avatar audit inspection failed: %v\n", err)
		return avatarAuditUsageExitCode
	}
	report.Apply = options.Apply

	if options.Apply {
		before := report.Totals
		repairs, repairErr := repairAvatarAudit(db, ossDomain)
		if repairErr != nil {
			fmt.Fprintf(stderr, "avatar audit apply failed: %v\n", repairErr)
			return avatarAuditUsageExitCode
		}
		report, err = inspectAvatarAudit(db, ossDomain)
		if err != nil {
			fmt.Fprintf(stderr, "avatar audit post-apply inspection failed: %v\n", err)
			return avatarAuditUsageExitCode
		}
		report.Apply = true
		report.BeforeApply = &before
		report.Repairs = &repairs
	}

	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintf(stderr, "avatar audit output failed: %v\n", err)
		return avatarAuditUsageExitCode
	}
	if report.Totals.HasDrift() {
		return avatarAuditDriftExitCode
	}
	return avatarAuditCleanExitCode
}

func parseAvatarAuditArgs(args []string, stderr io.Writer) (avatarAuditOptions, bool) {
	options := avatarAuditOptions{}
	for _, arg := range args {
		switch arg {
		case "--apply":
			options.Apply = true
		case "--maintenance-window-confirmed":
			options.MaintenanceWindowConfirmed = true
		default:
			fmt.Fprintf(stderr, "unsupported argument: %s\n", arg)
			return avatarAuditOptions{}, false
		}
	}
	if options.Apply && !options.MaintenanceWindowConfirmed {
		fmt.Fprintln(stderr, "--apply requires maintenance window confirmation: stop all application writers, then pass --maintenance-window-confirmed")
		return avatarAuditOptions{}, false
	}
	if options.MaintenanceWindowConfirmed && !options.Apply {
		fmt.Fprintln(stderr, "--maintenance-window-confirmed is only valid with --apply")
		return avatarAuditOptions{}, false
	}
	return options, true
}

func inspectAvatarAudit(db *gorm.DB, ossDomain string) (avatarAuditReport, error) {
	users, err := externalAvatarUsers(db, ossDomain)
	if err != nil {
		return avatarAuditReport{}, err
	}
	report := avatarAuditReport{Users: make([]avatarAuditRow, 0, len(users))}
	for _, u := range users {
		report.Users = append(report.Users, avatarAuditRow{UserID: u.ID, AvatarURL: u.AvatarURL})
	}
	report.Totals.ExternalAvatarUsers = int64(len(users))
	return report, nil
}

// externalAvatarUsers returns active users whose avatar_url is non-empty and
// is not a platform OSS object URL. Soft-deleted users are excluded.
func externalAvatarUsers(db *gorm.DB, ossDomain string) ([]model.User, error) {
	var users []model.User
	err := db.Where("avatar_url IS NOT NULL AND avatar_url <> '' AND deleted_at IS NULL").
		Order("id ASC").
		Find(&users).Error
	if err != nil {
		return nil, err
	}
	filtered := make([]model.User, 0, len(users))
	for _, u := range users {
		if !aliyun.IsPlatformObjectURL(ossDomain, u.AvatarURL) {
			filtered = append(filtered, u)
		}
	}
	return filtered, nil
}

func repairAvatarAudit(db *gorm.DB, ossDomain string) (avatarAuditRepairTotals, error) {
	repairs := avatarAuditRepairTotals{}
	err := db.Transaction(func(tx *gorm.DB) error {
		users, err := externalAvatarUsers(tx, ossDomain)
		if err != nil {
			return err
		}
		ids := make([]int64, 0, len(users))
		for _, u := range users {
			ids = append(ids, u.ID)
		}
		if len(ids) == 0 {
			return nil
		}
		// Downgrade to the default avatar: an empty avatar_url renders the
		// platform default, matching the account-deletion clear path.
		res := tx.Model(&model.User{}).
			Where("id IN ?", ids).
			Update("avatar_url", "")
		if res.Error != nil {
			return res.Error
		}
		repairs.DowngradedToDefault = res.RowsAffected
		return nil
	})
	return repairs, err
}
