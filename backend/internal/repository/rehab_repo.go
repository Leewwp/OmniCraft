package repository

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"omnicraft/backend/internal/model"
)

type RehabRepository struct {
	db *gorm.DB
}

func NewRehabRepository(db *gorm.DB) *RehabRepository {
	return &RehabRepository{db: db}
}

func (r *RehabRepository) ListCourses() ([]model.RehabCourse, error) {
	var courses []model.RehabCourse
	err := r.db.Order("id ASC").Find(&courses).Error
	return courses, err
}

func (r *RehabRepository) GetCourseByID(id int64) (*model.RehabCourse, error) {
	var course model.RehabCourse
	err := r.db.First(&course, id).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *RehabRepository) GetCourseByViolationType(vt string) (*model.RehabCourse, error) {
	var course model.RehabCourse
	err := r.db.Where("violation_type = ?", vt).First(&course).Error
	if err != nil {
		return nil, err
	}
	return &course, nil
}

func (r *RehabRepository) GetCompletionsByUser(userID int64) ([]model.RehabCompletion, error) {
	var completions []model.RehabCompletion
	err := r.db.Where("user_id = ?", userID).Find(&completions).Error
	return completions, err
}

func (r *RehabRepository) IsCompleted(userID, courseID int64) (bool, error) {
	var count int64
	err := r.db.Model(&model.RehabCompletion{}).
		Where("user_id = ? AND course_id = ? AND completed_at IS NOT NULL", userID, courseID).
		Count(&count).Error
	return count > 0, err
}

func (r *RehabRepository) CreateCompletion(completion *model.RehabCompletion) error {
	return r.db.Create(completion).Error
}

func (r *RehabRepository) GetCompletion(userID, courseID int64) (*model.RehabCompletion, error) {
	var completion model.RehabCompletion
	err := r.db.Where("user_id = ? AND course_id = ?", userID, courseID).First(&completion).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &completion, nil
}

func (r *RehabRepository) StartCourse(userID, courseID int64) error {
	now := time.Now()
	completion := model.RehabCompletion{UserID: userID, CourseID: courseID}
	return r.db.Where(model.RehabCompletion{UserID: userID, CourseID: courseID}).
		Attrs(model.RehabCompletion{StartedAt: &now}).
		FirstOrCreate(&completion).Error
}
