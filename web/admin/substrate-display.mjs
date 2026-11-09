// Handboard notation is independent of substrate_type.numeric_code.
// Older imports may retain the displayed English code rather than the input
// number; use the printed handboard legend in that case, not the DB number.
const handboard = {
  HC: '1', SC: '2', RKC: '3', NIA: '4', SP: '5', RC: '6', RB: '7',
  SD: '8', SI: '9', OT: '10', 'HC-a': 'a', 'HC-b': 'b', 'HC-c': 'c',
  'SI(HC)': '91', 'SI(SC)': '92', 'SI(RKC)': '93', 'SI(NIA)': '94',
  'SI(SP)': '95', 'SI(RC)': '96', 'SI(RB)': '97', 'SI(SD)': '98',
  'SI(HC-a)': '9a', 'SI(HC-b)': '9b', 'SI(HC-c)': '9c',
}
const inputCodes = new Map(Object.entries(handboard).map(([code, token]) => [token, code]))
inputCodes.set('0', 'OT')

export function substrateDisplay(point) {
  const code = point.substrate_code
  const raw = String(point.raw_input_code ?? '').trim()
  // Never make an NA observation look like a recorded zero. An edited code
  // must also not redisplay a stale raw number belonging to its old category.
  const alias = raw.toLowerCase().replace(/^(\d+)\.0+$/, '$1')
  const original = code !== 'NA' && inputCodes.get(alias) === code
  const token = code === 'NA' ? 'NA' : original ? raw : handboard[code] ?? code
  const note = original ? `原始輸入：${raw}` : raw ? `原始紀錄：${raw}；手板代碼對照：${token}` : `手板代碼對照：${token}（未保存原始數字）`
  return { token, note }
}
