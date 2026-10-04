package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrCourseNotFound         = errors.New("course not found")
	ErrAlreadyCompleted       = errors.New("course already completed")
	ErrCourseNotStarted       = errors.New("course has not been started")
	ErrReadingTooShort        = errors.New("minimum reading time has not elapsed")
	ErrStatusCacheUnavailable = errors.New("runtime status cache unavailable")
)

type RehabService struct {
	rehabRepo   *repository.RehabRepository
	db          *gorm.DB
	statusCache *RuntimeStatusCache
}

func NewRehabService(db *gorm.DB, statusCache *RuntimeStatusCache) *RehabService {
	return &RehabService{
		rehabRepo:   repository.NewRehabRepository(db),
		db:          db,
		statusCache: statusCache,
	}
}

type RehabCourseResponse struct {
	ID            int64  `json:"id"`
	ViolationType string `json:"violation_type"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	MinReadingSec int    `json:"min_reading_sec"`
	RewardPoints  int    `json:"reward_points"`
	Completed     bool   `json:"completed"`
}

// rehabCourseTitles（SP-26-C #782）：violation_type 枚举归属方（后端）持有
// 的唯一标题知识副本——与 migrations/026_rehab_seed.sql 的五个违规码一一
// 对应。Title 按 locale 本地化，避免中文界面直出英文 slug；未知违规码
// 原样回退（测试/扩展期不崩）。
var rehabCourseTitles = map[string]struct{ zh, en string }{
	"malicious_report_tag":     {zh: "恶意举报标记", en: "Malicious Report Tagging"},
	"malicious_comment":        {zh: "恶意评论", en: "Malicious Comments"},
	"malicious_contribution":   {zh: "恶意投稿", en: "Malicious Contributions"},
	"malicious_report_comment": {zh: "恶意举报评论", en: "Malicious Comment Reporting"},
	"judge_error":              {zh: "判官误判", en: "Judge Misjudgment"},
}

// rehabCourseTitle returns the localized course title for a violation type.
// Empty/unknown locale falls back to zh; unknown violation types return the
// raw code so diagnostics stay possible.
func rehabCourseTitle(violationType, locale string) string {
	titles, ok := rehabCourseTitles[violationType]
	if !ok {
		return violationType
	}
	if locale == "en" {
		return titles.en
	}
	return titles.zh
}

func (s *RehabService) GetAvailableCourses(userID int64, locale string) ([]RehabCourseResponse, error) {
	courses, err := s.rehabRepo.ListCourses()
	if err != nil {
		return nil, err
	}

	completions, err := s.rehabRepo.GetCompletionsByUser(userID)
	if err != nil {
		return nil, err
	}
	completedMap := make(map[int64]bool)
	for _, c := range completions {
		completedMap[c.CourseID] = c.CompletedAt != nil
	}

	if locale == "" {
		locale = "zh"
	}

	var result []RehabCourseResponse
	for _, course := range courses {
		contentI18n := course.ContentI18n
		content := ""
		if contentI18n != nil {
			if v, ok := contentI18n[locale]; ok {
				content = toString(v)
			} else if v, ok := contentI18n["zh"]; ok {
				content = toString(v)
			}
		}

		result = append(result, RehabCourseResponse{
			ID:            course.ID,
			ViolationType: course.ViolationType,
			Title:         rehabCourseTitle(course.ViolationType, locale),
			Content:       content,
			MinReadingSec: course.MinReadingSec,
			RewardPoints:  course.RewardPoints,
			Completed:     completedMap[course.ID],
		})
	}
	return result, nil
}

func (s *RehabService) GetCourseDetail(courseID int64, locale string) (*RehabCourseResponse, error) {
	course, err := s.rehabRepo.GetCourseByID(courseID)
	if err != nil {
		return nil, ErrCourseNotFound
	}
	if locale == "" {
		locale = "zh"
	}
	content := ""
	if course.ContentI18n != nil {
		if v, ok := course.ContentI18n[locale]; ok {
			content = toString(v)
		} else if v, ok := course.ContentI18n["zh"]; ok {
			content = toString(v)
		}
	}
	return &RehabCourseResponse{
		ID:            course.ID,
		ViolationType: course.ViolationType,
		Title:         rehabCourseTitle(course.ViolationType, locale),
		Content:       content,
		MinReadingSec: course.MinReadingSec,
		RewardPoints:  course.RewardPoints,
	}, nil
}

func (s *RehabService) CompleteCourse(userID int64, courseID int64) (int, error) {
	course, err := s.rehabRepo.GetCourseByID(courseID)
	if err != nil {
		return 0, ErrCourseNotFound
	}

	completed, err := s.rehabRepo.IsCompleted(userID, courseID)
	if err != nil {
		return 0, err
	}
	if completed {
		return s.completedCourseResult(userID)
	}
	completion, err := s.rehabRepo.GetCompletion(userID, courseID)
	if err != nil {
		return 0, err
	}
	if err := canCompleteRehabCourse(completionStartedAt(completion), course.MinReadingSec, time.Now()); err != nil {
		return 0, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.RehabCompletion{}).
			Where("user_id = ? AND course_id = ?", userID, courseID).
			Update("completed_at", now).Error; err != nil {
			return err
		}
		log := &model.ReputationLog{
			UserID: userID,
			Delta:  course.RewardPoints,
			Reason: "rehab_course_completed",
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}

		return tx.Model(&model.User{}).Where("id = ?", userID).
			Update("reputation", gorm.Expr("reputation + ?", course.RewardPoints)).Error
	})
	if err != nil {
		return 0, err
	}
	return s.completedCourseResult(userID)
}

func (s *RehabService) completedCourseResult(userID int64) (int, error) {
	var reputation int
	if err := s.db.Model(&model.User{}).
		Select("reputation").
		Where("id = ?", userID).
		Scan(&reputation).Error; err != nil {
		return 0, err
	}
	if s.statusCache != nil {
		if err := s.statusCache.Invalidate(userID); err != nil {
			return reputation, ErrStatusCacheUnavailable
		}
	}
	return reputation, nil
}

func (s *RehabService) GetMyProgress(userID int64) ([]RehabCourseResponse, error) {
	return s.GetAvailableCourses(userID, "zh")
}

func (s *RehabService) StartCourse(userID int64, courseID int64) error {
	if _, err := s.rehabRepo.GetCourseByID(courseID); err != nil {
		return ErrCourseNotFound
	}
	completed, err := s.rehabRepo.IsCompleted(userID, courseID)
	if err != nil {
		return err
	}
	if completed {
		return ErrAlreadyCompleted
	}
	return s.rehabRepo.StartCourse(userID, courseID)
}

func completionStartedAt(completion *model.RehabCompletion) *time.Time {
	if completion == nil {
		return nil
	}
	return completion.StartedAt
}

func canCompleteRehabCourse(startedAt *time.Time, minReadingSec int, now time.Time) error {
	if startedAt == nil {
		return ErrCourseNotStarted
	}
	if minReadingSec < 0 {
		minReadingSec = 0
	}
	if now.Sub(*startedAt) < time.Duration(minReadingSec)*time.Second {
		return ErrReadingTooShort
	}
	return nil
}

func toString(v interface{}) string {
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}
