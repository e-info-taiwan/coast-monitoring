const API_BASE = `${window.location.origin}/api`
const APP_API_BASE = `${API_BASE}/app`

const grid = document.querySelector("#species-grid")
const summary = document.querySelector("#species-summary")
const status = document.querySelector("#species-status")
const search = document.querySelector("#species-search")

let items = []
let csrfToken = ""

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;")
}

function setStatus(message) {
  if (status) {
    status.textContent = message || ""
  }
}

async function loadSpecies() {
  const sessionResponse = await fetch(`${API_BASE}/session`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  })
  const session = await sessionResponse.json()
  if (!sessionResponse.ok || !session?.authenticated) {
    throw new Error("請先登入後再查看物種清單。")
  }
  csrfToken = session.csrfToken || ""

  const response = await fetch(`${APP_API_BASE}/species`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
      "X-CSRF-Token": csrfToken,
    },
  })

  const data = await response.json()

  if (!response.ok) {
    throw new Error(data?.message || `Request failed (${response.status})`)
  }

  items = Array.isArray(data) ? data : []
}

function matches(record, query) {
  if (!query) {
    return true
  }

  const haystack = `${record.chineseName || ""} ${record.englishName || ""}`.toLowerCase()
  return haystack.includes(query.toLowerCase())
}

function render() {
  const query = String(search?.value || "").trim()
  const filtered = items.filter((record) => matches(record, query))

  if (summary) {
    summary.textContent = query
      ? `Showing ${filtered.length} of ${items.length} species`
      : `Showing ${items.length} species`
  }

  if (!filtered.length) {
    grid.innerHTML = `
      <div class="empty-state">
        <p>沒有符合條件的物種。</p>
      </div>
    `
    return
  }

  grid.innerHTML = filtered
    .map(
      (record) => `
        <article class="species-card">
          <div class="species-head">
            <span class="public-badge">${escapeHtml(record.chineseName || "—")}</span>
            <span class="species-id">${escapeHtml(record.id)}</span>
          </div>
          <h3>${escapeHtml(record.englishName || "—")}</h3>
          <div class="species-meta">
            <span>中文名稱</span>
            <strong>${escapeHtml(record.chineseName || "—")}</strong>
          </div>
          <div class="species-meta">
            <span>英文名稱</span>
            <strong>${escapeHtml(record.englishName || "—")}</strong>
          </div>
        </article>
      `,
    )
    .join("")
}

async function boot() {
  try {
    setStatus("Loading species...")
    await loadSpecies()
    setStatus("")
    render()
  } catch (error) {
    setStatus(error?.message || "Unable to load species.")
    grid.innerHTML = ""
  }

  search?.addEventListener("input", render)
}

void boot()
