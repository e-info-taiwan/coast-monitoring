const API_BASE = `${window.location.origin}/api`
const ADMIN_API_BASE = `${API_BASE}/admin`
const APP_API_BASE = `${API_BASE}/app`
const LOCAL_DEV_EMAIL = "hcchien@gmail.com"

const $ = (selector) => document.querySelector(selector)

const loginView = $("#login-view")
const accessView = $("#access-view")
const appView = $("#app-view")
const providerList = $("#provider-list")
const passwordLoginForm = $("#password-login-form")
const loginStatus = $("#login-status")
const workspaceStatus = $("#workspace-status")
const resourceNav = $("#resource-nav")
const resourcePanel = $("#resource-panel")
const accountName = $("#account-name")
const accountEmail = $("#account-email")
const sectionEyebrow = $("#section-eyebrow")
const sectionTitle = $("#section-title")
const sectionDescription = $("#section-description")
const sessionBadges = $("#session-badges")
const alertRegion = $("#alert-region")
const drawer = $("#drawer")
const drawerScrim = $("#drawer-scrim")
const drawerEyebrow = $("#drawer-eyebrow")
const drawerTitle = $("#drawer-title")
const drawerSubtitle = $("#drawer-subtitle")
const drawerBody = $("#drawer-body")
const drawerClose = $("#drawer-close")
const sidebar = $("#sidebar")
const sidebarScrim = $("#sidebar-scrim")
const openSidebarButton = $("#open-sidebar-button")
const mobileSectionTitle = $("#mobile-section-title")
const demoFab = $("#demo-fab")
const demoPanel = $("#demo-panel")
const demoClose = $("#demo-close")
const demoLoadingButton = $("#demo-loading")
const demoErrorButton = $("#demo-error")
const demoClearButton = $("#demo-clear")

const OVERVIEW_KEY = "overview"
const ENTRY_KEY = "observation"

const resourceConfigs = {
  users: {
    title: "Users",
    eyebrow: "People",
    description: "Login users and their admin or volunteer role.",
    note: "Create password users or maintain role, status, and display name.",
    listPath: "/admin/users",
    createPath: "/admin/users",
    updatePath: (id) => `/admin/users/${id}`,
    deletePath: (id) => `/admin/users/${id}`,
    tableColumns: [
      { key: "email", label: "Email" },
      { key: "name", label: "Name" },
      { key: "role", label: "Role" },
      { key: "status", label: "Status" },
      { key: "updatedAt", label: "Updated" },
    ],
    editableFields: [
      { key: "email", label: "Email", type: "email", required: true },
      { key: "name", label: "Name", type: "text", required: true },
      {
        key: "role",
        label: "Role",
        type: "select",
        required: true,
        options: [
          { value: "admin", label: "admin" },
          { value: "volunteer", label: "volunteer" },
        ],
      },
      {
        key: "status",
        label: "Status",
        type: "select",
        required: true,
        options: [
          { value: "active", label: "active" },
          { value: "disabled", label: "disabled" },
        ],
      },
      {
        key: "password",
        label: "Password",
        type: "password",
        placeholder: "Set a new password",
        omitWhenBlank: true,
      },
    ],
    canCreate: true,
  },
  location: {
    title: "Location",
    eyebrow: "Places",
    description: "Location names in Chinese and English.",
    note: "Use this table for bilingual location naming.",
    listPath: "/admin/locations",
    appListPath: "/app/locations",
    createPath: "/admin/locations",
    updatePath: (id) => `/admin/locations/${id}`,
    deletePath: (id) => `/admin/locations/${id}`,
    tableColumns: [
      { key: "chineseName", label: "Chinese name" },
      { key: "englishName", label: "English name" },
      { key: "updatedAt", label: "Updated" },
    ],
    editableFields: [
      {
        key: "chineseName",
        label: "Chinese name",
        type: "text",
        required: true,
        placeholder: "中文名稱",
      },
      {
        key: "englishName",
        label: "English name",
        type: "text",
        required: true,
        placeholder: "English name",
      },
    ],
    canCreate: true,
  },
  species: {
    title: "Species",
    eyebrow: "Biodiversity",
    description: "Species names in Chinese and English.",
    note: "Keep canonical species names here so other workflows can reference them cleanly.",
    listPath: "/admin/species",
    appListPath: "/app/species",
    createPath: "/admin/species",
    updatePath: (id) => `/admin/species/${id}`,
    deletePath: (id) => `/admin/species/${id}`,
    tableColumns: [
      { key: "chineseName", label: "Chinese name" },
      { key: "englishName", label: "English name" },
      { key: "updatedAt", label: "Updated" },
    ],
    editableFields: [
      {
        key: "chineseName",
        label: "Chinese name",
        type: "text",
        required: true,
        placeholder: "中文名稱",
      },
      {
        key: "englishName",
        label: "English name",
        type: "text",
        required: true,
        placeholder: "English name",
      },
    ],
    canCreate: true,
  },
  admin_observations: {
    title: "Observation Records",
    eyebrow: "Operations",
    description: "Admin-wide observation review and correction.",
    note: "Use the field entry page for daily entry; use this table for admin corrections.",
    listPath: "/admin/observations",
    updatePath: (id) => `/admin/observations/${id}`,
    deletePath: (id) => `/admin/observations/${id}`,
    tableColumns: [
      { key: "observedOn", label: "Observed on" },
      { key: "locationLabel", label: "Location" },
      { key: "speciesLabel", label: "Species" },
      { key: "count", label: "Count" },
      { key: "updatedAt", label: "Updated" },
    ],
    editableFields: [
      { key: "observedOn", label: "Observed on", type: "date", required: true },
      { key: "locationId", label: "Location ID", type: "text", required: true },
      { key: "speciesId", label: "Species ID", type: "text", required: true },
      { key: "observerId", label: "Observer ID", type: "text", required: true },
      { key: "count", label: "Count", type: "number", valueType: "number", required: true },
      { key: "notes", label: "Notes", type: "textarea" },
    ],
    canCreate: false,
  },
  audit_logs: {
    title: "Audit Logs",
    eyebrow: "Operations",
    description: "System-generated operation history.",
    note: "Read-only trail of create, update, and delete operations.",
    listPath: "/admin/audit-logs",
    tableColumns: [
      { key: "loggedAt", label: "Logged at" },
      { key: "action", label: "Action" },
      { key: "targetTable", label: "Table" },
      { key: "targetId", label: "Record id" },
      { key: "actorEmail", label: "Actor" },
    ],
    editableFields: [],
    canCreate: false,
    readOnly: true,
  },
}

