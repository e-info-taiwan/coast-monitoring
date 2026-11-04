import test from 'node:test'
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../reef-data.js', import.meta.url), 'utf8')
const tick = () => new Promise(resolve => setImmediate(resolve))

// Exercise the production click/change/submit handlers with the API's JSON contract.
class Element {
  constructor() { this.nodes = new Map(); this.handlers = {}; this.style = {}; this.value = ''; this.textContent = ''; this.isConnected = true }
  set innerHTML(html) {
    this.html = html
    this.nodes.clear()
    for (const match of html.matchAll(/<(input|select|small|button|form|p)\b([^>]*\bid="([^"]+)"[^>]*)>/g)) {
      const node = this.querySelector(`#${match[3]}`)
      node.value = match[2].match(/\bvalue="([^"]*)"/)?.[1] || ''
      node.disabled = /\bdisabled\b/.test(match[2])
      if (match[1] === 'select') {
        const body = html.slice(match.index + match[0].length).split('</select>')[0]
        node.options = [...body.matchAll(/value="([^"]*)"/g)].map(m => m[1])
        Object.defineProperty(node, 'value', {
          get() { return this.selected || '' },
          set(v) { this.selected = this.options.includes(v) ? v : '' },
        })
      }
    }
  }
  get innerHTML() { return this.html }
  querySelector(id) { if (!this.nodes.has(id)) this.nodes.set(id, new Element()); return this.nodes.get(id) }
  querySelectorAll() { return [] }
  addEventListener(type, fn) { this.handlers[type] = fn }
  remove() { this.isConnected = false }
}

async function setup({stations = [{id: '1106', name_zh: '龍洞', area_name: '東北部', station_type: '浮標'}], temperature = async () => ({sea_temp_c: 26.5})} = {}) {
  const calls = []
  let overlay
  const panel = new Element()
  const context = vm.createContext({
    URLSearchParams, console,
    window: {addEventListener() {}},
    document: {createElement: () => new Element(), body: {appendChild(el) { overlay = el }}},
    FormData: class { constructor(form) { this.data = form.data } get(k) { return this.data[k] ?? null } getAll(k) { return this.data[k] || [] } },
  })
  vm.runInContext(source.replace('export function createReefData', 'function createReefData'), context)
  const app = context.createReefData({onCount() {}, apiFetch: async (path, options) => {
    calls.push({path, options})
    if (path.endsWith('/stations')) { if (stations instanceof Error) throw stations; return stations }
    if (path.endsWith('/sites')) return [{id: 1, name_zh: '樣點 A', cwa_station_id: '1106'}, {id: 2, name_zh: '樣點 B'}, {id: 3, name_zh: '樣點 C', cwa_station_id: '1200'}]
    if (path.includes('/sea-temp?')) return temperature(path)
    if (options?.method === 'POST') return {event: {id: 9}}
    if (path.endsWith('/events/9')) return {event: {id: 9, event_time: '09:00'}, transects: []}
    return []
  }})
  await app.load()
  app.render(panel)
  await panel.querySelector('#rd-create-event').handlers.click()
  return {overlay, calls, panel, node: id => overlay.querySelector(`#${id}`)}
}

test('station array/id/name and sea_temp_c populate the form and submitted event', async () => {
  const {overlay, node, calls, panel} = await setup()
  assert.match(panel.innerHTML, /href="\/admin\/reef-activities.html"/)
  assert.doesNotMatch(overlay.innerHTML, /<select name="cwa_station_id"/)
  assert.equal(node('rd-create-station').value, '')
  assert.equal(node('rd-fetch-temp-btn').disabled, true)
  assert.match(overlay.innerHTML, /type="submit"[^>]*>建立場次/)
  node('rd-create-site').value = '1'
  node('rd-create-site').handlers.change()
  await tick()
  assert.equal(node('rd-create-station').value, '1106')
  assert.equal(node('rd-create-station-display').value, '龍洞 (1106)')
  assert.equal(node('rd-create-water-temp').value, '26.5')
  assert.match(calls.find(c => c.path.includes('/sea-temp?')).path, /station_id=1106/)
  const form = node('rd-dialog-form')
  form.data = {site_id: '1', survey_date: '2026-11-04', event_time: '09:00', depth_m: '5', methods: ['line'], fish_size_mode: 'split', cwa_station_id: node('rd-create-station').value, water_temp_c: node('rd-create-water-temp').value}
  await form.onsubmit({preventDefault() {}})
  const sent = calls.find(c => c.options?.method === 'POST').options.body
  assert.equal(sent.cwa_station_id, '1106')
  assert.equal(sent.water_temp_c, 26.5)
  assert.equal(overlay.isConnected, false)
})

