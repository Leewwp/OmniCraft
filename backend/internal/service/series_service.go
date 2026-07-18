package service

import (
	"context"
	"strings"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

type SeriesService struct {
	seriesRepo *repository.SeriesRepository
}

type SeriesDetail struct {
	Series    model.ContentSeries
	Items     []model.ContentSeriesItem
	Cover     *string
	ItemCount int64
}

func NewSeriesService(seriesRepo *repository.SeriesRepository) *SeriesService {
	return &SeriesService{seriesRepo: seriesRepo}
}

func (s *SeriesService) CreateSeries(ctx context.Context, ownerID int64, title, description, zone string) (*model.ContentSeries, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" || (zone != "original" && zone != "fanwork") {
		return nil, repository.ErrInvalidSeries
	}
	return s.seriesRepo.CreateSeries(ctx, &model.ContentSeries{
		Title:       title,
		Description: description,
		OwnerID:     ownerID,
		Zone:        zone,
	})
}

func (s *SeriesService) ListOwnedSeries(ctx context.Context, ownerID int64, zone string) ([]model.ContentSeries, error) {
	if zone != "" && zone != "original" && zone != "fanwork" {
		return nil, repository.ErrInvalidSeries
	}
	return s.seriesRepo.ListSeriesByOwner(ctx, ownerID, zone)
}

func (s *SeriesService) ListCandidateContents(ctx context.Context, ownerID int64, zone, search string, limit int) ([]model.ContentItem, error) {
	if zone != "original" && zone != "fanwork" {
		return nil, repository.ErrInvalidSeries
	}
	return s.seriesRepo.ListCandidateContents(ctx, ownerID, zone, search, limit)
}

func (s *SeriesService) GetSeriesDetail(ctx context.Context, seriesID, viewerID int64) (*SeriesDetail, error) {
	series, err := s.seriesRepo.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}

	items, err := s.seriesRepo.ListVisibleSeriesItems(ctx, seriesID, viewerID)
	if err != nil {
		return nil, err
	}
	return buildSeriesDetail(*series, items), nil
}

func (s *SeriesService) GetSeriesManagementDetail(ctx context.Context, seriesID, ownerID int64) (*SeriesDetail, error) {
	series, err := s.seriesRepo.GetSeries(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	if series.OwnerID != ownerID {
		return nil, repository.ErrNotSeriesOwner
	}
	items, err := s.seriesRepo.ListSeriesItems(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	return buildSeriesDetail(*series, items), nil
}

func buildSeriesDetail(series model.ContentSeries, items []model.ContentSeriesItem) *SeriesDetail {
	detail := &SeriesDetail{
		Series:    series,
		Items:     items,
		ItemCount: int64(len(items)),
	}
	detail.Cover = resolveSeriesCover(series.CoverContentID, items)
	return detail
}

func (s *SeriesService) UpdateSeries(ctx context.Context, seriesID, ownerID int64, patch repository.SeriesPatch) (*model.ContentSeries, error) {
	if patch.Title != nil {
		trimmed := strings.TrimSpace(*patch.Title)
		if trimmed == "" {
			return nil, repository.ErrInvalidSeries
		}
		patch.Title = &trimmed
	}
	if patch.Description != nil {
		trimmed := strings.TrimSpace(*patch.Description)
		patch.Description = &trimmed
	}
	return s.seriesRepo.UpdateSeries(ctx, seriesID, ownerID, patch)
}

func (s *SeriesService) DeleteSeries(ctx context.Context, seriesID, ownerID int64) error {
	return s.seriesRepo.DeleteSeries(ctx, seriesID, ownerID)
}

func (s *SeriesService) AddItem(ctx context.Context, seriesID, ownerID, contentID int64) (*model.ContentSeriesItem, error) {
	return s.seriesRepo.AddItem(ctx, seriesID, ownerID, contentID)
}

func (s *SeriesService) RemoveItem(ctx context.Context, seriesID, ownerID, itemID int64) error {
	return s.seriesRepo.RemoveItem(ctx, seriesID, ownerID, itemID)
}

func (s *SeriesService) ReorderItems(ctx context.Context, seriesID, ownerID int64, itemIDs []int64) error {
	return s.seriesRepo.ReorderItems(ctx, seriesID, ownerID, itemIDs)
}

func (s *SeriesService) ListMembershipsForContent(ctx context.Context, contentID int64) ([]repository.SeriesMembership, error) {
	return s.seriesRepo.ListMembershipsForContent(ctx, contentID)
}

func resolveSeriesCover(coverContentID *int64, items []model.ContentSeriesItem) *string {
	if coverContentID != nil {
		for _, item := range items {
			if item.ContentItemID == *coverContentID && item.ContentItem.CoverImageURL != "" {
				cover := item.ContentItem.CoverImageURL
				return &cover
			}
		}
	}
	for _, item := range items {
		if item.ContentItem.CoverImageURL != "" {
			cover := item.ContentItem.CoverImageURL
			return &cover
		}
	}
	return nil
}
