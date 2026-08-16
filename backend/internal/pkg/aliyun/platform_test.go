package aliyun

import "testing"

func TestIsPlatformObjectURL(t *testing.T) {
	const domain = "https://cdn.example.test/"
	cases := []struct {
		name   string
		domain string
		rawURL string
		want   bool
	}{
		{"plain object under domain", domain, domain + "uploads/7/avatar/ok.png", true},
		{"multiple trailing slashes tolerated on domain", "https://cdn.example.test///", domain + "uploads/7/avatar/ok.png", true},
		{"relative leading-slash path rejected (not a prefix of domain)", domain, "/uploads/7/avatar/ok.png", false},
		{"external url rejected", domain, "https://evil.example/uploads/7/avatar/ok.png", false},
		{"empty url rejected", domain, "", false},
		{"whitespace-only url rejected", domain, "   ", false},
		{"bare domain without object prefix rejected", domain, "https://cdn.example.test", false},
		{"domain subdomain confusion rejected", domain, "https://cdn.example.test.evil/ok.png", false},
		{"no domain never verifies", "", "https://cdn.example.test/uploads/7/avatar/ok.png", false},
		{"whitespace domain never verifies", "   ", "https://cdn.example.test/uploads/7/avatar/ok.png", false},
		{"sibling path that is not a prefix", domain, "https://cdn.example.test" + "x/uploads/7/avatar/ok.png", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsPlatformObjectURL(tc.domain, tc.rawURL); got != tc.want {
				t.Fatalf("IsPlatformObjectURL(%q, %q) = %v, want %v", tc.domain, tc.rawURL, got, tc.want)
			}
		})
	}
}

func TestObjectURL(t *testing.T) {
	const domain = "https://cdn.example.test/"
	cases := []struct {
		name   string
		domain string
		key    string
		want   string
	}{
		{"key under domain", domain, "uploads/7/avatar/ok.png", domain + "uploads/7/avatar/ok.png"},
		{"leading slash key normalized", domain, "/uploads/7/avatar/ok.png", domain + "uploads/7/avatar/ok.png"},
		{"trailing slash domain normalized", "https://cdn.example.test/", "uploads/7/avatar/ok.png", domain + "uploads/7/avatar/ok.png"},
		{"empty domain returns key unchanged", "", "uploads/7/avatar/ok.png", "uploads/7/avatar/ok.png"},
		{"whitespace domain returns key unchanged", "  ", "uploads/7/avatar/ok.png", "uploads/7/avatar/ok.png"},
		{"empty key keeps domain slash", domain, "", domain},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ObjectURL(tc.domain, tc.key); got != tc.want {
				t.Fatalf("ObjectURL(%q, %q) = %q, want %q", tc.domain, tc.key, got, tc.want)
			}
		})
	}
}
