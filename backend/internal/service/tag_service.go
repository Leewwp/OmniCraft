package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"omnicraft/backend/config"
	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrTagSuggestionNotFound  = errors.New("tag suggestion not found")
	ErrTagSuggestionForbidden = errors.New("not content author")
	ErrTagGroupNotFound       = errors.New("tag group not found")
	ErrTagGroupForbidden      = errors.New("not tag group owner")
	ErrTagSuggestRateLimited  = errors.New("tag suggestion daily limit exceeded")
)

type TagService struct {
	tagRepo     *repository.TagRepository
	contentRepo *repository.ContentRepository
	rdb         *redis.Client
	cfg         *config.CacheConfig
}

func NewTagService(tagRepo *repository.TagRepository, contentRepo *repository.ContentRepository, rdb *redis.Client, cfg *config.CacheConfig) *TagService {
	return &TagService{tagRepo: tagRepo, contentRepo: contentRepo, rdb: rdb, cfg: cfg}
}

func (s *TagService) GetFacetedTags(category string, selectedTags []string) ([]model.Tag, error) {
	if len(selectedTags) > 0 {
		return s.tagRepo.GetCooccurringTags(selectedTags, category, 50)
	}
	return s.tagRepo.GetTagsByCategory(category, 50)
}

func (s *TagService) SearchTags(q string) ([]model.Tag, error) {
	return s.tagRepo.SearchTagsByName(q, 20)
}

func (s *TagService) SuggestTag(contentItemID int64, userID int64, tag, action string) error {
	content, err := s.contentRepo.FindByID(contentItemID)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if err := s.checkTagSuggestRateLimit(userID, contentItemID, time.Now()); err != nil {
		return err
	}
	suggestion := &model.TagSuggestion{
		ContentItemID: contentItemID,
		UserID:        userID,
		Tag:           tag,
		Action:        action,
		Status:        "pending",
	}
	return s.tagRepo.CreateTagSuggestion(suggestion)
}

func (s *TagService) checkTagSuggestRateLimit(userID, contentItemID int64, now time.Time) error {
	if s.rdb == nil {
		return nil
	}
	ctx := context.Background()
	key := buildTagSuggestRateLimitKey(userID, contentItemID, now)
	count, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return nil
	}
	if count == 1 {
		tagTTL := 48 * time.Hour
		if s.cfg != nil && s.cfg.TagCacheTTL > 0 {
			tagTTL = time.Duration(s.cfg.TagCacheTTL) * time.Second
		}
		s.rdb.Expire(ctx, key, tagTTL)
	}
	if count > 10 {
		return ErrTagSuggestRateLimited
	}
	return nil
}

func buildTagSuggestRateLimitKey(userID, contentItemID int64, now time.Time) string {
	return fmt.Sprintf("tag:suggest:%d:%d:%s", userID, contentItemID, now.Format("2006-01-02"))
}

func (s *TagService) ListTagSuggestions(contentItemID int64, callerID int64) ([]model.TagSuggestion, error) {
	content, err := s.contentRepo.FindByID(contentItemID)
	if err != nil || content == nil {
		return nil, ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return nil, ErrTagSuggestionForbidden
	}
	return s.tagRepo.ListTagSuggestions(contentItemID, "pending")
}

func (s *TagService) ApproveTagSuggestion(id int64, callerID int64) error {
	sg, err := s.tagRepo.FindTagSuggestion(id)
	if err != nil || sg == nil {
		return ErrTagSuggestionNotFound
	}
	content, err := s.contentRepo.FindByID(sg.ContentItemID)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return ErrTagSuggestionForbidden
	}
	if err := s.tagRepo.UpdateTagSuggestionStatus(id, "approved"); err != nil {
		return err
	}
	if sg.Action == "add" {
		s.contentRepo.AddTag(sg.ContentItemID, sg.Tag)
		s.tagRepo.IncrementUsage(sg.Tag)
	} else if sg.Action == "remove" {
		s.contentRepo.RemoveTag(sg.ContentItemID, sg.Tag)
	}
	return nil
}

func (s *TagService) RejectTagSuggestion(id int64, callerID int64) error {
	sg, err := s.tagRepo.FindTagSuggestion(id)
	if err != nil || sg == nil {
		return ErrTagSuggestionNotFound
	}
	content, err := s.contentRepo.FindByID(sg.ContentItemID)
	if err != nil || content == nil {
		return ErrContentNotFound
	}
	if content.AuthorID != callerID {
		return ErrTagSuggestionForbidden
	}
	return s.tagRepo.UpdateTagSuggestionStatus(id, "rejected")
}

func (s *TagService) CreateTagGroup(userID int64, name string, tags []string) (*model.TagGroup, error) {
	g := &model.TagGroup{
		UserID: userID,
		Name:   name,
		Tags:   tags,
	}
	if err := s.tagRepo.CreateTagGroup(g); err != nil {
		return nil, err
	}
	return g, nil
}

func (s *TagService) ListTagGroups(userID int64) ([]model.TagGroup, error) {
	return s.tagRepo.ListTagGroups(userID)
}

func (s *TagService) UpdateTagGroup(id int64, userID int64, updates map[string]interface{}) error {
	g, err := s.tagRepo.FindTagGroup(id)
	if err != nil || g == nil {
		return ErrTagGroupNotFound
	}
	if g.UserID != userID {
		return ErrTagGroupForbidden
	}
	return s.tagRepo.UpdateTagGroup(id, updates)
}

func (s *TagService) DeleteTagGroup(id int64, userID int64) error {
	g, err := s.tagRepo.FindTagGroup(id)
	if err != nil || g == nil {
		return ErrTagGroupNotFound
	}
	if g.UserID != userID {
		return ErrTagGroupForbidden
	}
	return s.tagRepo.DeleteTagGroup(id)
}

func (s *TagService) CreateSavedSearch(userID int64, name string, config model.JSONMap) (*model.SavedSearch, error) {
	ss := &model.SavedSearch{
		UserID: userID,
		Name:   name,
		Config: config,
	}
	if err := s.tagRepo.CreateSavedSearch(ss); err != nil {
		return nil, err
	}
	return ss, nil
}

func (s *TagService) ListSavedSearches(userID int64) ([]model.SavedSearch, error) {
	return s.tagRepo.ListSavedSearches(userID)
}

func (s *TagService) DeleteSavedSearch(id int64, userID int64) error {
	return s.tagRepo.DeleteSavedSearch(id, userID)
}