const state = {
  activeKey: ENTRY_KEY,
  auth: null,
  csrfToken: "",
  records: {},
  selectedIds: {},
  history: {},
  draft: null,
  drawerMode: null,
  loading: false,
  demoLoading: false,
  demoError: false,
}

const NAV_TRANSITION_MS = 300
const DEMO_LOADING_MS = 2500
let navTransitionTimer = null
let demoLoadingTimer = null

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;")
}

function showView(mode) {
  loginView?.classList.toggle("hidden", mode !== "login")
  accessView?.classList.toggle("hidden", mode !== "access")
  appView?.classList.toggle("hidden", mode !== "app")
  demoFab?.classList.toggle("hidden", mode !== "app")
  if (mode !== "app") {
    demoPanel?.classList.add("hidden")
    demoFab?.setAttribute("aria-expanded", "false")
  }
}

function activeStatusElement() {
  if (appView && !appView.classList.contains("hidden")) {
    return workspaceStatus || loginStatus
  }
  return loginStatus
}

function setStatus(message, tone = "neutral") {
  const el = activeStatusElement()
  if (!el) {
    return
  }
  el.textContent = message || ""
  el.classList.remove("is-error", "is-hint")
  if (tone === "error") {
    el.classList.add("is-error")
  } else if (tone === "hint") {
    el.classList.add("is-hint")
  }
  if (el === workspaceStatus) {
    workspaceStatus.classList.toggle("hidden", !message)
  }
}

