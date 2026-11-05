import { test } from "node:test"
import assert from "node:assert/strict"
import { JSDOM } from "jsdom"
import { openSiteWorkspace, affectedSiteIDs, parseOptionalList } from "../../web/admin/site-workspace.js"

test("site areas save independently and unsaved changes retain leave guard", async () => {
  const dom = new JSDOM("<main></main>", { url: "https://admin.example.invalid" })
  const savedGlobals = { window: globalThis.window, document: globalThis.document, FormData: globalThis.FormData }
  globalThis.window = dom.window
  globalThis.document = dom.window.document
  globalThis.FormData = dom.window.FormData
  const writes = []
  let guard
  const sites = [{ id: 1, name_zh: "A", is_active: true, cwa_station_source: "manual", cwa_station_id: "X" }]
  const display = {
    site_id: 1,
    effective_settings: [{ chart_key: "taxa_density", source: "default", is_visible: true }],
    chart_options: {taxa_density: {years:[2018,2025],depths:[5,10],taxa:[{value:'3',label:'蝶魚'}]}},
    groups: [],
    report_links: [],
  }
  const apiFetch = async (path, opts) => {
    if (opts?.method) {
      writes.push({ path, body: opts.body })
      return path.endsWith("/charts") ? { ...opts.body, id: 10, version: "next" } : { ...sites[0], ...opts.body }
    }
    if (path === "/admin/sites") return sites
    if (path.endsWith("/display")) return display
    if (path.includes("sync-status")) return {}
    return []
  }
  const body = dom.window.document.querySelector("main")
  try {
    await openSiteWorkspace({
      siteID: 1,
      apiFetch,
      drawer: {
        open(o) {
          guard = o.guard
          body.innerHTML = o.html
          o.bind(body)
        },
      },
    })
    const basic = body.querySelector('[data-form="basic"]'),
      content = body.querySelector('[data-form="content"]'),
      chart = body.querySelector('[data-form="chart-taxa_density"]')
    const change = (form, key, value) => {
      const el = form.elements.namedItem(key)
      el.value = value
      el.dispatchEvent(new dom.window.Event("input", { bubbles: true }))
    }
    change(content, "summary", "unsaved public description")
    change(chart, "year_from", "2020")
    change(basic, "name_zh", "Renamed A")
    window.confirm = () => false
    assert.equal(guard(), false)
    await basic.onsubmit({ preventDefault() {} })
    assert.equal(writes.length, 1)
    assert.equal(writes[0].path, "/admin/sites/1")
    assert.equal(writes[0].body.name_zh, "Renamed A")
    assert.equal("cwa_station_id" in writes[0].body, false)
    assert.equal(content.elements.summary.value, "unsaved public description")
    assert.equal(chart.elements.year_from.value, "2020")
    assert.equal(guard(), false)
    await chart.onsubmit({ preventDefault() {} })
    assert.equal(writes.length, 2)
    assert.equal(writes[1].body.publication_status, "draft")
    assert.equal(guard(), false)
    window.confirm = () => true
    assert.equal(guard(), true)
    await chart.querySelector('[data-restore]').onclick()
    assert.equal(chart.elements.year_from.value, '2018')
    assert.equal(chart.elements.default_depth_m.value, '5')
    assert.equal(chart.querySelector('[name="taxon_ids"]').value, '3')
    assert.equal(chart.querySelector('[name="taxon_ids"]').checked, true)
    await chart.onsubmit({preventDefault(){}})
    assert.equal(writes.at(-1).body.year_from, null)
    assert.equal(writes.at(-1).body.taxon_ids, null)
  } finally {
    Object.assign(globalThis, savedGlobals)
    dom.window.close()
  }
})
test("shared changes include removed sites and nullable versus empty overrides", () => {
  assert.deepEqual(affectedSiteIDs([{ site_id: 1 }, { site_id: 2 }], [{ site_id: 1 }, { site_id: 3 }]), [1, 2, 3])
  assert.equal(parseOptionalList(""), null)
  assert.deepEqual(parseOptionalList("[]"), [])
  assert.deepEqual(parseOptionalList("2,3", true), [2, 3])
})
