package authflow

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteCallbackPageVariants(t *testing.T) {
	tests := []struct {
		name           string
		variant        CallbackPageVariant
		success        bool
		wantStatus     int
		wantLanguage   string
		wantPageStatus string
		wantRole       string
		wantText       []string
		unwantedText   []string
	}{
		{
			name:           "CogFoundry success",
			variant:        CallbackPageCogFoundry,
			success:        true,
			wantStatus:     http.StatusOK,
			wantLanguage:   "en",
			wantPageStatus: "success",
			wantRole:       "status",
			wantText: []string{
				"cogfoundry",
				"CogFoundry has sent the authorization response",
				"Authorization received",
				"Return to your terminal",
				"You can safely close this page",
			},
			unwantedText: []string{"胜算云", "请返回终端", "Authorization service"},
		},
		{
			name:           "CogFoundry failure",
			variant:        CallbackPageCogFoundry,
			success:        false,
			wantStatus:     http.StatusBadRequest,
			wantLanguage:   "en",
			wantPageStatus: "error",
			wantRole:       "alert",
			wantText: []string{
				"cogfoundry",
				"Authorization not completed",
				"Return to your terminal and try again",
				"loomloom login",
				"No authorization information was saved",
			},
			unwantedText: []string{"胜算云", "授权未完成", "Authorization received"},
		},
		{
			name:           "ShengSuanYun success",
			variant:        CallbackPageShengSuanYun,
			success:        true,
			wantStatus:     http.StatusOK,
			wantLanguage:   "zh-CN",
			wantPageStatus: "success",
			wantRole:       "status",
			wantText: []string{
				"胜算云",
				"胜算云已将授权响应发送给 LoomLoom CLI",
				"授权已返回",
				"请返回终端",
				"此页面现在可以安全关闭",
			},
			unwantedText: []string{"CogFoundry", "cogfoundry", "Authorization service", "Return to your terminal"},
		},
		{
			name:           "ShengSuanYun failure",
			variant:        CallbackPageShengSuanYun,
			success:        false,
			wantStatus:     http.StatusBadRequest,
			wantLanguage:   "zh-CN",
			wantPageStatus: "error",
			wantRole:       "alert",
			wantText: []string{
				"胜算云",
				"授权未完成",
				"请返回终端后重试",
				"重新运行：",
				"loomloom login",
				"未保存任何授权信息",
			},
			unwantedText: []string{"CogFoundry", "cogfoundry", "Authorization not completed"},
		},
		{
			name:           "missing variant uses generic fallback",
			variant:        CallbackPageGeneric,
			success:        true,
			wantStatus:     http.StatusOK,
			wantLanguage:   "en",
			wantPageStatus: "success",
			wantRole:       "status",
			wantText:       []string{"Authorization service", "Authorization received", "Return to your terminal"},
			unwantedText:   []string{"CogFoundry", "cogfoundry", "胜算云"},
		},
		{
			name:           "unknown variant uses generic fallback",
			variant:        CallbackPageVariant("<script>unknown-platform</script>"),
			success:        false,
			wantStatus:     http.StatusBadRequest,
			wantLanguage:   "en",
			wantPageStatus: "error",
			wantRole:       "alert",
			wantText:       []string{"Authorization service", "Authorization not completed", "loomloom login"},
			unwantedText:   []string{"unknown-platform", "<script>", "CogFoundry", "cogfoundry", "胜算云"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := httptest.NewRecorder()

			writeCallbackPage(response, tt.variant, tt.success)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d want %d", response.Code, tt.wantStatus)
			}
			assertCallbackPageHeaders(t, response.Header(), tt.wantLanguage)

			body := response.Body.String()
			for _, want := range tt.wantText {
				if !strings.Contains(body, want) {
					t.Errorf("body does not contain %q", want)
				}
			}
			for _, unwanted := range tt.unwantedText {
				if strings.Contains(body, unwanted) {
					t.Errorf("body unexpectedly contains %q", unwanted)
				}
			}
			if !strings.Contains(body, `data-status="`+tt.wantPageStatus+`"`) {
				t.Errorf("body does not identify the %q state", tt.wantPageStatus)
			}
			if !strings.Contains(body, `role="`+tt.wantRole+`"`) {
				t.Errorf("body does not use the %q accessibility role", tt.wantRole)
			}
			if strings.Contains(body, "{{") {
				t.Error("body contains an unresolved template action")
			}
		})
	}
}

