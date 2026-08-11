package authflow

const shengSuanYunCallbackPageHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="light">
  <title>{{.Title}} · LoomLoom CLI</title>
  <style>
    :root {
      color-scheme: light;
      font-family: Inter, "PingFang SC", "Microsoft YaHei", ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      --page-bg: #f7f7fb;
      --surface: #ffffff;
      --line: #dedde8;
      --line-soft: #ecebf2;
      --text: #24222d;
      --muted: #706d7b;
      --subtle: #9692a1;
      --brand: #5742ee;
      --danger: #d84949;
    }

    * {
      box-sizing: border-box;
    }

    body {
      min-height: 100vh;
      min-height: 100dvh;
      margin: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
      background: var(--page-bg);
      color: var(--text);
      -webkit-font-smoothing: antialiased;
      text-rendering: optimizeLegibility;
    }

    main {
      width: 100%;
      max-width: 480px;
    }

    .card {
      width: 100%;
      padding: 36px 38px 32px;
      border: 1px solid var(--line);
      border-radius: 14px;
      background: var(--surface);
      box-shadow: 0 18px 52px rgba(40, 35, 72, 0.09);
    }

    .connection {
      display: grid;
      grid-template-columns: 160px 36px 160px;
      align-items: center;
      justify-content: center;
      gap: 18px;
    }

    .platform,
    .client {
      min-width: 0;
      overflow: hidden;
      font-size: 22px;
      font-weight: 700;
      line-height: 1.25;
      letter-spacing: 0.01em;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .platform {
      justify-self: stretch;
      text-align: center;
      color: var(--brand);
    }

    .client {
      justify-self: stretch;
      text-align: center;
      color: var(--text);
      letter-spacing: -0.01em;
    }

    .transfer {
      width: 36px;
      height: 26px;
      color: #b0adb8;
    }

    .transfer path {
      fill: none;
      stroke: currentColor;
      stroke-width: 2.4;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .content {
      margin-top: 34px;
      text-align: center;
    }

    .kicker {
      margin: 0;
      color: var(--brand);
      font-size: 13px;
      font-weight: 650;
      line-height: 1.4;
    }

    .card[data-status="error"] .kicker {
      color: var(--danger);
    }

    h1 {
      margin: 9px 0 0;
      color: var(--text);
      font-size: 29px;
      font-weight: 680;
      line-height: 1.3;
      letter-spacing: -0.02em;
    }

    .description {
      margin: 14px auto 0;
      max-width: 350px;
      color: var(--muted);
      font-size: 15px;
      line-height: 1.75;
    }

    .footer {
      margin-top: 30px;
      padding-top: 22px;
      border-top: 1px solid var(--line-soft);
      color: var(--subtle);
      text-align: center;
      font-size: 13px;
      line-height: 1.55;
    }

    .footer-note {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
    }

    .footer-note svg {
      width: 17px;
      height: 17px;
      flex: 0 0 auto;
      fill: none;
      stroke: var(--brand);
      stroke-width: 1.8;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .command {
      margin: 10px 0 0;
    }

    code {
      color: var(--text);
      font: 500 12px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
    }

    @media (max-width: 480px) {
      body {
        align-items: start;
        padding: 16px;
      }

      main {
        margin-top: max(0px, calc((100dvh - 520px) / 2));
      }

      .card {
        padding: 30px 24px 27px;
      }

      .connection {
        grid-template-columns: minmax(0, 1fr) 30px minmax(0, 1fr);
        gap: 10px;
      }

      .platform,
      .client {
        font-size: 19px;
      }

      .transfer {
        width: 30px;
        height: 22px;
      }

      .content {
        margin-top: 30px;
      }

      h1 {
        font-size: 27px;
      }
    }
  </style>
</head>
<body>
  <main>
    <article class="card" data-status="{{.Status}}" role="{{.Role}}" aria-labelledby="page-title" aria-describedby="page-description">
      <header class="connection" aria-label="胜算云与 LoomLoom CLI">
        <span class="platform">胜算云</span>
        <svg class="transfer" viewBox="0 0 52 38" aria-hidden="true" focusable="false">
          <path d="M9 10h33m-6.5-6.5L42 10l-6.5 6.5" />
          <path d="M43 28H10m6.5-6.5L10 28l6.5 6.5" />
        </svg>
        <span class="client" lang="en">LoomLoom CLI</span>
      </header>

      <section class="content">
        <p class="kicker">{{.Kicker}}</p>
        <h1 id="page-title">{{.Heading}}</h1>
        <p class="description" id="page-description">{{.Description}}</p>
      </section>

      <footer class="footer">
        <div class="footer-note">
          <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
            <rect x="5" y="10" width="14" height="10" rx="2" />
            <path d="M8 10V7a4 4 0 0 1 8 0v3" />
          </svg>
          <span>{{.Footer}}</span>
        </div>
        {{if .ShowCommand}}
        <p class="command">重新运行：<code>loomloom login</code></p>
        {{end}}
      </footer>
    </article>
  </main>
</body>
</html>`

var shengSuanYunCallbackPageRenderer = callbackPageRenderer{
	template: mustParseCallbackPage("shengsuanyun-callback-page", shengSuanYunCallbackPageHTML),
	language: "zh-CN",
	success: callbackPageView{
		Status:      "success",
		Role:        "status",
		Title:       "授权响应已接收",
		Kicker:      "授权已返回",
		Heading:     "请返回终端",
		Description: "胜算云已将授权响应发送给 LoomLoom CLI。CLI 将在终端中继续完成登录。",
		Footer:      "此页面现在可以安全关闭",
		ShowCommand: false,
	},
	failure: callbackPageView{
		Status:      "error",
		Role:        "alert",
		Title:       "授权未完成",
		Kicker:      "授权未完成",
		Heading:     "请返回终端后重试",
		Description: "本次授权请求未能完成。请关闭此页面，然后在终端中重新发起登录。",
		Footer:      "未保存任何授权信息",
		ShowCommand: true,
	},
}
