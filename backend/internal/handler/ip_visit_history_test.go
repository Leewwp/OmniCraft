package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"omnicraft/backend/internal/middleware"
	"omnicraft/backend/internal/testutil"
)

func setupIPVisitHistoryHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db := testutil.OpenEphemeralPostgres(t)
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	createIPVisitHistoryBaseSchema(t, db)
	testutil.ApplyMigrationFile(t, db, filepath.Join("..", "..", "migrations", "066_ip_visit_history.sql"))

	h := NewIPVisitHistoryHandler(db)
	router := gin.New()
	testAuth := func(c *gin.Context) {
		if raw := c.GetHeader("X-Test-User-ID"); raw != "" {
			if userID, err := strconv.ParseInt(raw, 10, 64); err == nil {
				c.Set(middleware.UserIDKey, userID)
			}
		}
	}
	router.GET("/api/v1/users/me/ip-visits", testAuth, h.ListRecent)
	router.PUT("/api/v1/users/me/ip-visits/:ipId", testAuth, h.RecordVisit)
	router.POST("/api/v1/users/me/ip-visits/merge", testAuth, h.MergeVisits)
	return router, db
}

func createIPVisitHistoryBaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	if err := db.Exec(`
		CREATE TABLE users (
			id BIGSERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(64) UNIQUE NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create users base table: %v", err)
	}
	if err := db.Exec(`
		CREATE TABLE ips (
			id BIGSERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) UNIQUE NOT NULL,
			description TEXT,
			cover_url TEXT,
			category VARCHAR(50),
			status VARCHAR(20) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`).Error; err != nil {
		t.Fatalf("create ips base table: %v", err)
	}
}

func seedIPVisitHistoryTestUsers(t *testing.T, db *gorm.DB, ids ...int64) {
	t.Helper()
	for _, id := range ids {
		if err := db.Exec(
			`INSERT INTO users (id, email, username) VALUES (?, ?, ?)`,
			id, fmt.Sprintf("user%d@example.test", id), fmt.Sprintf("user%d", id),
		).Error; err != nil {
			t.Fatalf("seed user %d: %v", id, err)
		}
	}
}

func seedIPVisitHistoryTestIPs(t *testing.T, db *gorm.DB, statusByID map[int64]string) {
	t.Helper()
	for id, status := range statusByID {
		if err := db.Exec(
			`INSERT INTO ips (id, name, slug, status) VALUES (?, ?, ?, ?)`,
			id, fmt.Sprintf("IP%d", id), fmt.Sprintf("ip-%d", id), status,
		).Error; err != nil {
			t.Fatalf("seed ip %d: %v", id, err)
		}
	}
}

func seedIPVisitHistoryRow(t *testing.T, db *gorm.DB, userID, ipID int64, visitedAt time.Time) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO ip_visit_history (user_id, ip_id, visited_at) VALUES (?, ?, ?)`,
		userID, ipID, visitedAt,
	).Error; err != nil {
		t.Fatalf("seed ip_visit_history (%d, %d): %v", userID, ipID, err)
	}
}

func requestIPVisitHistory(t *testing.T, router *gin.Engine, userID int64, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if userID > 0 {
		req.Header.Set("X-Test-User-ID", strconv.FormatInt(userID, 10))
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(rec, req)
	return rec
}

type ipVisitListPayload struct {
	Items []struct {
		IP        json.RawMessage `json:"ip"`
		VisitedAt string          `json:"visited_at"`
	} `json:"items"`
	Limit int `json:"limit"`
}

func assertIPVisitHistoryError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, status, rec.Body.String())
	}
	var payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	decodeJSON(t, rec, &payload)
	if payload.Code != code {
		t.Fatalf("code = %q, want %q; body = %s", payload.Code, code, rec.Body.String())
	}
}

func assertIPVisitHistoryRowCount(t *testing.T, db *gorm.DB, userID int64, want int64) {
	t.Helper()
	var count int64
	if err := db.Raw(
		`SELECT count(*) FROM ip_visit_history WHERE user_id = ?`, userID,
	).Scan(&count).Error; err != nil {
		t.Fatalf("count ip_visit_history for user %d: %v", userID, err)
	}
	if count != want {
		t.Fatalf("ip_visit_history rows for user %d = %d, want %d", userID, count, want)
	}
}

