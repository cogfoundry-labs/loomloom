package platform

import "testing"

func TestInferFromServer(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want ID
	}{
		{name: "shengsuanyun product host", raw: "https://loomloom.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun test host", raw: "https://loomloom-test.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun uppercase host", raw: "https://LOOMLOOM.SHENGSUANYUN.COM/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun api host", raw: "https://api.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun typo subdomain still platform-owned", raw: "https://loomloomdds.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun bare host", raw: "shengsuanyun.com", want: ShengSuanYun},
		{name: "domain suffix attack does not match", raw: "https://evilshengsuanyun.com/loom/v1", want: Unknown},
		{name: "domain suffix attack with extra label does not match", raw: "https://shengsuanyun.com.evil.test/loom/v1", want: Unknown},
		{name: "cogfoundry host", raw: "https://api.cogfoundry.ai/loom/v1", want: CogFoundry},
		{name: "cogfoundry uppercase host", raw: "https://API.COGFOUNDRY.AI/loom/v1", want: CogFoundry},
		{name: "unknown host", raw: "https://example.com/loom/v1", want: Unknown},
		{name: "localhost", raw: "127.0.0.1:8080", want: Unknown},
		{name: "empty", raw: "", want: Unknown},
		{name: "malformed", raw: "http://[::1", want: Unknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferFromServer(tt.raw)
			if got.ID != tt.want {
				t.Fatalf("InferFromServer(%q)=%q want %q", tt.raw, got.ID, tt.want)
			}
		})
	}
}

func TestCogFoundryIsNotOperational(t *testing.T) {
	got, ok := ByID(CogFoundry)
	if !ok {
		t.Fatal("CogFoundry not registered")
	}
	if got.Operational {
		t.Fatal("CogFoundry should not be operational in MVP")
	}
	if got.KeysURL != "" || got.RechargeURL != "" {
		t.Fatalf("CogFoundry should not expose console URLs: %+v", got)
	}
}
