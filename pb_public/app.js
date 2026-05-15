const API_BASE = `${window.location.origin}/api`
const AUTH_STORAGE_KEY = "coast-monitoring-auth"
const OAUTH_STORAGE_KEY = "coast-monitoring-oauth-provider"
const OAUTH_REDIRECT_PATH = "/oauth-callback.html"
const LOCAL_SUPERUSER_EMAIL = "hcchien@gmail.com"

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

const baseResourceConfigs = {
  users: {
    title: "Users",
    eyebrow: "People",
    description: "Google login users, their display name, and role.",
    collection: "users",
    expandFields: ["createUser", "updateUser"],
    tableColumns: [
      { key: "email", label: "Email" },
      { key: "name", label: "Name" },
      { key: "role", label: "Role" },
      { key: "createUser", label: "Create user" },
      { key: "updateUser", label: "Update user" },
      { key: "createDate", label: "Create date" },
      { key: "updateDate", label: "Update date" },
    ],
    editableFields: [
      {
        key: "name",
        label: "Name",
        type: "text",
        placeholder: "Display name",
      },
      {
        key: "role",
        label: "Role",
        type: "select",
        options: [
          { value: "admin", label: "admin" },
          { value: "volunteer", label: "volunteer" },
        ],
      },
    ],
    note:
      "Users are created automatically the first time they sign in with Google. This page is for role and profile maintenance.",
    canCreate: false,
  },
  location: {
    title: "Location",
    eyebrow: "Places",
    description: "Location names in Chinese and English.",
    collection: "location",
    expandFields: ["createUser", "updateUser"],
    tableColumns: [
      { key: "chineseName", label: "Chinese name" },
      { key: "englishName", label: "English name" },
      { key: "createUser", label: "Create user" },
      { key: "updateUser", label: "Update user" },
      { key: "createDate", label: "Create date" },
      { key: "updateDate", label: "Update date" },
    ],
    editableFields: [
      {
        key: "chineseName",
        label: "Chinese name",
        type: "text",
        placeholder: "中文名稱",
        required: true,
      },
      {
        key: "englishName",
        label: "English name",
        type: "text",
        placeholder: "English name",
        required: true,
      },
    ],
    note: "Use this table for bilingual location naming.",
    canCreate: true,
  },
  species: {
    title: "Species",
    eyebrow: "Biodiversity",
    description: "Species names in Chinese and English.",
    collection: "species",
    expandFields: ["createUser", "updateUser"],
    tableColumns: [
      { key: "chineseName", label: "Chinese name" },
      { key: "englishName", label: "English name" },
      { key: "createUser", label: "Create user" },
      { key: "updateUser", label: "Update user" },
      { key: "createDate", label: "Create date" },
      { key: "updateDate", label: "Update date" },
    ],
    editableFields: [
      {
        key: "chineseName",
        label: "Chinese name",
        type: "text",
        placeholder: "中文名稱",
        required: true,
      },
      {
        key: "englishName",
        label: "English name",
        type: "text",
        placeholder: "English name",
        required: true,
      },
    ],
    note:
      "Keep canonical species names here so other workflows can reference them cleanly.",
    canCreate: true,
  },
  audit_logs: {
    title: "Audit Logs",
    eyebrow: "Operations",
    description: "System-generated operation history.",
    collection: "audit_logs",
    expandFields: [],
    tableColumns: [
      { key: "loggedAt", label: "Logged at" },
      { key: "action", label: "Action" },
      { key: "targetCollection", label: "Collection" },
      { key: "targetRecordId", label: "Record id" },
      { key: "actorLabel", label: "Actor" },
      { key: "actorType", label: "Actor type" },
    ],
    editableFields: [],
    note: "Read-only trail of create, update, and delete operations.",
    canCreate: false,
    readOnly: true,
  },
  observation: {
    title: "Observation",
    eyebrow: "Field entry",
    description:
      "依日期與地點，紀錄每個物種觀測到的數量。submit 後會批次寫入 observation collection。",
    collection: "observation",
    expandFields: ["location", "species", "observer"],
    sortField: "date",
    tableColumns: [],
    editableFields: [],
    note: "由 admin、volunteer 與 superuser 共用的資料輸入頁。",
    canCreate: false,
    readOnly: true,
  },
}

const resourceConfigs = { ...baseResourceConfigs }

const EXCLUDED_COLLECTIONS = new Set([
  "_superusers",
  "_externalAuths",
  "_mfas",
  "_otps",
  "_authOrigins",
  "AuthOrigins",
  "event",
])

function toStartCase(value) {
  return String(value || "")
    .replaceAll(/[_-]+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase())
}

function isEditableField(field) {
  if (!field || field.hidden || field.system) {
    return false
  }
  if (["id", "password", "tokenKey"].includes(field.name)) {
    return false
  }
  return [
    "text",
    "number",
    "bool",
    "email",
    "url",
    "date",
    "select",
    "editor",
    "json",
  ].includes(field.type)
}

function fieldInputConfig(field) {
  const label = toStartCase(field.name)
  if (field.type === "select") {
    return {
      key: field.name,
      label,
      type: "select",
      required: Boolean(field.required),
      valueType: "string",
      options: (field.values || []).map((value) => ({
        value,
        label: String(value),
      })),
    }
  }

  if (field.type === "bool") {
    return {
      key: field.name,
      label,
      type: "checkbox",
      required: false,
      valueType: "boolean",
    }
  }

  if (field.type === "number") {
    return {
      key: field.name,
      label,
      type: "number",
      required: Boolean(field.required),
      valueType: "number",
      placeholder: label,
    }
  }

  if (field.type === "editor" || field.type === "json") {
    return {
      key: field.name,
      label,
      type: "textarea",
      required: Boolean(field.required),
      valueType: field.type === "json" ? "json" : "string",
      placeholder: label,
    }
  }

  return {
    key: field.name,
    label,
    type: field.type === "date" ? "date" : "text",
    required: Boolean(field.required),
    valueType: "string",
    placeholder: label,
  }
}

function tableColumnsFromFields(fields) {
  const preferred = fields
    .filter((field) => !field.hidden && !field.system)
    .slice(0, 5)
  const columns = preferred.map((field) => ({
    key: field.name,
    label: toStartCase(field.name),
  }))
  if (!columns.some((item) => item.key === "id")) {
    columns.push({ key: "id", label: "Id" })
  }
  return columns
}

function defaultSortFieldFromFields(fields) {
  const names = new Set(
    (Array.isArray(fields) ? fields : []).map((field) => field?.name),
  )
  if (names.has("updateDate")) {
    return "updateDate"
  }
  if (names.has("created")) {
    return "created"
  }
  if (names.has("loggedAt")) {
    return "loggedAt"
  }
  return "id"
}

function configFromCollection(collection) {
  const fields = Array.isArray(collection?.fields) ? collection.fields : []
  const editable = fields.filter(isEditableField).map(fieldInputConfig)
  return {
    title: toStartCase(collection.name),
    eyebrow: collection.type === "auth" ? "Auth" : "Collection",
    description: `Manage ${collection.name} records.`,
    collection: collection.name,
    tableColumns: tableColumnsFromFields(fields),
    editableFields: editable,
    sortField: defaultSortFieldFromFields(fields),
    note:
      editable.length > 0
        ? "Auto-generated from PocketBase schema."
        : "This collection has no editable primitive fields in the generated UI.",
    canCreate: editable.length > 0,
  }
}

const OVERVIEW_KEY = "overview"
const OBSERVATION_KEY = "observation"

