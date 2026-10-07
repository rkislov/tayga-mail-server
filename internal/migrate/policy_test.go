package migrate

import "testing"

func TestResolveAllowed(t *testing.T) {
	cases := []struct {
		user, domain string
		cos          CoSFeatures
		want         bool
	}{
		{"", "", nil, false},
		{"inherit", "inherit", nil, false},
		{"inherit", "inherit", CoSFeatures{"migration": true}, true},
		{"inherit", "inherit", CoSFeatures{"migration": false}, false},
		{"on", "off", CoSFeatures{"migration": false}, true},
		{"off", "on", CoSFeatures{"migration": true}, false},
		{"inherit", "on", CoSFeatures{"migration": false}, true},
		{"inherit", "off", CoSFeatures{"migration": true}, false},
	}
	for _, tc := range cases {
		got := ResolveAllowed(tc.user, tc.domain, tc.cos)
		if got != tc.want {
			t.Fatalf("user=%q domain=%q cos=%v got %v want %v", tc.user, tc.domain, tc.cos, got, tc.want)
		}
	}
}