func TestIPVisitHistoryRecordIsIdempotentAndRefreshesRecency(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{101: "approved"})

	first := requestIPVisitHistory(t, router, 1, http.MethodPut, "/api/v1/users/me/ip-visits/101", "")
	if first.Code != http.StatusNoContent {
		t.Fatalf("first record status = %d, want 204; body = %s", first.Code, first.Body.String())
	}
	assertIPVisitHistoryRowCount(t, db, 1, 1)

	var before time.Time
	if err := db.Raw(`SELECT visited_at FROM ip_visit_history WHERE user_id = 1 AND ip_id = 101`).Scan(&before).Error; err != nil {
		t.Fatalf("read first visited_at: %v", err)
	}

	second := requestIPVisitHistory(t, router, 1, http.MethodPut, "/api/v1/users/me/ip-visits/101", "")
	if second.Code != http.StatusNoContent {
		t.Fatalf("repeat record status = %d, want 204; body = %s", second.Code, second.Body.String())
	}
	assertIPVisitHistoryRowCount(t, db, 1, 1)

	var after time.Time
	if err := db.Raw(`SELECT visited_at FROM ip_visit_history WHERE user_id = 1 AND ip_id = 101`).Scan(&after).Error; err != nil {
		t.Fatalf("read second visited_at: %v", err)
	}
	if !after.After(before) {
		t.Fatalf("repeat record must refresh visited_at: before=%s after=%s", before.Format(time.RFC3339), after.Format(time.RFC3339))
	}
}

func TestIPVisitHistoryRecordRejectsUnknownOrNonPublicIP(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{
		101: "approved",
		102: "pending",
		103: "banned",
	})

	for _, path := range []string{
		"/api/v1/users/me/ip-visits/999",
		"/api/v1/users/me/ip-visits/102",
		"/api/v1/users/me/ip-visits/103",
		"/api/v1/users/me/ip-visits/not-a-number",
	} {
		rec := requestIPVisitHistory(t, router, 1, http.MethodPut, path, "")
		assertIPVisitHistoryError(t, rec, http.StatusNotFound, "IP_NOT_FOUND")
	}
	assertIPVisitHistoryRowCount(t, db, 1, 0)
}

func TestIPVisitHistoryListRecentReturnsSixOrderedAndIsolated(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1, 2)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{
		101: "approved", 102: "approved", 103: "approved", 104: "approved",
		105: "approved", 106: "approved", 107: "approved",
	})

	now := time.Now().UTC().Truncate(time.Second)
	// user 1 visits seven IPs; 101 is the most recent.
	for i := 0; i < 7; i++ {
		seedIPVisitHistoryRow(t, db, 1, int64(101+i), now.Add(-time.Duration(i)*time.Hour))
	}
	// user 2 only visited 101; isolation must keep their list separate.
	seedIPVisitHistoryRow(t, db, 2, 101, now)

	rec := requestIPVisitHistory(t, router, 1, http.MethodGet, "/api/v1/users/me/ip-visits", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload ipVisitListPayload
	decodeJSON(t, rec, &payload)
	if payload.Limit != 6 {
		t.Fatalf("limit = %d, want 6", payload.Limit)
	}
	if len(payload.Items) != 6 {
		t.Fatalf("items len = %d, want 6 (recent cap)", len(payload.Items))
	}
	for i, item := range payload.Items {
		var ip struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(item.IP, &ip); err != nil {
			t.Fatalf("item[%d] ip is not the public summary: %v", i, err)
		}
		if ip.ID == 0 || ip.Name == "" {
			t.Fatalf("item[%d] ip summary missing id/name: %s", i, item.IP)
		}
		if _, err := time.Parse(time.RFC3339Nano, item.VisitedAt); err != nil {
			t.Fatalf("item[%d] visited_at %q is not RFC3339: %v", i, item.VisitedAt, err)
		}
		if i > 0 {
			prev, _ := time.Parse(time.RFC3339Nano, payload.Items[i-1].VisitedAt)
			cur, _ := time.Parse(time.RFC3339Nano, item.VisitedAt)
			if cur.After(prev) {
				t.Fatalf("items not ordered by visited_at DESC at index %d", i)
			}
		}
	}
	if payload.Items[0].IP == nil {
		t.Fatal("most recent IP must come first")
	}

	other := requestIPVisitHistory(t, router, 2, http.MethodGet, "/api/v1/users/me/ip-visits", "")
	var otherPayload ipVisitListPayload
	decodeJSON(t, other, &otherPayload)
	if len(otherPayload.Items) != 1 {
		t.Fatalf("user 2 items len = %d, want 1 (per-user isolation)", len(otherPayload.Items))
	}
}

