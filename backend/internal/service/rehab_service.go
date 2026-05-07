package service

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrCourseNotFound   = errors.New("course not found")
	ErrAlreadyCompleted = errors.New("course already completed")
	ErrCourseNotStarted = errors.New("course has not been started")
	ErrReadingTooShort  = errors.New("minimum reading time has not elapsed")
)

type RehabService struct {
	rehabRepo *repository.RehabRepository
	db        *gorm.DB
}

func NewRehabService(db *gorm.DB) *RehabService {
	return &RehabService{
		rehabRepo: repository.NewRehabRepository(db),
		db:        db,
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
			Title:         course.ViolationType,
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
		Title:         course.ViolationType,
		Content:       content,
		MinReadingSec: course.MinReadingSec,
		RewardPoints:  course.RewardPoints,
	}, nil
}

func (s *RehabService) CompleteCourse(userID int64, courseID int64) error {
	course, err := s.rehabRepo.GetCourseByID(courseID)
	if err != nil {
		return ErrCourseNotFound
	}

	completed, err := s.rehabRepo.IsCompleted(userID, courseID)
	if err != nil {
		return err
	}
	if completed {
		return ErrAlreadyCompleted
	}
	completion, err := s.rehabRepo.GetCompletion(userID, courseID)
	if err != nil {
		return err
	}
	if err := canCompleteRehabCourse(completionStartedAt(completion), course.MinReadingSec, time.Now()); err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&model.RehabCompletion{}).
			Where("user_id = ? AND course_id = ?", userID, courseID).
			Update("completed_at", now).Error; err != nil {
			return err
		}
		log := &model.ReputationLog{
			UserID: uint(userID),
			Delta:  course.RewardPoints,
			Reason: "rehab_course_completed",
		}
		if err := tx.Create(log).Error; err != nil {
			return err
		}

		return tx.Model(&model.User{}).Where("id = ?", uint(userID)).
			Update("reputation", gorm.Expr("reputation + ?", course.RewardPoints)).Error
	})
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
