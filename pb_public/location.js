const API_BASE = `${window.location.origin}/api`

const grid = document.querySelector("#location-grid")
const summary = document.querySelector("#location-summary")
const status = document.querySelector("#location-status")
const search = document.querySelector("#location-search")

let items = []

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

async function loadLocations() {
  const response = await fetch(`${API_BASE}/collections/location/records?page=1&perPage=500&sort=chineseName`, {
    headers: {
      Accept: "application/json",
    },
  })

  const data = await response.json()

  if (!response.ok) {
    throw new Error(data?.message || `Request failed (${response.status})`)
  }

  items = data.items || []
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
      ? `Showing ${filtered.length} of ${items.length} locations`
      : `Showing ${items.length} locations`
  }

  if (!filtered.length) {
    grid.innerHTML = `
      <div class="empty-state">
        <p>沒有符合條件的地點。</p>
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
    setStatus("Loading locations...")
    await loadLocations()
    setStatus("")
    render()
  } catch (error) {
    setStatus(error?.message || "Unable to load locations.")
    grid.innerHTML = ""
  }

  search?.addEventListener("input", render)
}

void boot()
