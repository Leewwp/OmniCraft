package service

import (
	"testing"

	"omnicraft/backend/config"
)

func TestMinScoreForInteraction(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want int
	}{
		{name: "nil config uses product default", cfg: nil, want: 3},
		{name: "missing configured score uses product default", cfg: &config.Config{}, want: 3},
		{name: "configured score is used", cfg: &config.Config{Reputation: config.ReputationConfig{MinScoreForInteraction: 5}}, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := minScoreForInteraction(tt.cfg)
			if got != tt.want {
				t.Fatalf("minScoreForInteraction() = %d, want %d", got, tt.want)
			}
		})
	}
}