func TestIPVisitHistoryListTieBreaksByIPIDDesc(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{
		101: "approved", 102: "approved",
	})

	now := time.Now().UTC().Truncate(time.Second)
	seedIPVisitHistoryRow(t, db, 1, 101, now)
	seedIPVisitHistoryRow(t, db, 1, 102, now)

	rec := requestIPVisitHistory(t, router, 1, http.MethodGet, "/api/v1/users/me/ip-visits", "")
	var payload ipVisitListPayload
	decodeJSON(t, rec, &payload)
	if len(payload.Items) != 2 {
		t.Fatalf("items len = %d, want 2", len(payload.Items))
	}
	var first struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(payload.Items[0].IP, &first); err != nil {
		t.Fatal(err)
	}
	if first.ID != 102 {
		t.Fatalf("same-timestamp tie-break: first ip = %d, want 102 (ip_id DESC)", first.ID)
	}
}

func TestIPVisitHistoryMergeClampsFutureAndFoldsDuplicates(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{101: "approved"})

	requestTime := time.Now().UTC()
	future := requestTime.Add(2 * time.Hour)
	older := requestTime.Add(-2 * time.Hour)
	body := fmt.Sprintf(`{"visits": [
		{"ip_id": 101, "visited_at": %q},
		{"ip_id": 101, "visited_at": %q}
	]}`, future.Format(time.RFC3339), older.Format(time.RFC3339))

	rec := requestIPVisitHistory(t, router, 1, http.MethodPost, "/api/v1/users/me/ip-visits/merge", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Accepted  []int64 `json:"accepted_ip_ids"`
		Discarded []int64 `json:"discarded_ip_ids"`
		Items     []struct {
			VisitedAt string `json:"visited_at"`
		} `json:"items"`
	}
	decodeJSON(t, rec, &payload)
	if len(payload.Accepted) != 1 || payload.Accepted[0] != 101 {
		t.Fatalf("accepted = %v, want [101] (duplicate folded)", payload.Accepted)
	}
	if len(payload.Discarded) != 0 {
		t.Fatalf("discarded = %v, want []", payload.Discarded)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("items len = %d, want 1", len(payload.Items))
	}
	stored, err := time.Parse(time.RFC3339Nano, payload.Items[0].VisitedAt)
	if err != nil {
		t.Fatal(err)
	}
	if stored.After(requestTime.Add(2 * time.Second)) {
		t.Fatalf("future visit must be clamped to server receive time, got %s", stored.Format(time.RFC3339))
	}
	if stored.Before(requestTime.Add(-2 * time.Second)) {
		t.Fatalf("stored time must reflect the newest duplicate, got %s", stored.Format(time.RFC3339))
	}
	assertIPVisitHistoryRowCount(t, db, 1, 1)
}