const state = {
  activeKey: OBSERVATION_KEY,
  records: {},
  draft: null,
  authMethods: null,
  auth: null,
  collectionFields: {},
  history: {},
  selectedIds: {},
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
  loginView.classList.toggle("hidden", mode !== "login")
  accessView.classList.toggle("hidden", mode !== "access")
  appView.classList.toggle("hidden", mode !== "app")
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

function clearWorkspaceBanner() {
  if (!workspaceStatus) {
    return
  }
  workspaceStatus.textContent = ""
  workspaceStatus.classList.add("hidden")
  workspaceStatus.classList.remove("is-error", "is-hint")
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

const ALERT_ICONS = {
  info: "i",
  ok: "✓",
  caution: "!",
  warning: "!",
  danger: "×",
}

function showAlert({ tone = "info", title, message, id, dismissible = true } = {}) {
  if (!alertRegion) {
    return null
  }
  const alertId =
    id || `alert-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

  if (id) {
    dismissAlert(id)
  }

  const node = document.createElement("div")
  node.className = `alert tone-${tone}`
  node.dataset.alertId = alertId
  node.innerHTML = `
    <span class="alert-icon" aria-hidden="true">${ALERT_ICONS[tone] || "i"}</span>
    <div class="alert-body">
      ${title ? `<div class="alert-title">${escapeHtml(title)}</div>` : ""}
      ${message ? `<div class="alert-message">${escapeHtml(message)}</div>` : ""}
    </div>
    ${dismissible ? `<button type="button" class="alert-close" aria-label="Dismiss">×</button>` : ""}
  `

  if (dismissible) {
    node
      .querySelector(".alert-close")
      ?.addEventListener("click", () => dismissAlert(alertId))
  }

  alertRegion.appendChild(node)
  return alertId
}

function dismissAlert(id) {
  if (!alertRegion || !id) {
    return
  }
  const node = alertRegion.querySelector(`[data-alert-id="${id}"]`)
  if (node) {
    node.remove()
  }
}

function clearAlerts() {
  if (alertRegion) {
    alertRegion.innerHTML = ""
  }
}

function readAuth() {
  try {
    return JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY) || "null")
  } catch (_) {
    return null
  }
}

function saveAuth(auth) {
  state.auth = auth
  localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth))
}

function normalizeAuthResponse(data, fallbackCollection) {
  if (!data?.record || !fallbackCollection) {
    return data
  }
  return {
    ...data,
    record: {
      ...data.record,
      collectionName: data.record.collectionName || fallbackCollection,
    },
  }
}

function clearAuth() {
  state.auth = null
  localStorage.removeItem(AUTH_STORAGE_KEY)
  localStorage.removeItem(OAUTH_STORAGE_KEY)
}

function currentUser() {
  return state.auth?.record || null
}

function isAdmin() {
  const record = currentUser()
  return record?.role === "admin" || record?.collectionName === "_superusers"
}

function isSuperuser() {
  return currentUser()?.collectionName === "_superusers"
}

function isVolunteer() {
  return currentUser()?.role === "volunteer"
}

function hasAccess() {
  if (!currentUser()) {
    return false
  }
  return isSuperuser() || isAdmin() || isVolunteer()
}

function canSeeNav(key) {
  if (isSuperuser()) {
    return true
  }
  if (isAdmin()) {
    return key !== "users"
  }
  return key === OBSERVATION_KEY
}

function canLoadCollection(key) {
  if (isSuperuser()) {
    return true
  }
  if (isAdmin()) {
    return true
  }
  // Volunteer: only the data needed for the observation page.
  return ["location", "species", "observation"].includes(key)
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

function selectedCount(resourceKey) {
  return selectionSet(resourceKey).size
}

function selectedIds(resourceKey) {
  return [...selectionSet(resourceKey)]
}

function clearSelection(resourceKey) {
  selectionSet(resourceKey).clear()
}

function invalidateHistory(resourceKey, recordId) {
  if (!recordId) {
    return
  }
  delete state.history[historyCacheKey(resourceKey, recordId)]
}

function setSelected(resourceKey, id, checked) {
  const set = selectionSet(resourceKey)
  if (checked) {
    set.add(id)
  } else {
    set.delete(id)
  }
}

function isSelected(resourceKey, id) {
  return selectionSet(resourceKey).has(id)
}

function setPageSelection(resourceKey, ids, checked) {
  const set = selectionSet(resourceKey)
  ids.forEach((id) => {
    if (checked) {
      set.add(id)
    } else {
      set.delete(id)
    }
  })
}

function selectedLabelForRecord(resourceKey, record) {
  if (!record) {
    return ""
  }

  if (resourceKey === "users") {
    return record.email || record.name || record.id
  }

  return record.chineseName || record.englishName || record.id
}

function batchDeleteTitle(resourceKey) {
  return resourceKey === "users" ? "users" : resourceKey
}

function historyCacheKey(collectionName, recordId) {
  return `${collectionName}:${recordId}`
}

function historyFields(entry, side) {
  return entry?.[side]?.fields || {}
}

function formatHistoryValue(value) {
  if (value === null || value === undefined || value === "") {
    return "—"
  }
  if (Array.isArray(value)) {
    return value.length
      ? value.map((item) => formatHistoryValue(item)).join(", ")
      : "—"
  }
  if (typeof value === "object") {
    return JSON.stringify(value)
  }
  return String(value)
}

function diffHistoryFields(beforeFields, afterFields) {
  const ignored = new Set(["id", "createDate", "updateDate"])
  const keys = new Set([
    ...Object.keys(beforeFields || {}),
    ...Object.keys(afterFields || {}),
  ])
  const diffs = []

  keys.forEach((key) => {
    if (ignored.has(key)) {
      return
    }
    const beforeValue = beforeFields?.[key]
    const afterValue = afterFields?.[key]
    if (
      JSON.stringify(beforeValue ?? null) === JSON.stringify(afterValue ?? null)
    ) {
      return
    }
    diffs.push({ key, beforeValue, afterValue })
  })

  return diffs
}

function summarizeHistoryAction(entry) {
  if (!entry) {
    return ""
  }
  if (entry.action === "create") {
    return "Created record"
  }
  if (entry.action === "delete") {
    return "Deleted record"
  }
  const diffs = diffHistoryFields(
    historyFields(entry, "before"),
    historyFields(entry, "after"),
  )
  if (!diffs.length) {
    return "Updated record"
  }
  const keys = diffs.slice(0, 3).map((diff) => diff.key)
  const suffix = diffs.length > 3 ? ` +${diffs.length - 3} more` : ""
  return `Updated ${keys.join(", ")}${suffix}`
}

async function loadRecordHistory(collectionName, recordId, force = false) {
  if (!collectionName || !recordId) {
    return []
  }

  const key = historyCacheKey(collectionName, recordId)
  if (!force && Array.isArray(state.history[key])) {
    return state.history[key]
  }

  try {
    const filter = encodeURIComponent(
      `targetCollection = '${collectionName}' && targetRecordId = '${recordId}'`,
    )
    const data = await apiFetch(
      `/collections/audit_logs/records?page=1&perPage=50&sort=-loggedAt&filter=${filter}`,
    )
    const items = Array.isArray(data.items) ? data.items : []
    state.history[key] = items
    return items
  } catch (error) {
    state.history[key] = []
    throw error
  }
}

function renderHistoryEntry(entry) {
  const timestamp = entry.loggedAt || entry.created || ""
  const actor = entry.actorLabel || entry.actorType || "anonymous"
  const diffs = diffHistoryFields(
    historyFields(entry, "before"),
    historyFields(entry, "after"),
  )
  const diffBlock =
    entry.action === "update" && diffs.length
      ? `
        <div class="history-diff-list">
          ${diffs
            .map(
              (diff) => `
                <div class="history-diff">
                  <div class="history-diff-field">${escapeHtml(diff.key)}</div>
                  <div class="history-diff-values">
                    <span>Before: ${escapeHtml(formatHistoryValue(diff.beforeValue))}</span>
                    <span>After: ${escapeHtml(formatHistoryValue(diff.afterValue))}</span>
                  </div>
                </div>
              `,
            )
            .join("")}
        </div>
      `
      : ""

  return `
    <article class="history-entry">
      <div class="history-entry-top">
        <div>
          <strong>${escapeHtml(entry.action || "event")}</strong>
          <p>${escapeHtml(summarizeHistoryAction(entry))}</p>
        </div>
        <div class="history-meta">
          <span>${escapeHtml(actor)}</span>
          <span>${escapeHtml(timestamp)}</span>
        </div>
      </div>
      ${diffBlock}
    </article>
  `
}

function renderRecordHistory(resourceKey) {
  const draft = state.draft
  if (!draft || resourceKey === "audit_logs") {
    return ""
  }

  const key = historyCacheKey(resourceKey, draft.id)
  const entries = state.history[key] || []

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
          ? `<div class="history-list">${entries.map((entry) => renderHistoryEntry(entry)).join("")}</div>`
          : `<div class="empty-state"><p>No history for this record yet.</p></div>`
      }
    </section>
  `
}

function authToken() {
  return state.auth?.token || ""
}

async function apiFetch(
  path,
  { method = "GET", body, auth = true, headers = {} } = {},
) {
  const init = {
    method,
    headers: {
      Accept: "application/json",
      ...headers,
    },
  }

  if (auth && authToken()) {
    init.headers.Authorization = authToken()
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
      typeof data === "object" && data && data.message
        ? data.message
        : `Request failed (${response.status})`
    throw new Error(message)
  }

  return data
}

async function loadAuthMethods() {
  state.authMethods = await apiFetch("/collections/users/auth-methods", {
    auth: false,
  })
}

async function loadDynamicCollections() {
  let collections = []
  try {
    const data = await apiFetch("/collections?page=1&perPage=200&sort=name")
    collections = data?.items || []
  } catch (_) {
    return
  }

  collections.forEach((collection) => {
    if (
      !collection?.name ||
      collection.system ||
      EXCLUDED_COLLECTIONS.has(collection.name)
    ) {
      return
    }

    if (resourceConfigs[collection.name]) {
      state.collectionFields[collection.name] = Array.isArray(collection.fields)
        ? collection.fields
        : []
      return
    }

    resourceConfigs[collection.name] = configFromCollection(collection)
    state.collectionFields[collection.name] = Array.isArray(collection.fields)
      ? collection.fields
      : []
  })
}

function providerItems() {
  return (
    state.authMethods?.oauth2?.providers ||
    state.authMethods?.authProviders ||
    []
  )
}

function getProviderLabel(provider) {
  if (!provider) {
    return "Google"
  }
  return (
    provider.displayName ||
    (provider.name === "google" ? "Google" : provider.name)
  )
}

function beginOAuth(provider) {
  localStorage.setItem(OAUTH_STORAGE_KEY, JSON.stringify(provider))
  const redirectURL = `${window.location.origin}${OAUTH_REDIRECT_PATH}`
  window.location.href = `${provider.authURL}${redirectURL}`
}

function renderProviderList(providers) {
  providerList.innerHTML = ""

  if (!providers.length) {
    providerList.innerHTML = `
      <div class="empty-state">
        <p>目前沒有可用的 OAuth2 provider。</p>
        <p class="meta-line">請先到 PocketBase 的 users collection 設定 Google OAuth2 provider。</p>
      </div>
    `
    return
  }

  providers.forEach((provider) => {
    const button = document.createElement("button")
    button.type = "button"
    button.className = "provider-button"
    button.innerHTML = `<span class="session-pill">${escapeHtml(
      getProviderLabel(provider).slice(0, 1).toUpperCase(),
    )}</span><span>Sign in with ${escapeHtml(getProviderLabel(provider))}</span>`
    button.addEventListener("click", () => {
      setStatus("正在開啟 Google 登入…", "hint")
      beginOAuth(provider)
    })
    providerList.appendChild(button)
  })
}

function renderPasswordLogin(enabled) {
  if (!passwordLoginForm) {
    return
  }

  passwordLoginForm.classList.toggle("hidden", !enabled)
  if (enabled) {
    const emailInput = passwordLoginForm.querySelector("#dev-email")
    if (emailInput && !emailInput.value) {
      emailInput.value = LOCAL_SUPERUSER_EMAIL
    }
  }
}

async function ensureSession() {
  if (!authToken()) {
    clearAuth()
    return false
  }

  const preferred = currentUser()?.collectionName
  const candidates = [
    ...new Set([
      ...(preferred ? [preferred] : []),
      "_superusers",
      "users",
    ]),
  ]

  for (const targetCollection of candidates) {
    try {
      const data = await apiFetch(
        `/collections/${targetCollection}/auth-refresh`,
        { method: "POST" },
      )
      saveAuth(normalizeAuthResponse(data, targetCollection))
      return true
    } catch (_) {
      // try next candidate
    }
  }

  clearAuth()
  return false
}

async function authWithPassword(targetCollection, identity, password) {
  return apiFetch(`/collections/${targetCollection}/auth-with-password`, {
    method: "POST",
    auth: false,
    body: { identity, password },
  })
}

async function authWithPasswordAuto(identity, password) {
  const attempts = ["_superusers", "users"]
  let lastError = null
  for (const targetCollection of attempts) {
    try {
      const data = await authWithPassword(targetCollection, identity, password)
      return normalizeAuthResponse(data, targetCollection)
    } catch (error) {
      lastError = error
    }
  }
  throw lastError || new Error("Unable to sign in.")
}

function renderSessionBadges() {
  const record = currentUser()
  const badges = []

  if (record) {
    badges.push(
      `<span class="session-pill">role: ${escapeHtml(
        record.collectionName === "_superusers"
          ? "superuser"
          : record.role || "unknown",
      )}</span>`,
    )
    badges.push(
      `<span class="session-pill">${escapeHtml(record.email || "-")}</span>`,
    )
  }

  sessionBadges.innerHTML = badges.join("")
}

function renderSidebar() {
  resourceNav.innerHTML = ""

  const navItems = [
    { key: OBSERVATION_KEY, title: "Observation", count: null },
    { key: OVERVIEW_KEY, title: "Overview", count: null },
    ...Object.entries(resourceConfigs)
      .filter(([key]) => key !== OBSERVATION_KEY)
      .map(([key, config]) => ({
        key,
        title: config.title,
        count: state.records[key]?.length ?? 0,
      })),
  ].filter(({ key }) => canSeeNav(key))

  navItems.forEach(({ key, title, count }) => {
    const button = document.createElement("button")
    button.type = "button"
    button.className = `nav-button ${state.activeKey === key ? "active" : ""}`
    const countHtml = count !== null ? `<span>${count}</span>` : ""
    button.innerHTML = `<span>${escapeHtml(title)}</span>${countHtml}`
    button.addEventListener("click", () => {
      if (state.activeKey === key) {
        closeSidebar()
        return
      }
      state.activeKey = key
      state.draft = null
      closeDrawer({ rerender: false })
      closeSidebar()
      triggerNavTransition()
    })
    resourceNav.appendChild(button)
  })
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

function valueForRecord(record, key) {
  const value = record?.[key]

  if (value == null || value === "") {
    return ""
  }

  if (
    key === "createDate" ||
    key === "updateDate" ||
    key === "loggedAt" ||
    key === "created" ||
    key === "updated"
  ) {
    const date = new Date(value)
    if (!Number.isNaN(date.getTime())) {
      return date.toLocaleString("zh-TW", {
        dateStyle: "short",
        timeStyle: "short",
      })
    }
  }

  if (key === "createUser" || key === "updateUser") {
    const expanded = record?.expand?.[key]
    if (expanded) {
      return expanded.name || expanded.email || expanded.id || value
    }
  }

  return value
}

function parseFieldValue(field, rawValue) {
  if (field.type === "checkbox") {
    return Boolean(rawValue)
  }

  if (field.valueType === "number") {
    if (rawValue === "" || rawValue == null) {
      return null
    }
    const parsed = Number(rawValue)
    return Number.isFinite(parsed) ? parsed : rawValue
  }

  if (field.valueType === "json") {
    if (!rawValue) {
      return null
    }
    try {
      return JSON.parse(rawValue)
    } catch (_) {
      throw new Error(`${field.label} 需要有效的 JSON 格式。`)
    }
  }

  return rawValue
}

function payloadFromEditorForm(editorForm, config) {
  const formData = new FormData(editorForm)
  const payload = {}

  config.editableFields.forEach((field) => {
    if (field.type === "checkbox") {
      payload[field.key] =
        editorForm.querySelector(`[name="${field.key}"]`)?.checked || false
      return
    }
    const raw = formData.get(field.key)
    payload[field.key] = parseFieldValue(field, raw)
  })

  return payload
}

function renderTableRows(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const rows = state.records[resourceKey] || []

  if (!rows.length) {
    return `<div class="empty-state"><p>目前還沒有任何資料。</p></div>`
  }

  return rows
    .map((record) => {
      const cells = config.tableColumns
        .map((column) => {
          const rendered = valueForRecord(record, column.key)
          return `<div>${rendered ? escapeHtml(rendered) : "—"}</div>`
        })
        .join("")
      const selectionCell = supportsBatchDelete(resourceKey)
        ? `<div class="cell-select"><input type="checkbox" class="row-select" data-action="select-row" data-id="${escapeHtml(record.id)}" ${
            isSelected(resourceKey, record.id) ? "checked" : ""
          } aria-label="Select row ${escapeHtml(record.id)}" /></div>`
        : ""

      if (config.readOnly) {
        return `
          <div class="table-row" data-action="open" data-id="${escapeHtml(record.id)}" style="grid-template-columns: repeat(${config.tableColumns.length}, minmax(0, 1fr));">
            ${cells}
          </div>
        `
      }

      const isRowSelected = isSelected(resourceKey, record.id)
      return `
        <div class="table-row ${isRowSelected ? "is-selected" : ""}" data-action="open" data-id="${escapeHtml(record.id)}" style="grid-template-columns: ${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto;">
          ${selectionCell}
          ${cells}
          <div class="cell-actions">
            <button type="button" class="tiny-button" data-action="edit" data-id="${escapeHtml(record.id)}">Edit</button>
            <button type="button" class="tiny-button danger-tiny" data-action="delete" data-id="${escapeHtml(record.id)}">Delete</button>
          </div>
        </div>
      `
    })
    .join("")
}

function renderEditor(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const draft = state.draft
  const isUsers = resourceKey === "users"

  if (config.readOnly || config.editableFields.length === 0) {
    return `
      <div class="editor-card">
        <div class="empty-state">
          <p>This collection is read-only in the custom admin UI.</p>
          <p class="meta-line">${escapeHtml(config.note)}</p>
        </div>
      </div>
    `
  }

  const submitDisabled = isUsers && !draft
  const submitLabel = draft
    ? "Save changes"
    : isUsers
      ? "Select a user"
      : "Create"

  const fields = config.editableFields
    .map((field) => {
      const currentValue = draft ? draft[field.key] ?? "" : ""
      const control =
        field.type === "textarea"
          ? `<textarea id="${field.key}" name="${field.key}" placeholder="${escapeHtml(field.placeholder || "")}" ${field.required ? "required" : ""}>${escapeHtml(currentValue)}</textarea>`
          : field.type === "select"
            ? `<select id="${field.key}" name="${field.key}" ${field.required ? "required" : ""}>${field.options
                .map(
                  (option) =>
                    `<option value="${escapeHtml(option.value)}" ${currentValue === option.value ? "selected" : ""}>${escapeHtml(option.label)}</option>`,
                )
                .join("")}</select>`
            : field.type === "checkbox"
              ? `<input id="${field.key}" name="${field.key}" type="checkbox" ${currentValue ? "checked" : ""} />`
              : `<input id="${field.key}" name="${field.key}" type="${field.type === "number" ? "number" : "text"}" value="${escapeHtml(currentValue)}" placeholder="${escapeHtml(field.placeholder || "")}" ${field.required ? "required" : ""} />`

      return `
        <div class="form-field">
          <label for="${field.key}">${escapeHtml(field.label)}${field.required ? " *" : ""}</label>
          ${control}
        </div>
      `
    })
    .join("")

  return `
    <div class="editor-card">
      <form class="editor-form" id="editor-form">
        ${isUsers ? `<div class="form-help">Google login 帳號會自動建立 user record，這裡主要用來修正名稱與 role。</div>` : ""}
        ${fields}
        <div class="editor-actions">
          <button type="button" class="ghost-button" id="cancel-editor-button">Cancel</button>
          <button type="submit" class="primary-button" ${submitDisabled ? "disabled" : ""}>${submitLabel}</button>
        </div>
      </form>
    </div>
  `
}

function formatShortDate(value) {
  if (!value) {
    return "—"
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return "—"
  }
  return date.toLocaleDateString("zh-TW", {
    month: "2-digit",
    day: "2-digit",
  })
}

function avatarInitial(resourceKey, draft) {
  if (resourceKey === "users") {
    const raw = String(draft?.name || draft?.email || "?").trim()
    return raw.slice(0, 1).toUpperCase()
  }
  if (resourceKey === "location") {
    return "L"
  }
  if (resourceKey === "species") {
    return "S"
  }
  return "?"
}

function userRoleTone(role) {
  if (role === "admin") {
    return "tone-primary"
  }
  if (role === "volunteer") {
    return ""
  }
  if (role === "superuser") {
    return "tone-accent"
  }
  return ""
}

function renderDrawerHero(resourceKey, draft) {
  if (!draft) {
    return ""
  }

  const initial = avatarInitial(resourceKey, draft)
  let title = ""
  let subtitle = ""
  let metaPills = []

  if (resourceKey === "users") {
    title = draft.name || draft.email || "User"
    subtitle = draft.email && draft.name ? draft.email : ""
    const role = draft.role || "unknown"
    metaPills.push(
      `<span class="pill ${userRoleTone(role)}">role · ${escapeHtml(role)}</span>`,
    )
    if (draft.updateDate) {
      metaPills.push(
        `<span class="pill">updated ${escapeHtml(formatShortDate(draft.updateDate))}</span>`,
      )
    }
  } else if (resourceKey === "location") {
    title = draft.chineseName || draft.englishName || "Location"
    subtitle = draft.englishName && draft.chineseName ? draft.englishName : ""
    if (draft.updateDate) {
      metaPills.push(
        `<span class="pill tone-primary">updated ${escapeHtml(formatShortDate(draft.updateDate))}</span>`,
      )
    }
  } else if (resourceKey === "species") {
    title = draft.chineseName || draft.englishName || "Species"
    subtitle = draft.englishName && draft.chineseName ? draft.englishName : ""
    metaPills.push(
      `<span class="pill tone-ok">conservation · NE</span>`,
    )
    if (draft.updateDate) {
      metaPills.push(
        `<span class="pill">updated ${escapeHtml(formatShortDate(draft.updateDate))}</span>`,
      )
    }
  } else {
    title = draft.id
  }

  return `
    <div class="drawer-hero">
      <div class="drawer-hero-avatar tone-${escapeHtml(resourceKey)}">${escapeHtml(initial)}</div>
      <div class="drawer-hero-text">
        <h3>${escapeHtml(title)}</h3>
        ${subtitle ? `<p>${escapeHtml(subtitle)}</p>` : ""}
        ${metaPills.length ? `<div class="drawer-hero-meta">${metaPills.join("")}</div>` : ""}
      </div>
    </div>
  `
}

function renderDrawerMiniStats(draft) {
  if (!draft) {
    return ""
  }

  const created = formatShortDate(draft.createDate || draft.created)
  const updated = formatShortDate(draft.updateDate || draft.updated)
  const recordIdShort = String(draft.id || "").slice(-6) || "—"

  return `
    <div class="drawer-section">
      <span class="section-title">Snapshot</span>
      <div class="mini-stats">
        <div class="mini-stat">
          <span class="mini-num mini-num-date">${escapeHtml(created)}</span>
          <span class="mini-label">Created</span>
        </div>
        <div class="mini-stat">
          <span class="mini-num mini-num-date">${escapeHtml(updated)}</span>
          <span class="mini-label">Updated</span>
        </div>
        <div class="mini-stat">
          <span class="mini-num mini-num-text">${escapeHtml(recordIdShort)}</span>
          <span class="mini-label">Record id</span>
        </div>
      </div>
    </div>
  `
}

const IUCN_STEPS = [
  { code: "LC", name: "Least", tone: "tone-ok" },
  { code: "NT", name: "Near", tone: "tone-caution" },
  { code: "VU", name: "Vulnerable", tone: "tone-warning" },
  { code: "EN", name: "Endangered", tone: "tone-warning" },
  { code: "CR", name: "Critical", tone: "tone-danger" },
]

function renderIucnScale(draft) {
  if (!draft) {
    return ""
  }
  const activeCode = (draft.iucn || "").toUpperCase()
  const footer = activeCode
    ? `IUCN status · ${escapeHtml(activeCode)}`
    : "目前 schema 沒有 IUCN 欄位，預設顯示「未評估 / NE」。"

  const steps = IUCN_STEPS.map((step) => {
    const isActive = step.code === activeCode
    return `
      <div class="iucn-step ${step.tone} ${isActive ? "is-active" : ""}">
        <span class="iucn-code">${step.code}</span>
        <span class="iucn-name">${step.name}</span>
      </div>
    `
  }).join("")

  return `
    <div class="drawer-section">
      <span class="section-title">Conservation status (IUCN)</span>
      <div class="iucn-scale">${steps}</div>
      <p class="iucn-footer">${footer}</p>
    </div>
  `
}

function renderDrawerBody() {
  const resourceKey = state.activeKey
  const config = resourceConfigs[resourceKey]
  const draft = state.draft
  const isUsers = resourceKey === "users"

  drawerEyebrow.textContent = config.eyebrow
  let titleText
  if (!draft) {
    titleText = `New ${config.title}`
  } else if (isUsers) {
    titleText = "User detail"
  } else {
    titleText = `Edit ${config.title}`
  }
  drawerTitle.textContent = titleText

  drawerSubtitle.textContent = draft
    ? `Record id: ${draft.id}`
    : config.note || ""

  const heroBlock = renderDrawerHero(resourceKey, draft)
  const miniStatsBlock = renderDrawerMiniStats(draft)
  const iucnBlock = resourceKey === "species" ? renderIucnScale(draft) : ""

  drawerBody.innerHTML = `
    ${heroBlock}
    ${miniStatsBlock}
    ${iucnBlock}
    ${renderEditor(resourceKey)}
    ${renderRecordHistory(resourceKey)}
  `

  const editorForm = drawerBody.querySelector("#editor-form")
  const cancelButton = drawerBody.querySelector("#cancel-editor-button")

  if (cancelButton) {
    cancelButton.addEventListener("click", () => closeDrawer())
  }

  if (editorForm) {
    editorForm.addEventListener("submit", (event) =>
      handleEditorSubmit(event, resourceKey),
    )
  }
}

async function handleEditorSubmit(event, resourceKey) {
  event.preventDefault()
  const config = resourceConfigs[resourceKey]
  const editorForm = event.target
  const authUser = currentUser()
  let payload = {}

  try {
    payload = payloadFromEditorForm(editorForm, config)

    if (resourceKey === "users") {
      const id = state.draft?.id
      if (!id) {
        showAlert({
          tone: "danger",
          title: "無法新增",
          message:
            "使用者帳號是由 Google 登入自動建立的，無法在此新增。",
          id: "editor",
        })
        return
      }
      if (authUser?.collectionName === "users") {
        payload.updateUser = authUser.id
      }
      const updated = await apiFetch(`/collections/users/records/${id}`, {
        method: "PATCH",
        body: payload,
      })
      state.draft = updated
      invalidateHistory(resourceKey, id)
      await loadRecordHistory(resourceKey, id, true).catch((error) => {
        console.warn("history refresh failed", error)
      })
    } else if (state.draft?.id) {
      if (authUser?.collectionName === "users") {
        payload.updateUser = authUser.id
      }
      const recordId = state.draft.id
      const updated = await apiFetch(
        `/collections/${config.collection}/records/${recordId}`,
        { method: "PATCH", body: payload },
      )
      state.draft = updated
      invalidateHistory(resourceKey, recordId)
      await loadRecordHistory(resourceKey, recordId, true).catch((error) => {
        console.warn("history refresh failed", error)
      })
    } else {
      if (authUser?.collectionName === "users") {
        payload.createUser = authUser.id
      }
      const created = await apiFetch(
        `/collections/${config.collection}/records`,
        { method: "POST", body: payload },
      )
      invalidateHistory(resourceKey, created?.id)
      state.draft = created
      await loadRecordHistory(resourceKey, created?.id, true).catch((error) => {
        console.warn("history refresh failed", error)
      })
    }

    await loadResource(resourceKey)
    dismissAlert("editor")
    showAlert({
      tone: "ok",
      title: "已儲存",
      message: `${config.title} 的變更已寫入 PocketBase。`,
      id: "editor",
    })
    renderWorkspace({ keepDrawer: true })
    renderDrawerBody()
  } catch (error) {
    showAlert({
      tone: "danger",
      title: "無法儲存",
      message: error?.message || "PocketBase 回傳錯誤。",
      id: "editor",
    })
  }
}

function renderTableSkeleton(resourceKey, columnCount) {
  const skeletonRows = 6
  const rowTemplate = supportsBatchDelete(resourceKey)
    ? `40px repeat(${columnCount}, minmax(0, 1fr)) auto`
    : `repeat(${columnCount}, minmax(0, 1fr))`

  const cells = Array.from({ length: columnCount })
    .map(() => `<div><span class="skel"></span></div>`)
    .join("")

  const checkCell = supportsBatchDelete(resourceKey)
    ? `<div><span class="skel" style="width: 18px; height: 18px; border-radius: 5px;"></span></div>`
    : ""
  const actionCell = supportsBatchDelete(resourceKey)
    ? `<div style="text-align:right"><span class="skel" style="width: 80px;"></span></div>`
    : ""

  return Array.from({ length: skeletonRows })
    .map(
      () => `
      <div class="table-row skel-table-row" style="grid-template-columns: ${rowTemplate}; cursor: default;">
        ${checkCell}${cells}${actionCell}
      </div>
    `,
    )
    .join("")
}

function renderErrorState(detail = "") {
  return `
    <div class="error-state">
      <div class="error-state-icon" aria-hidden="true">×</div>
      <h3>無法載入資料</h3>
      <p>後端連線似乎出了問題。可以等一下後重試，或直接到 PocketBase dashboard 確認服務狀態。</p>
      ${
        detail
          ? `<details class="error-state-details"><summary>技術細節</summary><code>${escapeHtml(detail)}</code></details>`
          : ""
      }
      <button type="button" class="primary-button" id="error-retry-button">重新載入</button>
    </div>
  `
}

const STAT_ICONS = {
  users: "U",
  location: "L",
  species: "S",
  audit_logs: "A",
}

function actionToneAndGlyph(action) {
  if (action === "create") {
    return { tone: "tone-ok", glyph: "+" }
  }
  if (action === "delete") {
    return { tone: "tone-danger", glyph: "×" }
  }
  if (action === "update") {
    return { tone: "tone-accent", glyph: "✎" }
  }
  return { tone: "tone-neutral", glyph: "•" }
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
  const minutes = Math.floor(diff / 60000)
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
  const days = Math.floor(hours / 24)
  if (days < 7) {
    return `${days}d ago`
  }
  return date.toLocaleDateString("zh-TW", {
    month: "2-digit",
    day: "2-digit",
  })
}

function dashboardCollectionKeys() {
  return ["users", "location", "species", "audit_logs"].filter(
    (key) => resourceConfigs[key],
  )
}

function lastUpdatedFor(resourceKey) {
  const records = state.records[resourceKey] || []
  if (!records.length) {
    return null
  }
  const sorted = [...records].sort((a, b) => {
    const ba = new Date(
      b.updateDate || b.loggedAt || b.updated || 0,
    ).getTime()
    const ab = new Date(
      a.updateDate || a.loggedAt || a.updated || 0,
    ).getTime()
    return ba - ab
  })
  return (
    sorted[0]?.updateDate || sorted[0]?.loggedAt || sorted[0]?.updated || null
  )
}

function renderStatCard(resourceKey, { loading = false } = {}) {
  const config = resourceConfigs[resourceKey]
  if (!config) {
    return ""
  }
  const count = state.records[resourceKey]?.length ?? 0
  const lastUpdated = lastUpdatedFor(resourceKey)
  const sub = lastUpdated
    ? `最近異動 ${formatRelativeTime(lastUpdated)}`
    : "尚無紀錄"
  const icon = STAT_ICONS[resourceKey] || config.title.slice(0, 1).toUpperCase()

  if (loading) {
    return `
      <div class="stat-card" aria-busy="true" style="cursor: default;">
        <div class="stat-head">
          <div class="stat-icon tone-${escapeHtml(resourceKey)}">${escapeHtml(icon)}</div>
          <span class="stat-label">${escapeHtml(config.title)}</span>
        </div>
        <div class="stat-value"><span class="skel" style="width: 56px; height: 28px;"></span></div>
        <div class="stat-sub"><span class="skel" style="width: 120px;"></span></div>
      </div>
    `
  }

  return `
    <button type="button" class="stat-card" data-jump="${escapeHtml(resourceKey)}">
      <div class="stat-head">
        <div class="stat-icon tone-${escapeHtml(resourceKey)}">${escapeHtml(icon)}</div>
        <span class="stat-label">${escapeHtml(config.title)}</span>
      </div>
      <div class="stat-value">${count}</div>
      <div class="stat-sub">${escapeHtml(sub)}</div>
    </button>
  `
}

function renderAuditMiniSkeleton(rows = 5) {
  return Array.from({ length: rows })
    .map(
      () => `
      <article class="audit-mini-row">
        <div class="audit-mini-pin tone-neutral" aria-hidden="true"><span class="skel" style="width: 14px; height: 14px; border-radius: 50%;"></span></div>
        <div class="audit-mini-text">
          <strong><span class="skel" style="width: 60%;"></span></strong>
          <p><span class="skel" style="width: 40%;"></span></p>
        </div>
        <time class="audit-mini-time"><span class="skel" style="width: 48px;"></span></time>
      </article>
    `,
    )
    .join("")
}

function recentAuditEntries(limit = 8) {
  const entries = state.records.audit_logs || []
  const sorted = [...entries].sort((a, b) => {
    const ta = new Date(a.loggedAt || a.created || 0).getTime()
    const tb = new Date(b.loggedAt || b.created || 0).getTime()
    return tb - ta
  })
  return sorted.slice(0, limit)
}

function describeAuditTarget(entry) {
  const collection = entry.targetCollection || "record"
  const id = entry.targetRecordId
    ? String(entry.targetRecordId).slice(-6)
    : "—"
  return `${collection} · ${id}`
}

function renderAuditMiniRow(entry) {
  const { tone, glyph } = actionToneAndGlyph(entry.action)
  const action = entry.action || "event"
  const actor = entry.actorLabel || entry.actorType || "system"
  const summary = describeAuditTarget(entry)
  const when = formatRelativeTime(entry.loggedAt || entry.created)

  return `
    <article class="audit-mini-row">
      <div class="audit-mini-pin ${tone}" aria-hidden="true">${glyph}</div>
      <div class="audit-mini-text">
        <strong>${escapeHtml(toStartCase(action))} ${escapeHtml(summary)}</strong>
        <p>by ${escapeHtml(actor)}</p>
      </div>
      <time class="audit-mini-time">${escapeHtml(when)}</time>
    </article>
  `
}

function renderDashboard() {
  sectionEyebrow.textContent = "Coast Monitoring"
  sectionTitle.textContent = "Overview"
  sectionDescription.textContent =
    "監測系統的高層次總覽 — 即時資料量與最近異動。"
  if (mobileSectionTitle) {
    mobileSectionTitle.textContent = "Overview"
  }

  if (state.demoError) {
    resourcePanel.innerHTML = renderErrorState(
      "GET /api/dashboard — Network request failed (simulated)",
    )
    const retryButton = $("#error-retry-button")
    if (retryButton) {
      retryButton.addEventListener("click", () => {
        state.demoError = false
        triggerNavTransition()
      })
    }
    return
  }

  const showSkeleton = state.loading || state.demoLoading
  const statCards = dashboardCollectionKeys()
    .map((key) => renderStatCard(key, { loading: showSkeleton }))
    .join("")
  const recent = showSkeleton ? [] : recentAuditEntries(8)

  const auditPanel = `
    <section class="dash-panel">
      <header class="dash-panel-head">
        <div>
          <h2>最近異動</h2>
          <p>${showSkeleton ? "正在載入…" : `audit log 的最後 ${recent.length || 0} 筆紀錄`}</p>
        </div>
        ${showSkeleton ? "" : `<button type="button" class="ghost-button" data-jump="audit_logs">View all</button>`}
      </header>
      ${
        showSkeleton
          ? `<div class="audit-mini-list">${renderAuditMiniSkeleton()}</div>`
          : recent.length
            ? `<div class="audit-mini-list">${recent.map(renderAuditMiniRow).join("")}</div>`
            : `<div class="empty-state"><p>目前還沒有任何 audit log。</p></div>`
      }
    </section>
  `

  const quickPanel = `
    <section class="dash-panel">
      <header class="dash-panel-head">
        <div>
          <h2>快速指引</h2>
          <p>常見操作與互動模式</p>
        </div>
      </header>
      <div class="quick-list">
        <div class="quick-tip">
          <span class="quick-tip-mark">01</span>
          <div class="quick-tip-text">
            <strong>點任一 row 開啟 drawer</strong>
            <span>右側滑出記錄詳情：hero、snapshot、IUCN（species 限定）、editor 與 change log。</span>
          </div>
        </div>
        <div class="quick-tip">
          <span class="quick-tip-mark">02</span>
          <div class="quick-tip-text">
            <strong>批次操作</strong>
            <span>勾選一筆以上 → 上方出現批次工具列，可一次刪除多筆。</span>
          </div>
        </div>
        <div class="quick-tip">
          <span class="quick-tip-mark">03</span>
          <div class="quick-tip-text">
            <strong>Demo controls</strong>
            <span>右下角齒輪可模擬 loading／error 狀態，方便確認 UI 視覺。</span>
          </div>
        </div>
      </div>
    </section>
  `

  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>系統概況</h2>
        <p class="meta-line">點任一卡片直接進入對應的 collection。</p>
      </div>
    </div>

    <div class="stats-grid">${statCards}</div>

    <div class="dash-grid">
      ${auditPanel}
      ${quickPanel}
    </div>
  `

  resourcePanel.querySelectorAll("[data-jump]").forEach((node) => {
    node.addEventListener("click", () => {
      const target = node.dataset.jump
      if (!target || state.activeKey === target) {
        return
      }
      state.activeKey = target
      state.draft = null
      closeDrawer({ rerender: false })
      triggerNavTransition()
    })
  })
}

function renderObservation() {
  sectionEyebrow.textContent = "Field entry"
  sectionTitle.textContent = "Observation"
  sectionDescription.textContent =
    "依日期 + 地點，紀錄每個物種觀測到的數量。submit 後會批次寫入 observation collection。"
  if (mobileSectionTitle) {
    mobileSectionTitle.textContent = "Observation"
  }

  if (state.demoError) {
    resourcePanel.innerHTML = renderErrorState(
      "GET /api/collections/observation — Network request failed (simulated)",
    )
    const retryButton = $("#error-retry-button")
    if (retryButton) {
      retryButton.addEventListener("click", () => {
        state.demoError = false
        triggerNavTransition()
      })
    }
    return
  }

  const showSkeleton = state.loading || state.demoLoading
  const user = currentUser()
  const isUsersRecord = user?.collectionName === "users"
  const locations = state.records.location || []
  const speciesList = [...(state.records.species || [])].sort((a, b) =>
    (a.chineseName || a.englishName || "").localeCompare(
      b.chineseName || b.englishName || "",
      "zh-Hant",
    ),
  )
  const recentObs = state.records.observation || []
  const recentLimit = recentObs.slice(0, 8)

  const today = new Date()
  const todayISO = today.toISOString().slice(0, 10)

  const locationOptions = locations
    .map(
      (loc) =>
        `<option value="${escapeHtml(loc.id)}">${escapeHtml(loc.chineseName || loc.englishName || loc.id)}</option>`,
    )
    .join("")

  const speciesRowsHtml = speciesList.length
    ? speciesList
        .map(
          (sp) => `
        <div class="species-row">
          <div class="species-name">
            <strong>${escapeHtml(sp.chineseName || sp.englishName || sp.id)}</strong>
            ${
              sp.englishName && sp.chineseName
                ? `<span>${escapeHtml(sp.englishName)}</span>`
                : ""
            }
          </div>
          <input
            type="number"
            min="0"
            step="1"
            name="count_${escapeHtml(sp.id)}"
            data-species-id="${escapeHtml(sp.id)}"
            placeholder="0"
            aria-label="Count for ${escapeHtml(sp.chineseName || sp.englishName || sp.id)}"
          />
        </div>
      `,
        )
        .join("")
    : `<div class="empty-state"><p>還沒有任何 species 資料。請先請 admin 在 Species 頁建立物種。</p></div>`

  const formDisabled = !isUsersRecord || !speciesList.length || !locations.length
  const submitNote = !isUsersRecord
    ? "目前以 superuser 登入，沒有對應的 users record；請改以 admin／volunteer 帳號登入後再提交。"
    : !locations.length
      ? "尚未有 location 可選；請先請 admin 建立 location。"
      : !speciesList.length
        ? "尚未有 species 可選；請先請 admin 建立 species。"
        : "只寫入 count > 0 的物種。"

  const blockerBanner = !isUsersRecord
    ? `
      <div class="alert tone-caution" role="status">
        <span class="alert-icon" aria-hidden="true">!</span>
        <div class="alert-body">
          <div class="alert-title">目前以 superuser 登入</div>
          <div>Observation 需要 observer 對應到 users record。請改以 admin 或 volunteer 帳號登入再輸入。</div>
        </div>
      </div>
    `
    : ""

  const recentPanel = `
    <section class="dash-panel">
      <header class="dash-panel-head">
        <div>
          <h2>最近紀錄</h2>
          <p>${showSkeleton ? "正在載入…" : `近期 ${recentLimit.length} 筆 observation`}</p>
        </div>
      </header>
      ${
        showSkeleton
          ? `<div class="audit-mini-list">${renderAuditMiniSkeleton()}</div>`
          : recentLimit.length
            ? `<div class="audit-mini-list">${recentLimit.map(renderObservationMiniRow).join("")}</div>`
            : `<div class="empty-state"><p>目前還沒有任何 observation 紀錄。</p></div>`
      }
    </section>
  `

  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>輸入今日觀測</h2>
        <p class="meta-line">挑日期跟地點之後，把每個物種觀測到的數量填上去；空白／0 會被自動略過。</p>
      </div>
    </div>

    ${blockerBanner}

    <form class="entry-panel" id="observation-form">
      <div class="entry-meta-grid">
        <div class="form-field">
          <label for="obs-date">Date *</label>
          <input id="obs-date" type="date" name="date" value="${escapeHtml(todayISO)}" required ${formDisabled ? "disabled" : ""} />
        </div>
        <div class="form-field">
          <label for="obs-location">Location *</label>
          <select id="obs-location" name="location" required ${formDisabled ? "disabled" : ""}>
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
        <div class="species-rows">${speciesRowsHtml}</div>
      </div>

      <div class="form-field">
        <label for="obs-notes">Notes (optional)</label>
        <textarea id="obs-notes" name="notes" placeholder="可選；天氣、潮汐、特殊現象等" ${formDisabled ? "disabled" : ""}></textarea>
      </div>

      <div class="entry-actions">
        <span class="submit-hint">${escapeHtml(submitNote)}</span>
        <button type="submit" class="primary-button" ${formDisabled ? "disabled" : ""}>Submit observations</button>
      </div>
    </form>

    ${recentPanel}
  `

  const form = $("#observation-form")
  if (form) {
    form.addEventListener("submit", handleObservationSubmit)
  }
}

function renderObservationMiniRow(entry) {
  const species = entry.expand?.species
  const location = entry.expand?.location
  const observer = entry.expand?.observer
  const speciesName =
    species?.chineseName || species?.englishName || entry.species || "—"
  const locationName =
    location?.chineseName || location?.englishName || entry.location || "—"
  const observerName = observer?.name || observer?.email || "unknown"
  const dateText = entry.date
    ? new Date(entry.date).toLocaleDateString("zh-TW", {
        month: "2-digit",
        day: "2-digit",
      })
    : "—"
  const count = Number.isFinite(Number(entry.count))
    ? Number(entry.count)
    : entry.count || 0

  return `
    <article class="audit-mini-row">
      <div class="audit-mini-pin tone-ok" aria-hidden="true">${escapeHtml(String(count))}</div>
      <div class="audit-mini-text">
        <strong>${escapeHtml(speciesName)} @ ${escapeHtml(locationName)}</strong>
        <p>${escapeHtml(dateText)} · by ${escapeHtml(observerName)}</p>
      </div>
      <time class="audit-mini-time">${escapeHtml(formatRelativeTime(entry.createDate || entry.created))}</time>
    </article>
  `
}

async function handleObservationSubmit(event) {
  event.preventDefault()
  const form = event.target
  const user = currentUser()

  if (user?.collectionName !== "users") {
    showAlert({
      tone: "caution",
      title: "目前帳號無法輸入 observation",
      message:
        "Observer 必須是 users record；請改以 admin 或 volunteer 帳號登入。",
      id: "observation-submit",
    })
    return
  }

  const formData = new FormData(form)
  const date = String(formData.get("date") || "").trim()
  const locationId = String(formData.get("location") || "").trim()
  const notes = String(formData.get("notes") || "").trim()

  if (!date || !locationId) {
    showAlert({
      tone: "caution",
      title: "資料不完整",
      message: "請先選日期與地點。",
      id: "observation-submit",
    })
    return
  }

  const entries = []
  form.querySelectorAll("input[data-species-id]").forEach((input) => {
    const raw = String(input.value || "").trim()
    if (!raw) {
      return
    }
    const count = Number(raw)
    if (!Number.isFinite(count) || count <= 0) {
      return
    }
    entries.push({ speciesId: input.dataset.speciesId, count })
  })

  if (!entries.length) {
    showAlert({
      tone: "caution",
      title: "沒有可送出的資料",
      message: "至少填一個 count > 0 的物種。",
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
      await apiFetch("/collections/observation/records", {
        method: "POST",
        body: {
          date,
          location: locationId,
          species: entry.speciesId,
          count: entry.count,
          notes: notes || undefined,
          observer: user.id,
          createUser: user.id,
        },
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

  if (saved > 0) {
    await loadResource("observation").catch(() => {})
    showAlert({
      tone: "ok",
      title: `已送出 ${saved} 筆 observation`,
      message: firstError
        ? `部分項目失敗：${firstError.message || "unknown error"}`
        : "感謝紀錄！",
      id: "observation-submit",
    })
    renderObservation()
  } else if (firstError) {
    showAlert({
      tone: "danger",
      title: "送出失敗",
      message: firstError.message || "PocketBase 回傳錯誤。",
      id: "observation-submit",
    })
  }
}

function renderResource(resourceKey) {
  if (resourceKey === OBSERVATION_KEY) {
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

  const showSkeleton = state.loading || state.demoLoading
  const showError = state.demoError

  const headColumns = config.tableColumns
    .map((column) => `<div>${escapeHtml(column.label)}</div>`)
    .join("")
  const headSelect = supportsBatchDelete(resourceKey)
    ? '<div class="cell-select cell-select-head"><input type="checkbox" id="select-all-records" aria-label="Select all records" /></div>'
    : ""
  const headActions = config.readOnly
    ? ""
    : '<div style="text-align:right">Actions</div>'
  const headTemplate = config.readOnly
    ? `repeat(${config.tableColumns.length}, minmax(0, 1fr))`
    : `${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto`

  const recordCount = state.records[resourceKey]?.length ?? 0
  const selectedNow = selectedCount(resourceKey)

  const batchBar =
    !showSkeleton && !showError && supportsBatchDelete(resourceKey) && selectedNow
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

  const newButton =
    !showSkeleton && !showError && config.canCreate && !config.readOnly
      ? `<button type="button" class="primary-button" id="new-record-button">New ${escapeHtml(config.title)}</button>`
      : ""

  const countPill = showSkeleton
    ? `<span class="session-pill"><span class="skel" style="width: 56px;"></span></span>`
    : `<span class="session-pill">${recordCount} records</span>`

  let mainBlock
  if (showError) {
    mainBlock = renderErrorState(
      "GET /api/collections — Network request failed (simulated)",
    )
  } else {
    const rowsHtml = showSkeleton
      ? renderTableSkeleton(resourceKey, config.tableColumns.length)
      : renderTableRows(resourceKey)
    mainBlock = `
      <div class="resource-shell">
        <div class="resource-table${showSkeleton ? " skel-table" : ""}">
          <div class="table-head" style="grid-template-columns: ${headTemplate};">
            ${headSelect}${headColumns}${headActions}
          </div>
          ${rowsHtml}
        </div>
      </div>
    `
  }

  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>${escapeHtml(config.title)}</h2>
        <p class="meta-line">${escapeHtml(config.note)}</p>
      </div>
      <div class="panel-actions">
        ${countPill}
        ${newButton}
      </div>
    </div>

    ${batchBar}

    ${mainBlock}
  `

  if (showError) {
    const retryButton = $("#error-retry-button")
    if (retryButton) {
      retryButton.addEventListener("click", () => {
        state.demoError = false
        triggerNavTransition()
      })
    }
    return
  }

  if (showSkeleton) {
    return
  }

  const newRecordButton = $("#new-record-button")
  if (newRecordButton) {
    newRecordButton.addEventListener("click", () => {
      state.draft = null
      openDrawer()
    })
  }

  const clearButton = $("#batch-clear-button")
  if (clearButton) {
    clearButton.addEventListener("click", () => {
      clearSelection(resourceKey)
      renderResource(resourceKey)
    })
  }

  resourcePanel.querySelectorAll("[data-action]").forEach((node) => {
    node.addEventListener("click", async (event) => {
      const action = node.dataset.action
      const id = node.dataset.id

      if (action === "select-row") {
        // Stop event bubbling so row's "open" handler doesn't fire.
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
        if (config.readOnly) {
          return
        }
        state.draft = record
        try {
          await loadRecordHistory(resourceKey, record.id, true)
        } catch (error) {
          showAlert({
            tone: "caution",
            title: "無法載入修改紀錄",
            message: error?.message || "",
            id: "history",
          })
        }
        openDrawer()
        return
      }

      if (action === "delete") {
        event.stopPropagation()
        const label =
          resourceKey === "users"
            ? record.email || record.name || record.id
            : record.chineseName || record.englishName || record.id
        if (!window.confirm(`Delete ${label}?`)) {
          return
        }

        try {
          await apiFetch(
            `/collections/${config.collection}/records/${record.id}`,
            { method: "DELETE" },
          )
          invalidateHistory(resourceKey, record.id)
          setSelected(resourceKey, record.id, false)
          await loadResource(resourceKey)
          showAlert({
            tone: "ok",
            title: "已刪除",
            message: `已刪除 ${label}。`,
            id: "delete",
          })
          renderWorkspace()
        } catch (error) {
          showAlert({
            tone: "danger",
            title: "無法刪除",
            message: error?.message || "",
            id: "delete",
          })
        }
      }
    })
  })

  const selectAll = $("#select-all-records")
  if (selectAll) {
    const rows = state.records[resourceKey] || []
    const selectableIds = rows.map((record) => record.id)
    selectAll.checked =
      selectableIds.length > 0 &&
      selectableIds.every((id) => isSelected(resourceKey, id))
    selectAll.indeterminate =
      selectableIds.some((id) => isSelected(resourceKey, id)) &&
      !selectAll.checked

    selectAll.addEventListener("click", (event) => event.stopPropagation())
    selectAll.addEventListener("change", () => {
      setPageSelection(resourceKey, selectableIds, selectAll.checked)
      renderResource(resourceKey)
    })
  }

  const bulkDeleteButton = $("#bulk-delete-button")
  if (bulkDeleteButton) {
    bulkDeleteButton.addEventListener("click", async () => {
      const ids = selectedIds(resourceKey)
      if (!ids.length) {
        return
      }

      const sampleLabels = ids
        .map((id) =>
          state.records[resourceKey].find((item) => item.id === id),
        )
        .filter(Boolean)
        .slice(0, 5)
        .map((record) => `- ${selectedLabelForRecord(resourceKey, record)}`)

      const extra =
        ids.length > 5 ? `\n- ...and ${ids.length - 5} more` : ""
      const confirmed = window.confirm(
        `Delete ${ids.length} selected ${batchDeleteTitle(resourceKey)} record(s)?\n\nThis cannot be undone.\n\n${sampleLabels.join("\n")}${extra}`,
      )

      if (!confirmed) {
        return
      }

      try {
        for (const recordId of ids) {
          await apiFetch(
            `/collections/${config.collection}/records/${recordId}`,
            { method: "DELETE" },
          )
          invalidateHistory(resourceKey, recordId)
        }

        clearSelection(resourceKey)
        await loadResource(resourceKey)
        showAlert({
          tone: "ok",
          title: "批次刪除完成",
          message: `已刪除 ${ids.length} 筆 ${batchDeleteTitle(resourceKey)} 紀錄。`,
          id: "bulk-delete",
        })
        renderWorkspace()
      } catch (error) {
        showAlert({
          tone: "danger",
          title: "無法批次刪除",
          message: error?.message || "",
          id: "bulk-delete",
        })
      }
    })
  }
}

async function loadResource(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const sortField =
    config.sortField || (resourceKey === "audit_logs" ? "loggedAt" : "updateDate")
  const expandFields =
    Array.isArray(config.expandFields) && config.expandFields.length
      ? `&expand=${config.expandFields.join(",")}`
      : ""
  try {
    const data = await apiFetch(
      `/collections/${config.collection}/records?page=1&perPage=200&sort=-${sortField}${expandFields}`,
    )
    state.records[resourceKey] = data.items || []
    if (supportsBatchDelete(resourceKey)) {
      const validIds = new Set(
        (state.records[resourceKey] || []).map((record) => record.id),
      )
      const set = selectionSet(resourceKey)
      ;[...set].forEach((id) => {
        if (!validIds.has(id)) {
          set.delete(id)
        }
      })
    }
  } catch (error) {
    state.records[resourceKey] = []
    showAlert({
      tone: "danger",
      title: `無法載入「${config.title}」`,
      message: error?.message || "",
      id: `load-${resourceKey}`,
    })
  }
}

async function loadWorkspace() {
  const keys = Object.keys(resourceConfigs).filter((key) =>
    canLoadCollection(key),
  )
  await Promise.all(keys.map((key) => loadResource(key)))
}

function renderWorkspace({ keepDrawer = false } = {}) {
  renderSidebar()
  renderSessionBadges()
  renderResource(state.activeKey)

  accountName.textContent =
    currentUser()?.name || currentUser()?.email || "-"
  accountEmail.textContent = currentUser()?.email || "-"

  if (keepDrawer && state.drawerMode === "open") {
    renderDrawerBody()
  }
}

/* ============================================================
 * Drawer
 * ============================================================ */
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

/* ============================================================
 * Mobile sidebar
 * ============================================================ */
function openSidebar() {
  sidebar.classList.add("is-open")
  sidebarScrim.classList.add("is-open")
}

function closeSidebar() {
  sidebar.classList.remove("is-open")
  sidebarScrim.classList.remove("is-open")
}

function attachGlobalHandlers() {
  $("#sign-out-button").addEventListener("click", () => {
    clearAuth()
    window.location.reload()
  })

  $("#sign-out-from-denied").addEventListener("click", () => {
    clearAuth()
    window.location.reload()
  })

  drawerClose?.addEventListener("click", () => closeDrawer())
  drawerScrim?.addEventListener("click", () => closeDrawer())

  openSidebarButton?.addEventListener("click", () => openSidebar())
  sidebarScrim?.addEventListener("click", () => closeSidebar())

  demoFab?.addEventListener("click", () => toggleDemoPanel())
  demoClose?.addEventListener("click", () => toggleDemoPanel(false))
  demoLoadingButton?.addEventListener("click", () => startDemoLoading())
  demoErrorButton?.addEventListener("click", () => startDemoError())
  demoClearButton?.addEventListener("click", () => clearDemoState())

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") {
      return
    }
    if (drawer.classList.contains("is-open")) {
      closeDrawer()
    } else if (sidebar.classList.contains("is-open")) {
      closeSidebar()
    } else if (!demoPanel?.classList.contains("hidden")) {
      toggleDemoPanel(false)
    }
  })
}

/* ============================================================
 * Demo controls
 * ============================================================ */
function toggleDemoPanel(forceOpen) {
  if (!demoPanel || !demoFab) {
    return
  }
  const willOpen =
    typeof forceOpen === "boolean"
      ? forceOpen
      : demoPanel.classList.contains("hidden")
  demoPanel.classList.toggle("hidden", !willOpen)
  demoFab.setAttribute("aria-expanded", willOpen ? "true" : "false")
}

function syncDemoFabState() {
  if (!demoFab) {
    return
  }
  demoFab.classList.toggle(
    "is-active",
    Boolean(state.demoLoading || state.demoError),
  )
  if (demoLoadingButton) {
    demoLoadingButton.classList.toggle("is-active", state.demoLoading)
  }
  if (demoErrorButton) {
    demoErrorButton.classList.toggle("is-active", state.demoError)
  }
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

  const pwdSubmit = passwordLoginForm?.querySelector('button[type="submit"]')
  if (pwdSubmit) {
    pwdSubmit.disabled = true
  }

  try {
    const storedAuth = readAuth()
    if (storedAuth) {
      state.auth = storedAuth
    }

    const hasSession = await ensureSession()
    await loadAuthMethods()
    renderProviderList(providerItems())
    renderPasswordLogin(Boolean(state.authMethods?.password?.enabled))

    if (!hasSession || !state.auth?.record) {
      showView("login")
      if (state.authMethods?.password?.enabled) {
        setStatus("請使用下方的 Email／密碼登入（本機預設見說明）。", "hint")
      } else if (providerItems().length) {
        setStatus("請選擇上方的登入方式。", "hint")
      } else {
        setStatus(
          "目前未啟用任何登入方式，請檢查 PocketBase users 與環境變數。",
          "error",
        )
      }
      return
    }

    if (!hasAccess()) {
      showView("access")
      return
    }
    if (!canSeeNav(state.activeKey)) {
      state.activeKey = OBSERVATION_KEY
    }

    await loadDynamicCollections()
    showView("app")
    await loadWorkspace()
    renderWorkspace()
  } catch (error) {
    console.error(error)
    showView("login")
    setStatus(
      String(error?.message || "無法載入登入設定，請重新整理頁面後再試。"),
      "error",
    )
  } finally {
    if (pwdSubmit) {
      pwdSubmit.disabled = false
    }
  }
}

if (passwordLoginForm) {
  passwordLoginForm.addEventListener("submit", async (event) => {
    event.preventDefault()

    const formData = new FormData(passwordLoginForm)
    const email = String(formData.get("email") || "").trim()
    const password = String(formData.get("password") || "")

    try {
      const auth = await authWithPasswordAuto(email, password)
      saveAuth(auth)
      window.location.reload()
    } catch (error) {
      const raw = String(error?.message || "").trim()
      const base = raw || "登入失敗。"
      const lower = raw.toLowerCase()
      let hint = ""
      if (
        lower.includes("invalid login") ||
        lower.includes("failed to authenticate") ||
        lower.includes("authenticate")
      ) {
        hint =
          " 請確認帳密正確；本機請設定 PB_DEV_PASSWORD_AUTH=true（依 README）並使用設定的 superuser／users 帳號。"
      }
      setStatus(`${base}${hint}`, "error")
    }
  })
}

void boot()
