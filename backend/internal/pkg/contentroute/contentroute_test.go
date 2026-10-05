package contentroute

import "testing"

// 路由表（票面验收）：original、fanwork、空/未知 zone；生成与引用验真使用同一
// 规则（service 层两侧均调用本函数，见 agent_tools.go）；IP 引用不属本 module。
func TestContentDetailRoute(t *testing.T) {
	cases := []struct {
		name string
		zone string
		id   int64
		want string
	}{
		{name: "original zone routes to the original detail page", zone: "original", id: 101, want: "/original/101"},
		{name: "fanwork zone keeps the shared content fallback", zone: "fanwork", id: 102, want: "/content/102"},
		{name: "empty zone falls back to the shared content route", zone: "", id: 103, want: "/content/103"},
		{name: "unknown zone falls back to the shared content route", zone: "whatever", id: 104, want: "/content/104"},
		{name: "zone match is exact, not case-insensitive", zone: "Original", id: 105, want: "/content/105"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ContentDetailRoute(tc.zone, tc.id); got != tc.want {
				t.Fatalf("ContentDetailRoute(%q, %d) = %q, want %q", tc.zone, tc.id, got, tc.want)
			}
		})
	}
}