function showAlert({ tone = "info", title, message, id, dismissible = true } = {}) {
  if (!alertRegion) {
    return null
  }
  if (id) {
    dismissAlert(id)
  }
  const alertId = id || `alert-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  const glyph = { info: "i", ok: "✓", caution: "!", warning: "!", danger: "×" }[tone] || "i"
  const node = document.createElement("div")
  node.className = `alert tone-${tone}`
  node.dataset.alertId = alertId
  node.innerHTML = `
    <span class="alert-icon" aria-hidden="true">${glyph}</span>
    <div class="alert-body">
      ${title ? `<div class="alert-title">${escapeHtml(title)}</div>` : ""}
      ${message ? `<div class="alert-message">${escapeHtml(message)}</div>` : ""}
    </div>
    ${dismissible ? `<button type="button" class="alert-close" aria-label="Dismiss">×</button>` : ""}
  `
  node.querySelector(".alert-close")?.addEventListener("click", () => dismissAlert(alertId))
  alertRegion.appendChild(node)
  return alertId
}

function dismissAlert(id) {
  alertRegion?.querySelector(`[data-alert-id="${id}"]`)?.remove()
}

function clearAlerts() {
  if (alertRegion) {
    alertRegion.innerHTML = ""
  }
}

async function apiFetch(path, { method = "GET", body, auth = true, headers = {} } = {}) {
  const init = {
    method,
    credentials: "include",
    headers: {
      Accept: "application/json",
      ...headers,
    },
  }
  if (auth && state.csrfToken) {
    init.headers["X-CSRF-Token"] = state.csrfToken
  }
  if (body !== undefined) {
    init.headers["Content-Type"] = "application/json"
    init.body = JSON.stringify(body)
  }

  const response = await fetch(`${API_BASE}${path}`, init)
  const raw = await response.text()
  let data = null
  if (raw) {
    try {
      data = JSON.parse(raw)
    } catch (_) {
      data = raw
    }
  }
  if (!response.ok) {
    const message =
      data && typeof data === "object" && (data.error || data.message)
        ? data.error || data.message
        : `Request failed (${response.status})`
    throw new Error(message)
  }
  return data
}

function saveSession(session) {
  state.auth = session?.user || null
  state.csrfToken = session?.csrfToken || ""
}

function clearSession() {
  state.auth = null
  state.csrfToken = ""
}

function currentUser() {
  return state.auth
}

function isAdmin() {
  return currentUser()?.role === "admin"
}

function isVolunteer() {
  return currentUser()?.role === "volunteer"
}

function hasAccess() {
  return isAdmin() || isVolunteer()
}

function canSeeNav(key) {
  if (isAdmin()) {
    return true
  }
  return key === ENTRY_KEY
}

function canLoadResource(key) {
  if (isAdmin()) {
    return true
  }
  return ["location", "species", ENTRY_KEY].includes(key)
}

function resourceListPath(key) {
  if ((key === "location" || key === "species") && !isAdmin()) {
    return resourceConfigs[key].appListPath
  }
  if (key === ENTRY_KEY) {
    return "/app/observations"
  }
  return resourceConfigs[key]?.listPath
}

function supportsBatchDelete(resourceKey) {
  return ["users", "location", "species"].includes(resourceKey)
}

function selectionSet(resourceKey) {
  if (!state.selectedIds[resourceKey]) {
    state.selectedIds[resourceKey] = new Set()
  }
  return state.selectedIds[resourceKey]
}

function selectedIds(resourceKey) {
  return [...selectionSet(resourceKey)]
}

function setSelected(resourceKey, id, checked) {
  const set = selectionSet(resourceKey)
  if (checked) {
    set.add(id)
  } else {
    set.delete(id)
  }
}

function clearSelection(resourceKey) {
  selectionSet(resourceKey).clear()
}

function formatTime(value) {
  if (!value) {
    return ""
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }
  return date.toLocaleString("zh-TW", { dateStyle: "short", timeStyle: "short" })
}

function formatDate(value) {
  if (!value) {
    return "—"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return String(value)
  }
  return date.toLocaleDateString("zh-TW", { month: "2-digit", day: "2-digit" })
}

function formatRelativeTime(value) {
  if (!value) {
    return ""
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return ""
  }
  const diff = Date.now() - date.getTime()
  const minutes = Math.max(0, Math.floor(diff / 60000))
  if (minutes < 1) {
    return "just now"
  }
  if (minutes < 60) {
    return `${minutes}m ago`
  }
  const hours = Math.floor(minutes / 60)
  if (hours < 24) {
    return `${hours}h ago`
  }
  return `${Math.floor(hours / 24)}d ago`
}

function mapObservation(record) {
  const location = (state.records.location || []).find((item) => item.id === record.locationId)
  const species = (state.records.species || []).find((item) => item.id === record.speciesId)
  return {
    ...record,
    locationLabel: location?.chineseName || location?.englishName || record.locationId,
    speciesLabel: species?.chineseName || species?.englishName || record.speciesId,
  }
}

function normalizeRecord(resourceKey, record) {
  if (!record || typeof record !== "object") {
    return record
  }
  const normalized = {
    ...record,
    createDate: record.createdAt || record.createDate,
    updateDate: record.updatedAt || record.updateDate,
  }
  if (resourceKey === "admin_observations" || resourceKey === ENTRY_KEY) {
    return mapObservation(normalized)
  }
  return normalized
}

async function loadResource(resourceKey) {
  const path = resourceListPath(resourceKey)
  if (!path) {
    state.records[resourceKey] = []
    return
  }
  try {
    const data = await apiFetch(path)
    const records = Array.isArray(data) ? data : data?.items || []
    state.records[resourceKey] = records.map((item) => normalizeRecord(resourceKey, item))
    if (supportsBatchDelete(resourceKey)) {
      const validIds = new Set(state.records[resourceKey].map((record) => record.id))
      ;[...selectionSet(resourceKey)].forEach((id) => {
        if (!validIds.has(id)) {
          setSelected(resourceKey, id, false)
        }
      })
    }
  } catch (error) {
    state.records[resourceKey] = []
    showAlert({
      tone: "danger",
      title: `無法載入「${resourceConfigs[resourceKey]?.title || resourceKey}」`,
      message: error?.message || "",
      id: `load-${resourceKey}`,
    })
  }
}

async function loadWorkspace() {
  const keys = Object.keys(resourceConfigs).filter(canLoadResource)
  await Promise.all(keys.map((key) => loadResource(key)))
  if (!keys.includes(ENTRY_KEY)) {
    await loadResource(ENTRY_KEY)
  }
}

function renderProviderList() {
  if (!providerList) {
    return
  }
  providerList.innerHTML = ""
  const button = document.createElement("button")
  button.type = "button"
  button.className = "provider-button"
  button.innerHTML = '<span class="session-pill">G</span><span>Sign in with Google</span>'
  button.addEventListener("click", () => {
    setStatus("正在開啟 Google 登入…", "hint")
    window.location.href = "/api/auth/google/start?redirect=/"
  })
  providerList.appendChild(button)
}

function renderPasswordLogin() {
  passwordLoginForm?.classList.remove("hidden")
  const emailInput = passwordLoginForm?.querySelector("#dev-email")
  if (emailInput && !emailInput.value) {
    emailInput.value = LOCAL_DEV_EMAIL
  }
}

async function refreshSession() {
  const session = await apiFetch("/session", { auth: false })
  saveSession(session)
  return Boolean(session?.authenticated && session?.user)
}

function renderSessionBadges() {
  const user = currentUser()
  sessionBadges.innerHTML = user
    ? `
      <span class="session-pill">role: ${escapeHtml(user.role || "unknown")}</span>
      <span class="session-pill">${escapeHtml(user.email || "-")}</span>
    `
    : ""
}

function renderSidebar() {
  const navItems = [
    { key: ENTRY_KEY, title: "Observation", count: null },
    { key: OVERVIEW_KEY, title: "Overview", count: null },
    ...Object.entries(resourceConfigs)
      .filter(([key]) => key !== ENTRY_KEY)
      .map(([key, config]) => ({
        key,
        title: config.title,
        count: state.records[key]?.length ?? 0,
      })),
  ].filter(({ key }) => key === OVERVIEW_KEY ? isAdmin() : canSeeNav(key))

  resourceNav.innerHTML = navItems
    .map(({ key, title, count }) => {
      const countHtml = count === null ? "" : `<span>${count}</span>`
      return `<button type="button" class="nav-button ${state.activeKey === key ? "active" : ""}" data-nav="${escapeHtml(key)}"><span>${escapeHtml(title)}</span>${countHtml}</button>`
    })
    .join("")

  resourceNav.querySelectorAll("[data-nav]").forEach((button) => {
    button.addEventListener("click", () => {
      const key = button.dataset.nav
      if (!key || state.activeKey === key) {
        closeSidebar()
        return
      }
      state.activeKey = key
      state.draft = null
      closeDrawer({ rerender: false })
      closeSidebar()
      triggerNavTransition()
    })
  })
}

function valueForRecord(record, key) {
  const value = record?.[key]
  if (value == null || value === "") {
    return ""
  }
  if (key.endsWith("At") || key === "createDate" || key === "updateDate" || key === "loggedAt") {
    return formatTime(value)
  }
  if (typeof value === "object") {
    return JSON.stringify(value)
  }
  return value
}

function renderTableRows(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const rows = state.records[resourceKey] || []
  if (!rows.length) {
    return '<div class="empty-state"><p>目前還沒有任何資料。</p></div>'
  }
  return rows.map((record) => {
    const cells = config.tableColumns
      .map((column) => `<div>${escapeHtml(valueForRecord(record, column.key) || "—")}</div>`)
      .join("")
    const selectionCell = supportsBatchDelete(resourceKey)
      ? `<div class="cell-select"><input type="checkbox" class="row-select" data-action="select-row" data-id="${escapeHtml(record.id)}" ${selectionSet(resourceKey).has(record.id) ? "checked" : ""} aria-label="Select row ${escapeHtml(record.id)}" /></div>`
      : ""
    if (config.readOnly) {
      return `<div class="table-row" style="grid-template-columns: repeat(${config.tableColumns.length}, minmax(0, 1fr));">${cells}</div>`
    }
    return `
      <div class="table-row" data-action="open" data-id="${escapeHtml(record.id)}" style="grid-template-columns: ${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto;">
        ${selectionCell}
        ${cells}
        <div class="cell-actions">
          <button type="button" class="tiny-button" data-action="edit" data-id="${escapeHtml(record.id)}">Edit</button>
          ${config.deletePath ? `<button type="button" class="tiny-button danger-tiny" data-action="delete" data-id="${escapeHtml(record.id)}">Delete</button>` : ""}
        </div>
      </div>
    `
  }).join("")
}

function parseFieldValue(field, rawValue) {
  if (field.valueType === "number") {
    const parsed = Number(rawValue)
    return Number.isFinite(parsed) ? parsed : rawValue
  }
  return rawValue
}

function payloadFromEditorForm(editorForm, config, isCreate) {
  const formData = new FormData(editorForm)
  const payload = {}
  config.editableFields.forEach((field) => {
    const raw = formData.get(field.key)
    if (!isCreate && field.omitWhenBlank && String(raw || "").trim() === "") {
      return
    }
    payload[field.key] = parseFieldValue(field, raw)
  })
  return payload
}

function renderEditor(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const draft = state.draft
  if (config.readOnly || !config.editableFields.length) {
    return `
      <div class="editor-card">
        <div class="empty-state">
          <p>This record is read-only in the admin UI.</p>
          <p class="meta-line">${escapeHtml(config.note)}</p>
        </div>
      </div>
    `
  }
  const fields = config.editableFields.map((field) => {
    const currentValue = draft ? draft[field.key] ?? "" : ""
    let control
    if (field.type === "textarea") {
      control = `<textarea id="${field.key}" name="${field.key}" placeholder="${escapeHtml(field.placeholder || "")}" ${field.required ? "required" : ""}>${escapeHtml(currentValue)}</textarea>`
    } else if (field.type === "select") {
      control = `<select id="${field.key}" name="${field.key}" ${field.required ? "required" : ""}>${field.options.map((option) => `<option value="${escapeHtml(option.value)}" ${currentValue === option.value ? "selected" : ""}>${escapeHtml(option.label)}</option>`).join("")}</select>`
    } else {
      const type = field.type === "number" ? "number" : field.type === "email" ? "email" : field.type === "password" ? "password" : field.type === "date" ? "date" : "text"
      control = `<input id="${field.key}" name="${field.key}" type="${type}" value="${escapeHtml(currentValue)}" placeholder="${escapeHtml(field.placeholder || "")}" ${field.required ? "required" : ""} />`
    }
    return `
      <div class="form-field">
        <label for="${field.key}">${escapeHtml(field.label)}${field.required ? " *" : ""}</label>
        ${control}
      </div>
    `
  }).join("")

  return `
    <div class="editor-card">
      <form class="editor-form" id="editor-form">
        ${fields}
        <div class="editor-actions">
          <button type="button" class="ghost-button" id="cancel-editor-button">Cancel</button>
          <button type="submit" class="primary-button">${draft ? "Save changes" : "Create"}</button>
        </div>
      </form>
    </div>
  `
}

function renderRecordHistory(resourceKey) {
  const draft = state.draft
  if (!draft || resourceKey === "audit_logs" || !isAdmin()) {
    return ""
  }
  const entries = (state.records.audit_logs || []).filter(
    (entry) => entry.targetTable === resourceApiTable(resourceKey) && entry.targetId === draft.id,
  )
  return `
    <section class="history-card">
      <div class="resource-toolbar">
        <div>
          <p class="eyebrow">History</p>
          <h3>Change log</h3>
          <p class="meta-line">Audit trail for this record.</p>
        </div>
        <span class="session-pill">${entries.length} entries</span>
      </div>
      ${
        entries.length
          ? `<div class="history-list">${entries.map(renderHistoryEntry).join("")}</div>`
          : '<div class="empty-state"><p>No history for this record yet.</p></div>'
      }
    </section>
  `
}

function resourceApiTable(resourceKey) {
  return {
    users: "users",
    location: "locations",
    species: "species",
    admin_observations: "observations",
    observation: "observations",
  }[resourceKey] || resourceKey
}

function renderHistoryEntry(entry) {
  const detail = entry.afterData || entry.beforeData || {}
  return `
    <article class="history-entry">
      <div class="history-entry-top">
        <div>
          <strong>${escapeHtml(entry.action || "event")}</strong>
          <p>${escapeHtml(entry.targetTable || "record")} · ${escapeHtml(entry.targetId || "")}</p>
        </div>
        <div class="history-meta">
          <span>${escapeHtml(entry.actorEmail || "system")}</span>
          <span>${escapeHtml(formatTime(entry.loggedAt))}</span>
        </div>
      </div>
      <details class="history-diff-list">
        <summary>Payload</summary>
        <code>${escapeHtml(JSON.stringify(detail, null, 2))}</code>
      </details>
    </article>
  `
}

function renderDrawerBody() {
  const resourceKey = state.activeKey
  const config = resourceConfigs[resourceKey]
  const draft = state.draft
  drawerEyebrow.textContent = config.eyebrow
  drawerTitle.textContent = draft ? `Edit ${config.title}` : `New ${config.title}`
  drawerSubtitle.textContent = draft ? `Record id: ${draft.id}` : config.note || ""
  drawerBody.innerHTML = `
    <div class="drawer-hero">
      <div class="drawer-hero-avatar tone-${escapeHtml(resourceKey)}">${escapeHtml(config.title.slice(0, 1))}</div>
      <div class="drawer-hero-text">
        <h3>${escapeHtml(draft?.name || draft?.chineseName || draft?.englishName || draft?.email || draft?.id || config.title)}</h3>
        <p>${escapeHtml(draft?.email || draft?.englishName || draft?.observedOn || config.description)}</p>
      </div>
    </div>
    ${renderEditor(resourceKey)}
    ${renderRecordHistory(resourceKey)}
  `
  drawerBody.querySelector("#cancel-editor-button")?.addEventListener("click", () => closeDrawer())
  drawerBody.querySelector("#editor-form")?.addEventListener("submit", (event) => handleEditorSubmit(event, resourceKey))
}

async function handleEditorSubmit(event, resourceKey) {
  event.preventDefault()
  const config = resourceConfigs[resourceKey]
  const isCreate = !state.draft?.id
  const path = isCreate ? config.createPath : config.updatePath?.(state.draft.id)
  if (!path) {
    return
  }
  try {
    const payload = payloadFromEditorForm(event.target, config, isCreate)
    const saved = await apiFetch(path, { method: isCreate ? "POST" : "PATCH", body: payload })
    state.draft = normalizeRecord(resourceKey, saved)
    await Promise.all([loadResource(resourceKey), isAdmin() ? loadResource("audit_logs") : Promise.resolve()])
    showAlert({
      tone: "ok",
      title: "已儲存",
      message: `${config.title} 的變更已寫入。`,
      id: "editor",
    })
    renderWorkspace({ keepDrawer: true })
    renderDrawerBody()
  } catch (error) {
    showAlert({
      tone: "danger",
      title: "無法儲存",
      message: error?.message || "",
      id: "editor",
    })
  }
}

function renderErrorState(detail = "") {
  return `
    <div class="error-state">
      <div class="error-state-icon" aria-hidden="true">×</div>
      <h3>無法載入資料</h3>
      <p>後端連線似乎出了問題。可以等一下後重試。</p>
      ${detail ? `<details class="error-state-details"><summary>技術細節</summary><code>${escapeHtml(detail)}</code></details>` : ""}
      <button type="button" class="primary-button" id="error-retry-button">重新載入</button>
    </div>
  `
}

function renderDashboard() {
  sectionEyebrow.textContent = "Coast Monitoring"
  sectionTitle.textContent = "Overview"
  sectionDescription.textContent = "監測系統的高層次總覽，即時資料量與最近異動。"
  if (mobileSectionTitle) {
    mobileSectionTitle.textContent = "Overview"
  }
  if (state.demoError) {
    resourcePanel.innerHTML = renderErrorState("GET /api/dashboard - simulated failure")
    $("#error-retry-button")?.addEventListener("click", () => {
      state.demoError = false
      triggerNavTransition()
    })
    return
  }
  const keys = ["users", "location", "species", "admin_observations", "audit_logs"]
  const cards = keys.map((key) => {
    const config = resourceConfigs[key]
    const count = state.records[key]?.length ?? 0
    return `
      <button type="button" class="stat-card" data-jump="${escapeHtml(key)}">
        <div class="stat-head">
          <div class="stat-icon tone-${escapeHtml(key)}">${escapeHtml(config.title.slice(0, 1))}</div>
          <span class="stat-label">${escapeHtml(config.title)}</span>
        </div>
        <div class="stat-value">${count}</div>
        <div class="stat-sub">目前資料量</div>
      </button>
    `
  }).join("")
  const recent = [...(state.records.audit_logs || [])]
    .sort((a, b) => new Date(b.loggedAt || 0) - new Date(a.loggedAt || 0))
    .slice(0, 8)
  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>系統概況</h2>
        <p class="meta-line">點任一卡片直接進入對應資料。</p>
      </div>
    </div>
    <div class="stats-grid">${cards}</div>
    <section class="dash-panel">
      <header class="dash-panel-head">
        <div>
          <h2>最近異動</h2>
          <p>audit log 的最後 ${recent.length} 筆紀錄</p>
        </div>
      </header>
      ${
        recent.length
          ? `<div class="audit-mini-list">${recent.map(renderAuditMiniRow).join("")}</div>`
          : '<div class="empty-state"><p>目前還沒有任何 audit log。</p></div>'
      }
    </section>
  `
  resourcePanel.querySelectorAll("[data-jump]").forEach((node) => {
    node.addEventListener("click", () => {
      state.activeKey = node.dataset.jump
      triggerNavTransition()
    })
  })
}

