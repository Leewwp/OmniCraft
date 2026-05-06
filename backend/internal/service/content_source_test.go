package service

import (
	"testing"

	"omnicraft/backend/internal/model"
)

func TestValidateSourceOriginalLink(t *testing.T) {
	sourceID := int64(10)

	tests := []struct {
		name    string
		zone    string
		source  *model.ContentItem
		wantErr bool
	}{
		{name: "fanwork without source is allowed", zone: "fanwork", source: nil, wantErr: false},
		{name: "original without source is allowed", zone: "original", source: nil, wantErr: false},
		{name: "original cannot reference another original", zone: "original", source: &model.ContentItem{ID: sourceID, Zone: "original", Status: "published"}, wantErr: true},
		{name: "fanwork can reference published original", zone: "fanwork", source: &model.ContentItem{ID: sourceID, Zone: "original", Status: "published"}, wantErr: false},
		{name: "fanwork cannot reference fanwork", zone: "fanwork", source: &model.ContentItem{ID: sourceID, Zone: "fanwork", Status: "published"}, wantErr: true},
		{name: "fanwork cannot reference unpublished original", zone: "fanwork", source: &model.ContentItem{ID: sourceID, Zone: "original", Status: "pending"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSourceOriginalLink(tt.zone, tt.source)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateSourceOriginalLink() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
