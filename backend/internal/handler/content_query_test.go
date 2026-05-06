package handler

import (
	"reflect"
	"testing"
)

func TestParseCSVQuery(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{name: "empty query", raw: "", want: nil},
		{name: "single value", raw: "image", want: []string{"image"}},
		{name: "trims and drops blanks", raw: "audio, sheet_music, ,article", want: []string{"audio", "sheet_music", "article"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSVQuery(tt.raw)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseCSVQuery(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}