function renderAuditMiniRow(entry) {
  const glyph = entry.action === "create" ? "+" : entry.action === "delete" ? "×" : "✎"
  return `
    <article class="audit-mini-row">
      <div class="audit-mini-pin tone-ok" aria-hidden="true">${glyph}</div>
      <div class="audit-mini-text">
        <strong>${escapeHtml(entry.action || "event")} ${escapeHtml(entry.targetTable || "record")}</strong>
        <p>${escapeHtml(entry.actorEmail || "system")} · ${escapeHtml(entry.targetId || "")}</p>
      </div>
      <time class="audit-mini-time">${escapeHtml(formatRelativeTime(entry.loggedAt))}</time>
    </article>
  `
}

function renderObservationMiniRow(entry) {
  return `
    <article class="audit-mini-row">
      <div class="audit-mini-pin tone-ok" aria-hidden="true">${escapeHtml(String(entry.count ?? 0))}</div>
      <div class="audit-mini-text">
        <strong>${escapeHtml(entry.speciesLabel || entry.speciesId)} @ ${escapeHtml(entry.locationLabel || entry.locationId)}</strong>
        <p>${escapeHtml(formatDate(entry.observedOn))}</p>
      </div>
      <time class="audit-mini-time">${escapeHtml(formatRelativeTime(entry.createdAt))}</time>
    </article>
  `
}

