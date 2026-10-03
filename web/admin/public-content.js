// 公開內容管理：嵌入 /admin/ 單頁後台的模組（與 reef-data.js 相同模式）。
// 列表在 resource panel 內渲染，編輯與核對使用後台共用的右側 drawer。

const esc = v => String(v ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;")

export const PUBLIC_CONTENT_PREFIX = "public_content:"

export const PUBLIC_CONTENT_KINDS = [
  { kind: "settings", title: "首頁顯示設定", description: "控制公開網站首頁的顯示方式，例如最新消息筆數。" },
  { kind: "event", title: "調查發布", description: "核對 Reef Check 調查場次的原始資料後，再決定是否公開。", creatable: false },
  { kind: "article", title: "文章連結", description: "首頁「最新消息」的外部文章連結與縮圖。" },
  { kind: "activity", title: "活動行程", description: "公開招募與內部活動的時間、地點與報名資訊。" },
  { kind: "development", title: "海岸開發案", description: "開發案基本資料、邊界、事件時間軸與圖片授權。" },
  { kind: "profile", title: "樣點公開資訊／最新報告", description: "各樣點的公開介紹、圖片與最新報告連結。" },
  { kind: "annotation", title: "樣點事件註記", description: "標註在樣點時間序列上的事件與查證來源。" },
  { kind: "chart", title: "樣點圖表／共用圖表", description: "設定公開頁面顯示的圖表類型、年份、深度與樣點。" },
  { kind: "audit", title: "公開內容稽核／匯入作業", description: "公開內容異動與資料匯入作業紀錄（唯讀）。", creatable: false, readOnly: true },
]

const STATUS = {
  draft: { label: "草稿", tone: "caution" },
  published: { label: "已發布", tone: "ok" },
  archived: { label: "封存", tone: "" },
}
const STATUS_ORDER = ["draft", "published", "archived"]
const CHART_TYPES = [["live_coral", "活珊瑚覆蓋率"], ["substrate", "底質"], ["taxa", "物種"], ["impact", "環境衝擊"], ["bleaching", "底質白化"]]
const ACTIVITY_STATUS = [["scheduled", "預定"], ["open", "報名中"], ["closed", "已截止"], ["cancelled", "已取消"], ["completed", "已完成"]]
const PAGE_SIZE = 30

export function kindMeta(kind) {
  return PUBLIC_CONTENT_KINDS.find(k => k.kind === kind) || null
}

function statusPill(status) {
  const s = STATUS[status] || { label: status || "未設定", tone: "" }
  return `<span class="pill ${s.tone ? `tone-${s.tone}` : ""}">${esc(s.label)}</span>`
}

function formatDateTime(value) {
  if (!value) return "—"
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return esc(value)
  return esc(d.toLocaleString("zh-TW", { timeZone: "Asia/Taipei", year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false }))
}

function toLocalInput(value) {
  return value ? new Date(value).toLocaleString("sv-SE", { timeZone: "Asia/Taipei" }).replace(" ", "T").slice(0, 16) : ""
}

// ---- form helpers -------------------------------------------------------
function field(name, label, value = "", type = "text", { help = "", wide = false } = {}) {
  return `<label class="form-field ${wide ? "pc-wide" : ""}"><span class="pc-label">${label}</span><input name="${name}" value="${esc(value)}" type="${type}">${help ? `<span class="form-help">${help}</span>` : ""}</label>`
}
function textarea(name, label, value = "", rows = 4, { help = "", mono = false } = {}) {
  return `<label class="form-field pc-wide"><span class="pc-label">${label}</span><textarea name="${name}" rows="${rows}" ${mono ? 'class="pc-mono"' : ""}>${esc(value)}</textarea>${help ? `<span class="form-help">${help}</span>` : ""}</label>`
}
function select(name, label, options, value, { wide = false } = {}) {
  return `<label class="form-field ${wide ? "pc-wide" : ""}"><span class="pc-label">${label}</span><select name="${name}">${options.map(([v, l]) => `<option value="${esc(v)}" ${String(v) === String(value ?? "") ? "selected" : ""}>${esc(l)}</option>`).join("")}</select></label>`
}
function section(title, body, note = "") {
  return `<section class="pc-form-section"><header><h3>${title}</h3>${note ? `<p class="meta-line">${note}</p>` : ""}</header><div class="pc-grid">${body}</div></section>`
}
function statusChoice(name, value, { legend = "發布狀態", help = "" } = {}) {
  return `<fieldset class="pc-status-choice"><legend class="pc-label">${legend}</legend><div class="pc-segmented">${STATUS_ORDER.map(s => `<label><input type="radio" name="${name}" value="${s}" ${s === (value || "draft") ? "checked" : ""}><span>${STATUS[s].label}</span></label>`).join("")}</div>${help ? `<p class="form-help">${help}</p>` : ""}</fieldset>`
}

function timelineRow(t = {}) {
  return `<fieldset class="pc-repeat timeline-row"><legend>時間軸事件</legend><div class="pc-grid">${field("t_title", "標題", t.title, "text", { wide: true })}${field("t_date", "日期", t.date, "text", { help: "依精度填 YYYY／YYYY-MM／YYYY-MM-DD" })}${select("t_precision", "日期精度", [["day", "日"], ["month", "月"], ["year", "年"], ["unknown", "未知"]], t.date_precision)}${field("t_description", "說明", t.description, "text", { wide: true })}${field("t_status", "狀態", t.status)}${field("t_source", "查證來源", t.source_url, "url")}${field("t_sort", "未知日期排序", t.sort_order ?? 0, "number")}${select("t_published", "發布此事件", [["false", "草稿"], ["true", "發布"]], String(t.is_published || false))}</div><button class="tiny-button danger-tiny" type="button" data-remove-row>移除此事件</button></fieldset>`
}
function mediaRow(m = {}) {
  return `<fieldset class="pc-repeat media-row"><legend>圖片</legend><div class="pc-grid">${field("m_url", "圖片網址", m.url, "url", { wide: true })}${field("m_alt", "替代文字", m.alt, "text", { wide: true })}${select("m_valid", "圖片有效性", [["false", "尚未確認"], ["true", "已確認有效"]], String(m.is_valid || false))}${select("m_licensed", "授權", [["false", "尚未確認"], ["true", "已確認授權"]], String(m.license_confirmed || false))}${select("m_published", "發布圖片", [["false", "草稿"], ["true", "發布"]], String(m.is_published || false))}</div><button class="tiny-button danger-tiny" type="button" data-remove-row>移除此圖片</button></fieldset>`
}

export function createPublicContent({ apiFetch, drawer, notify }) {
  const s = {
    kind: "",
    items: [],
    sites: [],
    sitesLoaded: false,
    loading: false,
    loaded: false,
    error: "",
    status: "",
    search: "",
    page: 0,
  }
  let panel = null
  let requestID = 0

  function listPath(kind) {
    if (kind === "audit") return "/admin/public-audit"
    if (kind === "event") return "/admin/reef-check-data/events"
    return `/admin/public-content/${kind}`
  }

  async function ensureSites() {
    if (s.sitesLoaded) return
    try {
      s.sites = (await apiFetch("/admin/sites")) || []
      s.sitesLoaded = true
    } catch (error) {
      s.sites = []
      notify({ tone: "caution", title: "無法載入樣點清單", message: error.message, id: "pc-sites" })
    }
  }

  async function load() {
    const request = ++requestID
    const kind = s.kind
    s.loading = true
    s.error = ""
    if (panel) renderPanel()
    try {
      const needsSites = ["profile", "annotation", "chart"].includes(kind)
      const [items] = await Promise.all([apiFetch(listPath(kind)), needsSites ? ensureSites() : null])
      if (request !== requestID) return
      s.items = Array.isArray(items) ? items : items?.items || []
      s.loaded = true
    } catch (error) {
      if (request !== requestID) return
      s.items = []
      s.loaded = false
      s.error = error.message || "載入失敗"
    } finally {
      if (request === requestID) {
        s.loading = false
        if (panel) renderPanel()
      }
    }
  }

  function render(target, kind) {
    panel = target
    if (kind !== s.kind) {
      s.kind = kind
      s.items = []
      s.loaded = false
      s.status = ""
      s.search = ""
      s.page = 0
      void load()
      return
    }
    renderPanel()
  }

  function siteName(id) {
    return s.sites.find(site => String(site.id) === String(id))?.name_zh || (id ? `樣點 #${id}` : "")
  }

  function rowStatus(item) {
    return item.publication_status || (s.kind === "event" ? "draft" : "")
  }

  function searchText(item) {
    if (s.kind === "audit") return [item.kind, item.id, JSON.stringify(item.details)].join(" ")
    if (s.kind === "event") return [item.site_name, item.site_english, item.event_id, item.survey_date, item.county].join(" ")
    const d = item.data || {}
    return [d.title, d.summary, d.location, d.county, d.authority, siteName(item.site_id)].join(" ")
  }

  function filtered() {
    const q = s.search.trim().toLowerCase()
    return s.items.filter(item => (!s.status || rowStatus(item) === s.status) && (!q || searchText(item).toLowerCase().includes(q)))
  }

  function secondaryLine(item) {
    const d = item.data || {}
    switch (s.kind) {
      case "article": return [d.date, d.external_url].filter(Boolean).join(" · ")
      case "activity": return [d.start_at ? formatDateTime(d.start_at) : "", d.location].filter(Boolean).join(" · ")
      case "development": return [d.county, d.authority, d.status].filter(Boolean).join(" · ")
      case "profile":
      case "annotation": return [siteName(item.site_id), d.date].filter(Boolean).join(" · ")
      case "chart": return [CHART_TYPES.find(([v]) => v === d.chart)?.[1], item.site_id ? siteName(item.site_id) : (d.site_ids?.length ? `共用 · ${d.site_ids.length} 個樣點` : "")].filter(Boolean).join(" · ")
      case "settings": return d.article_limit != null ? `首頁最新消息 ${d.article_limit} 筆` : ""
      default: return d.summary || ""
    }
  }

  // ---- list -------------------------------------------------------------
  function renderPanel() {
    if (!panel) return
    const meta = kindMeta(s.kind)
    if (!meta) {
      panel.innerHTML = '<div class="empty-state"><p>找不到這個管理項目。</p></div>'
      return
    }
    const creatable = meta.creatable !== false
    const header = `
      <div class="panel-header">
        <div>
          <h2>${esc(meta.title)}</h2>
          <p class="meta-line">${meta.readOnly ? "僅供查閱，紀錄由系統自動產生。" : s.kind === "event" ? "公開網站只會顯示「已發布」的調查；發布前請先核對原始資料。" : "公開網站只會顯示「已發布」的內容；新內容一律先存為草稿。"}</p>
        </div>
        <div class="panel-actions">
          ${s.loaded ? `<span class="session-pill">${s.items.length} 筆</span>` : ""}
          <button type="button" class="tiny-button" data-pc="refresh" ${s.loading ? "disabled" : ""}>重新整理</button>
          ${creatable ? '<button type="button" class="primary-button" data-pc="new">＋ 新增草稿</button>' : ""}
        </div>
      </div>`

    if (s.loading && !s.loaded) {
      panel.innerHTML = `${header}<div class="empty-state"><p role="status">正在載入${esc(meta.title)}…</p></div>`
      bindPanel()
      return
    }
    if (s.error) {
      panel.innerHTML = `${header}<div class="empty-state"><h3>無法載入${esc(meta.title)}</h3><p class="meta-line">${esc(s.error)}</p><button type="button" class="primary-button" data-pc="refresh">重新載入</button></div>`
      bindPanel()
      return
    }

    const rows = filtered()
    const pages = Math.max(1, Math.ceil(rows.length / PAGE_SIZE))
    s.page = Math.min(s.page, pages - 1)
    const pageRows = rows.slice(s.page * PAGE_SIZE, (s.page + 1) * PAGE_SIZE)
    const counts = Object.fromEntries(STATUS_ORDER.map(st => [st, s.items.filter(i => rowStatus(i) === st).length]))
    const statusFilter = meta.readOnly ? "" : `
      <div class="pc-segmented pc-filter" role="group" aria-label="依發布狀態篩選">
        <button type="button" data-status="" class="${!s.status ? "is-active" : ""}" aria-pressed="${!s.status}">全部 <span>${s.items.length}</span></button>
        ${STATUS_ORDER.map(st => `<button type="button" data-status="${st}" class="${s.status === st ? "is-active" : ""}" aria-pressed="${s.status === st}">${STATUS[st].label} <span>${counts[st]}</span></button>`).join("")}
      </div>`
    const toolbar = `
      <div class="pc-toolbar">
        ${statusFilter}
        <label class="pc-search"><span class="sr-only">搜尋</span><input type="search" value="${esc(s.search)}" placeholder="${s.kind === "event" ? "搜尋樣點、場次編號或日期" : "搜尋標題、摘要或樣點"}" data-pc="search"></label>
      </div>`

    let body
    if (!s.items.length) {
      body = `<div class="empty-state"><p>目前還沒有任何${esc(meta.title)}。</p>${creatable ? '<button type="button" class="primary-button" data-pc="new">＋ 新增第一筆草稿</button>' : ""}</div>`
    } else if (!rows.length) {
      body = '<div class="empty-state"><p>沒有符合篩選條件的資料。</p><button type="button" class="tiny-button" data-pc="clear">清除篩選</button></div>'
    } else if (s.kind === "audit") {
      body = table(["時間", "類型", "內容"], pageRows.map(c => `
        <tr>
          <td class="pc-nowrap">${formatDateTime(c.created_at)}<small>#${esc(c.id)}</small></td>
          <td><span class="pill tone-accent">${esc(c.kind)}</span></td>
          <td><details class="pc-details"><summary>查看詳細內容</summary><pre class="pc-pre">${esc(JSON.stringify(c.details, null, 2))}</pre></details></td>
        </tr>`))
    } else if (s.kind === "event") {
      body = table(["日期／時間", "樣點", "深度", "狀態", ""], pageRows.map(c => `
        <tr>
          <td class="pc-nowrap">${esc(c.survey_date)}<small>${!c.event_time || c.event_time === "na" ? "時間未記錄" : esc(String(c.event_time).replaceAll("-", ":"))}</small></td>
          <td><strong>${esc(c.site_name)}</strong><small>${esc(c.event_id)}</small></td>
          <td class="pc-nowrap">${esc(c.depth_m)} m</td>
          <td>${statusPill(rowStatus(c))}</td>
          <td class="pc-actions"><button type="button" class="tiny-button" data-review="${esc(c.id)}">核對與發布</button></td>
        </tr>`))
    } else {
      body = table(["標題", "狀態", "最後更新", ""], pageRows.map(c => `
        <tr class="pc-row" data-edit="${esc(c.id)}">
          <td><strong>${esc(c.data?.title || "（未命名）")}</strong>${secondaryLine(c) ? `<small>${esc(secondaryLine(c))}</small>` : ""}</td>
          <td>${statusPill(c.publication_status)}</td>
          <td class="pc-nowrap">${formatDateTime(c.updated_at)}</td>
          <td class="pc-actions"><button type="button" class="tiny-button" data-edit="${esc(c.id)}">編輯</button></td>
        </tr>`))
    }

    const pager = rows.length > PAGE_SIZE ? `
      <div class="pc-pagination">
        <span class="meta-line">符合 ${rows.length} 筆 · 第 ${s.page + 1} / ${pages} 頁</span>
        <button type="button" class="tiny-button" data-pc="prev" ${s.page === 0 ? "disabled" : ""}>上一頁</button>
        <button type="button" class="tiny-button" data-pc="next" ${s.page + 1 >= pages ? "disabled" : ""}>下一頁</button>
      </div>` : ""

    panel.innerHTML = `${header}${s.items.length ? toolbar : ""}${body}${pager}`
    bindPanel()
  }

  function table(head, rows) {
    return `<div class="rd-table-scroll"><table class="rd-table pc-table"><thead><tr>${head.map(h => `<th scope="col">${esc(h)}</th>`).join("")}</tr></thead><tbody>${rows.join("")}</tbody></table></div>`
  }

  function bindPanel() {
    panel.querySelectorAll('[data-pc="refresh"]').forEach(b => { b.onclick = () => load() })
    panel.querySelectorAll('[data-pc="new"]').forEach(b => { b.onclick = () => openEditor(null) })
    panel.querySelector('[data-pc="clear"]')?.addEventListener("click", () => { s.status = ""; s.search = ""; s.page = 0; renderPanel() })
    panel.querySelector('[data-pc="prev"]')?.addEventListener("click", () => { s.page--; renderPanel() })
    panel.querySelector('[data-pc="next"]')?.addEventListener("click", () => { s.page++; renderPanel() })
    panel.querySelectorAll("[data-status]").forEach(b => { b.onclick = () => { s.status = b.dataset.status; s.page = 0; renderPanel() } })
    const search = panel.querySelector('[data-pc="search"]')
    if (search) {
      // 中文輸入法組字期間不重繪，避免打斷注音／拼音輸入。
      const apply = () => {
        s.search = search.value
        s.page = 0
        renderPanel()
        const next = panel?.querySelector('[data-pc="search"]')
        next?.focus()
        next?.setSelectionRange(next.value.length, next.value.length)
      }
      search.addEventListener("input", event => { if (!event.isComposing) apply() })
      search.addEventListener("compositionend", apply)
    }
    panel.querySelectorAll("[data-edit]").forEach(el => {
      el.onclick = event => {
        event.stopPropagation()
        const item = s.items.find(c => String(c.id) === el.dataset.edit)
        if (item) openEditor(item)
      }
    })
    panel.querySelectorAll("[data-review]").forEach(b => {
      b.onclick = () => {
        const item = s.items.find(c => String(c.id) === b.dataset.review)
        if (item) openReview(item)
      }
    })
  }

  // ---- content editor (drawer) -------------------------------------------
  function editorFields(kind, item) {
    const d = item?.data || {}
    const siteOptions = [["", "請選擇"], ...s.sites.map(site => [String(site.id), site.name_zh])]
    const basic = field("title", "標題／名稱", d.title, "text", { wide: true }) + textarea("summary", "摘要", d.summary, 4) + field("sort_order", "排序", d.sort_order ?? 0, "number", { help: "數字越小越前面" })
    const parts = [section("基本資料", basic)]
    if (kind === "settings") parts.push(section("首頁設定", field("article_limit", "首頁最新消息顯示筆數", d.article_limit ?? 7, "number", { help: "可設定 1–50 筆" })))
    if (kind === "article") parts.push(section("文章資訊", field("external_url", "文章外部網址", d.external_url, "url", { wide: true }) + field("image_url", "縮圖網址", d.image_url, "url", { wide: true }) + field("date", "發布日期", d.date, "date")))
    if (kind === "activity") parts.push(section("活動資訊",
      field("start_at", "開始時間", toLocalInput(d.start_at), "datetime-local") + field("end_at", "結束時間", toLocalInput(d.end_at), "datetime-local") +
      field("location", "地點", d.location, "text", { wide: true }) +
      select("recruitment_type", "招募類型", [["public", "公開招募"], ["internal", "內部活動"]], d.recruitment_type) +
      select("status", "活動狀態", ACTIVITY_STATUS, d.status) +
      field("registration_url", "報名網址", d.registration_url, "url", { wide: true }), "時間以台灣時間（UTC+8）儲存。"))
    if (["profile", "annotation", "chart"].includes(kind)) {
      parts.push(section("樣點", select("site_id", kind === "chart" ? "樣點（共用圖表可留空）" : "樣點", siteOptions, String(item?.site_id || ""), { wide: true })))
    }
    if (kind === "profile") parts.push(section("報告與圖片", field("report_url", "最新報告網址", d.report_url, "url", { wide: true }) + field("image_url", "圖片網址", d.image_url, "url", { wide: true }) + field("date", "報告日期", d.date, "date")))
    if (kind === "annotation") parts.push(section("事件", field("date", "事件日期", d.date, "date") + field("source_url", "查證來源網址", d.source_url, "url", { wide: true })))
    if (kind === "chart") {
      parts.push(section("圖表設定",
        select("chart", "資料類型", CHART_TYPES, d.chart) +
        select("is_visible", "公開顯示", [["true", "顯示"], ["false", "隱藏"]], String(d.is_visible ?? true)) +
        field("year_from", "起始年份", d.year_from ?? "", "number") + field("year_to", "結束年份", d.year_to ?? "", "number") +
        field("depths_m", "深度（公尺）", (d.depths_m || []).join(","), "text", { help: "以逗號分隔，例如 5,10" }) +
        field("category_keys", "顯示項目代碼／物種編號", (d.category_keys || []).join(","), "text", { help: "以逗號分隔；留空代表全部", wide: true })))
      parts.push(`<section class="pc-form-section"><header><h3>共用圖表樣點</h3><p class="meta-line">勾選要一起比較的樣點。</p></header><div class="pc-checks">${s.sites.map(site => `<label><input name="site_ids" type="checkbox" value="${esc(site.id)}" ${(d.site_ids || []).includes(site.id) ? "checked" : ""}><span>${esc(site.name_zh)}</span></label>`).join("") || '<p class="meta-line">尚無樣點資料。</p>'}</div></section>`)
    }
    if (kind === "development") {
      parts.push(section("開發案資料",
        field("county", "縣市", d.county) + field("authority", "主管機關", d.authority) +
        field("area_m2", "面積 m²", d.area_m2 ?? "", "number", { help: "未知請留空" }) + field("investment_ntd", "投資金額 NTD", d.investment_ntd ?? "", "number", { help: "未知請留空" }) +
        field("status", "正式程序狀態", d.status, "text", { help: "需人工查證", wide: true }) + field("source_url", "資料來源網址", d.source_url, "url", { wide: true }) +
        textarea("geometry", "點／面邊界（GeoJSON Geometry）", JSON.stringify(d.geometry || { type: "Point", coordinates: [] }, null, 2), 6, { help: "保留完整座標，不要簡化。", mono: true })))
      parts.push(`<section class="pc-form-section"><header><h3>事件時間軸</h3></header><div id="pc-timeline" class="pc-repeat-list">${(d.timeline || []).map(timelineRow).join("")}</div><button class="ghost-button" type="button" id="pc-add-timeline">＋ 新增時間軸事件</button></section>`)
      parts.push(`<section class="pc-form-section"><header><h3>圖片及授權</h3><p class="meta-line">圖片需確認有效並取得授權才會公開。</p></header><div id="pc-media" class="pc-repeat-list">${(d.media || []).map(mediaRow).join("")}</div><button class="ghost-button" type="button" id="pc-add-media">＋ 新增圖片</button></section>`)
    }
    parts.push(`<section class="pc-form-section">${statusChoice("publication_status", item?.publication_status || "draft", { help: "核對完成後再改為「已發布」；封存後不會出現在公開網站。" })}</section>`)
    return parts.join("")
  }

  async function openEditor(item, { notice = "" } = {}) {
    const kind = s.kind
    const meta = kindMeta(kind)
    if (["profile", "annotation", "chart"].includes(kind)) await ensureSites()
    let dirty = false
    drawer.open({
      eyebrow: meta.title,
      title: item ? (item.data?.title || "（未命名）") : "新增草稿",
      subtitle: item ? `內容 #${item.id} · 最後更新 ${formatDateTime(item.updated_at)}` : meta.description,
      html: `
        ${item ? `<div class="drawer-hero-meta pc-hero">${statusPill(item.publication_status)}</div>` : ""}
        ${notice ? `<p class="pc-inline-notice" role="status">${esc(notice)}</p>` : ""}
        <form id="pc-edit-form" class="editor-form pc-form" novalidate>
          ${editorFields(kind, item)}
          <p id="pc-form-error" class="pc-form-error" role="alert" hidden></p>
          <div class="editor-actions pc-sticky-actions">
            <button type="button" class="ghost-button" data-pc-cancel>取消</button>
            <button type="submit" class="primary-button">${item ? "儲存變更" : "建立草稿"}</button>
          </div>
        </form>`,
      guard: () => !dirty || window.confirm("尚未儲存的修改將被捨棄，確定關閉？"),
      bind(body) {
        const form = body.querySelector("#pc-edit-form")
        form.addEventListener("input", () => { dirty = true })
        form.addEventListener("change", () => { dirty = true })
        body.querySelector("[data-pc-cancel]").onclick = () => drawer.close()
        body.querySelector("#pc-add-timeline")?.addEventListener("click", () => { body.querySelector("#pc-timeline").insertAdjacentHTML("beforeend", timelineRow({})); dirty = true })
        body.querySelector("#pc-add-media")?.addEventListener("click", () => { body.querySelector("#pc-media").insertAdjacentHTML("beforeend", mediaRow({})); dirty = true })
        form.addEventListener("click", event => {
          const remove = event.target.closest("[data-remove-row]")
          if (remove) { remove.closest(".pc-repeat")?.remove(); dirty = true }
        })
        form.onsubmit = async event => {
          event.preventDefault()
          const ok = await save(form, kind, item)
          if (ok) dirty = false
        }
      },
    })
  }

  function collect(form, kind) {
    const f = new FormData(form)
    const d = {}
    for (const [k, v] of f) {
      if (["site_id", "site_ids", "publication_status", "geometry"].includes(k) || k.startsWith("t_") || k.startsWith("m_")) continue
      if (["area_m2", "investment_ntd", "year_from", "year_to", "article_limit"].includes(k)) { if (v !== "") d[k] = Number(v) }
      else if (k === "sort_order") d[k] = Number(v)
      else if (k === "is_visible") d[k] = v === "true"
      else if (k === "depths_m") d[k] = v ? String(v).split(",").map(Number) : []
      else if (k === "category_keys") d[k] = v ? String(v).split(",").map(x => x.trim()) : []
      else if (k === "start_at" || k === "end_at") d[k] = v ? `${v}:00+08:00` : ""
      else d[k] = v
    }
    if (kind === "development") {
      try {
        d.geometry = JSON.parse(f.get("geometry"))
      } catch (_) {
        throw new Error("GeoJSON 格式錯誤，請檢查邊界欄位。")
      }
      const val = (el, n) => el.querySelector(`[name=${n}]`).value
      d.timeline = [...form.querySelectorAll(".timeline-row")].map(el => ({ title: val(el, "t_title"), date: val(el, "t_date"), date_precision: val(el, "t_precision"), description: val(el, "t_description"), status: val(el, "t_status"), source_url: val(el, "t_source"), sort_order: Number(val(el, "t_sort")), is_published: val(el, "t_published") === "true" }))
      d.media = [...form.querySelectorAll(".media-row")].map(el => ({ url: val(el, "m_url"), alt: val(el, "m_alt"), is_valid: val(el, "m_valid") === "true", license_confirmed: val(el, "m_licensed") === "true", is_published: val(el, "m_published") === "true" }))
    }
    if (kind === "chart") d.site_ids = f.getAll("site_ids").map(Number)
    return { data: d, siteID: f.get("site_id") ? Number(f.get("site_id")) : null, status: f.get("publication_status") || "draft" }
  }

  async function save(form, kind, current) {
    const errorEl = form.querySelector("#pc-form-error")
    const submit = form.querySelector('[type="submit"]')
    errorEl.hidden = true
    let payload
    try {
      payload = collect(form, kind)
    } catch (error) {
      errorEl.textContent = error.message
      errorEl.hidden = false
      return false
    }
    if (payload.status === "archived" && current?.publication_status !== "archived" && !window.confirm("確定封存此內容？封存後不再出現在公開網站。")) return false
    submit.disabled = true
    submit.textContent = "儲存中…"
    try {
      const body = { kind, data: payload.data, site_id: payload.siteID, publication_status: payload.status, version: current?.updated_at || "", confirm: payload.status === "archived" }
      const saved = await apiFetch(`/admin/public-content/${kind}${current ? `/${current.id}` : ""}`, { method: current ? "PATCH" : "POST", body })
      const message = current ? "已儲存變更。" : "已建立草稿；核對後可改為「已發布」。"
      notify({ tone: "ok", title: message, id: "pc-save" })
      await load()
      if (kind === s.kind) openEditor(saved || current, { notice: message })
      return true
    } catch (error) {
      errorEl.textContent = error.message || "儲存失敗"
      errorEl.hidden = false
      submit.disabled = false
      submit.textContent = current ? "儲存變更" : "建立草稿"
      return false
    }
  }

  // ---- survey publication review (drawer) --------------------------------
  function openReview(item) {
    const status = rowStatus(item)
    drawer.open({
      eyebrow: "調查發布",
      title: `${item.site_name || "調查場次"} · ${item.survey_date || ""}`,
      subtitle: item.event_id || `場次 #${item.id}`,
      html: `
        <div class="drawer-hero-meta pc-hero">${statusPill(status)}<span class="pill">${esc(item.depth_m)} m</span></div>
        <section class="pc-form-section">
          <header><h3>原始資料與記錄者</h3><p class="meta-line">發布前請先核對原始資料。</p></header>
          <div id="pc-review-detail" class="pc-review-detail"><p class="meta-line" role="status">正在載入核對資料…</p></div>
        </section>
        <form id="pc-publish-form" class="editor-form pc-form">
          <section class="pc-form-section">${statusChoice("publication_status", status, { legend: "變更發布狀態", help: "公開網站只會顯示「已發布」的調查。" })}</section>
          <p id="pc-form-error" class="pc-form-error" role="alert" hidden></p>
          <div class="editor-actions pc-sticky-actions">
            <button type="button" class="ghost-button" data-pc-cancel>關閉</button>
            <button type="submit" class="primary-button">更新發布狀態</button>
          </div>
        </form>`,
      bind(body) {
        body.querySelector("[data-pc-cancel]").onclick = () => drawer.close()
        const detailEl = body.querySelector("#pc-review-detail")
        apiFetch(`/admin/reef-check-data/events/${item.id}`)
          .then(detail => { detailEl.innerHTML = `<pre class="pc-pre">${esc(JSON.stringify(detail, null, 2))}</pre>` })
          .catch(error => { detailEl.innerHTML = `<p class="pc-form-error" role="alert">無法載入核對資料：${esc(error.message)}</p>` })
        const form = body.querySelector("#pc-publish-form")
        form.onsubmit = async event => {
          event.preventDefault()
          const next = new FormData(form).get("publication_status")
          const errorEl = form.querySelector("#pc-form-error")
          errorEl.hidden = true
          if (next === status) { errorEl.textContent = "發布狀態沒有變更。"; errorEl.hidden = false; return }
          if (!window.confirm(`確定將調查 #${item.id} 改為「${STATUS[next]?.label || next}」？請先核對原始資料。`)) return
          const submit = form.querySelector('[type="submit"]')
          submit.disabled = true
          try {
            await apiFetch(`/admin/reef-check-data/events/${item.id}/publication`, { method: "PATCH", body: { publication_status: next, confirm: true } })
            notify({ tone: "ok", title: "已更新發布狀態", message: `${item.site_name || ""} ${item.survey_date || ""} → ${STATUS[next]?.label || next}`, id: "pc-publish" })
            drawer.close()
            await load()
          } catch (error) {
            errorEl.textContent = error.message || "更新失敗"
            errorEl.hidden = false
            submit.disabled = false
          }
        }
      },
    })
  }

  return {
    render,
    load,
    leave() {
      // 進行中的載入仍可完成並更新狀態，只是不再寫入已切換的面板。
      panel = null
      return true
    },
    reset() {
      requestID++
      Object.assign(s, { kind: "", items: [], sites: [], sitesLoaded: false, loading: false, loaded: false, error: "", status: "", search: "", page: 0 })
      panel = null
    },
  }
}
