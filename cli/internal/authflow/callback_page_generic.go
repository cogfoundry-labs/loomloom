package authflow

const genericCallbackPageHTML = `<!doctype html>
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
      text-align: center;
    }

    .connection {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 11px;
      color: var(--muted);
      font-size: 16px;
      font-weight: 650;
    }

    .connection svg {
      width: 40px;
      height: 30px;
      fill: none;
      stroke: var(--subtle);
      stroke-width: 3.2;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .content {
      margin-top: 34px;
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
    }

    h1 {
      margin: 9px 0 0;
      font-size: 29px;
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
      fill: none;
      stroke: var(--state-color);
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

      .card {
        padding: 30px 24px 27px;
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
      <header class="connection" aria-label="Authorization service and LoomLoom CLI">
        <span>Authorization service</span>
        <svg viewBox="0 0 52 38" aria-hidden="true" focusable="false">
          <path d="M9 10h33m-6.5-6.5L42 10l-6.5 6.5" />
          <path d="M43 28H10m6.5-6.5L10 28l6.5 6.5" />
        </svg>
        <span>LoomLoom CLI</span>
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

var genericCallbackPageRenderer = callbackPageRenderer{
	template: mustParseCallbackPage("generic-callback-page", genericCallbackPageHTML),
	language: "en",
	success: callbackPageView{
		Status:      "success",
		Role:        "status",
		Title:       "Authorization received",
		Kicker:      "Authorization received",
		Heading:     "Return to your terminal",
		Description: "The authorization response has been sent to LoomLoom CLI. The CLI will finish signing you in from your terminal.",
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
