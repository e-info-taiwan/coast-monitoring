import { stationDescription, chartFields, bindChartFields, collectChartFields } from "./site-chart-fields.js"
const esc = (v) =>
  String(v ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
const field = (name, label, value = "", type = "text") =>
  `<label class="form-field"><span>${esc(label)}</span><input name="${name}" type="${type}" ${type === "number" ? 'step="any"' : ""} value="${esc(value)}"></label>`
const area = (name, label, value = "") =>
  `<label class="form-field pc-wide"><span>${esc(label)}</span><textarea name="${name}">${esc(value)}</textarea></label>`
const select = (name, label, options, value) =>
  `<label class="form-field"><span>${esc(label)}</span><select name="${name}">${options.map(([v, l]) => `<option value="${esc(v)}" ${String(v) === String(value ?? "") ? "selected" : ""}>${esc(l)}</option>`).join("")}</select></label>`
const statusOptions = [
  ["draft", "草稿"],
  ["published", "發布"],
  ["archived", "封存"],
]
const labels = {
  substrate_depth: "底質深度比較",
  taxa_density: "物種密度",
  rare_history: "罕見生物",
  live_coral_history: "活珊瑚覆蓋率",
  impact_history: "環境衝擊",
  bleaching_history: "白化",
}
const numeric = (v) => (v === "" ? null : Number(v))
export function affectedSiteIDs(before, after) {
  return [...new Set([...before, ...after].map((x) => Number(x.site_id)))].sort((a, b) => a - b)
}
export function parseOptionalList(value, numbers = false) {
  if (value.trim() === "") return null
  if (value.trim() === "[]") return []
  return value.split(",").map((x) => (numbers ? Number(x.trim()) : x.trim()))
}

export async function openSiteWorkspace({ siteID, apiFetch, drawer, notify, onSaved }) {
  const [sites, display, profiles, annotations, articles] = await Promise.all([
    apiFetch("/admin/sites"),
    apiFetch(`/admin/sites/${siteID}/display`),
    apiFetch("/admin/public-content/profile"),
    apiFetch("/admin/public-content/annotation"),
    apiFetch("/admin/public-content/article"),
  ])
  const site = sites.find((x) => String(x.id) === String(siteID))
  if (!site) throw new Error("找不到樣點")
  const siteName = (id) => sites.find((x) => Number(x.id) === Number(id))?.name_zh || `樣點 ${id}`
  const dirty = new Set()
  const revisions = new Map()
  const states = new Map()
  const form = (id, title, body) =>
    `<form data-form="${id}" class="editor-form pc-form"><h3>${esc(title)}</h3><div class="pc-grid">${body}</div><p role="status" data-result></p><div class="editor-actions sw-actions"><button class="primary-button" type="submit">儲存${esc(title)}</button></div></form>`
  const basic = [
    field("name_zh", "中文名稱", site.name_zh),
    field("name_en", "英文名稱", site.name_en),
    field("region", "分區", site.region),
    field("county", "縣市", site.county),
    field("location", "位置", site.location),
    field("latitude", "緯度", site.latitude, "number"),
    field("longitude", "經度", site.longitude, "number"),
    select(
      "is_active",
      "啟用",
      [
        ["true", "啟用"],
        ["false", "停用"],
      ],
      String(site.is_active),
    ),
    `<label class="form-field pc-wide">搜尋海溫測站<input type="search" id="sw-station-search" placeholder="站名或站號"><select name="cwa_station_id" id="sw-station"><option value="${esc(site.cwa_station_id || "")}">${esc(site.cwa_station_name || site.cwa_station_id || "未設定")}</option></select></label><p class="pc-wide" id="sw-station-status" role="status">設定來源：${esc(site.cwa_station_source || "unknown")} ${esc(site.cwa_station_updated_at || "")}</p><button type="button" id="sw-sync">更新測站清單</button><p id="sw-sync-status" role="status"></p>`,
  ].join("")
  const contentBody =
    select(
      "kind",
      "內容種類",
      [
        ["profile", "樣點介紹"],
        ["annotation", "事件註記"],
        ["article", "共用觀測報告內容"],
      ],
      "profile",
    ) +
    select("content_id", "編輯內容", [["", "新增草稿"]], "") +
    field("title", "標題") +
    area("summary", "說明") +
    field("date", "原文／事件日期（未知可空白）", "", "date") +
    field("external_url", "報告外部網址", "", "url") +
    field("source_url", "事件查證來源", "", "url") +
    field("image_url", "圖片網址", "", "url") +
    select("publication_status", "發布狀態", statusOptions, "draft")
  const chartForms = display.effective_settings
    .map((e) => {
      const c =
        display.draft_settings?.find((d) => d.chart_key === e.chart_key) ||
        (e.source === "site" ? e : { chart_key: e.chart_key, site_id: siteID })
      states.set(`chart-${e.chart_key}`, c)
      return form(
        `chart-${e.chart_key}`,
        labels[e.chart_key],
        chartFields(c, e, display.chart_options?.[e.chart_key]) +
          select("publication_status", "發布狀態", statusOptions, c.publication_status || "draft") +
          `<p>目前公開設定來源：${esc(e.source === "site" ? "本站設定" : "內建預設")}</p><button type="button" data-restore="${e.chart_key}">還原預設</button>`,
      )
    })
    .join("")
  drawer.open({
    eyebrow: "調查樣點",
    title: site.name_zh,
    subtitle: "各區獨立儲存；其他區的修改會保留。",
    guard: () => !dirty.size || window.confirm("此樣點仍有未儲存修改，確定離開？"),
    html: `<div class="site-workspace"><section class="pc-form-section">${form("basic", "基本資料", basic)}</section><section class="pc-form-section"><h2>公開資訊與報告</h2>${form("content", "公開內容", contentBody)}<div id="sw-reports"></div><div class="pc-grid">${select("report_article", "關聯既有報告", [["", "請選擇"], ...articles.filter((a) => a.data.category === "observation_report").map((a) => [a.id, a.data.title])], "")}<button type="button" id="sw-add-report">加入本站</button></div><p id="sw-report-result" role="status"></p></section><section class="pc-form-section"><h2>圖表顯示</h2>${chartForms}<h3>活珊瑚共用群組</h3><div id="sw-groups"></div><button type="button" id="sw-add-group">新增共用群組</button></section></div>`,
    bind(body) {
      const mark = (f) => {
        const key = f.dataset.form
        dirty.add(key)
        revisions.set(key, (revisions.get(key) || 0) + 1)
      }
      const bindForm = (f, save) => {
        f.addEventListener("input", () => mark(f))
        f.addEventListener("change", () => mark(f))
        f.onsubmit = async (event) => {
          event.preventDefault()
          const button = f.querySelector('[type="submit"]'),
            result = f.querySelector("[data-result]"),
            revision = revisions.get(f.dataset.form)
          button.disabled = true
          try {
            await save(new FormData(f), f)
            if (revisions.get(f.dataset.form) === revision) dirty.delete(f.dataset.form)
            result.textContent = "已儲存；其他區修改保留。"
            onSaved?.()
          } catch (error) {
            result.textContent = error.message || "儲存失敗"
          } finally {
            button.disabled = false
          }
        }
      }
      const basicForm = body.querySelector('[data-form="basic"]')
      bindForm(basicForm, async (f) => {
        const input = Object.fromEntries(f)
        delete input["undefined"]
        for (const key of ["latitude", "longitude"]) input[key] = numeric(input[key])
        input.is_active = input.is_active === "true"
        // Unrelated saves never turn an automatic/unknown mapping into manual.
        if (!basicForm.querySelector('[name="cwa_station_id"]').dataset.changed) delete input.cwa_station_id
        const saved = await apiFetch(`/admin/sites/${siteID}`, { method: "PATCH", body: input })
        Object.assign(site, saved)
        delete basicForm.querySelector('[name="cwa_station_id"]').dataset.changed
        body.querySelector("#sw-station-status").textContent =
          `設定來源：${site.cwa_station_source} ${site.cwa_station_updated_at || ""}`
      })
      let stations = []
      const stationSelect = body.querySelector("#sw-station")
      const renderStations = () => {
        const value = stationSelect.value,
          query = body.querySelector("#sw-station-search").value.toLowerCase()
        const rows = stations.filter((s) => s.id === value || `${s.id} ${s.name_zh}`.toLowerCase().includes(query))
        stationSelect.innerHTML = `<option value="">不設定測站</option>${rows.map((s) => `<option value="${esc(s.id)}" ${s.id === value ? "selected" : ""}>${esc(s.name_zh)} (${esc(s.id)}) · ${esc(stationDescription(s))}</option>`).join("")}`
        if (value && !rows.some((s) => s.id === value)) {
          const option = document.createElement("option")
          option.value = value
          option.textContent = `目前對應 ${value}（清單未提供）`
          option.selected = true
          stationSelect.append(option)
        }
      }
      const loadStations = async () => {
        try {
          stations = await apiFetch(`/admin/cwa-marine/stations?site_id=${siteID}`)
          renderStations()
        } catch (error) {
          body.querySelector("#sw-station-status").textContent = `測站清單載入失敗：${error.message}`
        }
      }
      stationSelect.onchange = () => {
        stationSelect.dataset.changed = "true"
      }
      body.querySelector("#sw-station-search").oninput = renderStations
      body.querySelector("#sw-sync").onclick = async () => {
        const button = body.querySelector("#sw-sync"),
          status = body.querySelector("#sw-sync-status")
        button.disabled = true
        status.textContent = "同步中…"
        try {
          const r = await apiFetch("/admin/cwa-marine/sync", { method: "POST", body: {} })
          status.textContent = r.errors?.length ? r.errors.join("；") : `已同步 ${r.stations_count} 個測站`
          await loadStations()
          await syncStatus()
        } catch (e) {
          status.textContent = e.message
        } finally {
          button.disabled = false
        }
      }
      async function syncStatus() {
        try {
          const s = await apiFetch("/admin/cwa-marine/sync-status")
          body.querySelector("#sw-sync-status").textContent =
            `最後成功：${s.last_success_at || "尚無紀錄"} ${s.last_run?.error_message || ""} ${s.last_run?.range_from || ""}—${s.last_run?.range_to || ""}`
        } catch {}
      }
      void loadStations()
      void syncStatus()

      const contentForm = body.querySelector('[data-form="content"]')
      let current = null
      const all = [...profiles, ...annotations, ...articles]
      const choices = () => {
        const kind = contentForm.elements.kind.value
        return all.filter(
          (c) =>
            c.kind === kind &&
            (kind === "article" ? c.data.category === "observation_report" : Number(c.site_id) === Number(siteID)),
        )
      }
      function renderContentOptions() {
        contentForm.elements.content_id.innerHTML = `<option value="">新增草稿</option>${choices()
          .map((c) => `<option value="${c.id}">${esc(c.data.title)} · ${esc(c.publication_status)}</option>`)
          .join("")}`
      }
      function fillContent() {
        current = choices().find((c) => String(c.id) === contentForm.elements.content_id.value) || null
        for (const key of ["title", "summary", "date", "external_url", "source_url", "image_url"])
          contentForm.elements[key].value = current?.data[key] || ""
        contentForm.elements.publication_status.value = current?.publication_status || "draft"
        dirty.delete("content")
      }
      contentForm.elements.kind.onchange = () => {
        if (dirty.has("content") && !window.confirm("切換內容會捨棄此內容尚未儲存的修改，確定？")) {
          contentForm.elements.kind.value = current?.kind || "profile"
          return
        }
        renderContentOptions()
        fillContent()
      }
      contentForm.elements.content_id.onchange = () => {
        if (dirty.has("content") && !window.confirm("切換內容會捨棄此內容尚未儲存的修改，確定？")) {
          contentForm.elements.content_id.value = current?.id || ""
          return
        }
        fillContent()
      }
      renderContentOptions()
      fillContent()
      bindForm(contentForm, async (f) => {
        const kind = String(f.get("kind")),
          data = { ...current?.data }
        for (const key of ["title", "summary", "date", "external_url", "source_url", "image_url"])
          data[key] = f.get(key)
        if (kind === "article") data.category = "observation_report"
        const related = current?.related_site_ids || []
        if (related.length && !window.confirm(`共用內容會影響：${related.map(siteName).join("、")}。確定修改？`))
          throw new Error("已取消")
        if (f.get("publication_status") === "archived" && !window.confirm("確定封存此內容？")) throw new Error("已取消")
        const saved = await apiFetch(`/admin/public-content/${kind}${current ? `/${current.id}` : ""}`, {
          method: current ? "PATCH" : "POST",
          body: {
            kind,
            site_id: kind === "article" ? null : Number(siteID),
            data,
            publication_status: f.get("publication_status"),
            version: current?.updated_at || "",
            confirm: true,
          },
        })
        const index = all.findIndex((x) => x.id === saved.id)
        if (index >= 0) all[index] = saved
        else all.push(saved)
        current = saved
        renderContentOptions()
        contentForm.elements.content_id.value = saved.id
        contentForm.elements.publication_status.value = saved.publication_status
        if (kind === "article") {
          body.querySelector('[name="report_article"]').innerHTML = `<option value="">請選擇</option>${all
            .filter((a) => a.kind === "article" && a.data.category === "observation_report")
            .map((a) => `<option value="${a.id}">${esc(a.data.title)}</option>`)
            .join("")}`
        }
      })
      let links = display.report_links || []
      function renderLinks() {
        body.querySelector("#sw-reports").innerHTML =
          links
            .map(
              (l) =>
                `<div class="pc-form-section sw-report"><b>${esc(all.find((a) => a.id === l.article_id)?.data.title || `報告 ${l.article_id}`)}</b><div class="sw-actions"><button type="button" data-report-toggle="${l.article_id}">${l.is_visible ? "在本站隱藏" : "在本站顯示"}</button><button type="button" data-report-remove="${l.article_id}">解除本站關聯</button></div></div>`,
            )
            .join("") || "<p>尚無關聯報告</p>"
      }
      async function changeLink(input) {
        await apiFetch(`/admin/sites/${siteID}/reports`, { method: "PATCH", body: input })
        const d = await apiFetch(`/admin/sites/${siteID}/display`)
        links = d.report_links || []
        renderLinks()
      }
      body.querySelector("#sw-reports").onclick = async (event) => {
        const button = event.target.closest("button")
        if (!button) return
        const id = Number(button.dataset.reportToggle || button.dataset.reportRemove),
          link = links.find((l) => l.article_id === id)
        button.disabled = true
        try {
          await changeLink({ ...link, is_visible: !link.is_visible, remove: Boolean(button.dataset.reportRemove) })
        } catch (e) {
          body.querySelector("#sw-report-result").textContent = e.message
          button.disabled = false
        }
      }
      body.querySelector("#sw-add-report").onclick = async () => {
        const id = Number(body.querySelector('[name="report_article"]').value)
        if (!id) return
        try {
          if (links.some((l) => l.article_id === id)) throw new Error("此報告已關聯本站")
          await changeLink({ article_id: id, is_visible: true })
        } catch (e) {
          body.querySelector("#sw-report-result").textContent = e.message
        }
      }
      renderLinks()
      for (const e of display.effective_settings) {
        const key = `chart-${e.chart_key}`,
          f = body.querySelector(`[data-form="${key}"]`)
        bindChartFields(f)
        bindForm(f, async (data) => {
          const input = collectChartFields(data, { ...states.get(key), chart_key: e.chart_key })
          input.publication_status = data.get("publication_status")
          if (input.publication_status === "archived" && !window.confirm("確定封存這項圖表設定？"))
            throw new Error("已取消")
          input.confirm = true
          states.set(key, await apiFetch(`/admin/sites/${siteID}/charts`, { method: "PATCH", body: input }))
        })
        const restoreHandler = async () => {
          if (!window.confirm("還原此圖的內建預設，不修改觀測或其他圖表。確定？")) return
          try {
            await apiFetch(`/admin/sites/${siteID}/charts`, {
              method: "PATCH",
              body: {
                ...states.get(key),
                site_id: Number(siteID),
                chart_key: e.chart_key,
                restore: true,
                confirm: true,
              },
            })
            states.set(key, { chart_key: e.chart_key, site_id: Number(siteID) })
            const fresh = { chart_key: e.chart_key, site_id: Number(siteID) }
            f.querySelector('.pc-grid').innerHTML = chartFields(fresh, e, display.chart_options?.[e.chart_key]) +
              select("publication_status", "發布狀態", statusOptions, "draft") +
              `<p>目前公開設定來源：內建預設</p>`
            // Keep the restore action available after re-rendering the fields.
            const restore = document.createElement("button")
            restore.type = "button"
            restore.textContent = "還原預設"
            restore.dataset.restore = e.chart_key
            restore.onclick = restoreHandler
            f.querySelector('.pc-grid').append(restore)
            bindChartFields(f)
            dirty.delete(key)
            f.querySelector("[data-result]").textContent = "已還原預設"
          } catch (e) {
            f.querySelector("[data-result]").textContent = e.message
          }
        }
        f.querySelector("[data-restore]").onclick = restoreHandler
      }
      function groupForm(group) {
        const key = `group-${group.id || Date.now()}`,
          holder = document.createElement("div")
        const member = (m) =>
          `<div data-member>${select(
            "member_site",
            "樣點",
            sites.map((s) => [s.id, s.name_zh]),
            m.site_id,
          )}${field("member_depth", "深度（公尺）", m.depth_m, "number")}<button type="button" data-remove-member>移除此系列</button></div>`
        holder.innerHTML = form(
          key,
          "共用群組",
          field("title", "群組名稱", group.title) +
            field("year_from", "起始年份", group.year_from, "number") +
            field("year_to", "結束年份", group.year_to, "number") +
            select("publication_status", "發布狀態", statusOptions, group.publication_status || "draft") +
            `<div data-members>${(group.members || [{ site_id: Number(siteID), depth_m: 5 }]).map(member).join("")}</div><button type="button" data-add-member>加入樣點＋深度</button>`,
        )
        body.querySelector("#sw-groups").append(holder)
        const f = holder.querySelector("form")
        let current = group
        f.querySelector("[data-add-member]").onclick = () => {
          f.querySelector("[data-members]").insertAdjacentHTML(
            "beforeend",
            member({ site_id: Number(siteID), depth_m: 5 }),
          )
          mark(f)
        }
        f.onclick = (event) => {
          const b = event.target.closest("[data-remove-member]")
          if (b) {
            b.closest("[data-member]").remove()
            mark(f)
          }
        }
        bindForm(f, async (data) => {
          const members = [...f.querySelectorAll("[data-member]")].map((el, i) => ({
            site_id: Number(el.querySelector('[name="member_site"]').value),
            depth_m: Number(el.querySelector('[name="member_depth"]').value),
            sort_order: i,
          }))
          const affected = affectedSiteIDs(current.members || [], members)
          if (!window.confirm(`共用群組變更會影響：${affected.map(siteName).join("、")}（包含移除的樣點）。確定儲存？`))
            throw new Error("已取消")
          current = await apiFetch(`/admin/sites/${siteID}/groups`, {
            method: "PATCH",
            body: {
              ...current,
              title: data.get("title"),
              year_from: numeric(String(data.get("year_from"))),
              year_to: numeric(String(data.get("year_to"))),
              publication_status: data.get("publication_status"),
              members,
              confirm: true,
            },
          })
        })
      }
      display.groups.forEach(groupForm)
      body.querySelector("#sw-add-group").onclick = () => groupForm({})
      notify?.({ tone: "info", title: "各區可獨立儲存，草稿不會連帶發布。", id: "site-workspace" })
    },
  })
}
