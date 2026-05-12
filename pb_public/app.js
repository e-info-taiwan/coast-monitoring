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
const overviewPanel = $("#overview-panel")
const resourcePanel = $("#resource-panel")
const accountName = $("#account-name")
const accountEmail = $("#account-email")
const sectionEyebrow = $("#section-eyebrow")
const sectionTitle = $("#section-title")
const sectionDescription = $("#section-description")
const sessionBadges = $("#session-badges")

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
    note: "Keep canonical species names here so other workflows can reference them cleanly.",
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
}

const resourceConfigs = { ...baseResourceConfigs }

const EXCLUDED_COLLECTIONS = new Set(["_superusers", "_externalAuths", "_mfas", "_otps", "_authOrigins", "AuthOrigins", "event"])

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
  return ["text", "number", "bool", "email", "url", "date", "select", "editor", "json"].includes(field.type)
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
      options: (field.values || []).map((value) => ({ value, label: String(value) })),
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
  const preferred = fields.filter((field) => !field.hidden && !field.system).slice(0, 5)
  const columns = preferred.map((field) => ({ key: field.name, label: toStartCase(field.name) }))
  if (!columns.some((item) => item.key === "id")) {
    columns.push({ key: "id", label: "Id" })
  }
  return columns
}

function defaultSortFieldFromFields(fields) {
  const names = new Set((Array.isArray(fields) ? fields : []).map((field) => field?.name))
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

const state = {
  activeKey: "users",
  records: {},
  draft: null,
  authMethods: null,
  auth: null,
  collectionFields: {},
  history: {},
  selectedIds: {},
}

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
    return value.length ? value.map((item) => formatHistoryValue(item)).join(", ") : "—"
  }
  if (typeof value === "object") {
    return JSON.stringify(value)
  }
  return String(value)
}

