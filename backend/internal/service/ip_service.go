package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"omnicraft/backend/internal/model"
	"omnicraft/backend/internal/repository"
)

var (
	ErrIPNotFound    = errors.New("ip not found")
	ErrIPSlugTaken   = errors.New("slug already taken")
	ErrIPForbidden   = errors.New("forbidden")
)

type IPService struct {
	ipRepo *repository.IPRepository
}

func NewIPService(ipRepo *repository.IPRepository) *IPService {
	return &IPService{ipRepo: ipRepo}
}

type CreateIPInput struct {
	Name        string `json:"name" binding:"required,min=1,max=255"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	Category    string `json:"category"`
	Tags        []string `json:"tags"`
}

func (s *IPService) CreateIP(input CreateIPInput, creatorID int64) (*model.IP, error) {
	slug := generateSlug(input.Name)

	existing, err := s.ipRepo.FindBySlug(slug)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		slug = fmt.Sprintf("%s-%d", slug, creatorID)
	}

	ip := &model.IP{
		Name:        input.Name,
		Slug:        slug,
		Description: input.Description,
		CoverURL:    input.CoverURL,
		Category:    input.Category,
		CreatorID:   &creatorID,
		Status:      "pending",
	}

	if err := s.ipRepo.CreateIP(ip); err != nil {
		return nil, err
	}

	return ip, nil
}

func (s *IPService) GetIP(id int64) (*model.IP, error) {
	ip, err := s.ipRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if ip == nil {
		return nil, ErrIPNotFound
	}
	return ip, nil
}

func (s *IPService) ListIPs(filter repository.ListIPsFilter) ([]model.IP, int64, error) {
	return s.ipRepo.ListIPs(filter)
}

func (s *IPService) ApproveIP(id int64) error {
	ip, err := s.ipRepo.FindByID(id)
	if err != nil || ip == nil {
		return ErrIPNotFound
	}
	return s.ipRepo.UpdateStatus(id, "approved")
}

func (s *IPService) RejectIP(id int64) error {
	ip, err := s.ipRepo.FindByID(id)
	if err != nil || ip == nil {
		return ErrIPNotFound
	}
	return s.ipRepo.UpdateStatus(id, "rejected")
}

func (s *IPService) BanIP(id int64) error {
	return s.ipRepo.BanIPAndContents(id)
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9-]`)
var multiDash = regexp.MustCompile(`-+`)

func generateSlug(name string) string {
	lower := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '-'
		}
		return unicode.ToLower(r)
	}, name)

	slug := nonAlphanumeric.ReplaceAllString(lower, "")
	slug = multiDash.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	if slug == "" {
		slug = fmt.Sprintf("ip-%d", len(name))
	}
	if len(slug) > 200 {
		slug = slug[:200]
	}
	return slug
}
