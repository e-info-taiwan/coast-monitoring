import { statistics, impactValue, substrateSummary, bleachingPercent } from "./reef-data-math.mjs"

const methods = { line: "底質 Line", belt_fish: "魚類 Belt", belt_invert: "無脊椎・罕見生物・環境影響" }
const roles = { member: "成員", team_leader: "隊長", team_scientist: "科學指導員" }
const groups = { fish: "魚類", invert: "無脊椎", rare: "罕見生物" }
const segments = ["0–20m", "25–45m", "50–70m", "75–95m"]
const esc = v => String(v ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;")
const text = v => v === null || v === undefined || v === "" ? "未記錄" : esc(v)
const number = v => v === null || v === undefined ? "—" : Number(v).toLocaleString("zh-TW", { maximumFractionDigits: 2 })
const table = (headers, rows) => `<div class="rd-table-scroll"><table class="rd-table"><thead><tr>${headers.map(h => `<th scope="col">${h}</th>`).join("")}</tr></thead><tbody>${rows}</tbody></table></div>`
const options = (values, selected, label) => `<option value="">${label}</option>${values.map(([value, title]) => `<option value="${esc(value)}" ${String(value) === selected ? "selected" : ""}>${esc(title)}</option>`).join("")}`

export function createReefData({ apiFetch, onCount }) {
  const s = {
    events: [],
    codes: [],
    sites: [],
    users: [],
    divers: [],
    error: "",
    loading: false,
    loaded: false,
    detail: null,
    transectID: null,
    year: "",
    site: "",
    method: "",
    search: "",
    page: 0,
    editing: false,
    saving: false,
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
    const query = s.search.trim().toLowerCase()
    const filtered = s.events.filter(e => (!s.year || e.survey_date.startsWith(s.year)) && (!s.site || String(e.site_id) === s.site) && (!s.method || e.methods.includes(s.method)) && (!query || [e.event_id, e.site_name, e.site_english, e.county, e.region, e.label].join(" ").toLowerCase().includes(query)))
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
        <label>年份<select name="year">${options(years, s.year, "全部年份")}</select></label>
        <label>樣點<select name="site">${options(sites, s.site, "全部樣點")}</select></label>
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
      for (const key of ["year", "site", "method", "search"]) s[key] = String(f.get(key) || "")
      s.page = 0
      renderList()
    }
    panel.querySelector("#rd-clear").onclick = () => {
      s.year = s.site = s.method = s.search = ""
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
    panel.querySelector("#rd-create-event")?.addEventListener("click", showCreateEventModal)
    panel.querySelectorAll("[data-event]").forEach(b => { b.onclick = () => openEvent(b.dataset.event) })
  }

  async function openEvent(id) {
    const current = panel
    const request = ++requestID
    current.innerHTML = '<p role="status">正在載入場次觀測…</p>'
    try {
      const detail = await apiFetch(`/admin/reef-check-data/events/${id}`)
      if (request !== requestID || current !== panel) return
      s.detail = detail
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
          <button type="button" class="danger-button" id="rd-delete-event">刪除此場次</button>
        </div>
      </div>
      <dl class="rd-info">
        <div><dt>出動 Survey #${e.survey_id}</dt><dd>${esc(e.start_date)} ～ ${esc(e.end_date)}<small>${esc(e.label)}</small></dd></div>
        <div><dt>調查地點</dt><dd>${esc([e.region, e.county, e.location].filter(Boolean).join(" / "))}<small>${esc(e.site_english)}</small></dd></div>
        <div><dt>場次時間</dt><dd>${e.event_time === "na" ? "未記錄" : esc(e.event_time.replaceAll("-", ":"))}</dd></div>
        <div><dt>樣點座標</dt><dd>${e.latitude == null || e.longitude == null ? "未提供座標" : `${esc(e.latitude)}, ${esc(e.longitude)}`}</dd></div>
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
    if (!s.editing || row.is_aggregate) return number(value)
    return `<input class="rd-number" type="number" data-kind="${kind}" data-id="${row.id}" aria-label="${esc(label)}" value="${esc(value)}" min="0" ${max != null ? `max="${max}"` : ""} step="${integer ? "1" : "any"}" required>`
  }

  function line(t) {
    const layers = [...new Set(t.points.map(p => p.substrate_layer))]
    const html = layers.map(layer => {
      const summary = substrateSummary(t.points, layer)
      const points = t.points.filter(p => p.substrate_layer === layer)
      return `
        <section class="rd-section">
          <h3>逐點底質 · ${layer === "surface" ? "表層（含泥沙複合代碼）" : "下層"}</h3>
          <p class="meta-line">${summary.recorded} 個位置 · ${summary.valid} 個有效觀測 · ${summary.unknown} 個 NA（未記錄／未知）。NA 不列入覆蓋率分母；OT 為其他。</p>
          <div class="rd-line-grid">
            ${[1, 2, 3, 4].map(segment => `
              <details ${segment === 1 ? "open" : ""}>
                <summary>第 ${segment} 段 · ${(segment - 1) * 25}–${(segment - 1) * 25 + 19.5}m</summary>
                <div class="rd-points">
                  ${points.filter(p => p.segment === segment).map(p => `
                    <label>${p.position_m}m
                      ${s.editing ? `<select data-kind="point" data-id="${p.id}" aria-label="${esc(layer)} ${p.position_m}m 底質">${s.codes.filter(c => c.is_active || c.code === p.substrate_code).map(c => `<option value="${esc(c.code)}" ${c.code === p.substrate_code ? "selected" : ""}>${c.numeric_code} ${esc(c.code)}</option>`).join("")}</select>` : `<strong class="${p.substrate_code === "NA" ? "rd-unknown" : ""}">${esc(p.substrate_code)}</strong>`}
                    </label>
                  `).join("") || '<p>未記錄</p>'}
                </div>
              </details>
            `).join("")}
          </div>
          <details class="rd-summary">
            <summary>底質統計（已儲存資料）</summary>
            ${summary.valid ? table(["底質", "S1", "S2", "S3", "S4", "合計", "覆蓋率", "平均點數", "SD"], summary.rows.map(r => `<tr><th scope="row">${esc(r.code)}</th>${r.counts.map(v => `<td>${number(v)}</td>`).join("")}<td>${number(r.total)}</td><td>${number(r.cover)}%</td><td>${number(r.mean)}</td><td>${number(r.sd)}</td></tr>`).join("")) : '<p>沒有有效底質觀測，無法計算覆蓋率。</p>'}
            <p class="meta-line">以實際有效點數計算覆蓋率；SI(...) 維持獨立類別。SD 為有觀測子樣區的樣本標準差，未記錄段落不補零。</p>
          </details>
        </section>
      `
    }).join("")

    return `
      ${html || '<p>未記錄逐點底質。</p>'}
      <section class="rd-section">
        <h3>底質白化</h3>
        ${t.bleaching.length ? table(["段落", "HC 白化點數", "HC 白化率", "SC 白化點數", "SC 白化率"], t.bleaching.map(r => `<tr><th scope="row">S${r.segment}</th><td>${numericInput("hc", r, r.hc_bleached_count, `S${r.segment} HC 白化點數`, 40)}</td><td>${percent(bleachingPercent(t.points, r.segment, "HC", r.hc_bleached_count))}</td><td>${numericInput("sc", r, r.sc_bleached_count, `S${r.segment} SC 白化點數`, 40)}</td><td>${percent(bleachingPercent(t.points, r.segment, "SC", r.sc_bleached_count))}</td></tr>`).join("")) : '<p>未記錄白化點數。</p>'}
        <p class="meta-line">白化率依已儲存的表層 HC／SC 點數計算；分母為零時顯示無法計算。</p>
      </section>
    `
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
          ${table(["物種／體長", ...segments, "合計", "平均", "SD", "有效段數"], taxa.map(taxon => {
            const rows = [1, 2, 3, 4].map(segment => t.belt.find(r => r.taxon_id === taxon.taxon_id && r.segment === segment))
            const stats = statistics(rows.map(r => r?.count))
            return `<tr><th scope="row">${esc(taxon.name_zh)}<small>${esc(taxon.name_en)} ${esc(taxon.size_class)}</small>${taxon.is_aggregate ? '<small>總數列（唯讀）</small>' : ""}</th>${rows.map((r, i) => `<td>${r ? numericInput("belt", r, r.count, `${taxon.name_en} ${taxon.size_class} S${i + 1}`, 2147483647) : '<span class="rd-missing">未記錄</span>'}</td>`).join("")}<td>${number(stats.total)}</td><td>${number(stats.mean)}</td><td>${number(stats.sd)}</td><td>${stats.n}/4</td></tr>`
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
        <p class="meta-line">件數保留原始值；有原始件數的指標以 min(原始值, 3) 衍生等級。百分比使用 0–100。</p>
        ${types.length ? table(["項目／單位", ...segments, "平均", "SD", "有效段數"], types.map(type => {
          const rows = [1, 2, 3, 4].map(segment => t.impacts.find(r => r.impact_type_id === type.impact_type_id && r.segment === segment))
          const stats = statistics(rows.map(r => r ? impactValue(r) : null))
          const isPercent = type.value_type === "percent"
          const unit = isPercent ? "%" : type.has_raw_count ? "原始件數 → 衍生等級" : type.value_type === "level" ? "等級 0–3" : "件數"
          return `<tr><th scope="row">${esc(type.name_zh)}<small>${esc(type.name_en)}</small><small>${unit}</small></th>${rows.map((r, i) => `<td>${r ? `${numericInput("impact", r, r.raw_value, `${type.name_en} S${i + 1}`, isPercent ? 100 : !type.has_raw_count && type.value_type === "level" ? 3 : null, !isPercent)}${type.has_raw_count ? `<small>等級 ${number(impactValue(r))}</small>` : isPercent ? " %" : ""}` : "未記錄"}</td>`).join("")}<td>${number(stats.mean)}${isPercent ? "%" : ""}</td><td>${number(stats.sd)}</td><td>${stats.n}/4</td></tr>`
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
        const change = bleaching.get(id) || { kind: "bleaching", id, hc: row.hc_bleached_count, sc: row.sc_bleached_count }
        change[kind] = Number(input.value)
        bleaching.set(id, change)
      } else {
        const row = (kind === "belt" ? t.belt : t.impacts).find(r => r.id === id)
        if (Number(input.value) !== (kind === "belt" ? row.count : row.raw_value)) changes.push({ kind, id, value: Number(input.value) })
      }
    })

    for (const c of bleaching.values()) {
      const r = t.bleaching.find(row => row.id === c.id)
      if (c.hc !== r.hc_bleached_count || c.sc !== r.sc_bleached_count) changes.push(c)
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

  function showDialog({ title, bodyHTML, onSubmit, submitText = "確認" }) {
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
    try {
      sites = await getSites()
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
      const label = `${site.name_zh} (${site.name_en || ""}) ${loc ? `[${loc}]` : ""}`
      return `<option value="${site.id}">${esc(label)}</option>`
    }).join("")

    const today = new Date().toISOString().slice(0, 10)
    const bodyHTML = `
      <label>
        樣點 (Site) *
        <select name="site_id" required>
          <option value="">請選擇樣點</option>
          ${siteOptions}
        </select>
      </label>
      <label>
        調查日期 (Survey Date) *
        <input type="date" name="survey_date" value="${today}" required>
      </label>
      <label>
        場次時間 (Event Time)
        <input type="text" name="event_time" value="09:00" placeholder="例如 09:00 或 na">
        <small class="meta-line">未記錄請輸入 na</small>
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

        const created = await apiFetch("/admin/reef-check-data/events", {
          method: "POST",
          body: {
            site_id: siteID,
            survey_date: surveyDate,
            event_time: eventTime,
            depth_m: depthM,
            label,
            methods: selectedMethods,
          },
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
