package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"omnicraft/backend/internal/model"
	redisclient "omnicraft/backend/internal/pkg/redis"
	"omnicraft/backend/internal/repository"
)

type CollectionService struct {
	collectionRepo *repository.CollectionRepository
	contentRepo    *repository.ContentRepository
}

func NewCollectionService(collectionRepo *repository.CollectionRepository, contentRepo *repository.ContentRepository) *CollectionService {
	return &CollectionService{
		collectionRepo: collectionRepo,
		contentRepo:    contentRepo,
	}
}

func (s *CollectionService) ListOwnCollections(ctx context.Context, ownerID int64, zone string, containsContentItemID *int64) ([]repository.CollectionSummary, error) {
	for _, defaultZone := range []string{"original", "fanwork"} {
		if _, err := s.ensureDefaultCollection(ctx, ownerID, defaultZone); err != nil {
			return nil, fmt.Errorf("ensure %s default collection: %w", defaultZone, err)
		}
	}
	return s.collectionRepo.ListCollectionsForViewer(ctx, ownerID, ownerID, zone, containsContentItemID)
}

func (s *CollectionService) ensureDefaultCollection(ctx context.Context, ownerID int64, zone string) (*model.Collection, error) {
	collection, err := s.collectionRepo.EnsureDefaultCollection(ctx, ownerID, zone)
	if err != nil {
		slog.Error("collection default repair failed",
			"user_id", ownerID,
			"zone", zone,
			"error_code", collectionDefaultRepairErrorCode(err),
		)
		return nil, err
	}
	return collection, nil
}

func collectionDefaultRepairErrorCode(err error) string {
	switch {
	case errors.Is(err, repository.ErrZoneMismatch):
		return repository.ErrZoneMismatch.Error()
	case errors.Is(err, repository.ErrCollectionNotFound):
		return repository.ErrCollectionNotFound.Error()
	default:
		return "COLLECTION_DEFAULT_REPAIR_FAILED"
	}
}

func (s *CollectionService) AddItem(ctx context.Context, collectionID, ownerID, contentID int64, note string) (*model.CollectionItem, error) {
	return s.addItem(ctx, collectionID, ownerID, contentID, note, true)
}

func (s *CollectionService) addItem(ctx context.Context, collectionID, ownerID, contentID int64, note string, clearCache bool) (*model.CollectionItem, error) {
	item, err := s.collectionRepo.AddItem(ctx, collectionID, ownerID, contentID, note)
	if err != nil {
		return nil, err
	}
	if clearCache {
		redisclient.ClearRecCache(ctx, ownerID)
	}
	return item, nil
}

func (s *CollectionService) RemoveItem(ctx context.Context, collectionID, ownerID, itemID int64) error {
	return s.removeItem(ctx, collectionID, ownerID, itemID, true)
}

func (s *CollectionService) removeItem(ctx context.Context, collectionID, ownerID, itemID int64, clearCache bool) error {
	if err := s.collectionRepo.RemoveItem(ctx, collectionID, ownerID, itemID); err != nil {
		return err
	}
	if clearCache {
		redisclient.ClearRecCache(ctx, ownerID)
	}
	return nil
}

func (s *CollectionService) AddToDefaultCollection(ctx context.Context, ownerID, contentID int64, note string) error {
	return s.addToDefaultCollection(ctx, ownerID, contentID, note, true)
}

func (s *CollectionService) addToDefaultCollection(ctx context.Context, ownerID, contentID int64, note string, clearCache bool) error {
	if s.contentRepo == nil {
		return repository.ErrInvalidContent
	}

	content, err := s.contentRepo.FindByID(contentID)
	if err != nil {
		return err
	}
	if content == nil {
		return repository.ErrInvalidContent
	}

	collection, err := s.ensureDefaultCollection(ctx, ownerID, content.Zone)
	if err != nil {
		return err
	}
	if _, err := s.collectionRepo.AddItem(ctx, collection.ID, ownerID, contentID, note); err != nil {
		if errors.Is(err, repository.ErrDuplicateCollectionItem) {
			if clearCache {
				redisclient.ClearRecCache(ctx, ownerID)
			}
			return nil
		}
		return err
	}
	if clearCache {
		redisclient.ClearRecCache(ctx, ownerID)
	}
	return nil
}

func (s *CollectionService) RemoveDefaultItemByContentID(ctx context.Context, ownerID, contentID int64) error {
	return s.removeDefaultItemByContentID(ctx, ownerID, contentID, true)
}

func (s *CollectionService) removeDefaultItemByContentID(ctx context.Context, ownerID, contentID int64, clearCache bool) error {
	if err := s.collectionRepo.RemoveDefaultItemByContentID(ctx, ownerID, contentID); err != nil {
		return err
	}
	if clearCache {
		redisclient.ClearRecCache(ctx, ownerID)
	}
	return nil
}