function renderObservation() {
  sectionEyebrow.textContent = "Field entry"
  sectionTitle.textContent = "Observation"
  sectionDescription.textContent = "依日期與地點，紀錄每個物種觀測到的數量。"
  if (mobileSectionTitle) {
    mobileSectionTitle.textContent = "Observation"
  }
  if (state.demoError) {
    resourcePanel.innerHTML = renderErrorState("GET /api/app/observations - simulated failure")
    $("#error-retry-button")?.addEventListener("click", () => {
      state.demoError = false
      triggerNavTransition()
    })
    return
  }

  const locations = state.records.location || []
  const speciesList = [...(state.records.species || [])].sort((a, b) =>
    (a.chineseName || a.englishName || "").localeCompare(b.chineseName || b.englishName || "", "zh-Hant"),
  )
  const recent = state.records[ENTRY_KEY] || []
  const todayISO = new Date().toISOString().slice(0, 10)
  const locationOptions = locations
    .map((loc) => `<option value="${escapeHtml(loc.id)}">${escapeHtml(loc.chineseName || loc.englishName || loc.id)}</option>`)
    .join("")
  const disabled = !locations.length || !speciesList.length
  const speciesRows = speciesList.length
    ? speciesList.map((sp) => `
      <div class="species-row">
        <div class="species-name">
          <strong>${escapeHtml(sp.chineseName || sp.englishName || sp.id)}</strong>
          ${sp.englishName && sp.chineseName ? `<span>${escapeHtml(sp.englishName)}</span>` : ""}
        </div>
        <input type="number" min="0" step="1" name="count_${escapeHtml(sp.id)}" data-species-id="${escapeHtml(sp.id)}" placeholder="0" />
      </div>
    `).join("")
    : '<div class="empty-state"><p>還沒有任何 species 資料。</p></div>'
  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>輸入今日觀測</h2>
        <p class="meta-line">空白或 0 會被略過；observer 會使用目前登入帳號。</p>
      </div>
    </div>
    <form class="entry-panel" id="observation-form">
      <div class="entry-meta-grid">
        <div class="form-field">
          <label for="obs-date">Date *</label>
          <input id="obs-date" type="date" name="observedOn" value="${escapeHtml(todayISO)}" required ${disabled ? "disabled" : ""} />
        </div>
        <div class="form-field">
          <label for="obs-location">Location *</label>
          <select id="obs-location" name="locationId" required ${disabled ? "disabled" : ""}>
            <option value="" disabled selected>選一個地點</option>
            ${locationOptions}
          </select>
        </div>
      </div>
      <div>
        <div class="entry-section-head">
          <h3>Species counts</h3>
          <p>${speciesList.length} 個物種</p>
        </div>
        <div class="species-rows">${speciesRows}</div>
      </div>
      <div class="form-field">
        <label for="obs-notes">Notes (optional)</label>
        <textarea id="obs-notes" name="notes" placeholder="可選；天氣、潮汐、特殊現象等" ${disabled ? "disabled" : ""}></textarea>
      </div>
      <div class="entry-actions">
        <span class="submit-hint">${disabled ? "請先建立 location 與 species。" : "只寫入 count > 0 的物種。"}</span>
        <button type="submit" class="primary-button" ${disabled ? "disabled" : ""}>Submit observations</button>
      </div>
    </form>
    <section class="dash-panel">
      <header class="dash-panel-head">
        <div>
          <h2>最近紀錄</h2>
          <p>近期 ${recent.slice(0, 8).length} 筆 observation</p>
        </div>
      </header>
      ${
        recent.length
          ? `<div class="audit-mini-list">${recent.slice(0, 8).map(renderObservationMiniRow).join("")}</div>`
          : '<div class="empty-state"><p>目前還沒有任何 observation 紀錄。</p></div>'
      }
    </section>
  `
  $("#observation-form")?.addEventListener("submit", handleObservationSubmit)
}

async function handleObservationSubmit(event) {
  event.preventDefault()
  const form = event.target
  const formData = new FormData(form)
  const observedOn = String(formData.get("observedOn") || "").trim()
  const locationId = String(formData.get("locationId") || "").trim()
  const notes = String(formData.get("notes") || "").trim()
  const entries = []
  form.querySelectorAll("input[data-species-id]").forEach((input) => {
    const count = Number(String(input.value || "").trim())
    if (Number.isFinite(count) && count > 0) {
      entries.push({ speciesId: input.dataset.speciesId, count })
    }
  })
  if (!observedOn || !locationId || !entries.length) {
    showAlert({
      tone: "caution",
      title: "資料不完整",
      message: "請選日期、地點，並至少填一個 count > 0 的物種。",
      id: "observation-submit",
    })
    return
  }
  const submitButton = form.querySelector('button[type="submit"]')
  if (submitButton) {
    submitButton.disabled = true
    submitButton.textContent = "Submitting…"
  }
  let saved = 0
  let firstError = null
  for (const entry of entries) {
    try {
      await apiFetch("/app/observations", {
        method: "POST",
        body: { observedOn, locationId, speciesId: entry.speciesId, count: entry.count, notes },
      })
      saved += 1
    } catch (error) {
      firstError = firstError || error
    }
  }
  if (submitButton) {
    submitButton.disabled = false
    submitButton.textContent = "Submit observations"
  }
  await loadResource(ENTRY_KEY)
  if (isAdmin()) {
    await Promise.all([loadResource("admin_observations"), loadResource("audit_logs")])
  }
  if (saved > 0) {
    showAlert({
      tone: "ok",
      title: `已送出 ${saved} 筆 observation`,
      message: firstError ? `部分項目失敗：${firstError.message || "unknown error"}` : "感謝紀錄！",
      id: "observation-submit",
    })
    renderObservation()
  } else {
    showAlert({
      tone: "danger",
      title: "送出失敗",
      message: firstError?.message || "",
      id: "observation-submit",
    })
  }
}

function renderResource(resourceKey) {
  if (resourceKey === ENTRY_KEY) {
    renderObservation()
    return
  }
  if (resourceKey === OVERVIEW_KEY) {
    renderDashboard()
    return
  }
  const config = resourceConfigs[resourceKey]
  sectionEyebrow.textContent = config.eyebrow
  sectionTitle.textContent = config.title
  sectionDescription.textContent = config.description
  if (mobileSectionTitle) {
    mobileSectionTitle.textContent = config.title
  }
  if (state.demoError) {
    resourcePanel.innerHTML = renderErrorState(`GET ${resourceListPath(resourceKey)} - simulated failure`)
    $("#error-retry-button")?.addEventListener("click", () => {
      state.demoError = false
      triggerNavTransition()
    })
    return
  }

  const headColumns = config.tableColumns.map((column) => `<div>${escapeHtml(column.label)}</div>`).join("")
  const headSelect = supportsBatchDelete(resourceKey)
    ? '<div class="cell-select cell-select-head"><input type="checkbox" id="select-all-records" aria-label="Select all records" /></div>'
    : ""
  const headActions = config.readOnly ? "" : '<div style="text-align:right">Actions</div>'
  const template = config.readOnly
    ? `repeat(${config.tableColumns.length}, minmax(0, 1fr))`
    : `${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto`
  const selectedNow = selectedIds(resourceKey).length
  const newButton = config.canCreate
    ? `<button type="button" class="primary-button" id="new-record-button">New ${escapeHtml(config.title)}</button>`
    : ""
  const batchBar = supportsBatchDelete(resourceKey) && selectedNow
    ? `
      <div class="batch-bar" role="region" aria-label="Batch actions">
        <span>已選 ${selectedNow} 筆</span>
        <div class="batch-bar-actions">
          <button type="button" class="tiny-button" id="batch-clear-button">Clear</button>
          <button type="button" class="tiny-button danger-tiny" id="bulk-delete-button">Delete (${selectedNow})</button>
        </div>
      </div>
    `
    : ""

  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>${escapeHtml(config.title)}</h2>
        <p class="meta-line">${escapeHtml(config.note)}</p>
      </div>
      <div class="panel-actions">
        <span class="session-pill">${state.records[resourceKey]?.length ?? 0} records</span>
        ${newButton}
      </div>
    </div>
    ${batchBar}
    <div class="resource-shell">
      <div class="resource-table">
        <div class="table-head" style="grid-template-columns: ${template};">${headSelect}${headColumns}${headActions}</div>
        ${renderTableRows(resourceKey)}
      </div>
    </div>
  `
  bindResourceEvents(resourceKey)
}

function bindResourceEvents(resourceKey) {
  const config = resourceConfigs[resourceKey]
  $("#new-record-button")?.addEventListener("click", () => {
    state.draft = null
    openDrawer()
  })
  $("#batch-clear-button")?.addEventListener("click", () => {
    clearSelection(resourceKey)
    renderResource(resourceKey)
  })
  resourcePanel.querySelectorAll("[data-action]").forEach((node) => {
    node.addEventListener("click", async (event) => {
      const action = node.dataset.action
      const id = node.dataset.id
      if (action === "select-row") {
        event.stopPropagation()
        setSelected(resourceKey, id, node.checked)
        renderResource(resourceKey)
        return
      }
      const record = state.records[resourceKey]?.find((item) => item.id === id)
      if (!record) {
        return
      }
      if (action === "open" || action === "edit") {
        event.stopPropagation()
        state.draft = record
        openDrawer()
        return
      }
      if (action === "delete") {
        event.stopPropagation()
        const label = record.email || record.chineseName || record.englishName || record.id
        if (!window.confirm(`Delete ${label}?`)) {
          return
        }
        try {
          await apiFetch(config.deletePath(record.id), { method: "DELETE" })
          setSelected(resourceKey, record.id, false)
          await Promise.all([loadResource(resourceKey), isAdmin() ? loadResource("audit_logs") : Promise.resolve()])
          showAlert({ tone: "ok", title: "已刪除", message: `已刪除 ${label}。`, id: "delete" })
          renderWorkspace()
        } catch (error) {
          showAlert({ tone: "danger", title: "無法刪除", message: error?.message || "", id: "delete" })
        }
      }
    })
  })
  $("#select-all-records")?.addEventListener("change", (event) => {
    ;(state.records[resourceKey] || []).forEach((record) => setSelected(resourceKey, record.id, event.target.checked))
    renderResource(resourceKey)
  })
  $("#bulk-delete-button")?.addEventListener("click", async () => {
    const ids = selectedIds(resourceKey)
    if (!ids.length || !window.confirm(`Delete ${ids.length} selected record(s)?`)) {
      return
    }
    try {
      for (const id of ids) {
        await apiFetch(config.deletePath(id), { method: "DELETE" })
      }
      clearSelection(resourceKey)
      await Promise.all([loadResource(resourceKey), isAdmin() ? loadResource("audit_logs") : Promise.resolve()])
      showAlert({ tone: "ok", title: "批次刪除完成", message: `已刪除 ${ids.length} 筆紀錄。`, id: "bulk-delete" })
      renderWorkspace()
    } catch (error) {
      showAlert({ tone: "danger", title: "無法批次刪除", message: error?.message || "", id: "bulk-delete" })
    }
  })
}

function renderWorkspace({ keepDrawer = false } = {}) {
  renderSidebar()
  renderSessionBadges()
  renderResource(state.activeKey)
  accountName.textContent = currentUser()?.name || currentUser()?.email || "-"
  accountEmail.textContent = currentUser()?.email || "-"
  if (keepDrawer && state.drawerMode === "open") {
    renderDrawerBody()
  }
}

function triggerNavTransition() {
  if (navTransitionTimer) {
    clearTimeout(navTransitionTimer)
  }
  state.loading = true
  renderWorkspace()
  navTransitionTimer = setTimeout(() => {
    state.loading = false
    navTransitionTimer = null
    renderWorkspace()
  }, NAV_TRANSITION_MS)
}

function openDrawer() {
  state.drawerMode = "open"
  renderDrawerBody()
  drawer.classList.add("is-open")
  drawer.setAttribute("aria-hidden", "false")
  drawerScrim.classList.add("is-open")
  document.body.classList.add("no-scroll")
}

function closeDrawer({ rerender = true } = {}) {
  state.drawerMode = null
  drawer.classList.remove("is-open")
  drawer.setAttribute("aria-hidden", "true")
  drawerScrim.classList.remove("is-open")
  document.body.classList.remove("no-scroll")
  state.draft = null
  if (rerender) {
    renderResource(state.activeKey)
  }
}

function openSidebar() {
  sidebar.classList.add("is-open")
  sidebarScrim.classList.add("is-open")
}

function closeSidebar() {
  sidebar.classList.remove("is-open")
  sidebarScrim.classList.remove("is-open")
}

async function signOut() {
  try {
    await apiFetch("/auth/logout", { method: "POST" })
  } catch (_) {
    // Clearing client state is still useful if the server session is already gone.
  }
  clearSession()
  window.location.reload()
}

function attachGlobalHandlers() {
  $("#sign-out-button")?.addEventListener("click", signOut)
  $("#sign-out-from-denied")?.addEventListener("click", signOut)
  drawerClose?.addEventListener("click", () => closeDrawer())
  drawerScrim?.addEventListener("click", () => closeDrawer())
  openSidebarButton?.addEventListener("click", openSidebar)
  sidebarScrim?.addEventListener("click", closeSidebar)
  demoFab?.addEventListener("click", () => toggleDemoPanel())
  demoClose?.addEventListener("click", () => toggleDemoPanel(false))
  demoLoadingButton?.addEventListener("click", startDemoLoading)
  demoErrorButton?.addEventListener("click", startDemoError)
  demoClearButton?.addEventListener("click", clearDemoState)
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape") {
      if (drawer.classList.contains("is-open")) {
        closeDrawer()
      } else if (sidebar.classList.contains("is-open")) {
        closeSidebar()
      } else if (!demoPanel?.classList.contains("hidden")) {
        toggleDemoPanel(false)
      }
    }
  })
}

