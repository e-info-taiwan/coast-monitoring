import test from 'node:test'
import assert from 'node:assert/strict'
import { substrateDisplay } from '../substrate-display.mjs'

test('OT uses printed handboard 10, never the mutable database numeric code', () => {
  for (const raw of [undefined, '', 'OT', 'ot']) {
    assert.equal(substrateDisplay({substrate_code: 'OT', raw_input_code: raw, numeric_code: 0}).token, '10')
  }
})

test('preserve recorded numeric tokens, including explicit zero, without changing the observation', () => {
  for (const raw of ['10', '0', 0, 10, '10.0']) {
    const p = {substrate_code: 'OT', raw_input_code: raw}
    const before = {...p}
    assert.equal(substrateDisplay(p).token, String(raw))
    assert.match(substrateDisplay(p).note, /原始輸入/)
    assert.deepEqual(p, before)
  }
})

test('NA is never a recorded zero and stale raw tokens do not replace corrected categories', () => {
  for (const raw of ['0', 0, '10', '', 'NA']) assert.equal(substrateDisplay({substrate_code: 'NA', raw_input_code: raw}).token, 'NA')
  assert.equal(substrateDisplay({substrate_code: 'RC', raw_input_code: '10'}).token, '6')
})

test('fallback is identified as a legend, not claimed to be the original input', () => {
  assert.match(substrateDisplay({substrate_code: 'OT'}).note, /未保存原始數字/)
  assert.match(substrateDisplay({substrate_code: 'OT', raw_input_code: 'OT'}).note, /手板代碼對照/)
})

test('hard coral forms and mud codes remain distinct', () => {
  assert.equal(substrateDisplay({substrate_code: 'HC-a', raw_input_code: 'A'}).token, 'A')
  assert.equal(substrateDisplay({substrate_code: 'HC-b'}).token, 'b')
  assert.equal(substrateDisplay({substrate_code: 'SI(NIA)'}).token, '94')
  assert.equal(substrateDisplay({substrate_code: 'UNKNOWN'}).token, 'UNKNOWN')
})