func TestIPVisitHistoryMergeDiscardsUnavailableIPsAndKeepsValidOnes(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{
		101: "approved",
		103: "banned",
	})

	now := time.Now().UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"visits": [
		{"ip_id": 101, "visited_at": %q},
		{"ip_id": 999, "visited_at": %q},
		{"ip_id": 103, "visited_at": %q}
	]}`, now, now, now)

	rec := requestIPVisitHistory(t, router, 1, http.MethodPost, "/api/v1/users/me/ip-visits/merge", body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Accepted  []int64 `json:"accepted_ip_ids"`
		Discarded []int64 `json:"discarded_ip_ids"`
		Items     []struct {
			IP struct {
				ID int64 `json:"id"`
			} `json:"ip"`
		} `json:"items"`
	}
	decodeJSON(t, rec, &payload)
	if len(payload.Accepted) != 1 || payload.Accepted[0] != 101 {
		t.Fatalf("accepted = %v, want [101]", payload.Accepted)
	}
	if len(payload.Discarded) != 2 || payload.Discarded[0] != 999 || payload.Discarded[1] != 103 {
		t.Fatalf("discarded = %v, want [999 103]", payload.Discarded)
	}
	if len(payload.Items) != 1 || payload.Items[0].IP.ID != 101 {
		t.Fatalf("items = %#v, want only the accepted public IP", payload.Items)
	}
	assertIPVisitHistoryRowCount(t, db, 1, 1)
}

func TestIPVisitHistoryMergePayloadValidation(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{101: "approved"})

	now := time.Now().UTC().Format(time.RFC3339)
	seven := make([]string, 0, 7)
	for i := 0; i < 7; i++ {
		seven = append(seven, fmt.Sprintf(`{"ip_id": %d, "visited_at": %q}`, 101+i, now))
	}

	tests := []struct {
		name string
		body string
	}{
		{name: "missing visits field", body: `{}`},
		{name: "null visits", body: `{"visits": null}`},
		{name: "item missing ip_id", body: `{"visits": [{"visited_at": "` + now + `"}]}`},
		{name: "item zero ip_id", body: `{"visits": [{"ip_id": 0, "visited_at": "` + now + `"}]}`},
		{name: "item missing visited_at", body: `{"visits": [{"ip_id": 101}]}`},
		{name: "item malformed visited_at", body: `{"visits": [{"ip_id": 101, "visited_at": "2026-08-12"}]}`},
		{name: "batch over cap", body: `{"visits": [` + strings.Join(seven, ",") + `]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := requestIPVisitHistory(t, router, 1, http.MethodPost, "/api/v1/users/me/ip-visits/merge", tt.body)
			assertIPVisitHistoryError(t, rec, http.StatusBadRequest, "INVALID_IP_VISIT_MERGE")
			assertIPVisitHistoryRowCount(t, db, 1, 0)
		})
	}
}

func TestIPVisitHistoryMergeEmptyArrayReturnsCurrentList(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	seedIPVisitHistoryTestUsers(t, db, 1)
	seedIPVisitHistoryTestIPs(t, db, map[int64]string{101: "approved"})
	seedIPVisitHistoryRow(t, db, 1, 101, time.Now().UTC())

	rec := requestIPVisitHistory(t, router, 1, http.MethodPost, "/api/v1/users/me/ip-visits/merge", `{"visits": []}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Accepted  []int64 `json:"accepted_ip_ids"`
		Discarded []int64 `json:"discarded_ip_ids"`
		Items     []struct {
			IP struct {
				ID int64 `json:"id"`
			} `json:"ip"`
		} `json:"items"`
	}
	decodeJSON(t, rec, &payload)
	if len(payload.Accepted) != 0 || len(payload.Discarded) != 0 {
		t.Fatalf("accepted/discarded = %v/%v, want empty", payload.Accepted, payload.Discarded)
	}
	if len(payload.Items) != 1 || payload.Items[0].IP.ID != 101 {
		t.Fatalf("items = %#v, want the existing recent list", payload.Items)
	}
}

func TestIPVisitHistoryRecordDBFailureIsMasked(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	rec := requestIPVisitHistory(t, router, 1, http.MethodPut, "/api/v1/users/me/ip-visits/101", "")
	assertIPVisitHistoryError(t, rec, http.StatusInternalServerError, "IP_VISIT_RECORD_FAILED")
	if strings.Contains(rec.Body.String(), "sql:") || strings.Contains(rec.Body.String(), "database is closed") {
		t.Fatalf("500 body must not expose the raw database error: %s", rec.Body.String())
	}
}

func TestIPVisitHistoryMergeDBFailureIsMasked(t *testing.T) {
	router, db := setupIPVisitHistoryHandlerTest(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	rec := requestIPVisitHistory(t, router, 1, http.MethodPost, "/api/v1/users/me/ip-visits/merge", `{"visits": [{"ip_id": 101, "visited_at": "2026-08-12T10:00:00Z"}]}`)
	assertIPVisitHistoryError(t, rec, http.StatusInternalServerError, "IP_VISIT_MERGE_FAILED")
	if strings.Contains(rec.Body.String(), "sql:") || strings.Contains(rec.Body.String(), "database is closed") {
		t.Fatalf("500 body must not expose the raw database error: %s", rec.Body.String())
	}
}
