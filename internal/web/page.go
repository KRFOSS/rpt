package web

// pageTemplate은 웹 대시보드의 화면입니다.
//
// 색과 간격, 서체는 :root 의 토큰 한 벌에서만 나옵니다. 예전에는 같은
// 색값을 곳곳에 직접 적어 두어 손댈 때마다 조금씩 어긋났습니다.
//
// 버튼은 크기 하나로 통일하고, 색은 강조가 아니라 상태를 뜻하도록 썼습니다.
// 화면에서 흰 버튼은 update 하나뿐이며, 그것이 가장 먼저 눌러야 할 것입니다.
const pageTemplate = `<!doctype html>
<html lang="ko">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>rpt 웹 대시보드</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.css">
  <style>
    :root {
      --bg: #1a1a1a;
      --surface: #242424;
      --surface-hi: #2c2c2e;
      --surface-hi2: #333335;
      --well: #161617;
      --line: rgba(84,84,88,0.4);
      --line-soft: rgba(84,84,88,0.22);
      --text: #f5f5f7;
      --text-2: rgba(235,235,245,0.62);
      --text-3: rgba(235,235,245,0.34);
      --ok: #34d399;
      --warn: #fbbf24;
      --danger: #ef4444;
      --info: #38bdf8;
      --focus-ring: #06b6d4;
      --radius-lg: 14px;
      --radius: 10px;
      --radius-sm: 8px;
      --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Monaco, Consolas, monospace;
      --sans: "Pretendard Variable", Pretendard, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "Noto Sans", sans-serif;
    }

    html { box-sizing: border-box; }
    *, *::before, *::after { box-sizing: inherit; }
    html, body { width: 100%; overflow-x: hidden; }

    body {
      margin: 0;
      padding: 0;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      background: var(--bg);
      color: #e5e5e5;
      font-family: var(--sans);
      font-size: 15px;
      line-height: 1.5;
      -webkit-font-smoothing: antialiased;
    }

    button, input, textarea, select { font-family: inherit; font-size: inherit; }
    :focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px; }

    .navbar {
      position: fixed;
      top: 0;
      left: 0;
      width: 100%;
      box-shadow: none !important;
      border: none !important;
      padding-bottom: 40px !important;
      z-index: 1000;
      background: linear-gradient(to bottom, rgba(0,0,0,0.8) 0%, rgba(0,0,0,0.65) 20%, rgba(0,0,0,0.50) 40%, rgba(0,0,0,0.35) 60%, rgba(0,0,0,0.20) 75%, rgba(0,0,0,0.10) 85%, rgba(0,0,0,0.04) 93%, rgba(0,0,0,0) 100%) !important;
      transition: transform 0.4s cubic-bezier(0.4,0,0.2,1), background 0.4s ease;
      padding: 23px !important;
      transform: translateY(0);
    }

    .navbar .container {
      display: flex;
      justify-content: space-between;
      align-items: center;
      flex-wrap: nowrap;
      width: 100%;
      max-width: 1200px;
      margin: 0 auto;
      padding: 0 1rem;
    }

    .navbar-brand img { height: 40px; }

    .main-container {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 132px 20px 72px;
    }

    .content-wrapper {
      width: 100%;
      max-width: 1080px;
      display: flex;
      flex-direction: column;
      gap: 20px;
    }

    .hero { text-align: center; padding: 4px 0 4px; }

    .hero h1 {
      margin: 0 0 6px;
      font-size: clamp(34px, 7vw, 48px);
      font-weight: 800;
      letter-spacing: -0.02em;
      color: #fff;
    }

    .hero .subtitle {
      margin: 0 0 10px;
      font-size: 18px;
      font-weight: 600;
      color: var(--text);
    }

    .hero .description {
      margin: 0 auto;
      max-width: 620px;
      font-size: 15px;
      line-height: 1.6;
      color: var(--text-2);
    }

    .stats {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
      gap: 12px;
    }

    .stat {
      display: flex;
      flex-direction: column;
      gap: 4px;
      padding: 14px 16px;
      background: var(--surface);
      border: 1px solid var(--line-soft);
      border-radius: var(--radius);
    }

    .stat-num {
      font-family: var(--mono);
      font-size: 24px;
      font-weight: 700;
      line-height: 1.1;
      color: #fff;
    }

    .stat-num.is-text { font-size: 18px; padding-top: 5px; }
    .stat-label { font-size: 12.5px; color: var(--text-2); }
    .stat.is-warn { border-color: rgba(251,191,36,0.35); }
    .stat.is-warn .stat-num { color: var(--warn); }

    .notice {
      display: flex;
      gap: 10px;
      align-items: flex-start;
      padding: 14px 16px;
      border-radius: var(--radius);
      background: rgba(251,191,36,0.08);
      border: 1px solid rgba(251,191,36,0.3);
      color: #f0d089;
      font-size: 14px;
      line-height: 1.6;
      text-align: left;
    }

    .notice strong { color: var(--warn); font-weight: 700; }

    .card {
      padding: 20px;
      background: var(--surface);
      border: 1px solid var(--line-soft);
      border-radius: var(--radius-lg);
      text-align: left;
    }

    .card-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      flex-wrap: wrap;
      margin: 0 0 16px;
      padding-bottom: 12px;
      border-bottom: 1px solid var(--line);
    }

    .card-title { margin: 0; font-size: 16px; font-weight: 700; color: #fff; }
    .card-note { font-family: var(--mono); font-size: 13px; color: var(--text-2); }
    .head-actions { display: flex; gap: 6px; }

    .cmd-tag {
      padding: 2px 7px;
      border-radius: 6px;
      background: var(--well);
      border: 1px solid var(--line-soft);
      font-family: var(--mono);
      font-size: 11.5px;
      font-weight: 600;
      color: var(--text-2);
    }

    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      height: 40px;
      padding: 0 18px;
      border: 1px solid var(--line);
      border-radius: var(--radius);
      background: var(--surface-hi);
      color: var(--text);
      font-size: 14px;
      font-weight: 600;
      cursor: pointer;
      user-select: none;
      transition: background-color 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
    }

    .btn:hover { background: var(--surface-hi2); border-color: rgba(84,84,88,0.7); }
    .btn:active { transform: scale(0.985); }
    .btn-block { width: 100%; }

    .btn-primary { background: #fff; border-color: #fff; color: #000; }
    .btn-primary:hover { background: rgba(255,255,255,0.86); border-color: rgba(255,255,255,0.86); }

    .btn-quiet {
      display: inline-flex;
      align-items: center;
      height: 30px;
      padding: 0 10px;
      border: 1px solid transparent;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--text-2);
      font-size: 13px;
      font-weight: 500;
      cursor: pointer;
      transition: background-color 0.15s ease, color 0.15s ease;
    }

    .btn-quiet:hover { background: var(--surface-hi); color: var(--text); }

    .tiles {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(196px, 1fr));
      gap: 12px;
    }

    .tile-form { display: flex; }

    .tile {
      flex: 1;
      display: flex;
      flex-direction: column;
      align-items: flex-start;
      gap: 3px;
      padding: 13px 15px;
      border: 1px solid var(--line);
      border-radius: var(--radius);
      background: var(--surface-hi);
      text-align: left;
      cursor: pointer;
      transition: background-color 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
    }

    .tile:hover { background: var(--surface-hi2); border-color: rgba(84,84,88,0.7); }
    .tile:active { transform: scale(0.99); }
    .tile-cmd { font-family: var(--mono); font-size: 14px; font-weight: 700; color: #fff; }
    .tile-desc { font-size: 12.5px; font-weight: 400; line-height: 1.4; color: var(--text-2); }

    .tile-primary { background: #fff; border-color: #fff; }
    .tile-primary:hover { background: rgba(255,255,255,0.86); border-color: rgba(255,255,255,0.86); }
    .tile-primary .tile-cmd { color: #000; }
    .tile-primary .tile-desc { color: rgba(0,0,0,0.62); }

    .form-grid {
      display: grid;
      grid-template-columns: 1fr;
      gap: 24px 28px;
    }

    .form-block { display: flex; flex-direction: column; gap: 12px; }

    .form-block .btn { margin-top: auto; }

    @media (min-width: 700px) {
      .form-grid { grid-template-columns: repeat(2, 1fr); }
      .form-grid .span-2 { grid-column: span 2; }
    }

    @media (min-width: 980px) {
      .form-grid { grid-template-columns: repeat(3, 1fr); }
    }

    .form-block-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
    }

    .form-block-title { font-size: 14px; font-weight: 600; color: var(--text); }

    .field { display: flex; flex-direction: column; gap: 6px; }
    .field-label { font-size: 13px; font-weight: 500; color: var(--text-2); }
    .field-hint { font-size: 12px; line-height: 1.5; color: var(--text-3); }

    .input {
      width: 100%;
      padding: 10px 12px;
      border: 1px solid var(--line);
      border-radius: var(--radius-sm);
      background: var(--well);
      color: var(--text);
      font-family: var(--mono);
      font-size: 13.5px;
      outline: none;
      transition: border-color 0.15s ease, background-color 0.15s ease;
    }

    .input::placeholder { color: var(--text-3); }
    .input:hover { border-color: rgba(84,84,88,0.65); }
    .input:focus { border-color: var(--focus-ring); background: #131314; }

    .picker { position: relative; }
    .picker .input { padding-right: 34px; }
    .picker .input[readonly] { cursor: pointer; }
    .picker .input[readonly]:hover { border-color: rgba(84,84,88,0.65); }

    .picker::after {
      content: "";
      position: absolute;
      right: 14px;
      top: 50%;
      width: 7px;
      height: 7px;
      margin-top: -6px;
      border-right: 1.5px solid var(--text-3);
      border-bottom: 1.5px solid var(--text-3);
      transform: rotate(45deg);
      pointer-events: none;
    }

    .modal {
      position: fixed;
      inset: 0;
      z-index: 100;
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 20px;
    }

    .modal[hidden] { display: none; }

    .modal-scrim { position: absolute; inset: 0; background: rgba(0,0,0,0.62); }

    .modal-panel {
      position: relative;
      display: flex;
      flex-direction: column;
      width: 100%;
      max-width: 560px;
      max-height: min(78vh, 640px);
      background: var(--surface);
      border: 1px solid var(--line);
      border-radius: var(--radius-lg);
      box-shadow: 0 24px 60px rgba(0,0,0,0.55);
      overflow: hidden;
    }

    .modal-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 16px 18px;
      border-bottom: 1px solid var(--line);
    }

    .modal-title { margin: 0; font-size: 15px; font-weight: 700; color: #fff; }

    .modal-x {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 30px;
      height: 30px;
      border: 1px solid transparent;
      border-radius: var(--radius-sm);
      background: transparent;
      color: var(--text-2);
      font-size: 15px;
      cursor: pointer;
    }

    .modal-x:hover { background: var(--surface-hi); color: var(--text); }

    .modal-search { padding: 14px 18px 10px; }
    .modal-hint { margin: 8px 0 0; font-size: 12px; color: var(--text-3); line-height: 1.5; }

    .modal-list {
      flex: 1;
      min-height: 0;
      overflow-y: auto;
      margin: 0;
      padding: 0 10px 10px;
      list-style: none;
    }

    .modal-item {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 8px;
      border-radius: var(--radius-sm);
      cursor: pointer;
    }

    .modal-item[hidden] { display: none; }
    .modal-item:hover, .modal-item.is-active { background: var(--surface-hi); }

    .modal-check {
      flex-shrink: 0;
      width: 16px;
      height: 16px;
      border: 1px solid var(--line);
      border-radius: 5px;
    }

    .modal-item[aria-selected="true"] .modal-check {
      background: var(--focus-ring);
      border-color: var(--focus-ring);
    }

    .modal-item-main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
    .modal-item-name { font-family: var(--mono); font-size: 13.5px; font-weight: 700; color: #fff; }

    .modal-item-desc {
      font-size: 12px;
      color: var(--text-2);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .modal-item-ver { flex-shrink: 0; font-family: var(--mono); font-size: 11.5px; color: var(--text-3); }

    .pick-tag { flex-shrink: 0; font-size: 11px; font-weight: 600; color: var(--ok); }
    .pick-tag.is-upgradable { color: var(--warn); }

    .modal-empty { padding: 26px 8px; text-align: center; font-size: 13px; color: var(--text-3); }
    .modal-empty[hidden] { display: none; }

    .modal-foot {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 14px 18px;
      border-top: 1px solid var(--line);
    }

    .modal-count { font-size: 12.5px; color: var(--text-2); }
    .modal-actions { display: flex; gap: 8px; }
    .modal-foot .btn { height: 36px; padding: 0 16px; }
    .modal-foot .btn[disabled] { opacity: 0.5; cursor: default; }

    .modal-body { flex: 1; min-height: 0; overflow-y: auto; padding: 16px 18px; }

    .running {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 26px 2px;
      font-size: 14px;
      color: var(--text-2);
    }

    .spinner {
      width: 18px;
      height: 18px;
      flex-shrink: 0;
      border: 2px solid var(--line);
      border-top-color: var(--text);
      border-radius: 50%;
      animation: spin 0.7s linear infinite;
    }

    @keyframes spin { to { transform: rotate(360deg); } }

    /* 조건부로 붙는 조각을 감싸되 자리는 차지하지 않게 합니다. */
    .live-slot { display: contents; }

    .checks { display: flex; flex-direction: column; gap: 10px; }

    .check {
      display: flex;
      align-items: center;
      gap: 9px;
      font-size: 13.5px;
      color: #e5e5e5;
      cursor: pointer;
    }

    .check input {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      accent-color: var(--focus-ring);
      cursor: pointer;
    }

    .inline-form { display: inline-flex; }

    .result {
      padding: 18px 20px;
      border: 1px solid var(--line);
      border-left-width: 3px;
      border-radius: var(--radius-lg);
      background: var(--surface);
      text-align: left;
    }

    .result-ok { border-left-color: var(--ok); }
    .result-err { border-left-color: var(--danger); }

    .result-head { display: flex; align-items: center; gap: 9px; margin-bottom: 14px; }
    .result-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
    .result-ok .result-dot { background: var(--ok); }
    .result-err .result-dot { background: var(--danger); }
    .result-label { font-size: 15px; font-weight: 700; }
    .result-ok .result-label { color: var(--ok); }
    .result-err .result-label { color: var(--danger); }

    .result-error {
      margin: 0 0 12px;
      padding: 12px 14px;
      border: 1px solid rgba(239,68,68,0.25);
      border-radius: var(--radius-sm);
      background: rgba(239,68,68,0.09);
      color: #ffb4b4;
      font-size: 13.5px;
      line-height: 1.6;
    }

    .result-pre {
      margin: 0;
      padding: 14px 16px;
      border: 1px solid var(--line-soft);
      border-radius: var(--radius-sm);
      background: var(--well);
      color: rgba(235,235,245,0.85);
      font-family: var(--mono);
      font-size: 13px;
      line-height: 1.65;
      white-space: pre-wrap;
      word-break: break-word;
      max-height: 420px;
      overflow-y: auto;
    }

    .table-wrap { overflow-x: auto; }
    .tbl { width: 100%; border-collapse: collapse; font-size: 14px; }

    .tbl th {
      padding: 0 12px 10px;
      border-bottom: 1px solid var(--line);
      color: var(--text-2);
      font-size: 12px;
      font-weight: 600;
      text-align: left;
      white-space: nowrap;
    }

    .tbl td {
      padding: 12px;
      border-bottom: 1px solid var(--line-soft);
      vertical-align: middle;
    }

    .tbl tbody tr:last-child td { border-bottom: none; }
    .tbl tbody tr:hover { background: rgba(255,255,255,0.025); }
    .tbl th:first-child, .tbl td:first-child { padding-left: 4px; }
    .tbl th:last-child, .tbl td:last-child { padding-right: 4px; }

    .cell-name { font-family: var(--mono); font-weight: 700; color: #fff; white-space: nowrap; }
    .cell-ver { font-family: var(--mono); font-size: 13px; color: var(--text-2); white-space: nowrap; }
    .cell-time { font-family: var(--mono); font-size: 12.5px; color: var(--text-3); white-space: nowrap; }

    .cell-desc {
      max-width: 320px;
      color: var(--text-2);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tbl th.col-actions, .col-actions { text-align: right; }
    .row-actions { display: inline-flex; gap: 6px; justify-content: flex-end; }

    .btn-row {
      height: 30px;
      padding: 0 11px;
      border: 1px solid var(--line);
      border-radius: 7px;
      background: transparent;
      color: var(--text-2);
      font-size: 12.5px;
      font-weight: 600;
      white-space: nowrap;
      cursor: pointer;
      transition: background-color 0.15s ease, border-color 0.15s ease, color 0.15s ease;
    }

    .btn-row:hover { background: var(--surface-hi); border-color: rgba(84,84,88,0.75); color: var(--text); }
    .btn-row-warn:hover { background: rgba(251,191,36,0.1); border-color: rgba(251,191,36,0.45); color: var(--warn); }
    .btn-row-danger:hover { background: rgba(239,68,68,0.1); border-color: rgba(239,68,68,0.45); color: var(--danger); }

    .badges { display: inline-flex; gap: 6px; flex-wrap: wrap; }

    .badge {
      display: inline-block;
      padding: 3px 8px;
      border-radius: 6px;
      font-size: 11.5px;
      font-weight: 600;
      white-space: nowrap;
    }

    .badge-warn { background: rgba(251,191,36,0.12); border: 1px solid rgba(251,191,36,0.28); color: var(--warn); }
    .badge-ok { background: rgba(52,211,153,0.1); border: 1px solid rgba(52,211,153,0.22); color: var(--ok); }
    .badge-info { background: rgba(56,189,248,0.1); border: 1px solid rgba(56,189,248,0.22); color: var(--info); }

    .kv { margin: 0; display: flex; flex-direction: column; }

    .kv-row {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 16px;
      padding: 11px 0;
      border-bottom: 1px solid var(--line-soft);
    }

    .kv-row:first-child { padding-top: 0; }
    .kv-row:last-child { padding-bottom: 0; border-bottom: none; }

    .kv-key { margin: 0; flex-shrink: 0; font-size: 13.5px; font-weight: 500; color: var(--text-2); }

    .kv-val {
      margin: 0;
      font-family: var(--mono);
      font-size: 13px;
      color: var(--text);
      text-align: right;
      word-break: break-all;
    }

    .empty {
      margin: 0;
      padding: 28px 0;
      text-align: center;
      color: var(--text-3);
      font-size: 14px;
      line-height: 1.6;
    }

    .muted { color: var(--text-3); }

    .footer-text {
      margin: 8px 0 0;
      text-align: center;
      font-family: var(--mono);
      font-size: 13px;
      color: var(--text-3);
    }

    @media (max-width: 760px) {
      .main-container { padding: 112px 16px 56px; }
      .card { padding: 16px; }
      .col-opt { display: none; }
    }

    @media (prefers-reduced-motion: reduce) {
      * { animation: none !important; transition: none !important; }
    }
  </style>
</head>
<body>
  <nav class="navbar">
    <div class="container">
      <a class="navbar-brand" href="/">
        <img loading="eager" decoding="async" src="https://cdn.krfoss.org/web/ROKFOSS.png" alt="ROKFOSS">
      </a>
    </div>
  </nav>

  <main class="main-container">
    <div class="content-wrapper">
      <header class="hero">
        <h1>RPT</h1>
        <p class="subtitle">ROKFOSS 패키지 관리자</p>
        <p class="description">패키지 설치와 업그레이드, 저장소 조회를 이 화면에서 바로 실행합니다. 결과는 명령줄에서 실행했을 때와 같습니다.</p>
      </header>

      {{if .Result}}
      <section class="result {{if .Result.OK}}result-ok{{else}}result-err{{end}}">
        <div class="result-head">
          <span class="result-dot"></span>
          <span class="result-label">{{if .Result.OK}}완료{{else}}실패{{end}}</span>
          <span class="cmd-tag">{{.Result.Command}}</span>
        </div>
        {{if .Result.Error}}<p class="result-error">{{.Result.Error}}</p>{{end}}
        {{if .Result.Lines}}<pre class="result-pre">{{range .Result.Lines}}{{.}}
{{end}}</pre>{{end}}
      </section>
      {{end}}

      <div class="live-slot" id="live-notice">
        {{if not .IndexAvailable}}
        <div class="notice">
          <span><strong>패키지 목록이 아직 없습니다.</strong> update 를 실행하면 저장소에서 목록을 받아옵니다. 목록이 있어야 설치와 검색을 할 수 있습니다.</span>
        </div>
        {{end}}
      </div>

      <div class="stats" id="live-stats">
        <div class="stat">
          <span class="stat-num">{{.PackageCount}}</span>
          <span class="stat-label">저장소 패키지</span>
        </div>
        <div class="stat">
          <span class="stat-num">{{.InstalledCount}}</span>
          <span class="stat-label">설치됨</span>
        </div>
        <div class="stat{{if .UpgradableCount}} is-warn{{end}}">
          <span class="stat-num">{{.UpgradableCount}}</span>
          <span class="stat-label">업그레이드 가능</span>
        </div>
        <div class="stat">
          <span class="stat-num is-text">{{.Arch}}</span>
          <span class="stat-label">아키텍처</span>
        </div>
      </div>

      <section class="card">
        <div class="card-head">
          <h2 class="card-title">빠른 작업</h2>
          <div class="head-actions">
            <form method="post" action="/command" class="inline-form" data-quick>
              <input type="hidden" name="cmd" value="version">
              <button type="submit" class="btn-quiet">버전</button>
            </form>
            <form method="post" action="/command" class="inline-form" data-quick>
              <input type="hidden" name="cmd" value="help">
              <button type="submit" class="btn-quiet">도움말</button>
            </form>
          </div>
        </div>
        <div class="tiles">
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="update">
            <button type="submit" class="tile tile-primary">
              <span class="tile-cmd">update</span>
              <span class="tile-desc">저장소 목록을 새로 받습니다</span>
            </button>
          </form>
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="upgrade">
            <button type="submit" class="tile">
              <span class="tile-cmd">upgrade</span>
              <span class="tile-desc">설치된 패키지를 모두 최신으로</span>
            </button>
          </form>
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="autoremove">
            <button type="submit" class="tile">
              <span class="tile-cmd">autoremove</span>
              <span class="tile-desc">딸려왔다 필요 없어진 것 정리</span>
            </button>
          </form>
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="clean">
            <button type="submit" class="tile">
              <span class="tile-cmd">clean</span>
              <span class="tile-desc">받아 둔 deb 를 모두 지웁니다</span>
            </button>
          </form>
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="autoclean">
            <button type="submit" class="tile">
              <span class="tile-cmd">autoclean</span>
              <span class="tile-desc">저장소에서 사라진 deb 만 지웁니다</span>
            </button>
          </form>
          <form method="post" action="/command" class="tile-form" data-quick>
            <input type="hidden" name="cmd" value="relink">
            <button type="submit" class="tile">
              <span class="tile-cmd">relink</span>
              <span class="tile-desc">시스템 심링크를 다시 만듭니다</span>
            </button>
          </form>
        </div>
      </section>

      <section class="card">
        <div class="card-head">
          <h2 class="card-title">패키지 작업</h2>
        </div>
        <div class="form-grid">
          <form method="post" action="/command" class="form-block span-2" data-quick>
            <input type="hidden" name="cmd" value="install">
            <div class="form-block-head">
              <span class="form-block-title">패키지 설치</span>
              <span class="cmd-tag">install</span>
            </div>
            <div class="field">
              <label class="field-label" for="install-names">패키지 이름</label>
              <div class="picker">
                <input type="text" id="install-names" name="names" class="input" placeholder="눌러서 고르기"
                       autocomplete="off" spellcheck="false" data-modal-open="modal-install">
              </div>
              <span class="field-hint">눌러서 목록에서 고릅니다. 여러 개를 골라도 되고, 의존성은 함께 설치됩니다.</span>
            </div>
            <label class="check">
              <input type="checkbox" name="reinstall">
              <span>이미 설치돼 있어도 다시 설치</span>
            </label>
            <button type="submit" class="btn btn-primary btn-block">설치</button>
          </form>

          <form method="post" action="/command" class="form-block" data-quick>
            <input type="hidden" name="cmd" value="upgrade">
            <div class="form-block-head">
              <span class="form-block-title">골라서 업그레이드</span>
              <span class="cmd-tag">upgrade</span>
            </div>
            <div class="field">
              <label class="field-label" for="upgrade-names">패키지 이름</label>
              <div class="picker">
                <input type="text" id="upgrade-names" name="names" class="input" placeholder="눌러서 고르기 (비우면 전체)"
                       autocomplete="off" spellcheck="false" data-modal-open="modal-upgrade">
              </div>
              <span class="field-hint">비워 두면 설치된 패키지를 모두 올립니다.</span>
            </div>
            <button type="submit" class="btn btn-block">업그레이드</button>
          </form>

          <form method="post" action="/command" class="form-block" data-quick>
            <input type="hidden" name="cmd" value="search">
            <div class="form-block-head">
              <span class="form-block-title">검색</span>
              <span class="cmd-tag">search</span>
            </div>
            <div class="field">
              <label class="field-label" for="search-q">검색어</label>
              <div class="picker">
                <input type="text" id="search-q" name="q" class="input" placeholder="눌러서 검색어 넣기"
                       autocomplete="off" spellcheck="false" data-modal-open="modal-search">
              </div>
              <span class="field-hint">이름과 설명에서 찾습니다.</span>
            </div>
            <button type="submit" class="btn btn-block">검색</button>
          </form>

          <form method="post" action="/command" class="form-block" data-quick>
            <input type="hidden" name="cmd" value="show">
            <div class="form-block-head">
              <span class="form-block-title">상세 정보</span>
              <span class="cmd-tag">show</span>
            </div>
            <div class="field">
              <label class="field-label" for="show-names">패키지 이름</label>
              <div class="picker">
                <input type="text" id="show-names" name="names" class="input" placeholder="눌러서 고르기"
                       autocomplete="off" spellcheck="false" data-modal-open="modal-show">
              </div>
              <span class="field-hint">버전과 의존성, 설치 여부를 함께 보여 줍니다.</span>
            </div>
            <button type="submit" class="btn btn-block">상세 보기</button>
          </form>

          <form method="post" action="/command" class="form-block" data-quick>
            <input type="hidden" name="cmd" value="list">
            <div class="form-block-head">
              <span class="form-block-title">목록</span>
              <span class="cmd-tag">list</span>
            </div>
            <div class="checks">
              <label class="check"><input type="checkbox" name="installed"><span>설치된 것만</span></label>
              <label class="check"><input type="checkbox" name="upgradable"><span>업그레이드 가능한 것만</span></label>
            </div>
            <button type="submit" class="btn btn-block">목록 보기</button>
          </form>
        </div>
      </section>

      <section class="card" id="live-installed">
        <div class="card-head">
          <h2 class="card-title">설치된 패키지</h2>
          <span class="card-note">{{.InstalledCount}}개</span>
        </div>
        {{if .Installed}}
        <div class="table-wrap">
          <table class="tbl">
            <thead>
              <tr>
                <th>패키지</th>
                <th>버전</th>
                <th>상태</th>
                <th class="col-opt">설명</th>
                <th class="col-opt">설치 시각</th>
                <th class="col-actions">작업</th>
              </tr>
            </thead>
            <tbody>
              {{range .Installed}}
              <tr>
                <td class="cell-name">{{.Name}}</td>
                <td class="cell-ver">{{.Version}}</td>
                <td>
                  <span class="badges">
                    {{if .Upgradable}}<span class="badge badge-warn">업그레이드 가능</span>{{else}}<span class="badge badge-ok">최신</span>{{end}}
                    {{if .Auto}}<span class="badge badge-info">자동</span>{{end}}
                  </span>
                </td>
                <td class="col-opt cell-desc" title="{{.Summary}}">{{if .Summary}}{{.Summary}}{{else}}<span class="muted">-</span>{{end}}</td>
                <td class="col-opt cell-time">{{.InstalledAtText}}</td>
                <td class="col-actions">
                  <span class="row-actions">
                    <form method="post" action="/command" class="inline-form" data-quick onsubmit="return confirm('{{.Name}} 를 지웁니다. 설정 파일은 남습니다. 계속할까요?')">
                      <input type="hidden" name="cmd" value="remove">
                      <input type="hidden" name="names" value="{{.Name}}">
                      <button type="submit" class="btn-row btn-row-warn">제거</button>
                    </form>
                    <form method="post" action="/command" class="inline-form" data-quick onsubmit="return confirm('{{.Name}} 를 설정 파일까지 지웁니다. 되돌릴 수 없습니다. 계속할까요?')">
                      <input type="hidden" name="cmd" value="purge">
                      <input type="hidden" name="names" value="{{.Name}}">
                      <button type="submit" class="btn-row btn-row-danger">완전 삭제</button>
                    </form>
                  </span>
                </td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </div>
        {{else}}
        <p class="empty">아직 설치된 패키지가 없습니다. 위의 설치 칸에 이름을 넣어 시작하십시오.</p>
        {{end}}
      </section>

      <section class="card">
        <div class="card-head">
          <h2 class="card-title">환경</h2>
        </div>
        <dl class="kv">
          <div class="kv-row"><dt class="kv-key">rpt 버전</dt><dd class="kv-val">{{.Version}}</dd></div>
          <div class="kv-row"><dt class="kv-key">저장소</dt><dd class="kv-val">{{.RepoURL}}</dd></div>
          {{if .IndexAvailable}}
          <div class="kv-row"><dt class="kv-key">배포판</dt><dd class="kv-val">{{.IndexOrigin}} / {{.IndexSuite}} / {{.IndexComponent}}</dd></div>
          {{end}}
          <div class="kv-row"><dt class="kv-key">마지막 갱신</dt><dd class="kv-val">{{.IndexFetchedAt}}</dd></div>
          <div class="kv-row"><dt class="kv-key">설치 루트</dt><dd class="kv-val">{{.Root}}</dd></div>
          <div class="kv-row"><dt class="kv-key">상태 파일</dt><dd class="kv-val">{{.StateRoot}}</dd></div>
          <div class="kv-row"><dt class="kv-key">캐시</dt><dd class="kv-val">{{.CacheRoot}}</dd></div>
          <div class="kv-row"><dt class="kv-key">서버 주소</dt><dd class="kv-val">{{.Addr}}</dd></div>
        </dl>
      </section>

      <p class="footer-text">rpt {{.Version}} · {{.Now}}</p>
    </div>
  </main>

  <div class="modal" id="modal-result" hidden>
    <div class="modal-scrim" data-result-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="modal-result-title">
      <div class="modal-head">
        <h3 class="modal-title" id="modal-result-title">실행 결과</h3>
        <button type="button" class="modal-x" data-result-close aria-label="닫기">✕</button>
      </div>
      <div class="modal-body" data-result-body aria-live="polite"></div>
      <div class="modal-foot">
        <span class="modal-count" data-result-state></span>
        <span class="modal-actions">
          <button type="button" class="btn btn-primary" data-result-close>닫기</button>
        </span>
      </div>
    </div>
  </div>

  <div class="modal" id="modal-install" data-multi="1" hidden>
    <div class="modal-scrim" data-modal-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="modal-install-title">
      <div class="modal-head">
        <h3 class="modal-title" id="modal-install-title">설치할 패키지 고르기</h3>
        <button type="button" class="modal-x" data-modal-close aria-label="닫기">✕</button>
      </div>
      <div class="modal-search">
        <input type="text" class="input" data-modal-query placeholder="이름이나 설명으로 찾기" autocomplete="off" spellcheck="false">
        <p class="modal-hint">여러 개를 골라도 됩니다. 의존성은 함께 설치되므로 따로 고르지 않아도 됩니다.</p>
      </div>
      <ul class="modal-list" id="install-list" data-modal-list role="listbox" aria-multiselectable="true">
        {{range $i, $p := .Available}}
        <li class="modal-item" id="install-pick-{{$i}}" role="option" aria-selected="false" data-value="{{$p.Name}}" data-search="{{$p.SearchKey}}">
          <span class="modal-check" aria-hidden="true"></span>
          <span class="modal-item-main">
            <span class="modal-item-name">{{$p.Name}}</span>
            <span class="modal-item-desc">{{$p.Summary}}</span>
          </span>
          <span class="modal-item-ver">{{$p.Version}}</span>
          {{if $p.Upgradable}}<span class="pick-tag is-upgradable">업그레이드 가능</span>{{else if $p.Installed}}<span class="pick-tag">설치됨</span>{{end}}
        </li>
        {{end}}
        <li class="modal-empty" role="presentation" hidden>{{if .Available}}맞는 패키지가 없습니다{{else}}저장소 목록이 없습니다. update 를 먼저 실행하십시오{{end}}</li>
      </ul>
      <div class="modal-foot">
        <span class="modal-count" data-modal-count>고른 것 없음</span>
        <span class="modal-actions">
          <button type="button" class="btn" data-modal-close>취소</button>
          <button type="button" class="btn btn-primary" data-modal-ok>확인</button>
        </span>
      </div>
    </div>
  </div>

  <div class="modal" id="modal-upgrade" data-multi="1" hidden>
    <div class="modal-scrim" data-modal-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="modal-upgrade-title">
      <div class="modal-head">
        <h3 class="modal-title" id="modal-upgrade-title">업그레이드할 패키지 고르기</h3>
        <button type="button" class="modal-x" data-modal-close aria-label="닫기">✕</button>
      </div>
      <div class="modal-search">
        <input type="text" class="input" data-modal-query placeholder="이름이나 설명으로 찾기" autocomplete="off" spellcheck="false">
        <p class="modal-hint">아무것도 고르지 않고 확인하면 설치된 패키지를 모두 올립니다.</p>
      </div>
      <ul class="modal-list" id="upgrade-list" data-modal-list role="listbox" aria-multiselectable="true">
        {{range $i, $p := .InstalledOptions}}
        <li class="modal-item" id="upgrade-pick-{{$i}}" role="option" aria-selected="false" data-value="{{$p.Name}}" data-search="{{$p.SearchKey}}">
          <span class="modal-check" aria-hidden="true"></span>
          <span class="modal-item-main">
            <span class="modal-item-name">{{$p.Name}}</span>
            <span class="modal-item-desc">{{$p.Summary}}</span>
          </span>
          <span class="modal-item-ver">{{$p.Version}}</span>
          {{if $p.Upgradable}}<span class="pick-tag is-upgradable">업그레이드 가능</span>{{end}}
        </li>
        {{end}}
        <li class="modal-empty" role="presentation" hidden>{{if .InstalledOptions}}맞는 패키지가 없습니다{{else}}설치된 패키지가 없습니다{{end}}</li>
      </ul>
      <div class="modal-foot">
        <span class="modal-count" data-modal-count>고른 것 없음</span>
        <span class="modal-actions">
          <button type="button" class="btn" data-modal-close>취소</button>
          <button type="button" class="btn btn-primary" data-modal-ok>확인</button>
        </span>
      </div>
    </div>
  </div>

  <div class="modal" id="modal-search" hidden>
    <div class="modal-scrim" data-modal-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="modal-search-title">
      <div class="modal-head">
        <h3 class="modal-title" id="modal-search-title">검색어 정하기</h3>
        <button type="button" class="modal-x" data-modal-close aria-label="닫기">✕</button>
      </div>
      <div class="modal-search">
        <input type="text" class="input" data-modal-query placeholder="찾을 말" autocomplete="off" spellcheck="false">
        <p class="modal-hint">적은 말을 그대로 검색어로 씁니다. 아래에서 고르면 그 이름이 들어갑니다.</p>
      </div>
      <ul class="modal-list" id="search-list" data-modal-list role="listbox">
        {{range $i, $p := .Available}}
        <li class="modal-item" id="search-pick-{{$i}}" role="option" aria-selected="false" data-value="{{$p.Name}}" data-search="{{$p.SearchKey}}">
          <span class="modal-item-main">
            <span class="modal-item-name">{{$p.Name}}</span>
            <span class="modal-item-desc">{{$p.Summary}}</span>
          </span>
          <span class="modal-item-ver">{{$p.Version}}</span>
        </li>
        {{end}}
        <li class="modal-empty" role="presentation" hidden>{{if .Available}}맞는 패키지가 없습니다{{else}}저장소 목록이 없습니다. update 를 먼저 실행하십시오{{end}}</li>
      </ul>
      <div class="modal-foot">
        <span class="modal-count" data-modal-count>적은 말로 찾습니다</span>
        <span class="modal-actions">
          <button type="button" class="btn" data-modal-close>취소</button>
          <button type="button" class="btn btn-primary" data-modal-ok>확인</button>
        </span>
      </div>
    </div>
  </div>

  <div class="modal" id="modal-show" data-multi="1" hidden>
    <div class="modal-scrim" data-modal-close></div>
    <div class="modal-panel" role="dialog" aria-modal="true" aria-labelledby="modal-show-title">
      <div class="modal-head">
        <h3 class="modal-title" id="modal-show-title">자세히 볼 패키지 고르기</h3>
        <button type="button" class="modal-x" data-modal-close aria-label="닫기">✕</button>
      </div>
      <div class="modal-search">
        <input type="text" class="input" data-modal-query placeholder="이름이나 설명으로 찾기" autocomplete="off" spellcheck="false">
        <p class="modal-hint">여러 개를 고르면 차례로 보여 줍니다.</p>
      </div>
      <ul class="modal-list" id="show-list" data-modal-list role="listbox" aria-multiselectable="true">
        {{range $i, $p := .Available}}
        <li class="modal-item" id="show-pick-{{$i}}" role="option" aria-selected="false" data-value="{{$p.Name}}" data-search="{{$p.SearchKey}}">
          <span class="modal-check" aria-hidden="true"></span>
          <span class="modal-item-main">
            <span class="modal-item-name">{{$p.Name}}</span>
            <span class="modal-item-desc">{{$p.Summary}}</span>
          </span>
          <span class="modal-item-ver">{{$p.Version}}</span>
          {{if $p.Installed}}<span class="pick-tag">설치됨</span>{{end}}
        </li>
        {{end}}
        <li class="modal-empty" role="presentation" hidden>{{if .Available}}맞는 패키지가 없습니다{{else}}저장소 목록이 없습니다. update 를 먼저 실행하십시오{{end}}</li>
      </ul>
      <div class="modal-foot">
        <span class="modal-count" data-modal-count>고른 것 없음</span>
        <span class="modal-actions">
          <button type="button" class="btn" data-modal-close>취소</button>
          <button type="button" class="btn btn-primary" data-modal-ok>확인</button>
        </span>
      </div>
    </div>
  </div>

  <script>
  (function () {
    var held = null;

    // 화면에 보이는 것을 바꾸지 않는 명령입니다.
    var READONLY = ["version", "help", "list", "search", "show"];

    var triggers = document.querySelectorAll("[data-modal-open]");
    for (var i = 0; i < triggers.length; i++) {
      wire(triggers[i]);
    }

    // 뒤 화면이 따라 스크롤되지 않게 막습니다.
    //
    // body 에만 걸면 듣지 않습니다. html 의 overflow 가 visible 일 때만
    // body 설정이 화면 전체로 전달되는데, 이 문서는 html 에 이미
    // overflow-x: hidden 이 걸려 있어 그 전달이 끊깁니다.
    function lockScroll() {
      var root = document.documentElement;
      var bar = window.innerWidth - root.clientWidth;
      held = {
        root: root.style.overflow,
        body: document.body.style.overflow,
        pad: root.style.paddingRight
      };
      root.style.overflow = "hidden";
      document.body.style.overflow = "hidden";
      // 스크롤 막대가 사라지면서 본문이 옆으로 튀는 것을 막습니다.
      if (bar > 0) {
        root.style.paddingRight = bar + "px";
      }
    }

    function unlockScroll() {
      if (!held) {
        return;
      }
      var root = document.documentElement;
      root.style.overflow = held.root;
      document.body.style.overflow = held.body;
      root.style.paddingRight = held.pad;
      held = null;
    }

    function wire(input) {
      var modal = document.getElementById(input.getAttribute("data-modal-open"));
      if (!modal) {
        return;
      }
      var multi = modal.getAttribute("data-multi") === "1";
      var query = modal.querySelector("[data-modal-query]");
      var list = modal.querySelector("[data-modal-list]");
      var count = modal.querySelector("[data-modal-count]");
      var okButton = modal.querySelector("[data-modal-ok]");
      var items = [];
      var empty = null;
      var chosen = [];
      var active = -1;

      // 목록은 명령을 실행한 뒤 통째로 갈릴 수 있으므로, 붙잡아 두지 않고
      // 열 때마다 다시 읽습니다.
      function readItems() {
        items = Array.prototype.slice.call(list.querySelectorAll(".modal-item"));
        empty = list.querySelector(".modal-empty");
      }
      readItems();

      // 스크립트가 살아 있을 때만 직접 입력을 막고 모달로 넘깁니다.
      // 스크립트가 없으면 예전처럼 그냥 입력칸으로 쓸 수 있습니다.
      input.readOnly = true;
      input.setAttribute("aria-haspopup", "dialog");

      function tokens(v) {
        var raw = v.split(/[\s,]+/);
        var out = [];
        for (var j = 0; j < raw.length; j++) {
          if (raw[j]) {
            out.push(raw[j]);
          }
        }
        return out;
      }

      function shownItems() {
        var out = [];
        for (var j = 0; j < items.length; j++) {
          if (!items[j].hidden) {
            out.push(items[j]);
          }
        }
        return out;
      }

      function syncMarks() {
        for (var j = 0; j < items.length; j++) {
          var on = chosen.indexOf(items[j].getAttribute("data-value")) >= 0;
          items[j].setAttribute("aria-selected", on ? "true" : "false");
        }
      }

      function tell() {
        if (!count) {
          return;
        }
        if (multi) {
          count.textContent = chosen.length ? chosen.length + "개 고름: " + chosen.join(", ") : "고른 것 없음";
          return;
        }
        var text = query.value.trim();
        count.textContent = text ? text + " 로 찾습니다" : "적은 말로 찾습니다";
      }

      function refine() {
        var q = query.value.trim().toLowerCase();
        var hits = 0;
        for (var j = 0; j < items.length; j++) {
          var hit = q === "" || items[j].getAttribute("data-search").indexOf(q) >= 0;
          items[j].hidden = !hit;
          if (hit) {
            hits++;
          }
        }
        if (empty) {
          empty.hidden = hits > 0;
        }
        setActive(-1);
        tell();
      }

      function setActive(n) {
        for (var j = 0; j < items.length; j++) {
          items[j].classList.remove("is-active");
        }
        var shown = shownItems();
        if (n < 0 || n >= shown.length) {
          active = -1;
          query.removeAttribute("aria-activedescendant");
          return;
        }
        active = n;
        shown[n].classList.add("is-active");
        query.setAttribute("aria-activedescendant", shown[n].id);
        var top = shown[n].offsetTop;
        var bottom = top + shown[n].offsetHeight;
        if (top < list.scrollTop) {
          list.scrollTop = top;
        } else if (bottom > list.scrollTop + list.clientHeight) {
          list.scrollTop = bottom - list.clientHeight;
        }
      }

      function pick(el) {
        var value = el.getAttribute("data-value");
        if (!multi) {
          query.value = value;
          chosen = [value];
          syncMarks();
          tell();
          return;
        }
        var at = chosen.indexOf(value);
        if (at >= 0) {
          chosen.splice(at, 1);
        } else {
          chosen.push(value);
        }
        syncMarks();
        tell();
      }

      function commit() {
        // 여러 개 고르는 모달은 목록에서 고른 것만 값이 됩니다. 저장소에
        // 없는 이름은 어차피 설치할 수 없으므로 받지 않습니다.
        input.value = multi ? chosen.join(" ") : query.value.trim();
        close();
      }

      function open() {
        if (!modal.hidden) {
          return;
        }
        readItems();
        // 목록에 없는 이름은 지울 방법이 없으므로 아예 들이지 않습니다.
        chosen = [];
        var had = tokens(input.value);
        for (var j = 0; j < items.length; j++) {
          var v = items[j].getAttribute("data-value");
          if (had.indexOf(v) >= 0) {
            chosen.push(v);
          }
        }
        query.value = multi ? "" : input.value;
        syncMarks();
        refine();
        modal.hidden = false;
        lockScroll();
        query.focus();
        query.select();
      }

      function close() {
        if (modal.hidden) {
          return;
        }
        modal.hidden = true;
        unlockScroll();
        input.focus();
      }

      input.addEventListener("click", open);

      input.addEventListener("keydown", function (e) {
        if (e.key === "Enter" || e.key === " " || e.key === "ArrowDown") {
          e.preventDefault();
          open();
        }
      });

      query.addEventListener("input", refine);

      query.addEventListener("keydown", function (e) {
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          var shown = shownItems();
          if (!shown.length) {
            return;
          }
          if (e.key === "ArrowDown") {
            setActive(active + 1 >= shown.length ? 0 : active + 1);
          } else {
            setActive(active - 1 < 0 ? shown.length - 1 : active - 1);
          }
          return;
        }
        if (e.key === "Enter") {
          e.preventDefault();
          var here = shownItems()[active];
          if (!here) {
            commit();
            return;
          }
          pick(here);
          if (!multi) {
            commit();
            return;
          }
          // 짚어 둔 것을 풀어 줘야 다음 엔터로 확인할 수 있습니다.
          // 안 그러면 엔터가 같은 항목을 켰다 껐다만 반복합니다.
          setActive(-1);
        }
      });

      list.addEventListener("click", function (e) {
        var el = e.target.closest(".modal-item");
        if (el) {
          pick(el);
        }
      });

      okButton.addEventListener("click", commit);

      var closers = modal.querySelectorAll("[data-modal-close]");
      for (var k = 0; k < closers.length; k++) {
        closers[k].addEventListener("click", close);
      }

      modal.addEventListener("keydown", function (e) {
        if (e.key === "Escape") {
          e.preventDefault();
          close();
          return;
        }
        if (e.key !== "Tab") {
          return;
        }
        // 열려 있는 동안에는 모달 안에서만 초점이 돌게 붙잡습니다.
        var focusable = modal.querySelectorAll("button, input");
        if (!focusable.length) {
          return;
        }
        var first = focusable[0];
        var last = focusable[focusable.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      });
    }

    // 빠른 작업은 화면을 새로 열지 않고 결과만 받아 모달에 띄웁니다.
    // 스크립트가 없으면 폼이 그대로 제출되어 예전처럼 동작합니다.
    var resultModal = document.getElementById("modal-result");
    if (resultModal) {
      wireQuick(resultModal);
    }

    function wireQuick(box) {
      var title = box.querySelector(".modal-title");
      var body = box.querySelector("[data-result-body]");
      var state = box.querySelector("[data-result-state]");
      var closers = box.querySelectorAll("[data-result-close]");
      var buttons = box.querySelectorAll("button");
      var opener = null;
      var busy = false;
      var dirty = false;

      // 설치된 패키지 표는 명령을 실행한 뒤 통째로 갈리므로, 폼마다 따로
      // 걸지 않고 문서에서 한 번만 받습니다. 갈아 끼운 폼도 그대로 됩니다.
      document.addEventListener("submit", function (e) {
        var form = e.target;
        if (!form || !form.hasAttribute || !form.hasAttribute("data-quick")) {
          return;
        }
        // 지우기 전에 묻는 창에서 취소했으면 여기서 멈춰야 합니다.
        // 안 그러면 취소해 놓고도 명령이 나갑니다.
        if (e.defaultPrevented) {
          return;
        }
        e.preventDefault();
        if (busy) {
          return;
        }
        run(form);
      });

      function run(form) {
        var body_ = new URLSearchParams(new FormData(form));
        var cmd = body_.get("cmd") || "";
        body_.set("partial", "1");
        opener = form.querySelector("button");
        start(cmd);
        fetch(form.action, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded; charset=UTF-8" },
          body: body_.toString()
        }).then(function (res) {
          if (!res.ok) {
            throw new Error("서버가 " + res.status + " 로 답했습니다");
          }
          return res.json();
        }).then(function (out) {
          finish(out);
        }).catch(function (err) {
          finish({ command: cmd, ok: false, lines: [], error: "결과를 받지 못했습니다: " + err.message });
        });
      }

      function start(cmd) {
        busy = true;
        dirty = false;
        title.textContent = cmd + " 실행 중";
        state.textContent = "";
        body.textContent = "";
        var wrap = document.createElement("div");
        wrap.className = "running";
        var dot = document.createElement("span");
        dot.className = "spinner";
        var say = document.createElement("span");
        say.textContent = "명령을 실행하고 있습니다. 잠시 기다려 주십시오.";
        wrap.appendChild(dot);
        wrap.appendChild(say);
        body.appendChild(wrap);
        setBusy(true);
        if (box.hidden) {
          box.hidden = false;
          lockScroll();
        }
      }

      function finish(out) {
        busy = false;
        setBusy(false);
        title.textContent = out.command + " 결과";
        state.textContent = out.ok ? "완료했습니다" : "실패했습니다";
        body.textContent = "";

        if (out.error) {
          var bad = document.createElement("p");
          bad.className = "result-error";
          bad.textContent = out.error;
          body.appendChild(bad);
        }
        if (out.lines && out.lines.length) {
          var pre = document.createElement("pre");
          pre.className = "result-pre";
          pre.textContent = out.lines.join("\n");
          body.appendChild(pre);
        }
        if (!out.error && (!out.lines || !out.lines.length)) {
          var none = document.createElement("p");
          none.className = "empty";
          none.textContent = "알려 줄 내용이 없습니다.";
          body.appendChild(none);
        }

        // 보기만 하는 명령은 화면을 바꾸지 않으므로 다시 읽지 않습니다.
        dirty = out.ok && READONLY.indexOf(out.command) < 0;
        closers[closers.length - 1].focus();
      }

      function setBusy(on) {
        for (var i = 0; i < buttons.length; i++) {
          buttons[i].disabled = on;
        }
      }

      function shut() {
        if (busy || box.hidden) {
          return;
        }
        box.hidden = true;
        unlockScroll();
        if (opener) {
          opener.focus();
        }
        if (dirty) {
          dirty = false;
          refreshLive();
        }
      }

      for (var j = 0; j < closers.length; j++) {
        closers[j].addEventListener("click", shut);
      }

      box.addEventListener("keydown", function (e) {
        if (e.key === "Escape") {
          e.preventDefault();
          shut();
        }
      });
    }

    // 명령이 바꿔 놓은 부분만 조용히 다시 읽어 옵니다.
    // 화면을 새로 열지 않으므로 보던 자리가 그대로 남습니다.
    function refreshLive() {
      return fetch(window.location.pathname, { headers: { "X-Rpt-Refresh": "1" } })
        .then(function (res) { return res.text(); })
        .then(function (text) {
          var fresh = new DOMParser().parseFromString(text, "text/html");
          var slots = ["live-notice", "live-stats", "live-installed"];
          for (var i = 0; i < slots.length; i++) {
            swap(fresh, slots[i]);
          }
          var lists = document.querySelectorAll("[data-modal-list]");
          for (var j = 0; j < lists.length; j++) {
            swap(fresh, lists[j].id);
          }
        })
        .catch(function () {});
    }

    function swap(fresh, id) {
      if (!id) {
        return;
      }
      var from = fresh.getElementById(id);
      var to = document.getElementById(id);
      if (from && to) {
        to.innerHTML = from.innerHTML;
      }
    }
  })();
  </script>
</body>
</html>`
