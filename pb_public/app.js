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
const resourceNav = $("#resource-nav")
const overviewPanel = $("#overview-panel")
const resourcePanel = $("#resource-panel")
const accountName = $("#account-name")
const accountEmail = $("#account-email")
const sectionEyebrow = $("#section-eyebrow")
const sectionTitle = $("#section-title")
const sectionDescription = $("#section-description")
const sessionBadges = $("#session-badges")

const resourceConfigs = {
  users: {
    title: "Users",
    eyebrow: "People",
    description: "Google login users, their display name, and role.",
    collection: "users",
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
}

const state = {
  activeKey: "users",
  records: {},
  draft: null,
  authMethods: null,
  auth: null,
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

function setStatus(message) {
  loginStatus.textContent = message || ""
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
      setStatus("Opening Google sign-in...")
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

  try {
    const collectionName = currentUser()?.collectionName || "users"
    const data = await apiFetch(`/collections/${collectionName}/auth-refresh`, { method: "POST" })
    saveAuth(data)
    return true
  } catch (_) {
    clearAuth()
    return false
  }
}

async function authWithPassword(collectionName, identity, password) {
  return apiFetch(`/collections/${collectionName}/auth-with-password`, {
    method: "POST",
    auth: false,
    body: { identity, password },
  })
}

async function authWithPasswordAuto(identity, password) {
  const attempts = ["_superusers", "users"]

  let lastError = null
  for (const collectionName of attempts) {
    try {
      return await authWithPassword(collectionName, identity, password)
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

  if (key === "createDate" || key === "updateDate" || key === "created" || key === "updated") {
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

      return `
        <div class="table-row" style="grid-template-columns: repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto;">
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
  const title = isUsers ? (draft ? "Edit Users" : "User details") : draft ? `Edit ${config.title}` : `New ${config.title}`
  const submitDisabled = isUsers && !draft
  const submitLabel = draft ? "Save changes" : isUsers ? "Select a user" : "Create"

  const fields = config.editableFields
    .map((field) => {
      const currentValue = draft ? draft[field.key] ?? "" : ""
      const control =
        field.type === "textarea"
          ? `<textarea id="${field.key}" name="${field.key}" placeholder="${escapeHtml(field.placeholder || "")}">${escapeHtml(currentValue)}</textarea>`
          : field.type === "select"
            ? `<select id="${field.key}" name="${field.key}">${field.options
                .map((option) => `<option value="${escapeHtml(option.value)}" ${currentValue === option.value ? "selected" : ""}>${escapeHtml(option.label)}</option>`)
                .join("")}</select>`
            : `<input id="${field.key}" name="${field.key}" type="text" value="${escapeHtml(currentValue)}" placeholder="${escapeHtml(field.placeholder || "")}" />`

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

  resourcePanel.innerHTML = `
    <div class="panel-header">
      <div>
        <h2>${escapeHtml(config.title)}</h2>
        <p class="meta-line">${escapeHtml(config.note)}</p>
      </div>
      <div class="session-pill">${(state.records[resourceKey]?.length ?? 0)} records</div>
    </div>

    <div class="resource-shell">
      <div class="resource-table">
        <div class="table-head" style="grid-template-columns: repeat(${config.tableColumns.length}, minmax(0, 1fr)) auto;">
          ${columns}
          <div style="text-align:right">Actions</div>
        </div>
        ${renderTableRows(resourceKey)}
      </div>
      ${renderEditor(resourceKey)}
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
    const formData = new FormData(editorForm)
    const payload = Object.fromEntries(formData.entries())
    const authUser = currentUser()

    try {
      if (resourceKey === "users") {
        const id = state.draft?.id
        if (!id) {
          setStatus("User records are created automatically from Google login.")
          return
        }
        if (authUser?.collectionName === "users") {
          payload.updateUser = authUser.id
        }
        await apiFetch(`/collections/users/records/${id}`, { method: "PATCH", body: payload })
      } else if (state.draft?.id) {
        if (authUser?.collectionName === "users") {
          payload.updateUser = authUser.id
        }
        await apiFetch(`/collections/${config.collection}/records/${state.draft.id}`, { method: "PATCH", body: payload })
      } else {
        if (authUser?.collectionName === "users") {
          payload.createUser = authUser.id
        }
        await apiFetch(`/collections/${config.collection}/records`, { method: "POST", body: payload })
      }

      state.draft = null
      await loadResource(resourceKey)
      renderWorkspace()
    } catch (error) {
      setStatus(error?.message || "Unable to save record.")
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
          await loadResource(resourceKey)
          renderWorkspace()
        } catch (error) {
          setStatus(error?.message || "Unable to delete record.")
        }
      }
    })
  })
}

async function loadResource(resourceKey) {
  const config = resourceConfigs[resourceKey]
  try {
    const data = await apiFetch(
      `/collections/${config.collection}/records?page=1&perPage=200&sort=-created&expand=createUser,updateUser`,
    )
    state.records[resourceKey] = data.items || []
  } catch (error) {
    state.records[resourceKey] = []
    setStatus(`Unable to load ${config.title.toLowerCase()} records.`)
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
  setStatus("Checking session...")
  attachGlobalHandlers()

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
      setStatus("You can use the local superuser login for development.")
    } else if (providerItems().length) {
      setStatus("Choose a provider to sign in.")
    } else {
      setStatus("No sign-in method is currently enabled.")
    }
    return
  }

  if (!isAdmin()) {
    showView("access")
    return
  }

  showView("app")
  await loadWorkspace()
  renderWorkspace()
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
      const message = String(error?.message || "")
      if (message.toLowerCase().includes("invalid login credentials")) {
        setStatus("請使用本機 superuser 帳號 hcchien@gmail.com / Test1234!。")
        return
      }
      setStatus(message || "Unable to sign in.")
    }
  })
}

void boot()
