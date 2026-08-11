package authflow

const cogFoundryCallbackPageHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>{{.Title}} · LoomLoom CLI</title>
  <style>
    :root {
      color-scheme: dark;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      --page-bg: #0b0c0e;
      --surface: #191a1e;
      --line: #383a40;
      --line-soft: #2c2e33;
      --text: #f3f3f4;
      --muted: #a3a5ab;
      --subtle: #7f8289;
      --brand: #11d7aa;
      --success: #19c99d;
      --danger: #ff6b67;
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
      max-width: 448px;
    }

    .card {
      width: 100%;
      padding: 36px 38px 32px;
      border: 1px solid var(--line);
      border-radius: 10px;
      background: var(--surface);
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.22);
    }

    .connection {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 48px minmax(0, 1fr);
      align-items: center;
      gap: 12px;
    }

    .platform,
    .client {
      min-width: 0;
      overflow: hidden;
      font-size: 18px;
      font-weight: 700;
      line-height: 1.2;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .platform {
      text-align: right;
      letter-spacing: -0.02em;
      color: var(--text);
    }

    .client {
      color: var(--brand);
    }

    .transfer {
      width: 48px;
      height: 34px;
      color: var(--subtle);
    }

    .transfer path {
      fill: none;
      stroke: currentColor;
      stroke-width: 3.5;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .content {
      margin-top: 34px;
      text-align: center;
    }

    .card[data-status="success"] {
      --state-color: var(--success);
    }

    .card[data-status="error"] {
      --state-color: var(--danger);
    }

    .kicker {
      margin: 0;
      color: var(--state-color);
      font-size: 13px;
      font-weight: 650;
      line-height: 1.4;
    }

    h1 {
      margin: 9px 0 0;
      color: var(--text);
      font-size: 29px;
      font-weight: 680;
      line-height: 1.25;
      letter-spacing: -0.02em;
    }

    .description {
      margin: 14px auto 0;
      max-width: 350px;
      color: var(--muted);
      font-size: 15px;
      line-height: 1.65;
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
        margin-top: max(0px, calc((100dvh - 500px) / 2));
      }

      .card {
        padding: 30px 24px 27px;
      }

      .connection {
        grid-template-columns: minmax(0, 1fr) 42px minmax(0, 1fr);
        gap: 9px;
      }

      .platform,
      .client {
        font-size: 16px;
      }

      .transfer {
        width: 42px;
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
      <header class="connection" aria-label="CogFoundry and LoomLoom CLI">
        <span class="platform" lang="en">cogfoundry</span>
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
        <p class="command">Run again: <code>loomloom login</code></p>
        {{end}}
      </footer>
    </article>
  </main>
</body>
</html>`

var cogFoundryCallbackPageRenderer = callbackPageRenderer{
	template: mustParseCallbackPage("cogfoundry-callback-page", cogFoundryCallbackPageHTML),
	language: "en",
	success: callbackPageView{
		Status:      "success",
		Role:        "status",
		Title:       "Authorization received",
		Kicker:      "Authorization received",
		Heading:     "Return to your terminal",
		Description: "CogFoundry has sent the authorization response to LoomLoom CLI. The CLI will finish signing you in from your terminal.",
		Footer:      "You can safely close this page",
		ShowCommand: false,
	},
	failure: callbackPageView{
		Status:      "error",
		Role:        "alert",
		Title:       "Authorization not completed",
		Kicker:      "Authorization not completed",
		Heading:     "Return to your terminal and try again",
		Description: "This authorization request was not completed. Close this page, then start the login flow again from your terminal.",
		Footer:      "No authorization information was saved",
		ShowCommand: true,
	},
}
