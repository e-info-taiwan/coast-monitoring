# Record History Timeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the audit history for the currently selected `users`, `location`, or `species` record inside the custom admin UI.

**Architecture:** Reuse the existing `/` custom admin page and the `audit_logs` collection. When a user selects a record for edit/view, fetch matching audit rows by `targetCollection` and `targetRecordId`, render them as a compact timeline beneath the editor, and keep the implementation read-only. No schema changes are needed because the audit log already stores a polymorphic reference to the source collection and record id.

**Tech Stack:** PocketBase JS hooks, the existing `pb_public/app.js` admin UI, HTML/CSS, PocketBase REST API.

---

### Task 1: Add record-history data loading to the admin UI

**Files:**
- Modify: `/Users/hcchien/e-info/coast-monitoring/pb_public/app.js`

- [ ] **Step 1: Add a history cache and loader**

```javascript
const state = {
  auth: readAuth(),
  records: {},
  draft: null,
  history: {},
}

async function loadRecordHistory(collectionName, recordId, force = false) {
  if (!collectionName || !recordId) return []
  const cacheKey = `${collectionName}:${recordId}`
  if (!force && Array.isArray(state.history[cacheKey])) return state.history[cacheKey]

  const data = await apiFetch(
    `/collections/audit_logs/records?perPage=50&sort=-loggedAt&filter=(targetCollection='${collectionName}'&&targetRecordId='${recordId}')`
  )
  const items = Array.isArray(data?.items) ? data.items : []
  state.history[cacheKey] = items
  return items
}
```

- [ ] **Step 2: Wire the loader into the record selection flow**

```javascript
if (action === "edit") {
  state.draft = record
  await loadRecordHistory(resourceKey, record.id, true)
  renderWorkspace()
  return
}
```

- [ ] **Step 3: Rework the editor renderer to include a history section**

```javascript
function renderRecordHistory(resourceKey) {
  const draft = state.draft
  if (!draft || resourceKey === "audit_logs") return ""

  const cacheKey = `${resourceKey}:${draft.id}`
  const rows = state.history[cacheKey] || []

  return rows.length
    ? `<section class="history-card"><div class="history-list">${rows
        .map((entry) => renderHistoryEntry(entry))
        .join("")}</div></section>`
    : `<section class="history-card"><div class="empty-state"><p>No history for this record yet.</p></div></section>`
}
```

- [ ] **Step 4: Update the main resource layout to render the history section below the editor**

```javascript
resourcePanel.innerHTML = `
  <div class="panel-header">
    <div>
      <h2>${config.title}</h2>
      <p class="meta-line">${config.note}</p>
    </div>
    <div class="session-pill">${state.records[resourceKey].length} records</div>
  </div>
  <div class="resource-shell">
    <div class="resource-table">${renderTableRows(resourceKey)}</div>
    <div class="editor-column">
      ${renderEditor(resourceKey)}
      ${renderRecordHistory(resourceKey)}
    </div>
  </div>
`
```

- [ ] **Step 5: Run a syntax check on the updated UI file**

Run: `node --check /Users/hcchien/e-info/coast-monitoring/pb_public/app.js`
Expected: exit code 0.

### Task 2: Style the history timeline for readability

**Files:**
- Modify: `/Users/hcchien/e-info/coast-monitoring/pb_public/styles.css`

- [ ] **Step 1: Add timeline styles**

```css
.editor-column {
  display: grid;
  gap: 16px;
}

.history-card {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 18px;
  padding: 18px;
  background: rgba(255, 255, 255, 0.03);
}

.history-list {
  display: grid;
  gap: 12px;
  margin-top: 14px;
}

.history-entry {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.04);
}
```

- [ ] **Step 2: Make the layout responsive on narrow screens**

```css
@media (max-width: 960px) {
  .editor-column {
    gap: 12px;
  }
}
```

- [ ] **Step 3: Run a stylesheet sanity check by loading the local page in the browser**

Run: `http://127.0.0.1:8090/`
Expected: the editor and history cards stack cleanly without breaking the existing table layout.

### Task 3: Document the history view for future maintainers

**Files:**
- Modify: `/Users/hcchien/e-info/coast-monitoring/README.md`

- [ ] **Step 1: Add a short note explaining the history panel**

```markdown
## Record History

When you open a `users`, `location`, or `species` record in the custom admin UI, the editor now shows the matching `audit_logs` entries below the form. The timeline is read-only and is keyed by `targetCollection` + `targetRecordId`.
```

- [ ] **Step 2: Run a quick Markdown consistency check**

Run: `rg -n "Record History|audit_logs|targetCollection|targetRecordId" README.md`
Expected: the new section is present and references the correct fields.

- [ ] **Step 3: Commit the documentation and implementation together**

```bash
git add pb_public/app.js pb_public/styles.css README.md docs/superpowers/plans/2026-05-12-record-history-timeline.md
git commit -m "feat: show record audit history in admin UI"
```
