package service

import (
	"errors"
	"testing"

	"omnicraft/backend/internal/model"
)

func TestValidateSource(t *testing.T) {
	publishedOriginal := &model.ContentItem{ID: 1, Zone: "original", Status: "published"}
	publishedFanwork := &model.ContentItem{ID: 2, Zone: "fanwork", Status: "published"}

	tests := []struct {
		name           string
		zone           string
		ipID           *int64
		sourceOriginal *model.ContentItem
		sourceFanwork  *model.ContentItem
		wantErr        error
	}{
		{
			name:           "original with source_original_id is rejected",
			zone:           "original",
			sourceOriginal: publishedOriginal,
			wantErr:        ErrSourceNotAllowedForOriginal,
		},
		{
			name:          "original with source_fanwork_id is rejected",
			zone:          "original",
			sourceFanwork: publishedFanwork,
			wantErr:       ErrSourceNotAllowedForOriginal,
		},
		{
			name:    "fanwork without ip or source is rejected",
			zone:    "fanwork",
			wantErr: ErrFanworkSourceRequired,
		},
		{
			name:           "fanwork with both sources is rejected",
			zone:           "fanwork",
			sourceOriginal: publishedOriginal,
			sourceFanwork:  publishedFanwork,
			wantErr:        ErrMultipleSourceConflict,
		},
		{
			name:           "source original not original/published is unavailable",
			zone:           "fanwork",
			sourceOriginal: publishedFanwork,
			wantErr:        ErrSourceOriginalUnavailable,
		},
		{
			name:          "source fanwork not fanwork/published is unavailable",
			zone:          "fanwork",
			sourceFanwork: publishedOriginal,
			wantErr:       ErrSourceFanworkUnavailable,
		},
		{
			name: "ip-only fanwork succeeds",
			zone: "fanwork",
			ipID: ptrInt64(7),
		},
		{
			name:           "original-source fanwork succeeds",
			zone:           "fanwork",
			sourceOriginal: publishedOriginal,
		},
		{
			name:          "fanwork-source fanwork succeeds",
			zone:          "fanwork",
			sourceFanwork: publishedFanwork,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceLink(tt.zone, tt.ipID, tt.sourceOriginal, tt.sourceFanwork)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("validateSourceLink() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateSourceLink() error = %v, want nil", err)
			}
		})
	}
}