func TestCallbackPagesAreSelfContainedAndUnanimated(t *testing.T) {
	for _, variant := range []CallbackPageVariant{
		CallbackPageCogFoundry,
		CallbackPageShengSuanYun,
		CallbackPageGeneric,
	} {
		t.Run(string(variant), func(t *testing.T) {
			response := httptest.NewRecorder()
			writeCallbackPage(response, variant, true)
			body := response.Body.String()

			for _, forbidden := range []string{
				"<script",
				"<link",
				"<iframe",
				"<object",
				"<embed",
				"@import",
				"url(",
				" src=",
				"animation:",
				"transition:",
			} {
				if strings.Contains(body, forbidden) {
					t.Errorf("callback page contains forbidden external or animated styling %q", forbidden)
				}
			}

			for _, required := range []string{
				`aria-labelledby="page-title"`,
				`aria-describedby="page-description"`,
				"background: var(--page-bg)",
			} {
				if !strings.Contains(body, required) {
					t.Errorf("callback page does not contain %q", required)
				}
			}
		})
	}
}

func TestCallbackPageBrandThemesAreDistinct(t *testing.T) {
	cogFoundry := httptest.NewRecorder()
	writeCallbackPage(cogFoundry, CallbackPageCogFoundry, true)
	if body := cogFoundry.Body.String(); !strings.Contains(body, "color-scheme: dark") || !strings.Contains(body, "--page-bg: #0b0c0e") {
		t.Fatal("CogFoundry callback page must retain its dark theme")
	}

	shengSuanYun := httptest.NewRecorder()
	writeCallbackPage(shengSuanYun, CallbackPageShengSuanYun, true)
	if body := shengSuanYun.Body.String(); !strings.Contains(body, "color-scheme: light") || !strings.Contains(body, "--brand: #5742ee") || !strings.Contains(body, "--surface: #ffffff") {
		t.Fatal("ShengSuanYun callback page must use its light purple theme")
	}
}

func assertCallbackPageHeaders(t *testing.T, header http.Header, language string) {
	t.Helper()

	wantHeaders := map[string]string{
		"Cache-Control":          "no-store",
		"Content-Language":       language,
		"Content-Type":           "text/html; charset=utf-8",
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
	}
	for name, want := range wantHeaders {
		if got := header.Get(name); got != want {
			t.Errorf("%s = %q want %q", name, got, want)
		}
	}

	directives := make(map[string]string)
	for _, directive := range strings.Split(header.Get("Content-Security-Policy"), ";") {
		fields := strings.Fields(directive)
		if len(fields) == 0 {
			continue
		}
		directives[fields[0]] = strings.Join(fields[1:], " ")
	}

	wantDirectives := map[string]string{
		"default-src":     "'none'",
		"style-src":       "'unsafe-inline'",
		"base-uri":        "'none'",
		"form-action":     "'none'",
		"frame-ancestors": "'none'",
	}
	for name, want := range wantDirectives {
		if got := directives[name]; got != want {
			t.Errorf("CSP %s = %q want %q", name, got, want)
		}
	}

	for _, sourceDirective := range []string{"script-src", "img-src", "font-src", "connect-src"} {
		if value, exists := directives[sourceDirective]; exists && value != "'none'" {
			t.Errorf("CSP %s must not loosen default-src: %q", sourceDirective, value)
		}
	}
}
