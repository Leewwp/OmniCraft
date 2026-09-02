package repository

import (
	"omnicraft/backend/internal/model"

	"gorm.io/gorm"
)

type IPProposalRepository struct {
	db *gorm.DB
}

func NewIPProposalRepository(db *gorm.DB) *IPProposalRepository {
	return &IPProposalRepository{db: db}
}

func (r *IPProposalRepository) DB() *gorm.DB { return r.db }

func (r *IPProposalRepository) FindOpenByIP(ipID int64) (*model.IPProposal, error) {
	var p model.IPProposal
	err := r.db.Where("ip_id = ? AND status = ?", ipID, "open").First(&p).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *IPProposalRepository) FindByID(id int64) (*model.IPProposal, error) {
	var p model.IPProposal
	err := r.db.First(&p, id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

type ListIPProposalsFilter struct {
	IPID   int64
	Status string
	Query  string
	Page   int
	// PageSize <= 0 = 不分页（全量）；handler 层负责上限
	PageSize int
}

// applyListFilter centralizes the status/query semantics shared by List and
// Count so the tab-count total always matches what the list would return.
func (r *IPProposalRepository) applyListFilter(f ListIPProposalsFilter) *gorm.DB {
	q := r.db.Model(&model.IPProposal{}).Where("ip_id = ?", f.IPID)
	// "" = 改动历史视角：已生效时间线；"all" = 跨状态计数口径（tab 计数）；
	// 其余显式状态按各自口径。
	switch f.Status {
	case "open", "adopted", "rejected":
		q = q.Where("status = ?", f.Status)
	case "all":
	default:
		q = q.Where("status = ?", "adopted")
	}
	if f.Query != "" {
		q = q.Where("description_change LIKE ?", "%"+f.Query+"%")
	}
	return q
}

func (r *IPProposalRepository) List(f ListIPProposalsFilter) ([]model.IPProposal, error) {
	q := r.applyListFilter(f)
	order := "created_at DESC"
	switch f.Status {
	case "open":
		order = "deadline_at ASC, created_at DESC"
	case "", "adopted":
		order = "COALESCE(effective_at, closed_at) DESC"
	case "all", "rejected":
	}
	if f.PageSize > 0 {
		page := f.Page
		if page < 1 {
			page = 1
		}
		q = q.Offset((page - 1) * f.PageSize).Limit(f.PageSize)
	}
	var rows []model.IPProposal
	err := q.Preload("Proposer").Order(order).Find(&rows).Error
	return rows, err
}

// ListMyVotes returns the viewer's votes for a batch of proposals in one
// query (avoids the per-row N+1 in the list view).
func (r *IPProposalRepository) ListMyVotes(proposalIDs []int64, voterID int64) ([]model.IPProposalVote, error) {
	if len(proposalIDs) == 0 || voterID <= 0 {
		return nil, nil
	}
	var votes []model.IPProposalVote
	err := r.db.Where("proposal_id IN ? AND voter_id = ?", proposalIDs, voterID).Find(&votes).Error
	return votes, err
}

// Count returns the total matching the same filter semantics as List; it
// powers the proposals tab count and its search shrink (#290).
func (r *IPProposalRepository) Count(f ListIPProposalsFilter) (int64, error) {
	var total int64
	err := r.applyListFilter(f).Count(&total).Error
	return total, err
}

func (r *IPProposalRepository) FindVote(proposalID, voterID int64) (*model.IPProposalVote, error) {
	var v model.IPProposalVote
	err := r.db.Where("proposal_id = ? AND voter_id = ?", proposalID, voterID).First(&v).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *IPProposalRepository) ListVoterIDs(proposalID int64) ([]int64, error) {
	var ids []int64
	err := r.db.Model(&model.IPProposalVote{}).
		Where("proposal_id = ?", proposalID).
		Pluck("voter_id", &ids).Error
	return ids, err
}

func (r *IPProposalRepository) ListVersions(ipID int64) ([]model.IPProfileVersion, error) {
	var rows []model.IPProfileVersion
	err := r.db.Where("ip_id = ?", ipID).Order("created_at DESC").Find(&rows).Error
	return rows, err
}
