package rediskeys

import "testing"

func TestPublishFreezeKey(t *testing.T) {
	tests := []struct {
		userID int64
		want   string
	}{
		{userID: 1, want: "publish:freeze:1"},
		{userID: 42, want: "publish:freeze:42"},
	}
	for _, tt := range tests {
		if got := PublishFreezeKey(tt.userID); got != tt.want {
			t.Fatalf("PublishFreezeKey(%d) = %q, want %q", tt.userID, got, tt.want)
		}
	}
}