function toggleDemoPanel(forceOpen) {
  const willOpen = typeof forceOpen === "boolean" ? forceOpen : demoPanel.classList.contains("hidden")
  demoPanel?.classList.toggle("hidden", !willOpen)
  demoFab?.setAttribute("aria-expanded", willOpen ? "true" : "false")
}

function syncDemoFabState() {
  demoFab?.classList.toggle("is-active", Boolean(state.demoLoading || state.demoError))
  demoLoadingButton?.classList.toggle("is-active", state.demoLoading)
  demoErrorButton?.classList.toggle("is-active", state.demoError)
}

function startDemoLoading() {
  if (demoLoadingTimer) {
    clearTimeout(demoLoadingTimer)
  }
  state.demoLoading = true
  state.demoError = false
  syncDemoFabState()
  renderWorkspace()
  demoLoadingTimer = setTimeout(() => {
    state.demoLoading = false
    demoLoadingTimer = null
    syncDemoFabState()
    renderWorkspace()
  }, DEMO_LOADING_MS)
}

function startDemoError() {
  if (demoLoadingTimer) {
    clearTimeout(demoLoadingTimer)
    demoLoadingTimer = null
  }
  state.demoLoading = false
  state.demoError = true
  syncDemoFabState()
  renderWorkspace()
}

