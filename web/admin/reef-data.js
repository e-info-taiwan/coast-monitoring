

const methods = { line: "底質 Line", belt_fish: "魚類 Belt", belt_invert: "無脊椎・罕見生物・環境影響" }
const roles = { member: "成員", team_leader: "隊長", team_scientist: "科學指導員" }
const groups = { fish: "魚類", invert: "無脊椎", rare: "罕見生物" }
const segments = ["0–20m", "25–45m", "50–70m", "75–95m"]
const esc = v => String(v ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;")
const text = v => v === null || v === undefined || v === "" ? "未記錄" : esc(v)
const number = v => v === null || v === undefined ? "—" : Number(v).toLocaleString("zh-TW", { maximumFractionDigits: 2 })
const table = (headers, rows) => `<div class="rd-table-scroll"><table class="rd-table"><thead><tr>${headers.map(h => `<th scope="col">${h}</th>`).join("")}</tr></thead><tbody>${rows}</tbody></table></div>`
const options = (values, selected, label) => `<option value="">${label}</option>${values.map(([value, title]) => `<option value="${esc(value)}" ${String(value) === selected ? "selected" : ""}>${esc(title)}</option>`).join("")}`

export function createReefData({ apiFetch, onCount, onManageActivities }) {
  const s = {
    events: [],
    codes: [],
    sites: [],
    users: [],
    divers: [],
    cwaStations: [],
    cwaStationsError: "",
    error: "",
    loading: false,
    loaded: false,
    detail: null,
    transectID: null,
    publication_status:"published", activity_id:"", event_ids:"", survey_date:"", depth_m:"",
    year: "",
    site: "",
    method: "",
    diver: "",
    search: "",
    page: 0,
    editing: false,
    boardMode: "raw",
    saving: false,
    exporting: false,
    notice: "",
  }
  let panel = null
  let requestID = 0

  async function load() {
    s.loading = true
    s.error = ""
    try {
      const [events, codes] = await Promise.all([
        apiFetch("/admin/reef-check-data/events"),
        apiFetch("/admin/reef-check-data/codes"),
      ])
      s.events = events || []
      s.codes = codes || []
      s.loaded = true
      onCount(s.events.length)
    } catch (error) {
      s.error = error.message
      s.loaded = false
    } finally {
      s.loading = false
    }
  }

  async function getSites() {
    if (!s.sites.length) {
      s.sites = (await apiFetch("/admin/reef-check-data/sites")) || []
    }
    return s.sites
  }

  async function getCWAStations() {
    if (!s.cwaStations.length) {
      s.cwaStationsError = ""
      try {
        const res = await apiFetch("/admin/cwa-marine/stations")
        s.cwaStations = res || []
      } catch (err) {
        s.cwaStationsError = `無法載入測站清單：${err.message || "請稍後重試"}。可先手動填寫水溫，或關閉視窗後重新開啟以重試。`
        s.cwaStations = []
      }
    }
    return s.cwaStations
  }

  async function getParticipantsLookup() {
    if (!s.users.length || !s.divers.length) {
      const [users, divers] = await Promise.all([
        apiFetch("/admin/reef-check-data/users"),
        apiFetch("/admin/reef-check-data/divers"),
      ])
      s.users = users || []
      s.divers = divers || []
    }
    return { users: s.users, divers: s.divers }
  }

  function leave() {
    if (s.saving) return false
    if (s.editing && !window.confirm("尚未儲存的修改將被捨棄，確定離開？")) return false
    s.editing = false
    s.notice = ""
    requestID++
    panel = null
    return true
  }

  function render(target) {
    panel = target
    if (s.loading) {
      panel.innerHTML = '<p role="status">正在載入實際觀測資料…</p>'
      return
    }
    if (!s.loaded) {
      panel.innerHTML = `<div class="empty-state"><h2>觀測資料尚未載入</h2><p>${esc(s.error)}</p><button class="primary-button" id="rd-retry">重新載入</button></div>`
      panel.querySelector("#rd-retry").onclick = async () => {
        const current = panel
        const pending = load()
        render(current)
        await pending
        if (panel === current) render(current)
      }
      return
    }
    if (s.detail) renderDetail()
    else renderList()
  }

  function renderList() {
    const years = [...new Set(s.events.map(e => e.survey_date.slice(0, 4)))].sort().reverse().map(y => [y, y])
    const sites = [...new Map(s.events.map(e => [e.site_id, [e.site_id, `${e.site_name} · ${e.site_english}`]])).values()].sort((a, b) => a[1].localeCompare(b[1], "zh-TW"))
    const diverNames = [...new Set(s.events.flatMap(event => (event.divers || []).map(person => person.label)))].sort((a, b) => a.localeCompare(b, "zh-TW"))
    const diverQuery = s.diver.trim().toLowerCase()
    const matchesDiver = event => !diverQuery || (event.divers || []).some(person =>
      (!s.method || person.method === s.method) && person.label.toLowerCase().includes(diverQuery))
    const query = s.search.trim().toLowerCase()
    const filtered = s.events.filter(e => matchesDiver(e) && (s.publication_status==="all" || e.publication_status===s.publication_status) && (!s.activity_id || String(e.activity_id)===s.activity_id) && (!s.event_ids || s.event_ids.split(",").map(x=>x.trim()).includes(e.event_id)) && (!s.survey_date || e.survey_date===s.survey_date) && (!s.depth_m || e.depth_m===Number(s.depth_m)) && (!s.year || e.survey_date.startsWith(s.year)) && (!s.site || String(e.site_id) === s.site) && (!s.method || e.methods.includes(s.method)) && (!query || [e.event_id, e.site_name, e.site_english, e.county, e.region, e.label].join(" ").toLowerCase().includes(query)))
    const pages = Math.max(1, Math.ceil(filtered.length / 30))
    s.page = Math.min(s.page, pages - 1)
    const rows = filtered.slice(s.page * 30, (s.page + 1) * 30)

    panel.innerHTML = `
      <div class="panel-header">
        <div>
          <h2>Reef Check 觀測資料</h2>
          <p class="meta-line">依樣點、日期、時間與深度查看實際調查；未記錄的調查方法不補成零。</p>
        </div>
        <div class="rd-header-actions">
          <button type="button" class="tiny-button rd-report-button" id="rd-export-filtered" data-empty="${!filtered.length}" ${s.exporting || !filtered.length ? "disabled" : ""}>下載篩選報告 ZIP（${filtered.length}）</button>
          <a class="tiny-button" id="rd-manage-activities" href="/admin/#public_content:activity" title="開啟活動行程，建立活動、產生或重發志工填報代碼">管理活動與志工填報代碼</a>
          <button type="button" class="primary-button" id="rd-create-event">＋ 新增調查場次</button>
          <button type="button" class="tiny-button" id="rd-refresh">重新整理</button>
        </div>
      </div>
      <div class="rd-stats">
        <div><strong>${new Set(s.events.map(e => e.survey_id)).size}</strong><span>出動 Survey</span></div>
        <div><strong>${s.events.length}</strong><span>場次 Event</span></div>
        <div><strong>${s.events.reduce((n, e) => n + e.methods.length, 0)}</strong><span>穿越線 Transect</span></div>
        <div><strong>${sites.length}</strong><span>有觀測的樣點</span></div>
      </div>
      <form id="rd-filters" class="rd-filters">
        <label>發布狀態<select name="publication_status">${[["published","已發布"],["draft","草稿"],["archived","封存"],["all","全部"]].map(([v,l])=>`<option value="${v}" ${s.publication_status===v?"selected":""}>${l}</option>`).join("")}</select></label>
        <label>活動<select name="activity_id">${options([...new Map(s.events.filter(e=>e.activity_id).map(e=>[e.activity_id,[e.activity_id,e.activity_name||`活動 ${e.activity_id}`]])).values()],s.activity_id,"全部活動")}</select></label>
        <label>場次編號（逗號分隔）<input name="event_ids" value="${esc(s.event_ids)}"></label>
        <label>日期<input name="survey_date" type="date" value="${esc(s.survey_date)}"></label>
        <label>深度 m<input name="depth_m" type="number" min="0" step="any" value="${esc(s.depth_m)}"></label>
        <label>年份<select name="year">${options(years, s.year, "全部年份")}</select></label>
        <label>樣點<select name="site">${options(sites, s.site, "全部樣點")}</select></label>
        <label>潛水員<input type="search" name="diver" list="rd-diver-choices" value="${esc(s.diver)}" placeholder="姓名或證號"></label>
        <datalist id="rd-diver-choices">${diverNames.map(label => `<option value="${esc(label)}"></option>`).join("")}</datalist>
        <label>調查方法<select name="method">${options(Object.entries(methods), s.method, "全部方法")}</select></label>
        <label>搜尋<input type="search" name="search" value="${esc(s.search)}" placeholder="樣點、縣市或場次編號"></label>
        <button class="primary-button">篩選</button>
        <button type="button" class="tiny-button" id="rd-clear">清除</button>
      </form>
      <p class="meta-line" role="status">符合 ${filtered.length} 個場次 · 第 ${s.page + 1} / ${pages} 頁</p>
      ${s.notice ? `<p id="rd-notice" role="status" class="rd-notice">${esc(s.notice)}</p>` : ""}
      ${rows.length ? table(["日期／時間", "樣點", "深度", "已記錄方法", ""], rows.map(e => `
        <tr>
          <td>${esc(e.survey_date)}<small>${e.event_time === "na" ? "時間未記錄" : esc(e.event_time.replaceAll("-", ":"))}</small></td>
          <td><strong>${esc(e.site_name)}</strong><small>${esc(e.site_english)} · ${esc(e.county || e.region)}</small></td>
          <td>${number(e.depth_m)} m</td>
          <td><div class="rd-tags">${e.methods.map(m => `<span>${esc(methods[m] || m)}</span>`).join("")}</div></td>
          <td><button class="tiny-button" data-event="${e.id}" aria-label="查看 ${esc(e.event_id)}">查看</button></td>
        </tr>`).join("")) : '<div class="empty-state">沒有符合條件的場次。</div>'}
      <div class="rd-pagination">
        <button class="tiny-button" id="rd-prev" ${s.page === 0 ? "disabled" : ""}>上一頁</button>
        <button class="tiny-button" id="rd-next" ${s.page + 1 >= pages ? "disabled" : ""}>下一頁</button>
      </div>
    `
    panel.querySelector("#rd-filters").onsubmit = event => {
      event.preventDefault()
      const f = new FormData(event.currentTarget)
      for (const key of ["year", "site", "method", "diver", "search", "publication_status","activity_id","event_ids","survey_date","depth_m"]) s[key] = String(f.get(key) || "")
      s.page = 0
      renderList()
    }
    panel.querySelector("#rd-clear").onclick = () => {
      s.year = s.site = s.method = s.diver = s.search = s.activity_id = s.event_ids = s.survey_date = s.depth_m = "";s.publication_status="published"
      s.page = 0
      renderList()
    }
    panel.querySelector("#rd-prev").onclick = () => { s.page--; renderList() }
    panel.querySelector("#rd-next").onclick = () => { s.page++; renderList() }
    panel.querySelector("#rd-refresh").onclick = async () => {
      const current = panel
      const pending = load()
      render(current)
      await pending
      if (panel === current) render(current)
    }
    panel.querySelector("#rd-export-filtered")?.addEventListener("click", event => {
      const query = new URLSearchParams({ year: s.year, site: s.site, method: s.method, diver: s.diver, search: s.search,publication_status:s.publication_status,activity_id:s.activity_id,event_ids:s.event_ids,survey_date:s.survey_date,depth_m:s.depth_m })
      downloadReport(`/admin/reef-check-data/reports.zip?${query}`, "ReefCheck-範本報告套組.zip", event.currentTarget)
    })
    panel.querySelector("#rd-create-event")?.addEventListener("click", showCreateEventModal)
    panel.querySelector("#rd-manage-activities")?.addEventListener("click", event => {
      if (onManageActivities) { event.preventDefault(); onManageActivities() }
    })
    panel.querySelectorAll("[data-event]").forEach(b => { b.onclick = () => openEvent(b.dataset.event) })
  }

  async function downloadReport(path, filename, button) {
    if (s.exporting || s.editing) return
    s.exporting = true
    const label = button.textContent
    panel?.querySelectorAll(".rd-report-button").forEach(b => { b.disabled = true })
    button.textContent = "正在產生報告…"
    try {
      const blob = await apiFetch(path, { responseType: "blob" })
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = filename
      document.body.append(link)
      link.click()
      link.remove()
      window.setTimeout(() => URL.revokeObjectURL(url), 60000)
    } catch (error) {
      window.alert(`報告下載失敗：${error.message}`)
    } finally {
      s.exporting = false
      button.disabled = false
      button.textContent = label
      if (panel) {
        panel.querySelectorAll(".rd-report-button").forEach(b => { b.disabled = s.editing || b.dataset.empty === "true" })
      }
    }
  }

  async function openEvent(id) {
    const current = panel
    const request = ++requestID
    current.innerHTML = '<p role="status">正在載入場次觀測…</p>'
    try {
      const detail = await apiFetch(`/admin/reef-check-data/events/${id}`)
      if (request !== requestID || current !== panel) return
      s.detail = detail
      s.boardMode = "raw"
      s.transectID = detail.transects?.[0]?.id || null
      s.notice = ""
      renderDetail()
    } catch (error) {
      console.error("Failed to load event:", error)
      if (request !== requestID || current !== panel) return
      renderList()
      const notice = document.createElement("p")
      notice.setAttribute("role", "alert")
      notice.className = "rd-notice"
      notice.textContent = `無法載入場次觀測 (${error.message})`
      panel.prepend(notice)
    }
  }

  function currentTransect() {
    return s.detail?.transects?.find(t => t.id === s.transectID)
  }

  function renderDetail() {
    const e = s.detail?.event
    if (!e) {
      renderList()
      return
    }
    const t = currentTransect()
    if (t) {
      t.points = t.points || []
      t.bleaching = t.bleaching || []
      t.belt = t.belt || []
      t.impacts = t.impacts || []
      t.participants = t.participants || []
    }
    panel.innerHTML = `
      <div class="panel-header">
        <div>
          <button class="tiny-button" id="rd-back">← 返回場次列表</button>
          <h2>${esc(e.site_name)} · ${esc(e.survey_date)} · ${number(e.depth_m)} m</h2>
          <p class="rd-id">${esc(e.event_id)}</p>
        </div>
        <div class="rd-header-actions">
          <button type="button" class="tiny-button rd-report-button" id="rd-export-event" ${s.editing || s.exporting ? "disabled" : ""}>下載報告套組 ZIP</button>
          ${[["summary", "Summary"], ...(s.detail.transects.some(tr => tr.method === "line") ? [["line", "底質 Excel"]] : []), ...(s.detail.transects.some(tr => tr.method === "belt_fish" || tr.method === "belt_invert") ? [["belt", "魚類／無脊椎 Excel"]] : [])].map(([kind, label]) => `<button type="button" class="tiny-button rd-report-button" data-report-kind="${kind}" ${s.editing || s.exporting ? "disabled" : ""}>${label}</button>`).join("")}
          <button type="button" class="danger-button" id="rd-delete-event">刪除此場次</button>
        </div>
      </div>
      <dl class="rd-info">
        <div><dt>出動 Survey #${e.survey_id}</dt><dd>${esc(e.start_date)} ～ ${esc(e.end_date)}<small>${esc(e.label)}</small></dd></div>
        <div><dt>調查地點</dt><dd>${esc([e.region, e.county, e.location].filter(Boolean).join(" / "))}<small>${esc(e.site_english)}</small></dd></div>
        <div><dt>場次時間</dt><dd>${e.event_time === "na" ? "未記錄" : esc(e.event_time.replaceAll("-", ":"))}</dd></div>
        <div><dt>樣點座標</dt><dd>${e.latitude == null || e.longitude == null ? "未提供座標" : `${esc(e.latitude)}, ${esc(e.longitude)}`}</dd></div>
        <div><dt>氣象署測站</dt><dd>${e.cwa_station_name ? `${esc(e.cwa_station_name)} <small>(${esc(e.cwa_station_id)})</small>` : (e.cwa_station_id ? esc(e.cwa_station_id) : "未指定測站")}</dd></div>
      </dl>
      <div class="rd-tabs" role="group" aria-label="調查方法">
        ${Object.entries(methods).map(([method, label]) => {
          const row = s.detail.transects.find(tr => tr.method === method)
          return row ? `<button class="tiny-button ${row.id === s.transectID ? "rd-selected" : ""}" data-transect="${row.id}" aria-pressed="${row.id === s.transectID}">${label}</button>` : `<span class="rd-missing">${label} · 未記錄</span>`
        }).join("")}
      </div>
      <p id="rd-notice" role="status" class="rd-notice">${esc(s.notice)}</p>
      ${t ? `
        <form id="rd-form">
          <div class="rd-toolbar">
            <h3>${esc(methods[t.method])} #${t.id}</h3>
            <div>
              ${s.editing ? '<button type="button" class="tiny-button" id="rd-cancel">取消修改</button> <button class="primary-button" id="rd-save">儲存修改</button>' : '<button type="button" class="primary-button" id="rd-edit">編輯這條穿越線</button>'}
            </div>
          </div>
          ${metadata(t)}
          ${t.other_damage_note ? `<section class="rd-section"><h3>其他珊瑚損害說明</h3><p>${esc(t.other_damage_note)}</p></section>` : ''}
          ${participants(t)}
          ${t.method === "line" ? line(t) : belt(t)}
          ${t.method === "belt_invert" ? impacts(t) : ""}
        </form>
      ` : '<p class="empty-state">此場次沒有穿越線。</p>'}
    `
    panel.querySelector("#rd-back").onclick = () => {
      const current = panel
      if (!leave()) return
      panel = current
      s.detail = null
      renderList()
    }
    panel.querySelector("#rd-export-event")?.addEventListener("click", event => {
      downloadReport(`/admin/reef-check-data/events/${e.id}/report.zip`, `${e.site_name}-${e.survey_date}-${e.depth_m}m-報告套組.zip`, event.currentTarget)
    })
    panel.querySelectorAll("[data-report-kind]").forEach(button => {
      button.onclick = () => downloadReport(`/admin/reef-check-data/events/${e.id}/report.xlsx?kind=${button.dataset.reportKind}`, `${e.site_name}-${e.survey_date}-${e.depth_m}m-${button.dataset.reportKind}.xlsx`, button)
    })
    panel.querySelector("#rd-delete-event")?.addEventListener("click", deleteCurrentEvent)
    panel.querySelectorAll("[data-transect]").forEach(b => {
      b.onclick = () => {
        const current = panel
        if (!leave()) return
        panel = current
        s.transectID = Number(b.dataset.transect)
        renderDetail()
      }
    })
    panel.querySelector("#rd-edit")?.addEventListener("click", () => {
      s.editing = true
      s.notice = ""
      renderDetail()
    })
    panel.querySelector("#rd-cancel")?.addEventListener("click", () => {
      const current = panel
      if (!leave()) return
      panel = current
      renderDetail()
    })
    const surfaceBoard = panel.querySelector("#rd-surface-board")
    if (surfaceBoard) bindBoardTabs(surfaceBoard)
    panel.querySelector("#rd-form")?.addEventListener("submit", save)
    panel.querySelectorAll(".rd-remove-participant").forEach(btn => {
      btn.onclick = () => removeParticipant(t, Number(btn.dataset.participantId))
    })
    panel.querySelector("#rd-add-participant-btn")?.addEventListener("click", () => showAddParticipantModal(t))
    panel.querySelector("#rd-add-participant-inline")?.addEventListener("click", () => showAddParticipantModal(t))
  }

  function metadata(t) {
    const fields = [
      ["start_time", "開始時間", "text"],
      ["water_temp_c", "水溫 °C", "number"],
      ["visibility_min_m", "能見度下限 m", "number"],
      ["visibility_max_m", "能見度上限 m", "number"],
      ["comments", "調查備註", "textarea"],
      ["rkc_bleaching_note", "RKC／白化備註", "textarea"],
    ]
    return `
      <section class="rd-section">
        <h3>穿越線基本資料</h3>
        <div class="rd-metadata">
          ${fields.map(([key, label, type]) => `
            <label>${label}
              ${s.editing ? type === "textarea" ? `<textarea name="${key}" rows="3">${esc(t[key])}</textarea>` : `<input name="${key}" type="${type}" value="${esc(t[key])}" ${type === "number" ? `min="0" step="any" ${key === "water_temp_c" ? 'max="40"' : ""}` : ""}>` : `<span class="rd-value">${text(t[key])}</span>`}
            </label>
          `).join("")}
        </div>
      </section>
    `
  }

  function participants(t) {
    const tagList = t.participants.map(p => `
      <span class="rd-participant-tag">
        <strong>${esc(p.name_zh || p.name_en)}</strong>
        <span>· ${esc(roles[p.role] || p.role)}</span>
        ${p.reef_check_code ? `<small>(${esc(p.reef_check_code)})</small>` : ""}
        ${p.user_email ? `<span class="rd-user-badge" title="連結的使用者帳號: ${esc(p.user_email)}">👤 ${esc(p.user_email)}</span>` : ""}
        <button type="button" class="rd-remove-participant" data-participant-id="${p.id}" aria-label="移除 ${esc(p.name_zh || p.name_en)}">✕</button>
      </span>
    `).join("")

    return `
      <section class="rd-section">
        <div class="rd-section-head">
          <h3>參與人員 (${t.participants.length} 人)</h3>
          <button type="button" class="tiny-button" id="rd-add-participant-btn">＋ 指派人員</button>
        </div>
        ${t.participants.length ? `<div class="rd-tags">${tagList}</div>` : '<p class="meta-line">尚無已建立的參與人員關聯。<button type="button" class="tiny-button" id="rd-add-participant-inline">立即指派</button></p>'}
      </section>
    `
  }

  function numericInput(kind, row, value, label, max, integer = true) {
    const na=kind==="hc"?row.hc_record_status==="not_recorded":kind==="sc"?row.sc_record_status==="not_recorded":row.record_status==="not_recorded"
    if (!s.editing || row.is_aggregate) return na?"NA（未記錄）":number(value)
 return `<input class="rd-number" type="text" inputmode="decimal" data-kind="${kind}" data-id="${row.id}" aria-label="${esc(label)}" value="${na?"NA":esc(value)}" min="0" ${max != null ? `max="${max}"` : ""} step="${integer ? "1" : "any"}" required>`
  }

  function bindBoardTabs(board) {
    const tabs = [...panel.querySelectorAll("[data-board-mode]")]
    function select(tab) {
      s.boardMode = tab.dataset.boardMode
      const showCode = s.boardMode === "code"
      for (const item of tabs) {
        item.setAttribute("aria-selected", String(item === tab))
        item.tabIndex = item === tab ? 0 : -1
      }
      board.setAttribute("aria-labelledby", tab.id)
      board.querySelectorAll("[data-board-heading]").forEach(cell => {
        cell.textContent = showCode ? "代碼" : "數字"
      })
      board.querySelectorAll("[data-board-raw]").forEach(cell => {
        cell.textContent = showCode ? cell.dataset.boardCode : cell.dataset.boardRaw
      })
    }
    for (const tab of tabs) {
      tab.onclick = () => select(tab)
      tab.onkeydown = event => {
        let next
        switch (event.key) {
          case "ArrowRight":
          case "ArrowLeft": next = tabs.find(item => item !== tab); break
          case "Home": next = tabs[0]; break
          case "End": next = tabs[tabs.length - 1]; break
          default: return
        }
        event.preventDefault()
        select(next)
        next.focus()
      }
    }
  }

  function substrateBoard(points, layer, pointMarks) {
    const starts = [0, 25, 50, 75]
    const surface = layer === "surface"
    const showCode = surface && s.boardMode === "code"
    const layerName = surface ? "表層" : "下層"
    function displayAttrs(raw, code) {
      return surface ? `data-board-raw="${esc(raw)}" data-board-code="${esc(code)}"` : ""
    }
    const cells = new Map(points.map(p => [`${p.segment}:${p.position_m}`, p]))
    const codes = new Map(s.codes.map(c => [c.code, c]))
    const letterTokens = { "HC-a": "a", "HC-b": "b", "HC-c": "c", "SI(HC-a)": "9a", "SI(HC-b)": "9b", "SI(HC-c)": "9c" }

    function token(code) {
      if (code === "NA") return "NA"
      return letterTokens[code] ?? codes.get(code)?.numeric_code ?? code
    }

    function reading(point) {
      const code = point.substrate_code
      const label = `${layerName}第 ${point.segment} 段 ${point.position_m} m 底質`
      if (s.editing) {
        const available = s.codes.filter(c => c.is_active || c.code === code).map(c => c.code)
        if (!codes.has(code)) available.push(code)
        return `<select data-kind="point" data-id="${esc(point.id)}" aria-label="${esc(label)}">${available.map(c => {
          const mark = token(c)
          const rawLabel = mark === c ? c : mark + " " + c
          return `<option value="${esc(c)}" ${displayAttrs(rawLabel, c)} ${c === code ? "selected" : ""}>${esc(showCode ? c : rawLabel)}</option>`
        }).join("")}</select>`
      }
      const desc = [code, codes.get(code)?.name_zh].filter(Boolean).join(" · ")
      const raw = token(code)
      let health = ""
      if (pointMarks && surface && ["HC", "HC-a", "HC-b", "HC-c", "SC"].includes(code)) {
        const status = point.health_status ?? (point.is_bleached == null ? null : point.is_bleached ? "bleached" : "not_bleached")
        health = `<small>${status === "bleached" ? "已白化" : status === "not_bleached" ? "未白化" : "未標記"}</small>`
      }
      return `<span title="${esc(desc)}" aria-label="${esc(label + "：" + desc)}" ${displayAttrs(raw, code)}>${esc(showCode ? code : raw)}</span>${health}`
    }

    const rows = Array.from({ length: 20 }, (_, row) => {
      const pairs = starts.flatMap((start, i) => [start, start + 10].map(columnStart => {
        const position = columnStart + row * 0.5
        const key = `${i + 1}:${position}`
        const point = cells.get(key)
        cells.delete(key)
        return `<td class="rd-distance">${position}</td><td>${point ? reading(point) : '<span class="rd-missing" aria-label="無點位資料">—</span>'}</td>`
      }))
      return `<tr>${pairs.join("")}</tr>`
    })
    // Historical observations outside the printed positions must remain accessible for editing.
    const extra = [...cells.values()]
    const board = `<div class="rd-table-scroll" role="region" tabindex="0" aria-label="${layerName}底質手板">
      <table class="rd-board" aria-label="${layerName}逐點底質">
        ${starts.map(() => '<colgroup span="4"></colgroup>').join("")}
        <thead><tr>${starts.map((start, i) => `<th colspan="4" scope="colgroup">第 ${i + 1} 段<small>${start}–${start + 19.5} m</small></th>`).join("")}</tr>
        ${surface ? `<tr>${starts.map(start => `<th colspan="2">左欄 · ${start}–${start + 9.5} m</th><th colspan="2">右欄 · ${start + 10}–${start + 19.5} m</th>`).join("")}</tr>` : ""}
        <tr>${starts.map(() => `<th scope="col">m</th><th scope="col" ${surface ? "data-board-heading" : ""}>${surface ? (showCode ? "代碼" : "數字") : "底質"}</th>`.repeat(2)).join("")}</tr></thead>
        <tbody>${rows.join("")}</tbody>
      </table></div>
      ${extra.length ? table(["表外點位", "距離（m）", "底質"], extra.map(p => `<tr><td>第 ${esc(p.segment)} 段</td><td>${esc(p.position_m)}</td><td>${reading(p)}</td></tr>`).join("")) : ""}`
    if (!surface) return board
    return `<div class="rd-board-tabs" role="tablist" aria-label="表層底質顯示方式">
      ${[["raw", "原始數字"], ["code", "轉換代碼（HC／SC）"]].map(([mode, title]) =>
        `<button type="button" role="tab" id="rd-mode-${mode}" data-board-mode="${mode}" aria-selected="${s.boardMode === mode}" aria-controls="rd-surface-board" tabindex="${s.boardMode === mode ? 0 : -1}">${title}</button>`).join("")}
    </div><div id="rd-surface-board" role="tabpanel" aria-labelledby="rd-mode-${s.boardMode}">${board}</div>`
  }

  function line(t) {
    const layers = [...new Set(t.points.map(p => p.substrate_layer))]
    const html = layers.map(layer => {
      const selected=t.points.filter(p=>p.substrate_layer===layer)
      const summary={recorded:selected.length,valid:selected.filter(p=>p.substrate_code!=="NA").length,unknown:selected.filter(p=>p.substrate_code==="NA").length,rows:layer==="surface"?(t.summaries?.substrate||[]):[]}
      return `
        <section class="rd-section">
          <h3>逐點底質 · ${layer === "surface" ? "表層（含泥沙複合代碼）" : "下層"}</h3>
          <p class="meta-line">${summary.recorded} 個位置 · ${summary.valid} 個有效觀測 · ${summary.unknown} 個 NA（未記錄／未知）。NA 不列入覆蓋率分母；OT 為其他。</p>
          ${substrateBoard(selected, layer, t.point_marks)}
          <details class="rd-summary">
            <summary>底質統計（已儲存資料）</summary>
            ${summary.rows.length ? table(["底質", "合計點數", "覆蓋率", "平均段覆蓋率", "SD", "SE", "有效段數"], summary.rows.map(r => `<tr><th scope="row">${esc(r.code)}</th><td>${number(r.total)}</td><td>${percent(r.coverage_percent)}</td><td>${percent(r.mean)}</td><td>${number(r.sd)}</td><td>${number(r.se)}</td><td>${r.n}/4</td></tr>`).join("")) : '<p>此層不提供覆蓋率統計；正式統計僅使用表層 canonical 位置。</p>'}
            <p class="meta-line">以實際有效點數計算覆蓋率；SI(...) 維持獨立類別。SD 為有觀測子樣區的樣本標準差，未記錄段落不補零。</p>
          </details>
        </section>
      `
    }).join("")

    return `
      ${html || '<p>未記錄逐點底質。</p>'}
      ${bleachingSection(t)}
    `
  }

  function bleachingSection(t) {
    const pointMarks = t.point_marks
    const records = pointMarks ? t.derived_bleaching || [] : t.bleaching
    const bySegment = new Map(records.map(row => [row.segment, row]))
    function cells(row, kind) {
      const label = kind.toUpperCase()
      const count = pointMarks ? row[`derived_${kind}_bleached_point_count`]
        : row[`${kind}_record_status`] === "not_recorded" ? null : row[`${kind}_bleached_count`]
      const total = row[`${kind}_points`]
      const rate = row[`${kind}_percent`]
      const unknown = pointMarks ? (total == null ? "無有效底質點" : "白化標記未齊") : "未記錄白化點數"
      const displayCount = pointMarks ? (count == null ? unknown : number(count))
        : numericInput(kind, row, count, `S${row.segment} ${label} 白化點數`, 40)
      let displayRate
      if (count == null) displayRate = unknown
      else if (total == null) displayRate = "無有效底質點"
      else if (total === 0) displayRate = `無表層 ${label} 點`
      else displayRate = `${percent(rate)}（${number(count)}／${number(total)}）`
      return `<td>${displayCount}</td><td>${displayRate}</td>`
    }

    const rows = [1, 2, 3, 4].map(segment => {
      const row = bySegment.get(segment)
      return `<tr><th scope="row">S${segment}</th>${row ? cells(row, "hc") + cells(row, "sc") : '<td colspan="4">本段未記錄</td>'}</tr>`
    })
    const stats = t.summaries?.bleaching || []
    const summaryRows = stats.map(r => {
      const rate = r.bleaching_percent == null ? "無法計算"
        : `${percent(r.bleaching_percent)}（${number(r.rate_count)}／${number(r.rate_points)}）`
      return `<tr><th scope="row">${r.code === "HC" ? "硬珊瑚（HC）" : "軟珊瑚（SC）"}</th><td>${number(r.total)}</td><td>${rate}</td><td>${number(r.mean)}</td><td>${number(r.sd)}</td><td>${number(r.se)}</td><td>${number(r.n)}/4</td></tr>`
    }).join("")
    return `<section class="rd-section" aria-label="底質白化">
      <h3>底質白化</h3>
      <p class="meta-line">${pointMarks ? "依逐格白化標記自動統計。" : "依各段填報的白化總點數計算。"}白化率為該段白化點數除以同類表層珊瑚點數；硬珊瑚（HC）含 a、b、c 分型。</p>
      ${table(["段落", "HC 白化點數", "HC 白化率", "SC 白化點數", "SC 白化率"], rows.join(""))}
      <section aria-label="穿越線白化統計">
        <h4>穿越線白化統計</h4>
        ${summaryRows ? table(["珊瑚類別", "合計白化點數", "整線白化率", "平均每段白化點數", "SD（點）", "SE（點）", "已記錄段數"], summaryRows)
          : '<p class="meta-line">本條穿越線沒有已記錄的白化點數。</p>'}
        ${summaryRows ? '<p class="meta-line">整線白化率使用有白化紀錄且有同類表層珊瑚的段落，將白化點數加總後除以珊瑚點數。SD 與 SE 至少需要兩段點數紀錄。</p>' : ""}
      </section>
    </section>`
  }

  function percent(v) {
    return v == null ? "無法計算" : `${number(v)}%`
  }

  function belt(t) {
    return Object.entries(groups).filter(([group]) => t.belt.some(r => r.taxon_group === group)).map(([group, label]) => {
      const taxa = [...new Map(t.belt.filter(r => r.taxon_group === group).map(r => [r.taxon_id, r])).values()]
      return `
        <section class="rd-section">
          <h3>${label}</h3>
          ${table(["物種／體長", ...segments, "合計", "平均", "SD", "SE", "有效段數"], taxa.map(taxon => {
            const rows = [1, 2, 3, 4].map(segment => t.belt.find(r => r.taxon_id === taxon.taxon_id && r.segment === segment))
            const stats = (t.summaries?.belt||[]).find(r=>r.taxon_id===taxon.taxon_id)||{n:0}
            return `<tr><th scope="row">${esc(taxon.name_zh)}<small>${esc(taxon.name_en)} ${esc(taxon.size_class)}</small>${taxon.observed_size ? `<small>尺寸：${esc(taxon.observed_size)}</small>` : ""}${taxon.notes ? `<small>備註：${esc(taxon.notes)}</small>` : ""}${taxon.is_aggregate ? '<small>總數列（唯讀）</small>' : ""}</th>${rows.map((r, i) => `<td>${r ? numericInput("belt", r, r.count, `${taxon.name_en} ${taxon.size_class} S${i + 1}`, 2147483647) : '<span class="rd-missing">未記錄</span>'}</td>`).join("")}<td>${number(stats.total)}</td><td>${number(stats.mean)}</td><td>${number(stats.sd)}</td><td>${number(stats.se)}</td><td>${stats.n}/4</td></tr>`
          }).join(""))}
          <p class="meta-line">合計、平均與 SD 依已儲存的原始值計算。未記錄與 0 分開呈現。</p>
        </section>
      `
    }).join("") || '<p class="empty-state">此穿越線沒有生物觀測。</p>'
  }

  function impacts(t) {
    const types = [...new Map(t.impacts.map(r => [r.impact_type_id, r])).values()]
    return `
      <section class="rd-section">
        <h3>環境影響</h3>
        <p class="meta-line">件數保留原始值；0 件＝0 級、1 件＝1 級、2–4 件＝2 級、5 件以上＝3 級；平均與誤差使用原始單位。百分比使用 0–100。</p>
        ${types.length ? table(["項目／單位", ...segments, "平均", "SD", "SE", "有效段數"], types.map(type => {
          const rows = [1, 2, 3, 4].map(segment => t.impacts.find(r => r.impact_type_id === type.impact_type_id && r.segment === segment))
          const stats = (t.summaries?.impact||[]).find(r=>r.impact_type_id===type.impact_type_id)||{n:0}
          const isPercent = type.value_type === "percent"
          const unit = isPercent ? "%" : type.has_raw_count ? "原始件數 → 衍生等級" : type.value_type === "level" ? "等級 0–3" : "件數"
          return `<tr><th scope="row">${esc(type.name_zh)}<small>${esc(type.name_en)}</small><small>${unit}</small></th>${rows.map((r, i) => `<td>${r ? `${numericInput("impact", r, r.raw_value, `${type.name_en} S${i + 1}`, isPercent ? 100 : !type.has_raw_count && type.value_type === "level" ? 3 : null, !isPercent)}${type.has_raw_count ? `<small>等級 ${number(r.derived_level)}</small>` : isPercent ? " %" : ""}` : "未記錄"}</td>`).join("")}<td>${number(stats.mean)}${isPercent ? "%" : ""}</td><td>${number(stats.sd)}</td><td>${number(stats.se)}</td><td>${stats.n}/4</td></tr>`
        }).join("")) : '<p>未記錄環境影響。</p>'}
        <p class="meta-line">平均與 SD 為已儲存資料的衍生結果，儲存後更新。</p>
      </section>
    `
  }

  async function save(event) {
    event.preventDefault()
    if (s.saving) return
    const t = currentTransect()
    const form = event.currentTarget
    const f = new FormData(form)
    const metadata = Object.fromEntries(["start_time", "water_temp_c", "visibility_min_m", "visibility_max_m", "comments", "rkc_bleaching_note"].map(k => {
      const v = String(f.get(k) ?? "")
      return [k, v === "" ? null : ["water_temp_c", "visibility_min_m", "visibility_max_m"].includes(k) ? Number(v) : v]
    }))
    const changes = []
    const bleaching = new Map()

    form.querySelectorAll("[data-kind]").forEach(input => {
      const id = Number(input.dataset.id)
      const kind = input.dataset.kind
      if (kind === "point") {
        if (input.value !== t.points.find(r => r.id === id).substrate_code) changes.push({ kind, id, code: input.value })
      } else if (kind === "hc" || kind === "sc") {
        const row = t.bleaching.find(r => r.id === id)
        const change = bleaching.get(id) || { kind: "bleaching", id, hc:row.hc_record_status==="not_recorded"?null:row.hc_bleached_count,sc:row.sc_record_status==="not_recorded"?null:row.sc_bleached_count,hc_record_status:row.hc_record_status||"recorded",sc_record_status:row.sc_record_status||"recorded" }
        change[kind] = input.value.trim().toUpperCase()==="NA"?null:Number(input.value)
 change[kind+"_record_status"]=change[kind]===null?"not_recorded":"recorded"
        bleaching.set(id, change)
      } else {
        const row = (kind === "belt" ? t.belt : t.impacts).find(r => r.id === id)
        const value=input.value.trim().toUpperCase()==="NA"?null:Number(input.value),status=value===null?"not_recorded":"recorded"
        if(status!==(row.record_status||"recorded") || value!== (row.record_status==="not_recorded"?null:kind==="belt"?row.count:row.raw_value)) changes.push({kind,id,value,record_status:status})
      }
    })

    for (const c of bleaching.values()) {
      const r = t.bleaching.find(row => row.id === c.id)
      if (c.hc !== (r.hc_record_status==="not_recorded"?null:r.hc_bleached_count) || c.sc !== (r.sc_record_status==="not_recorded"?null:r.sc_bleached_count)) changes.push(c)
    }

    const original = Object.fromEntries(Object.keys(metadata).map(k => [k, t[k]]))
    const metadataChanged = JSON.stringify(original) !== JSON.stringify(metadata)
    if (!metadataChanged && !changes.length) {
      s.editing = false
      s.notice = "沒有變更。"
      renderDetail()
      return
    }

    s.saving = true
    panel.querySelectorAll("button, input, select, textarea").forEach(el => { el.disabled = true })
    panel.querySelector("#rd-notice").textContent = "正在儲存…"
    try {
      const updated = await apiFetch(`/admin/reef-check-data/transects/${t.id}`, {
        method: "PATCH",
        body: { version: t.version, ...(metadataChanged ? { metadata } : {}), changes },
      })
      s.detail.transects = s.detail.transects.map(row => row.id === updated.id ? updated : row)
      s.editing = false
      s.notice = "修改已儲存，統計已更新。"
      renderDetail()
    } catch (error) {
      panel.querySelectorAll("button, input, select, textarea").forEach(el => { el.disabled = false })
      const notice = panel.querySelector("#rd-notice")
      notice.setAttribute("role", "alert")
      notice.textContent = `${error.message} 目前輸入仍保留；可取消修改並返回列表重新載入。`
    } finally {
      s.saving = false
    }
  }

  function showDialog({ title, bodyHTML, onReady, onSubmit, submitText = "確認" }) {
    const overlay = document.createElement("div")
    overlay.className = "rd-dialog-overlay"
    overlay.innerHTML = `
      <div class="rd-dialog" role="dialog" aria-modal="true">
        <div class="rd-dialog-head">
          <h3>${esc(title)}</h3>
          <button type="button" class="drawer-close rd-dialog-close" aria-label="關閉">✕</button>
        </div>
        <form id="rd-dialog-form">
          <div class="rd-dialog-body">${bodyHTML}</div>
          <p id="rd-dialog-error" role="alert" class="rd-notice" style="padding: 0 1.5rem; color: #a32d21; margin: 0;"></p>
          <div class="rd-dialog-foot">
            <button type="button" class="tiny-button rd-dialog-close">取消</button>
            <button type="submit" class="primary-button" id="rd-dialog-submit-btn">${esc(submitText)}</button>
          </div>
        </form>
      </div>
    `
    document.body.appendChild(overlay)

    if (typeof onReady === "function") {
      onReady(overlay)
    }

    function close() {
      overlay.remove()
    }

    overlay.querySelectorAll(".rd-dialog-close").forEach(b => {
      b.onclick = close
    })
    overlay.onclick = (e) => {
      if (e.target === overlay) close()
    }

    const form = overlay.querySelector("#rd-dialog-form")
    const submitBtn = overlay.querySelector("#rd-dialog-submit-btn")
    const errP = overlay.querySelector("#rd-dialog-error")

    form.onsubmit = async (e) => {
      e.preventDefault()
      errP.textContent = ""
      submitBtn.disabled = true
      try {
        await onSubmit(new FormData(form), close)
      } catch (err) {
        errP.textContent = err.message || "操作失敗"
        submitBtn.disabled = false
      }
    }
  }

  async function showCreateEventModal() {
    let sites = []
    let stations = []
    let activities=[]
    try {
      [sites, stations,activities] = await Promise.all([
        getSites(),
        getCWAStations(),
        apiFetch("/admin/public-content/activity"),
      ])
    } catch (err) {
      alert("無法載入樣點清單: " + err.message)
      return
    }

    const sortedSites = [...sites].sort((a, b) => {
      const regA = [a.region, a.county].filter(Boolean).join(" ")
      const regB = [b.region, b.county].filter(Boolean).join(" ")
      const cmp = regA.localeCompare(regB, "zh-TW")
      return cmp !== 0 ? cmp : (a.name_zh || "").localeCompare(b.name_zh || "", "zh-TW")
    })

    const siteOptions = sortedSites.map(site => {
      const loc = [site.region, site.county].filter(Boolean).join(" · ")
      const stationTag = site.cwa_station_name ? ` · 測站: ${site.cwa_station_name}` : ""
      const label = `${site.name_zh} (${site.name_en || ""}) ${loc ? `[${loc}]` : ""}${stationTag}`
      return `<option value="${site.id}">${esc(label)}</option>`
    }).join("")

    const today = new Date().toISOString().slice(0, 10)
    const bodyHTML = `
      <label>所屬活動（選填）<select name="activity_id"><option value="">獨立調查／歷史資料</option>${activities.filter(a=>a.data.entry_enabled).map(a=>`<option value="${a.id}">${esc(a.data.title)}</option>`).join("")}</select></label>
      <label>
        樣點 (Site) *
        <select name="site_id" id="rd-create-site" required>
          <option value="">請選擇樣點</option>
          ${siteOptions}
        </select>
      </label>
      <label>
        調查日期 (Survey Date) *
        <input type="date" name="survey_date" id="rd-create-date" value="${today}" required>
      </label>
      <label>
        魚類體長模式<select name="fish_size_mode"><option value="split">分體長</option><option value="combined">不分體長</option></select>
      </label><label>
        場次時間 (Event Time)
        <input type="text" name="event_time" id="rd-create-time" value="09:00" placeholder="例如 09:00">
        <small class="meta-line">請填此支氣瓶的開始時間</small>
      </label>
      <label>
        氣象署海象測站 (CWA Station)
        <input type="text" id="rd-create-station-display" value="請先選擇樣點" readonly aria-describedby="rd-cwa-station-hint">
        <input type="hidden" name="cwa_station_id" id="rd-create-station">
        <input type="hidden" name="sea_temp_source" id="rd-temperature-source" value="manual">
        <button type="button" id="rd-station-override">變更本次測站</button>
        <select id="rd-station-override-select" aria-label="本次觀測測站" hidden><option value="">請選擇</option></select>
        <small class="meta-line" id="rd-cwa-station-hint" role="status">${esc(s.cwaStationsError || (stations.length ? "依選擇的樣點自動帶入對應測站。" : "目前沒有可用測站，請先同步氣象署測站資料；水溫可手動填寫。"))}</small>
      </label>
      <label>
        當日水溫 (°C)
        <div style="display:flex;gap:0.5rem;align-items:center;">
          <input type="number" name="water_temp_c" id="rd-create-water-temp" step="0.1" min="0" max="45" placeholder="例如 26.5" style="flex:1;">
          <button type="button" class="tiny-button" id="rd-fetch-temp-btn" style="white-space:nowrap;" disabled>查詢海溫</button>
        </div>
        <small class="meta-line" id="rd-cwa-temp-hint">選擇有對應測站的樣點與日期後，系統將自動查詢海溫；也可手動填寫水溫。</small>
      </label>
      <label>
        水深 (Depth m) *
        <input type="number" name="depth_m" step="0.1" min="0.1" max="100" value="5.0" required>
      </label>
      <label>
        活動說明／標籤 (Label)
        <input type="text" name="label" placeholder="例如 2026 秋季調查">
      </label>
      <div>
        <span style="font-size:0.85rem;font-weight:500;color:var(--text,#27473e);">調查穿越線方法 *</span>
        <div class="rd-checkbox-group">
          <label><input type="checkbox" name="methods" value="line" checked> 底質 (Line)</label>
          <label><input type="checkbox" name="methods" value="belt_fish" checked> 魚類 (Belt)</label>
          <label><input type="checkbox" name="methods" value="belt_invert" checked> 無脊椎・罕見生物・環境影響</label>
        </div>
      </div>
    `

    showDialog({
      title: "新增 Reef Check 調查場次",
      bodyHTML,
      submitText: "建立場次",
      onReady: (overlay) => {
        const siteSel = overlay.querySelector("#rd-create-site")
        const dateInput = overlay.querySelector("#rd-create-date")
        const timeInput = overlay.querySelector("#rd-create-time")
        const stationInput = overlay.querySelector("#rd-create-station")
        const stationDisplay = overlay.querySelector("#rd-create-station-display")
        const tempInput = overlay.querySelector("#rd-create-water-temp")
        const fetchBtn = overlay.querySelector("#rd-fetch-temp-btn")
        const hintP = overlay.querySelector("#rd-cwa-temp-hint")
        const stationHint = overlay.querySelector("#rd-cwa-station-hint")

        let fetchSeq = 0
        let autoFilledTemperature = null
        async function fetchSeaTemp() {
          const currentSeq = ++fetchSeq
          if (autoFilledTemperature !== null && tempInput.value === autoFilledTemperature) {
            tempInput.value = ""
          }
          autoFilledTemperature = null
          const stationId = stationInput.value.trim()
          const dateVal = dateInput?.value?.trim()
          const timeVal = timeInput?.value?.trim() || "09:00"

          if (!stationId || !dateVal) {
            if (fetchBtn) fetchBtn.disabled = true
            if (hintP) {
              hintP.textContent = "需有對應測站及調查日期才能查詢海溫；也可手動填寫水溫。"
              hintP.style.color = "#c05621"
            }
            return
          }

          const initialTemperature = tempInput.value
          if (hintP) {
            hintP.textContent = "正在向氣象署檢索海溫..."
            hintP.style.color = "var(--primary,#00705a)"
          }
          if (fetchBtn) fetchBtn.disabled = true

          try {
            const query = new URLSearchParams({
              station_id: stationId,
              date: dateVal,
              time: timeVal,
            })
            const data = await apiFetch(`/admin/cwa-marine/sea-temp?${query.toString()}`)
            if (currentSeq !== fetchSeq || !overlay.isConnected) return

            if (data && typeof data.sea_temp_c === "number") {
              // Preserve a manual value, including edits made while the request was pending.
              if (initialTemperature !== "" || tempInput.value !== initialTemperature) {
                hintP.textContent = `氣象署海溫 ${data.sea_temp_c}°C；已保留手動填寫的水溫。`
                return
              }
              overlay.querySelector("#rd-temperature-source").value="automatic"
              autoFilledTemperature = data.sea_temp_c.toFixed(1)
              tempInput.value = autoFilledTemperature
              if (hintP) {
                const obsTime = data.observed_at ? data.observed_at.slice(0, 16).replace("T", " ") : ""
                hintP.textContent = `✓ 已自動帶入氣象署海溫 ${data.sea_temp_c}°C${obsTime ? `（觀測時間：${obsTime}）` : ""}`
                hintP.style.color = "#2f855a"
              }
            } else {
              if (hintP) {
                hintP.textContent = "氣象署目前無該日海溫記錄（可手動輸入）"
                hintP.style.color = "#718096"
              }
            }
          } catch (err) {
            if (currentSeq !== fetchSeq || !overlay.isConnected) return
            if (hintP) {
              hintP.textContent = `查詢海溫提示: ${err.message || "未取得海溫紀錄，可手動填寫"}`
              hintP.style.color = "#718096"
            }
          } finally {
            if (fetchBtn && currentSeq === fetchSeq) fetchBtn.disabled = false
          }
        }

        tempInput.addEventListener("input",()=>{overlay.querySelector("#rd-temperature-source").value="manual"})
        const override=overlay.querySelector("#rd-station-override-select")
        overlay.querySelector("#rd-station-override").onclick=()=>{
          override.innerHTML=`<option value="">請選擇</option>${stations.filter(st=>st.is_active!==false).map(st=>`<option value="${esc(st.id)}">${esc(st.name_zh)} (${esc(st.id)})</option>`).join("")}`
          override.hidden=false
        }
        override.onchange=()=>{
          const st=stations.find(x=>x.id===override.value);if(!st)return
          fetchSeq++;if(tempInput.value===autoFilledTemperature)tempInput.value="";autoFilledTemperature=""
          stationInput.value=st.id;stationDisplay.value=`${st.name_zh} (${st.id})`
          stationHint.textContent="僅變更本次觀測測站，不修改樣點預設。";fetchSeaTemp()
        }
        // Defaults still show only the mapped station; override is explicit.
        siteSel?.addEventListener("change", () => {
          const siteId = Number(siteSel.value)
          const site = sortedSites.find(s => s.id === siteId)
          const station = stations.find(st => st.id === site?.cwa_station_id)
          stationInput.value = station?.id || ""
          if (station) {
            stationDisplay.value = `${station.name_zh} (${station.id})`
            stationHint.textContent = "已自動帶入此樣點的對應測站。"
          } else if (!site) {
            stationDisplay.value = "請先選擇樣點"
            stationHint.textContent = "依選擇的樣點自動帶入對應測站。"
          } else {
            stationDisplay.value = site.cwa_station_id ? "對應測站目前不可用" : "未設定對應測站"
            stationHint.textContent = !site.cwa_station_id
              ? "此樣點尚未設定對應測站，水溫請手動填寫；可至樣點管理設定對應測站。"
              : s.cwaStationsError || "此樣點的對應測站目前不可用，水溫請手動填寫；請確認測站設定或同步測站資料。"
          }
          fetchSeaTemp()
        })

        // Date and time changes query the same site-linked station.
        dateInput?.addEventListener("change", () => fetchSeaTemp())
        timeInput?.addEventListener("change", () => fetchSeaTemp())
        fetchBtn?.addEventListener("click", () => fetchSeaTemp())
      },
      onSubmit: async (f, close) => {
        const siteID = Number(f.get("site_id"))
        if (!siteID) throw new Error("請選擇樣點")
        const surveyDate = String(f.get("survey_date") || "").trim()
        if (!surveyDate) throw new Error("請填寫調查日期")
        let eventTime = String(f.get("event_time") || "").trim()
        if (!eventTime) eventTime = "na"
        const depthM = Number(f.get("depth_m"))
        if (!depthM || depthM <= 0 || depthM > 100) throw new Error("水深必須介於 0–100 公尺")
        const label = String(f.get("label") || "").trim()
        const selectedMethods = f.getAll("methods").map(String)
        if (!selectedMethods.length) throw new Error("請至少勾選一種調查方法")

        const cwaStationID = String(f.get("cwa_station_id") || "").trim() || null
        const waterTempRaw = String(f.get("water_temp_c") || "").trim()
        const waterTempC = waterTempRaw !== "" ? Number(waterTempRaw) : null
        if (waterTempC !== null && (isNaN(waterTempC) || waterTempC < -5 || waterTempC > 50)) {
          throw new Error("水溫數值無效")
        }

        const body = {
          activity_id:f.get("activity_id")?Number(f.get("activity_id")):null,
          site_id: siteID,
          survey_date: surveyDate,
          event_time: eventTime,
          depth_m: depthM,
          label,
          methods: selectedMethods,
 fish_size_mode:String(f.get("fish_size_mode")),
          cwa_station_id: cwaStationID,
          sea_temp_source:String(f.get("sea_temp_source")||"manual"),
        }
        if (waterTempC !== null) {
          body.water_temp_c = waterTempC
        }

        const created = await apiFetch("/admin/reef-check-data/events", {
          method: "POST",
          body,
        })
        close()
        s.events.unshift(created.event)
        onCount(s.events.length)
        await openEvent(created.event.id)
      },
    })
  }

  async function deleteCurrentEvent() {
    const e = s.detail?.event
    if (!e) return
    if (!window.confirm(`確定要刪除場次「${e.site_name} (${e.event_id})」嗎？\n\n此動作將連同刪除該場次下的所有穿越線、底質點位與生物觀測資料，且無法復原。`)) {
      return
    }
    panel.querySelectorAll("button").forEach(b => { b.disabled = true })
    try {
      await apiFetch(`/admin/reef-check-data/events/${e.id}`, { method: "DELETE" })
      s.events = s.events.filter(item => item.id !== e.id)
      onCount(s.events.length)
      s.detail = null
      s.notice = `場次 ${e.event_id} 已成功刪除。`
      renderList()
    } catch (err) {
      panel.querySelectorAll("button").forEach(b => { b.disabled = false })
      alert("刪除失敗: " + err.message)
    }
  }

  async function showAddParticipantModal(t) {
    let divers = []
    try {
      if (!s.divers || !s.divers.length) {
        s.divers = await apiFetch("/admin/reef-check-data/divers") || []
      }
      divers = s.divers
    } catch (err) {
      alert("無法載入名冊: " + err.message)
      return
    }

    const diverOptions = divers.map(d => `<option value="${d.id}">${esc(d.name_zh || d.name_en)}${d.reef_check_code ? ` · ${esc(d.reef_check_code)}` : ""}</option>`).join("")

    const bodyHTML = `
      <label>
        角色 (Role) *
        <select name="role" required>
          <option value="member">成員 (member)</option>
          <option value="team_leader">隊長 (team_leader)</option>
          <option value="team_scientist">科學指導員 (team_scientist)</option>
        </select>
      </label>
      <div>
        <span style="font-size:0.85rem;font-weight:500;color:var(--text,#27473e);">指派方式 *</span>
        <div class="rd-tabs-group" style="margin-top:0.3rem;">
          <button type="button" class="rd-tab-btn active" data-mode="diver">既有潛水員名冊</button>
          <button type="button" class="rd-tab-btn" data-mode="new">快速建立新潛水員</button>
        </div>
      </div>
      <div id="rd-mode-diver" class="rd-mode-section">
        <label>
          選擇潛水員 *
          <select name="diver_id">
            <option value="">請選擇潛水員</option>
            ${diverOptions}
          </select>
        </label>
      </div>
      <div id="rd-mode-new" class="rd-mode-section" style="display:none;">
        <label>
          中文姓名 *
          <input type="text" name="name_zh" placeholder="中文姓名">
        </label>
        <label>
          英文姓名 (選填)
          <input type="text" name="name_en" placeholder="英文姓名">
        </label>
        <label>
          Reef Check 識別代碼 (選填)
          <input type="text" name="reef_check_code" placeholder="例如：TW-001">
        </label>
      </div>
    `

    let activeMode = "diver"
    showDialog({
      title: `指派參與人員 — ${methods[t.method]}`,
      bodyHTML,
      submitText: "確認指派",
      onSubmit: async (f, close) => {
        const role = String(f.get("role") || "member")
        const body = { role }
        if (activeMode === "diver") {
          const did = Number(f.get("diver_id"))
          if (!did) throw new Error("請選擇潛水員")
          body.diver_id = did
        } else {
          const nameZh = String(f.get("name_zh") || "").trim()
          const nameEn = String(f.get("name_en") || "").trim()
          const code = String(f.get("reef_check_code") || "").trim()
          if (!nameZh && !nameEn) throw new Error("請輸入中文或英文姓名")
          body.name_zh = nameZh
          body.name_en = nameEn
          body.reef_check_code = code
        }

        const updatedTransect = await apiFetch(`/admin/reef-check-data/transects/${t.id}/participants`, {
          method: "POST",
          body,
        })
        close()
        s.detail.transects = s.detail.transects.map(row => row.id === updatedTransect.id ? updatedTransect : row)
        s.divers = []
        s.notice = "參與人員已成功指派。"
        renderDetail()
      },
    })

    const dialog = document.querySelector(".rd-dialog")
    if (dialog) {
      const tabBtns = dialog.querySelectorAll(".rd-tab-btn")
      const sections = {
        diver: dialog.querySelector("#rd-mode-diver"),
        new: dialog.querySelector("#rd-mode-new"),
      }
      tabBtns.forEach(btn => {
        btn.onclick = () => {
          activeMode = btn.dataset.mode
          tabBtns.forEach(b => b.classList.toggle("active", b === btn))
          Object.entries(sections).forEach(([k, sec]) => {
            if (sec) sec.style.display = k === activeMode ? "block" : "none"
          })
        }
      })
    }
  }

  async function removeParticipant(t, participantID) {
    const p = t.participants.find(item => item.id === participantID)
    const pName = p ? (p.name_zh || p.name_en) : `#${participantID}`
    if (!window.confirm(`確定要從此穿越線移除「${pName}」？`)) return
    try {
      await apiFetch(`/admin/reef-check-data/participants/${participantID}`, { method: "DELETE" })
      t.participants = t.participants.filter(item => item.id !== participantID)
      s.notice = `已移除參與人員 ${pName}。`
      renderDetail()
    } catch (err) {
      alert("移除失敗: " + err.message)
    }
  }

  window.addEventListener("beforeunload", e => {
    if (s.editing || s.saving) {
      e.preventDefault()
      e.returnValue = ""
    }
  })

  return {
    load,
    render,
    leave,
    reset() {
      requestID++
      s.events = []
      s.codes = []
      s.sites = []
      s.users = []
      s.divers = []
      s.cwaStations = []
      s.cwaStationsError = ""
      s.detail = null
      s.loaded = false
      s.editing = false
      panel = null
    },
    get count() {
      return s.loaded ? s.events.length : null
    },
  }
}
