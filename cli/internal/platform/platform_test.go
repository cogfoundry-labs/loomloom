package platform

import (
	"strings"
	"testing"
)

func TestInferFromServer(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want ID
	}{
		{name: "shengsuanyun", raw: "https://loomloom.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "shengsuanyun subdomain", raw: "https://api.test.shengsuanyun.com/loom/v1", want: ShengSuanYun},
		{name: "cogfoundry", raw: "https://api.cogfoundry.ai/loom/v1", want: CogFoundry},
		{name: "suffix attack", raw: "https://evilcogfoundry.ai.example.com/loom/v1", want: Custom},
		{name: "custom", raw: "https://example.com/loom/v1", want: Custom},
		{name: "localhost", raw: "127.0.0.1:8080/loom/v1", want: Custom},
		{name: "empty", raw: "", want: Unknown},
		{name: "malformed", raw: "http://[::1", want: Unknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := InferFromServer(tt.raw).ID; got != tt.want {
				t.Fatalf("InferFromServer(%q)=%q want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCogFoundryIsOperationalWithKnownEndpoints(t *testing.T) {
	got, ok := ByID(CogFoundry)
	if !ok || !got.Operational {
		t.Fatalf("CogFoundry=%+v ok=%t want operational", got, ok)
	}
	for name, values := range map[string][2]string{
		"keys URL":       {got.KeysURL, "https://console.cogfoundry.ai/api-keys"},
		"recharge URL":   {got.RechargeURL, "https://console.cogfoundry.ai/credits"},
		"default server": {got.DefaultServer, "https://loomloom.cogfoundry.ai/loom/v1"},
	} {
		if values[0] != values[1] {
			t.Fatalf("CogFoundry %s=%q want %q", name, values[0], values[1])
		}
	}
	if got.AuthPageURL != "" || got.AccountAPIURL != "" {
		t.Fatalf("CogFoundry must not enable browser login: %+v", got)
	}
}

func TestNormalizeServer(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "remote defaults https", raw: "API.Example.com/loom/v1/", want: "https://api.example.com/loom/v1"},
		{name: "default https port removed", raw: "https://API.Example.com:443/loom/v1/", want: "https://api.example.com/loom/v1"},
		{name: "loopback defaults http", raw: "127.0.0.1:8080/loom/v1/", want: "http://127.0.0.1:8080/loom/v1"},
		{name: "localhost http", raw: "http://localhost:8080/loom/v1", want: "http://localhost:8080/loom/v1"},
		{name: "remote http rejected", raw: "http://api.example.com/loom/v1", wantErr: "must use https"},
		{name: "query rejected", raw: "https://api.example.com/loom/v1?x=1", wantErr: "must not contain a query"},
		{name: "fragment rejected", raw: "https://api.example.com/loom/v1#x", wantErr: "must not contain a fragment"},
		{name: "userinfo rejected", raw: "https://user:pass@api.example.com/loom/v1", wantErr: "must not contain user information"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeServer(tt.raw)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NormalizeServer error=%v want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("NormalizeServer=%q error=%v want %q", got, err, tt.want)
			}
		})
	}
}

func TestGenerateProfileNameAndTokenEnv(t *testing.T) {
	tests := []struct {
		server   string
		platform ID
		wantName string
		wantEnv  string
	}{
		{
			server:   "https://loomloom.cogfoundry.ai/loom/v1",
			platform: CogFoundry,
			wantName: "cogfoundry",
			wantEnv:  "LOOMLOOM_TOKEN_COGFOUNDRY",
		},
		{
			server:   "https://loomloom-integration.test.cogfoundry.ai/loom/v1",
			platform: CogFoundry,
			wantName: "cogfoundry-integration-test",
			wantEnv:  "LOOMLOOM_TOKEN_COGFOUNDRY_INTEGRATION_TEST",
		},
		{
			server:   "https://api.company.com/loom/v1",
			platform: Custom,
			wantName: "custom-api-company-com",
			wantEnv:  "LOOMLOOM_TOKEN_CUSTOM_API_COMPANY_COM",
		},
		{
			server:   "http://localhost:8080/loom/v1",
			platform: Custom,
			wantName: "custom-localhost-p8080",
			wantEnv:  "LOOMLOOM_TOKEN_CUSTOM_LOCALHOST_P8080",
		},
	}
	for _, tt := range tests {
		name, err := GenerateProfileName(tt.server, tt.platform, nil, "")
		if err != nil {
			t.Fatalf("GenerateProfileName(%q) error=%v", tt.server, err)
		}
		if name != tt.wantName || TokenEnvName(name) != tt.wantEnv {
			t.Fatalf("name=%q env=%q want name=%q env=%q", name, TokenEnvName(name), tt.wantName, tt.wantEnv)
		}
	}
}

func TestGenerateProfileNameUsesStableHashOnCollision(t *testing.T) {
	const first = "https://api.company.com/loom/v1"
	const second = "https://api.company.com/other/v1"
	existing := []Profile{{Name: "custom-api-company-com", Server: first}}
	got, err := GenerateProfileName(second, Custom, existing, "")
	if err != nil {
		t.Fatalf("GenerateProfileName error=%v", err)
	}
	if !strings.HasPrefix(got, "custom-api-company-com-") || len(got) != len("custom-api-company-com-")+8 {
		t.Fatalf("name=%q want base plus 8-character hash", got)
	}
	again, err := GenerateProfileName(second, Custom, existing, "")
	if err != nil || again != got {
		t.Fatalf("second generation=%q error=%v want stable %q", again, err, got)
	}
}

func TestGenerateProfileNameRejectsConflictingRequestedName(t *testing.T) {
	existing := []Profile{{Name: "company", Server: "https://one.example.com/loom/v1"}}
	_, err := GenerateProfileName("https://two.example.com/loom/v1", Custom, existing, "company")
	if err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("error=%v want name conflict", err)
	}
}
