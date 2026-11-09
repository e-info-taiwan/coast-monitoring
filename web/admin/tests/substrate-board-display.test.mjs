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
    .replace('return {\n    load,', 'return {\n    board(points, editing) { s.editing = editing; return substrateBoard(points, "surface") },\n    load,')
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
    assert.match(render([point(raw)], true), new RegExp(`<option value="OT" selected>${raw === '0' ? '0' : '10'} OT</option>`))
  }
  const html = render([{...point('0'), substrate_code: 'NA'}], false)
  assert.match(html, /底質：NA[^>]*>NA<\/span>/)
})
