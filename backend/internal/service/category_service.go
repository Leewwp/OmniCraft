package service

import (
	"errors"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrCategoryNotFound = errors.New("category not found")
	ErrCategoryHasChildren = errors.New("category has child categories")
	ErrCategoryHasContent  = errors.New("category has linked content")
)

type CategoryService struct {
	catRepo *repository.CategoryRepository
}

func NewCategoryService(catRepo *repository.CategoryRepository) *CategoryService {
	return &CategoryService{catRepo: catRepo}
}

func (s *CategoryService) GetCategories(zone, level string, parentID *int64) ([]model.Category, error) {
	return s.catRepo.ListByZoneAndLevel(zone, level, parentID)
}

func (s *CategoryService) AdminCreateCategory(cat *model.Category) error {
	return s.catRepo.Create(cat)
}

func (s *CategoryService) AdminUpdateCategory(id int64, updates map[string]interface{}) error {
	cat, err := s.catRepo.FindByID(id)
	if err != nil || cat == nil {
		return ErrCategoryNotFound
	}
	return s.catRepo.Update(id, updates)
}

func (s *CategoryService) AdminDeleteCategory(id int64) error {
	hasChildren, err := s.catRepo.HasChildren(id)
	if err != nil {
		return err
	}
	if hasChildren {
		return ErrCategoryHasChildren
	}
	hasContent, err := s.catRepo.HasLinkedContent(id)
	if err != nil {
		return err
	}
	if hasContent {
		return ErrCategoryHasContent
	}
	return s.catRepo.Delete(id)
}

func (s *CategoryService) AdminReorderCategories(updates []struct {
	ID        int64 `json:"id"`
	SortOrder int   `json:"sort_order"`
}) error {
	return s.catRepo.UpdateSortOrder(updates)
}