function diffHistoryFields(beforeFields, afterFields) {
  const ignored = new Set(["id", "createDate", "updateDate"])
  const keys = new Set([...Object.keys(beforeFields || {}), ...Object.keys(afterFields || {})])
  const diffs = []

  keys.forEach((key) => {
    if (ignored.has(key)) {
      return
    }
    const beforeValue = beforeFields?.[key]
    const afterValue = afterFields?.[key]
    if (JSON.stringify(beforeValue ?? null) === JSON.stringify(afterValue ?? null)) {
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
  const diffs = diffHistoryFields(historyFields(entry, "before"), historyFields(entry, "after"))
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
    const filter = encodeURIComponent(`targetCollection = '${collectionName}' && targetRecordId = '${recordId}'`)
    const data = await apiFetch(`/collections/audit_logs/records?page=1&perPage=50&sort=-loggedAt&filter=${filter}`)
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
  const diffs = diffHistoryFields(historyFields(entry, "before"), historyFields(entry, "after"))
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
          <p class="meta-line">This shows the audit trail for the selected record.</p>
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

async function apiFetch(path, { method = "GET", body, auth = true, headers = {} } = {}) {
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
    const message = typeof data === "object" && data && data.message ? data.message : `Request failed (${response.status})`
    throw new Error(message)
  }

  return data
}

async function loadAuthMethods() {
  state.authMethods = await apiFetch("/collections/users/auth-methods", { auth: false })
}

async function loadDynamicCollections() {
  let collections = []
  try {
    const data = await apiFetch("/collections?page=1&perPage=200&sort=name")
    collections = data?.items || []
  } catch (_) {
    // This endpoint is usually superuser-only. Keep static collections for regular admin accounts.
    return
  }

  collections.forEach((collection) => {
    if (!collection?.name || collection.system || EXCLUDED_COLLECTIONS.has(collection.name)) {
      return
    }

    if (resourceConfigs[collection.name]) {
      state.collectionFields[collection.name] = Array.isArray(collection.fields) ? collection.fields : []
      return
    }

    resourceConfigs[collection.name] = configFromCollection(collection)
    state.collectionFields[collection.name] = Array.isArray(collection.fields) ? collection.fields : []
  })
}

function providerItems() {
  return state.authMethods?.oauth2?.providers || state.authMethods?.authProviders || []
}

function getProviderLabel(provider) {
  if (!provider) {
    return "Google"
  }

  return provider.displayName || (provider.name === "google" ? "Google" : provider.name)
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
    button.className = "provider-button"
    button.innerHTML = `<span class="session-pill">${escapeHtml(getProviderLabel(provider).slice(0, 1).toUpperCase())}</span><span>Sign in with ${escapeHtml(getProviderLabel(provider))}</span>`
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
  const candidates = [...new Set([...(preferred ? [preferred] : []), "_superusers", "users"])]

  for (const targetCollection of candidates) {
    try {
      const data = await apiFetch(`/collections/${targetCollection}/auth-refresh`, { method: "POST" })
      saveAuth(normalizeAuthResponse(data, targetCollection))
      return true
    } catch (_) {
      // Try next candidate (e.g. cached auth missing collectionName, or legacy localStorage).
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
      `<span class="session-pill">role: ${escapeHtml(record.collectionName === "_superusers" ? "superuser" : record.role || "unknown")}</span>`,
    )
    badges.push(`<span class="session-pill">${escapeHtml(record.email || "-")}</span>`)
  }

  sessionBadges.innerHTML = badges.join("")
}

function renderSidebar() {
  resourceNav.innerHTML = ""

  Object.entries(resourceConfigs).forEach(([key, config]) => {
    const button = document.createElement("button")
    button.className = `nav-button ${state.activeKey === key ? "active" : ""}`
    button.innerHTML = `<span>${escapeHtml(config.title)}</span><span>${state.records[key]?.length ?? 0}</span>`
    button.addEventListener("click", () => {
      state.activeKey = key
      state.draft = null
      renderWorkspace()
    })
    resourceNav.appendChild(button)
  })
}

function renderOverview() {
  const cards = Object.entries(resourceConfigs).map(([key, config]) => {
    const count = state.records[key]?.length ?? 0
    return `
      <article class="summary-card">
        <span>${escapeHtml(config.title)}</span>
        <strong>${count}</strong>
        <p class="meta-line">${escapeHtml(config.description)}</p>
      </article>
    `
  })

  overviewPanel.innerHTML = `<div class="summary-grid">${cards.join("")}</div>`
}

function valueForRecord(record, key) {
  const value = record?.[key]

  if (value == null || value === "") {
    return ""
  }

  if (key === "createDate" || key === "updateDate" || key === "loggedAt" || key === "created" || key === "updated") {
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
      payload[field.key] = editorForm.querySelector(`[name="${field.key}"]`)?.checked || false
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
    return `<div class="empty-state">目前還沒有任何資料。</div>`
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
        ? `<div class="cell-select"><input type="checkbox" class="row-select" data-action="select-row" data-id="${escapeHtml(record.id)}" ${isSelected(resourceKey, record.id) ? "checked" : ""} aria-label="Select row ${escapeHtml(record.id)}" /></div>`
        : ""

      if (config.readOnly) {
        return `
          <div class="table-row" style="grid-template-columns: repeat(${config.tableColumns.length}, minmax(0, 1fr));">
            ${cells}
          </div>
        `
      }

      return `
        <div class="table-row" style="grid-template-columns: ${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto;">
          ${selectionCell}
          ${cells}
          <div class="cell-actions">
            <button class="tiny-button" data-action="edit" data-id="${escapeHtml(record.id)}">Edit</button>
            <button class="tiny-button danger-tiny" data-action="delete" data-id="${escapeHtml(record.id)}">Delete</button>
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
        <div class="resource-toolbar">
          <div>
            <p class="eyebrow">${escapeHtml(config.eyebrow)}</p>
            <h3>${escapeHtml(config.title)}</h3>
            <p class="meta-line">${escapeHtml(config.note)}</p>
          </div>
        </div>
        <div class="empty-state">
          <p>This collection is read-only in the custom admin UI.</p>
        </div>
      </div>
    `
  }
  const title = isUsers ? (draft ? "Edit Users" : "User details") : draft ? `Edit ${config.title}` : `New ${config.title}`
  const submitDisabled = isUsers && !draft
  const submitLabel = draft ? "Save changes" : isUsers ? "Select a user" : "Create"

  const fields = config.editableFields
    .map((field) => {
      const currentValue = draft ? draft[field.key] ?? "" : ""
      const control =
        field.type === "textarea"
          ? `<textarea id="${field.key}" name="${field.key}" placeholder="${escapeHtml(field.placeholder || "")}" ${field.required ? "required" : ""}>${escapeHtml(currentValue)}</textarea>`
          : field.type === "select"
            ? `<select id="${field.key}" name="${field.key}" ${field.required ? "required" : ""}>${field.options
                .map((option) => `<option value="${escapeHtml(option.value)}" ${currentValue === option.value ? "selected" : ""}>${escapeHtml(option.label)}</option>`)
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

  const createButton = config.canCreate
    ? `<button type="button" class="ghost-button" id="new-record-button">New ${escapeHtml(config.title)}</button>`
    : `<p class="form-help">${escapeHtml(config.note)}</p>`

  return `
    <div class="editor-card">
      <div class="resource-toolbar">
        <div>
          <p class="eyebrow">${escapeHtml(config.eyebrow)}</p>
          <h3>${escapeHtml(title)}</h3>
          <p class="meta-line">${escapeHtml(config.note)}</p>
        </div>
        ${createButton}
      </div>

      <form class="editor-form" id="editor-form">
        ${isUsers ? `<div class="form-help">Google login 帳號會自動建立 user record，這裡主要用來修正名稱與 role。</div>` : ""}
        ${fields}
        <div class="editor-actions">
          <button type="button" class="ghost-button" id="reset-editor-button">Reset</button>
          <button type="submit" class="primary-button" ${submitDisabled ? "disabled" : ""}>${submitLabel}</button>
        </div>
      </form>
    </div>
  `
}

function renderResource(resourceKey) {
  const config = resourceConfigs[resourceKey]
  sectionEyebrow.textContent = config.eyebrow
  sectionTitle.textContent = config.title
  sectionDescription.textContent = config.description

  const columns = config.tableColumns.map((column) => `<div>${escapeHtml(column.label)}</div>`).join("")
  const headColumns = `${supportsBatchDelete(resourceKey) ? '<div class="cell-select cell-select-head"><input type="checkbox" id="select-all-records" aria-label="Select all records" /></div>' : ""}${columns}${config.readOnly ? "" : '<div style="text-align:right">Actions</div>'}`
  const headTemplate = config.readOnly
    ? `repeat(${config.tableColumns.length}, minmax(0, 1fr))`
    : `${supportsBatchDelete(resourceKey) ? "40px " : ""}repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto`
  const bulkDeleteControls = supportsBatchDelete(resourceKey)
    ? `
      <div class="bulk-actions">
        <button type="button" class="danger-button" id="bulk-delete-button" ${selectedCount(resourceKey) ? "" : "disabled"}>
          Delete selected${selectedCount(resourceKey) ? ` (${selectedCount(resourceKey)})` : ""}
        </button>
        <span class="meta-line">${selectedCount(resourceKey)} selected</span>
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
        <div class="session-pill">${(state.records[resourceKey]?.length ?? 0)} records</div>
        ${bulkDeleteControls}
      </div>
    </div>

    <div class="resource-shell">
      <div class="resource-table">
        <div class="table-head" style="grid-template-columns: ${headTemplate};">
          ${headColumns}
        </div>
        ${renderTableRows(resourceKey)}
      </div>
      <div class="editor-column">
        ${renderEditor(resourceKey)}
        ${renderRecordHistory(resourceKey)}
      </div>
    </div>
  `

  const editorForm = $("#editor-form")
  const resetButton = $("#reset-editor-button")
  const newButton = $("#new-record-button")

  if (newButton) {
    newButton.addEventListener("click", () => {
      state.draft = null
      renderWorkspace()
    })
  }

  resetButton.addEventListener("click", () => {
    state.draft = null
    renderWorkspace()
  })

  editorForm.addEventListener("submit", async (event) => {
    event.preventDefault()
    const authUser = currentUser()
    let payload = {}

    try {
      payload = payloadFromEditorForm(editorForm, config)
      if (resourceKey === "users") {
        const id = state.draft?.id
        if (!id) {
          setStatus("使用者帳號是由 Google 登入自動建立的，無法在此新增。", "error")
          return
        }
        if (authUser?.collectionName === "users") {
          payload.updateUser = authUser.id
        }
        const updated = await apiFetch(`/collections/users/records/${id}`, { method: "PATCH", body: payload })
        state.draft = updated
        invalidateHistory(resourceKey, id)
        loadRecordHistory(resourceKey, id, true).catch((error) => {
          console.warn("history refresh failed", error)
          setStatus("儲存成功，但修改紀錄暫時無法更新。", "hint")
        })
      } else if (state.draft?.id) {
        if (authUser?.collectionName === "users") {
          payload.updateUser = authUser.id
        }
        const recordId = state.draft.id
        const updated = await apiFetch(`/collections/${config.collection}/records/${recordId}`, { method: "PATCH", body: payload })
        state.draft = updated
        invalidateHistory(resourceKey, recordId)
        loadRecordHistory(resourceKey, recordId, true).catch((error) => {
          console.warn("history refresh failed", error)
          setStatus("儲存成功，但修改紀錄暫時無法更新。", "hint")
        })
      } else {
        if (authUser?.collectionName === "users") {
          payload.createUser = authUser.id
        }
        const created = await apiFetch(`/collections/${config.collection}/records`, { method: "POST", body: payload })
        invalidateHistory(resourceKey, created?.id)
        state.draft = created
        loadRecordHistory(resourceKey, created?.id, true).catch((error) => {
          console.warn("history refresh failed", error)
          setStatus("儲存成功，但修改紀錄暫時無法更新。", "hint")
        })
      }

      await loadResource(resourceKey)
      clearWorkspaceBanner()
      renderWorkspace()
    } catch (error) {
      setStatus(error?.message || "無法儲存。", "error")
    }
  })

  resourcePanel.querySelectorAll("[data-action]").forEach((button) => {
    button.addEventListener("click", async () => {
      const action = button.dataset.action
      const id = button.dataset.id
      const record = state.records[resourceKey].find((item) => item.id === id)

      if (!record) {
        return
      }

      if (action === "edit") {
        state.draft = record
        try {
          await loadRecordHistory(resourceKey, record.id, true)
        } catch (error) {
          setStatus(error?.message || "無法載入修改紀錄。", "error")
        }
        renderWorkspace()
        return
      }

      if (action === "delete") {
        const label =
          resourceKey === "users"
            ? record.email || record.name || record.id
            : record.chineseName || record.englishName || record.id
        if (!window.confirm(`Delete ${label}?`)) {
          return
        }

        try {
          await apiFetch(`/collections/${config.collection}/records/${record.id}`, { method: "DELETE" })
          invalidateHistory(resourceKey, record.id)
          setSelected(resourceKey, record.id, false)
          await loadResource(resourceKey)
          clearWorkspaceBanner()
          renderWorkspace()
        } catch (error) {
          setStatus(error?.message || "無法刪除。", "error")
        }
      }
    })
  })

  resourcePanel.querySelectorAll(".row-select").forEach((checkbox) => {
    checkbox.addEventListener("change", () => {
      setSelected(resourceKey, checkbox.dataset.id, checkbox.checked)
      renderWorkspace()
    })
  })

  const selectAll = $("#select-all-records")
  if (selectAll) {
    const rows = state.records[resourceKey] || []
    const selectableIds = rows.map((record) => record.id)
    selectAll.checked = selectableIds.length > 0 && selectableIds.every((id) => isSelected(resourceKey, id))
    selectAll.indeterminate = selectableIds.some((id) => isSelected(resourceKey, id)) && !selectAll.checked

    selectAll.addEventListener("change", () => {
      setPageSelection(resourceKey, selectableIds, selectAll.checked)
      renderWorkspace()
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
        .map((id) => state.records[resourceKey].find((item) => item.id === id))
        .filter(Boolean)
        .slice(0, 5)
        .map((record) => `- ${selectedLabelForRecord(resourceKey, record)}`)

      const extra = ids.length > 5 ? `\n- ...and ${ids.length - 5} more` : ""
      const confirmed = window.confirm(
        `Delete ${ids.length} selected ${batchDeleteTitle(resourceKey)} record(s)?\n\nThis cannot be undone.\n\n${sampleLabels.join("\n")}${extra}`,
      )

      if (!confirmed) {
        return
      }

      try {
        for (const recordId of ids) {
          await apiFetch(`/collections/${config.collection}/records/${recordId}`, { method: "DELETE" })
          invalidateHistory(resourceKey, recordId)
        }

        clearSelection(resourceKey)
        await loadResource(resourceKey)
        clearWorkspaceBanner()
        renderWorkspace()
      } catch (error) {
        setStatus(error?.message || "無法批次刪除。", "error")
      }
    })
  }
}

async function loadResource(resourceKey) {
  const config = resourceConfigs[resourceKey]
  const sortField = config.sortField || (resourceKey === "audit_logs" ? "loggedAt" : "updateDate")
  const expandFields = Array.isArray(config.expandFields) && config.expandFields.length ? `&expand=${config.expandFields.join(",")}` : ""
  try {
    const data = await apiFetch(
      `/collections/${config.collection}/records?page=1&perPage=200&sort=-${sortField}${expandFields}`,
    )
    state.records[resourceKey] = data.items || []
    if (supportsBatchDelete(resourceKey)) {
      const validIds = new Set((state.records[resourceKey] || []).map((record) => record.id))
      const set = selectionSet(resourceKey)
      ;[...set].forEach((id) => {
        if (!validIds.has(id)) {
          set.delete(id)
        }
      })
    }
  } catch (error) {
    state.records[resourceKey] = []
    const detail = error?.message ? `（${error.message}）` : ""
    setStatus(`無法載入「${config.title}」資料${detail}`, "error")
  }
}

async function loadWorkspace() {
  await Promise.all(Object.keys(resourceConfigs).map((key) => loadResource(key)))
}

function renderWorkspace() {
  renderSidebar()
  renderOverview()
  renderSessionBadges()
  renderResource(state.activeKey)

  accountName.textContent = currentUser()?.name || currentUser()?.email || "-"
  accountEmail.textContent = currentUser()?.email || "-"
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
        setStatus("目前未啟用任何登入方式，請檢查 PocketBase users 與環境變數。", "error")
      }
      return
    }

    if (!isAdmin()) {
      showView("access")
      return
    }

    await loadDynamicCollections()
    showView("app")
    await loadWorkspace()
    renderWorkspace()
  } catch (error) {
    console.error(error)
    showView("login")
    setStatus(String(error?.message || "無法載入登入設定，請重新整理頁面後再試。"), "error")
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
        hint = " 請確認帳密正確；本機請設定 PB_DEV_PASSWORD_AUTH=true（依 README）並使用設定的 superuser／users 帳號。"
      }
      setStatus(`${base}${hint}`, "error")
    }
  })
}

void boot()
