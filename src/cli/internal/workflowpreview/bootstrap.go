package workflowpreview

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/url"
	"path"
	"strings"
)

// ViewerURL is explicit deployment configuration, never inferred from a cloud API.
func validateViewerURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" || path.Clean(u.Path) != u.Path || !strings.HasSuffix(u.Path, "/viewer/v1/bootstrap.js") {
		return nil, fmt.Errorf("set --viewer-url or LOOMLOOM_VIEWER_BOOTSTRAP_URL to the published HTTPS /viewer/v1/bootstrap.js address")
	}
	return u, nil
}

var startupPage = template.Must(template.New("preview").Parse(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>LoomLoom 本机预览</title></head>
<body><div id="viewer-startup" role="status">正在加载工作流查看器…</div><div id="root"></div>
<script nonce="{{.Nonce}}">
(() => {
 const status = document.getElementById('viewer-startup');
 let loaded = false;
 const fail = () => { if (!loaded) { status.textContent = '查看器资源加载失败。请检查网络及查看器地址，然后刷新；本地文件未上传。'; } };
 const timer = setTimeout(fail, 15000);
 window.addEventListener('loomloom-viewer-ready', () => { loaded = true; clearTimeout(timer); status.remove(); }, {once:true});
 window.addEventListener('loomloom-viewer-error', fail);
 const entry = document.createElement('script');
 entry.type = 'module'; entry.crossOrigin = 'anonymous'; entry.referrerPolicy = 'no-referrer';
 entry.src = {{.ViewerURL}};
 entry.addEventListener('error', fail); document.head.appendChild(entry);
})();
</script></body></html>`))

func bootstrap(viewer *url.URL) ([]byte, string, error) {
	random := make([]byte, 24)
	if _, err := rand.Read(random); err != nil {
		return nil, "", err
	}
	nonce := base64.RawStdEncoding.EncodeToString(random)
	var body bytes.Buffer
	if err := startupPage.Execute(&body, struct{ Nonce, ViewerURL string }{nonce, viewer.String()}); err != nil {
		return nil, "", err
	}
	origin := viewer.Scheme + "://" + viewer.Host
	policy := "default-src 'none'; script-src 'nonce-" + nonce + "' " + origin + "; style-src 'unsafe-inline' " + origin + "; img-src data: " + origin + "; font-src " + origin + "; connect-src 'self'; worker-src 'self' blob:; frame-ancestors 'none'; base-uri 'none'; form-action 'none'; object-src 'none'"
	return body.Bytes(), policy, nil
}