test('switching to an unlinked site clears station, automatic temperature and pending results', async () => {
  let resolve
  const {node} = await setup({temperature: () => new Promise(r => { resolve = r })})
  node('rd-create-site').value = '1'
  node('rd-create-site').handlers.change()
  node('rd-create-site').value = '2'
  node('rd-create-site').handlers.change()
  resolve({sea_temp_c: 28})
  await tick()
  assert.equal(node('rd-create-station').value, '')
  assert.equal(node('rd-create-water-temp').value, '')
  assert.equal(node('rd-fetch-temp-btn').disabled, true)
  assert.match(node('rd-cwa-station-hint').textContent, /尚未設定/)
})

test('only the selected site station is shown and queried, even when other stations exist', async () => {
  const {node, calls, overlay} = await setup({stations: [{id: '1106', name_zh: '龍洞'}, {id: '1200', name_zh: '另一測站'}]})
  assert.doesNotMatch(overlay.innerHTML, /另一測站/)
  for (const [site, id, name] of [['1', '1106', '龍洞'], ['3', '1200', '另一測站']]) {
    node('rd-create-site').value = site
    node('rd-create-site').handlers.change()
    await tick()
    assert.equal(node('rd-create-station').value, id)
    assert.equal(node('rd-create-station-display').value, `${name} (${id})`)
    assert.match(calls.at(-1).path, new RegExp(`station_id=${id}`))
  }
})

test('unavailable mapped station never falls back to an unrelated station', async () => {
  const {node, calls} = await setup({stations: [{id: '1200', name_zh: '另一測站'}]})
  node('rd-create-site').value = '1'
  node('rd-create-site').handlers.change()
  await tick()
  assert.equal(node('rd-create-station').value, '')
  assert.equal(node('rd-create-station-display').value, '對應測站目前不可用')
  assert.equal(node('rd-fetch-temp-btn').disabled, true)
  assert.equal(calls.some(c => c.path.includes('/sea-temp?')), false)
})

test('automatic temperatures are cleared on site change; manual values are retained', async () => {
  const {node} = await setup()
  const changeSite = async value => { node('rd-create-site').value = value; node('rd-create-site').handlers.change(); await tick() }
  await changeSite('1')
  assert.equal(node('rd-create-water-temp').value, '26.5')
  await changeSite('2')
  assert.equal(node('rd-create-water-temp').value, '')
  node('rd-create-water-temp').value = '24.0'
  await changeSite('1')
  assert.equal(node('rd-create-water-temp').value, '24.0')
  assert.match(node('rd-cwa-temp-hint').textContent, /保留手動/)
})

test('empty stations and load errors are visible without blocking event creation', async () => {
  for (const [stations, message] of [[[], /目前沒有可用測站/], [null, /目前沒有可用測站/], [new Error('連線失敗'), /無法載入測站清單：連線失敗/]]) {
    const {overlay} = await setup({stations})
    assert.match(overlay.innerHTML, message)
    assert.match(overlay.innerHTML, />建立場次</)
  }
})

test('no observation leaves temperature empty and reports missing data', async () => {
  const {node} = await setup({temperature: async () => ({sea_temp_c: null})})
  node('rd-create-site').value = '1'
  node('rd-create-site').handlers.change()
  await tick()
  assert.equal(node('rd-create-water-temp').value, '')
  assert.match(node('rd-cwa-temp-hint').textContent, /無該日海溫記錄/)
})
