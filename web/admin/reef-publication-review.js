const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))
const starts = [0,25,50,75]
const morphology = {'HC-a':'a','HC-b':'b','HC-c':'c','SI(HC-a)':'9a','SI(HC-b)':'9b','SI(HC-c)':'9c'}
const value = v => v === null || v === undefined || v === '' ? '<span class="pr-missing">未填</span>' : esc(v)
function table(title, heads, rows) {
 return `<div class="pr-scroll" tabindex="0" role="region" aria-label="${esc(title)}"><table class="pr-table"><caption>${esc(title)}</caption><thead><tr>${heads.map(h=>`<th scope="col">${esc(h)}</th>`).join('')}</tr></thead><tbody>${rows.join('')}</tbody></table></div>`
}
function board(t, layer, numeric) {
 const points = (t.points || []).filter(p=>p.substrate_layer===layer)
 const cells = new Map(points.map(p=>[`${p.segment}:${p.position_m}`,p]))
 return `<div class="pr-scroll" tabindex="0" role="region" aria-label="${layer==='surface'?'表面':'泥下'}底質手板"><table class="pr-table pr-board"><caption>${layer==='surface'?'表面':'泥下'}底質 · 數字與代碼</caption><thead><tr>${starts.map((s,i)=>`<th colspan="4" scope="colgroup">子樣區 ${i+1}<small>${s}–${s+19.5} m</small></th>`).join('')}</tr><tr>${starts.map(s=>`<th colspan="2">左欄 · ${s}–${s+9.5} m</th><th colspan="2">右欄 · ${s+10}–${s+19.5} m</th>`).join('')}</tr><tr>${starts.map(()=>'<th>m</th><th>數字／代碼</th><th>m</th><th>數字／代碼</th>').join('')}</tr></thead><tbody>${Array.from({length:20},(_,r)=>`<tr>${starts.map((s,i)=>[0,1].map(c=>{const pos=s+c*10+r/2,p=cells.get(`${i+1}:${pos}`);return `<th scope="row">${pos}</th><td>${p ? `${value(p.substrate_code==='NA'?'NA（未記錄）':morphology[p.substrate_code] ?? numeric.get(p.substrate_code) ?? p.substrate_code)}<small>${esc(p.substrate_code)}${p.is_bleached===true?' · 白化':p.is_bleached===false?' · 無白化':''}</small>` : value(null)}</td>`}).join('')).join('')}</tr>`).join('')}</tbody></table></div>`
}
export function renderPublicationReview(detail, codes) {
 const numeric = new Map(codes.map(c => [c.code, c.numeric_code]))
 const e=detail.event || {}
 let html=`<dl class="pr-meta">${[['樣點',e.site_name],['日期',e.survey_date],['時間',e.event_time],['深度',e.depth_m==null?null:`${e.depth_m} m`],['活動',e.activity_name],['缺值原因',e.missing_reason]].map(([k,v])=>`<div><dt>${k}</dt><dd>${value(v)}</dd></div>`).join('')}</dl>`
 for(const t of detail.transects || []) {
  html+=`<section class="pr-transect"><h4>${esc({line:'底質調查',belt_fish:'魚類調查',belt_invert:'無脊椎與環境衝擊調查'}[t.method] || t.method)}</h4><p>記錄者：${(t.participants||[]).map(p=>esc(p.name_zh||p.name_en||'未填')).join('、') || '未填'}</p><p>水溫：${value(t.water_temp_c)} °C · 能見度：${value(t.visibility_min_m)}–${value(t.visibility_max_m)} m</p>`
  if(t.method==='line') {
   html+=board(t,'surface',numeric)
   if((t.points||[]).some(p=>p.substrate_layer==='down')) html+=board(t,'down',numeric)
   if((t.derived_bleaching||[]).length) html+=table('逐格白化統計',['子樣區','HC 白化點數','SC 白化點數'],t.derived_bleaching.map(b=>`<tr><th>${esc(b.segment)}</th><td>${value(b.derived_hc_bleached_point_count)}</td><td>${value(b.derived_sc_bleached_point_count)}</td></tr>`))
   html+=table('歷史白化紀錄',['子樣區','HC 白化點數','SC 白化點數'],(t.bleaching||[]).map(b=>`<tr><th>${esc(b.segment)}</th><td>${b.hc_record_status==='not_recorded'?'NA（未記錄）':value(b.hc_bleached_count)}</td><td>${b.sc_record_status==='not_recorded'?'NA（未記錄）':value(b.sc_bleached_count)}</td></tr>`))
  }
  for(const [key,title] of [['belt','生物數量'],['impacts','環境衝擊']]) {
   const groups=new Map()
   for(const row of t[key]||[]) {
    const k=JSON.stringify([row.taxon_id ?? row.impact_type_id,row.size_class])
    if(!groups.has(k)) groups.set(k,{label:`${row.name_zh||row.name_en||''}${row.size_class?`（${row.size_class}）`:''}`,rows:new Map()})
    groups.get(k).rows.set(row.segment,row)
   }
   if(groups.size) html+=table(title,['項目',...starts.map((_,i)=>`子樣區 ${i+1}`)],[...groups.values()].map(g=>`<tr><th scope="row">${esc(g.label)}</th>${starts.map((_,i)=>{const r=g.rows.get(i+1);return `<td>${!r?value(null):r.record_status==='not_recorded'?'NA（未記錄）':value(key==='belt'?r.count:r.raw_value)}</td>`}).join('')}</tr>`))
  }
  html+=`<p>備註：${value(t.comments)}</p><p>RKC 說明：${value(t.rkc_bleaching_note)}</p><p>其他損害說明：${value(t.other_damage_note)}</p></section>`
 }
 return html
}
