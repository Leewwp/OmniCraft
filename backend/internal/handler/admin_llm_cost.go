package handler

import (
	"net/http"
	"omnicraft/backend/internal/pkg/response"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"omnicraft/backend/config"
	"omnicraft/backend/internal/repository"
)

// costWindowDefault is the ledger window when the caller sends no bounds.
const costWindowDefault = 7 * 24 * time.Hour

// topConversationsLimit caps the "top conversations by spend" table.
const topConversationsLimit = 10

// AdminLLMCostHandler serves the token/cost ledger (SP-21 T7): terminal
// llm_round trace-node tokens priced at query time by the agent.models rate
// table (a rate change re-values history). Models without a configured rate
// are aggregated but never priced ("unknown 不估算").
type AdminLLMCostHandler struct {
	repo *repository.AgentTraceRepository
	cfg  *config.Config
}

func NewAdminLLMCostHandler(repo *repository.AgentTraceRepository, cfg *config.Config) *AdminLLMCostHandler {
	return &AdminLLMCostHandler{repo: repo, cfg: cfg}
}

type llmCostRateView struct {
	In  float64 `json:"in_per_m_tokens"`
	Out float64 `json:"out_per_m_tokens"`
}

type llmCostModelRow struct {
	Model     string  `json:"model"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
	CostCNY   float64 `json:"cost_cny"`
	Estimated bool    `json:"estimated"`
}

type llmCostDayRow struct {
	Day       string  `json:"day"`
	TokensIn  int64   `json:"tokens_in"`
	TokensOut int64   `json:"tokens_out"`
	CostCNY   float64 `json:"cost_cny"`
}

type llmCostConversationRow struct {
	ConversationID int64   `json:"conversation_id"`
	Turns          int64   `json:"turns"`
	TokensIn       int64   `json:"tokens_in"`
	TokensOut      int64   `json:"tokens_out"`
	CostCNY        float64 `json:"cost_cny"`
	Estimated      bool    `json:"estimated"`
}

type llmCostTotals struct {
	TokensIn       int64   `json:"tokens_in"`
	TokensOut      int64   `json:"tokens_out"`
	CostCNY        float64 `json:"cost_cny"`
	UnpricedModels int     `json:"unpriced_models"`
}

// rateTable maps every model label the ledger can bucket (lowercased — the
// ledger buckets are case-normalized) to CNY per 1M tokens from the SP-20
// models registry. Entries with no usable rate stay out (unknown rate).
func (h *AdminLLMCostHandler) rateTable() map[string]llmCostRateView {
	rates := map[string]llmCostRateView{}
	if h.cfg == nil {
		return rates
	}
	for _, m := range h.cfg.Agent.Models {
		if m.Model == "" || (m.CostInPerMTokens <= 0 && m.CostOutPerMTokens <= 0) {
			continue
		}
		view := llmCostRateView{In: m.CostInPerMTokens, Out: m.CostOutPerMTokens}
		// Trace rows carry whichever model label served the turn
		// (servingModel): the config model name for primary turns
		// ("deepseek-chat"), the registry id for preference-pinned or
		// failover turns ("deepseek"), the display name lowercased for the
		// single-provider wiring ("minimax-m3"). Index every spelling so
		// each label prices; the model-name key wins on collision.
		rates[strings.ToLower(m.Model)] = view
		if id := strings.ToLower(strings.TrimSpace(m.ID)); id != "" {
			if _, dup := rates[id]; !dup {
				rates[id] = view
			}
		}
	}
	return rates
}

func priceTokens(rates map[string]llmCostRateView, model string, tokensIn, tokensOut int64) (float64, bool) {
	rate, ok := rates[model]
	if !ok {
		return 0, false
	}
	return float64(tokensIn)/1_000_000*rate.In + float64(tokensOut)/1_000_000*rate.Out, true
}

// parseCostWindow resolves the from/to bounds: both optional RFC3339; a
// missing bound is filled so the window always spans costWindowDefault at
// most (from defaults to to-7d, to defaults to now).
func (h *AdminLLMCostHandler) parseCostWindow(c *gin.Context) (*time.Time, *time.Time, bool) {
	var from, to *time.Time
	if raw := c.Query("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_ARGS", "from must be RFC3339")
			return nil, nil, false
		}
		from = &t
	}
	if raw := c.Query("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_ARGS", "to must be RFC3339")
			return nil, nil, false
		}
		to = &t
	}
	if from == nil {
		anchor := time.Now()
		if to != nil {
			anchor = *to
		}
		defaultFrom := anchor.Add(-costWindowDefault)
		from = &defaultFrom
	}
	if to == nil {
		now := time.Now()
		to = &now
	}
	return from, to, true
}

// Ledger returns the cost dashboard payload: rate table echo, totals, rows
// by model / by day and the top conversations by spend.
func (h *AdminLLMCostHandler) Ledger(c *gin.Context) {
	from, to, ok := h.parseCostWindow(c)
	if !ok {
		return
	}
	dayModel, err := h.repo.AggregateLLMCostsByDayModel(c.Request.Context(), from, to)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to aggregate llm costs")
		return
	}
	convModel, err := h.repo.AggregateLLMCostsByConversationModel(c.Request.Context(), from, to)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to aggregate llm costs")
		return
	}

	rates := h.rateTable()
	totals := llmCostTotals{}
	unpriced := map[string]bool{}

	byModelIdx := map[string]int{}
	byModel := []llmCostModelRow{}
	byDayIdx := map[string]int{}
	byDay := []llmCostDayRow{}
	for _, cell := range dayModel {
		cost, priced := priceTokens(rates, cell.Model, cell.TokensIn, cell.TokensOut)
		if !priced {
			unpriced[cell.Model] = true
		}
		if mi, ok := byModelIdx[cell.Model]; ok {
			byModel[mi].TokensIn += cell.TokensIn
			byModel[mi].TokensOut += cell.TokensOut
			byModel[mi].CostCNY += cost
		} else {
			byModelIdx[cell.Model] = len(byModel)
			byModel = append(byModel, llmCostModelRow{
				Model: cell.Model, TokensIn: cell.TokensIn, TokensOut: cell.TokensOut,
				CostCNY: cost, Estimated: priced,
			})
		}
		if di, ok := byDayIdx[cell.Day]; ok {
			byDay[di].TokensIn += cell.TokensIn
			byDay[di].TokensOut += cell.TokensOut
			byDay[di].CostCNY += cost
		} else {
			byDayIdx[cell.Day] = len(byDay)
			byDay = append(byDay, llmCostDayRow{
				Day: cell.Day, TokensIn: cell.TokensIn, TokensOut: cell.TokensOut, CostCNY: cost,
			})
		}
		totals.TokensIn += cell.TokensIn
		totals.TokensOut += cell.TokensOut
		totals.CostCNY += cost
	}
	sort.Slice(byModel, func(i, j int) bool {
		return byModel[i].TokensIn+byModel[i].TokensOut > byModel[j].TokensIn+byModel[j].TokensOut
	})
	totals.UnpricedModels = len(unpriced)

	// Conversation rows aggregate their model cells; a conversation is
	// "estimated" only when every model it used carries a rate.
	convIdx := map[int64]int{}
	convs := []llmCostConversationRow{}
	convPriced := map[int64]bool{}
	for _, cell := range convModel {
		cost, priced := priceTokens(rates, cell.Model, cell.TokensIn, cell.TokensOut)
		if ci, ok := convIdx[cell.ConversationID]; ok {
			convs[ci].TokensIn += cell.TokensIn
			convs[ci].TokensOut += cell.TokensOut
			convs[ci].CostCNY += cost
			convPriced[cell.ConversationID] = convPriced[cell.ConversationID] && priced
		} else {
			convIdx[cell.ConversationID] = len(convs)
			convs = append(convs, llmCostConversationRow{
				ConversationID: cell.ConversationID, TokensIn: cell.TokensIn,
				TokensOut: cell.TokensOut, CostCNY: cost,
			})
			convPriced[cell.ConversationID] = priced
		}
	}
	sort.Slice(convs, func(i, j int) bool {
		if convs[i].CostCNY != convs[j].CostCNY {
			return convs[i].CostCNY > convs[j].CostCNY
		}
		return convs[i].TokensIn+convs[i].TokensOut > convs[j].TokensIn+convs[j].TokensOut
	})
	if len(convs) > topConversationsLimit {
		convs = convs[:topConversationsLimit]
	}
	ids := make([]int64, 0, len(convs))
	for _, row := range convs {
		ids = append(ids, row.ConversationID)
	}
	turns, err := h.repo.CountRunsByConversationIDs(c.Request.Context(), ids, from, to)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DB_ERROR", "failed to aggregate llm costs")
		return
	}
	for i := range convs {
		convs[i].Turns = turns[convs[i].ConversationID]
		convs[i].Estimated = convPriced[convs[i].ConversationID]
	}

	c.JSON(http.StatusOK, gin.H{
		"window":            gin.H{"from": from, "to": to},
		"rates":             rates,
		"totals":            totals,
		"by_model":          byModel,
		"by_day":            byDay,
		"top_conversations": convs,
	})
}
