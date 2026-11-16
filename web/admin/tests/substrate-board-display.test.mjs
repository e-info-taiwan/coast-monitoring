import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'
import { substrateDisplay } from '../substrate-display.mjs'

const source = await readFile(new URL('../reef-data.js', import.meta.url), 'utf8')
async function board() {
  const context = vm.createContext({ substrateDisplay, window: {addEventListener() {}} })
  const testSource = source.replace(/^import .*substrate-display\.mjs.*$/m, '')
    .replace('export function createReefData', 'function createReefData')
    .replace('return {\n    load,', 'return {\n    board(points, editing, mode = "raw", pointMarks = false) { s.editing = editing; s.boardMode = mode; return substrateBoard(points, "surface", pointMarks) },\n    load,')
  vm.runInContext(testSource, context)
  const app = context.createReefData({onCount() {}, apiFetch: async path => path.endsWith('/codes') ? [
    {code: 'OT', numeric_code: 0, name_zh: '其他', is_active: true},
    {code: 'NA', numeric_code: -1, name_zh: '未記錄', is_active: true},
  ] : []})
  await app.load()
  return app.board
}
const point = raw => ({id: 61, segment: 3, position_m: 61, substrate_code: 'OT', raw_input_code: raw})

test('production handboard displays BeautyCave 61m as 10 even when dictionary OT=0', async () => {
  const render = await board()
  for (const raw of ['OT', '10', undefined]) {
    const html = render([point(raw)], false)
    assert.match(html, /第 3 段 61 m 底質：OT · 其他[^>]*>10<\/span>/)
    assert.doesNotMatch(html, /底質：OT[^>]*>0<\/span>/)
  }
})

test('edit selection remains canonical OT and preserves explicit original 0 or 10', async () => {
  const render = await board()
  for (const raw of ['0', '10', 'OT']) {
    assert.match(render([point(raw)], true), new RegExp(`<option value="OT"[^>]* selected>${raw === '0' ? '0' : '10'} OT</option>`))
  }
  const html = render([{...point('0'), substrate_code: 'NA'}], false)
  assert.match(html, /底質：NA[^>]*>NA<\/span>/)
})

test('raw/code tabs retain original tokens, canonical edit values and bleaching labels', async () => {
  const render = await board()
  const points = [point('0'), {...point('A'), id: 62, position_m: 61.5, substrate_code: 'HC-a', health_status: 'bleached'}]
  const before = JSON.stringify(points)
  for (const mode of ['raw', 'code']) {
    const html = render(points, false, mode, true)
    assert.match(html, /data-board-raw="0" data-board-code="OT"/)
    assert.match(html, /data-board-raw="A" data-board-code="HC-a"/)
    assert.match(html, mode === 'raw' ? />0<\/span>/ : />OT<\/span>/)
    assert.match(html, mode === 'raw' ? />A<\/span><small>已白化<\/small>/ : />HC-a<\/span><small>已白化<\/small>/)
    const edit = render(points, true, mode, true)
    assert.match(edit, /<option value="OT" data-board-raw="0 OT" data-board-code="OT" selected>/)
    assert.match(edit, mode === 'raw' ? /selected>0 OT<\/option>/ : /selected>OT<\/option>/)
  }
  assert.equal(JSON.stringify(points), before)
})

test('corrected categories ignore stale raw tokens and health states remain distinct', async () => {
  const render = await board()
  for (const [health, label] of [['bleached', '已白化'], ['not_bleached', '未白化'], [null, '未標記']]) {
    const html = render([{...point('10'), substrate_code: 'HC', health_status: health}], false, 'raw', true)
    assert.match(html, new RegExp(`data-board-raw="1" data-board-code="HC">1</span><small>${label}</small>`))
  }
})