function clearDemoState() {
  if (demoLoadingTimer) {
    clearTimeout(demoLoadingTimer)
    demoLoadingTimer = null
  }
  state.demoLoading = false
  state.demoError = false
  syncDemoFabState()
  renderWorkspace()
}

async function boot() {
  showView("login")
  setStatus("正在確認登入狀態…")
  attachGlobalHandlers()
  renderProviderList()
  renderPasswordLogin()

  try {
    const hasSession = await refreshSession()
    if (!hasSession) {
      showView("login")
      setStatus("請使用 Google 或 Email／密碼登入。", "hint")
      return
    }
    if (!hasAccess()) {
      showView("access")
      return
    }
    if (!canSeeNav(state.activeKey)) {
      state.activeKey = ENTRY_KEY
    }
    showView("app")
    await loadWorkspace()
    renderWorkspace()
  } catch (error) {
    console.error(error)
    showView("login")
    setStatus(error?.message || "無法確認登入狀態，請稍後再試。", "error")
  }
}

passwordLoginForm?.addEventListener("submit", async (event) => {
  event.preventDefault()
  const formData = new FormData(passwordLoginForm)
  const email = String(formData.get("email") || "").trim()
  const password = String(formData.get("password") || "")
  try {
    const session = await apiFetch("/auth/password", {
      method: "POST",
      auth: false,
      body: { email, password },
    })
    saveSession(session)
    if (!hasAccess()) {
      showView("access")
      return
    }
    showView("app")
    await loadWorkspace()
    renderWorkspace()
  } catch (error) {
    setStatus(error?.message || "登入失敗。", "error")
  }
})

void boot()
