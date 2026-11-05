const esc = v => String(v ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;").replaceAll('"', "&quot;")

export function stationDescription(s) {
  const parts = [s.is_active === false ? "停用" : s.has_sea_temperature ? "可提供海溫" : "尚無海溫紀錄"]
  if (s.latitude == null || s.longitude == null) parts.push("測站座標未提供")
  else if (s.availability === "missing_site_coordinates") parts.push("樣點尚未設定座標，無法計算距離")
  if (s.distance_km != null) parts.push(`${s.distance_km.toFixed(1)} km`)
  return parts.join(" · ")
}

export function chartDefaults(e, options = {}) {
  const order = ['substrate_depth','taxa_density','rare_history','live_coral_history','impact_history','bleaching_history'].indexOf(e.chart_key)
  return { is_visible: true, sort_order: Math.max(0, order),
    year_from: options.years?.[0] ?? null, year_to: options.years?.at(-1) ?? null,
    default_depth_m: options.depths?.[0] ?? null }
}

function numberField(key, label, value, fallback) {
  return `<div class="form-field"><label>${esc(label)}<input name="${key}" type="number" step="any" value="${esc(value ?? fallback)}" data-auto-value="${esc(fallback)}"></label><label class="sw-auto"><input type="checkbox" name="auto_${key}" ${value == null ? "checked" : ""}>自動依資料更新</label></div>`
}
function choices(name, label, choices, values) {
  const selected = values == null ? choices.map(x => String(x.value)) : values.map(String)
  const available = [...choices]
  for (const value of selected) if (!available.some(c => String(c.value) === value)) available.push({value, label: `原設定 ${value}（目前無資料）`})
  return `<fieldset class="sw-choices pc-wide"><legend>${label}</legend><label class="sw-auto"><input name="auto_${name}" type="checkbox" ${values == null ? "checked" : ""}>自動包含全部項目（含日後新增資料）</label><div class="pc-checks">${available.map(c => `<label><input type="checkbox" name="${name}" value="${esc(c.value)}" ${selected.includes(String(c.value)) ? "checked" : ""}><span>${esc(c.label)}</span></label>`).join("") || '<p class="meta-line">目前沒有已發布的對應觀測資料</p>'}</div></fieldset>`
}
export function chartFields(c, e, options = {}) {
  const defaults = chartDefaults(e, options)
  return `<label class="form-field"><span>顯示</span><select name="is_visible"><option value="" ${c.is_visible == null ? "selected" : ""}>自動（顯示）</option><option value="true" ${c.is_visible === true ? "selected" : ""}>顯示</option><option value="false" ${c.is_visible === false ? "selected" : ""}>隱藏</option></select></label>` +
    [["sort_order","順序"],["year_from","起始年份"],["year_to","結束年份"],["default_depth_m","初始深度（公尺）"]].map(([k,l]) => numberField(k,l,c[k],defaults[k])).join("") +
    (['taxa_density','rare_history'].includes(e.chart_key) ? choices("taxon_ids","顯示物種",options.taxa || [],c.taxon_ids) : choices("category_keys","顯示項目",options.categories || [],c.category_keys)) +
    `<p class="meta-line pc-wide">數值依本站已發布資料帶入；勾選「自動」會持續跟隨資料更新。取消全部項目表示不顯示任何系列。</p>`
}
export function bindChartFields(form) {
  for (const el of form.querySelectorAll('input[type="number"]')) el.addEventListener("input", () => { form.elements[`auto_${el.name}`].checked = false })
  for (const el of form.querySelectorAll('input[name="category_keys"], input[name="taxon_ids"]')) el.addEventListener("change", () => { form.elements[`auto_${el.name}`].checked = false })
  for (const el of form.querySelectorAll('.sw-auto input')) el.addEventListener("change", () => {
    if (!el.checked) return
    const name = el.name.slice(5)
    for (const input of form.querySelectorAll(`[name="${name}"]`)) {
      if (input.type === "checkbox") input.checked = true
      else input.value = input.dataset.autoValue ?? ""
    }
  })
}
export function collectChartFields(data, previous) {
  const out = {...previous}
  for (const k of ['sort_order','year_from','year_to','default_depth_m']) out[k] = data.has(`auto_${k}`) || data.get(k) === '' ? null : Number(data.get(k))
  out.is_visible = data.get('is_visible') === '' ? null : data.get('is_visible') === 'true'
  const name = ['taxa_density','rare_history'].includes(previous.chart_key) ? 'taxon_ids' : 'category_keys'
  out[name] = data.has(`auto_${name}`) ? null : data.getAll(name).map(v => name === 'taxon_ids' ? Number(v) : String(v))
  return out
}
