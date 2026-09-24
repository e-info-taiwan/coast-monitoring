'use strict';
/* Offline, framework-free prototype. All persisted values are escaped on render. */
const C = window.REEF;
const KEY = 'teia-reefcheck-records-v2';
const $ = s => document.querySelector(s);
const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const starts = [0,25,50,75];
const points = starts.flatMap((start,s) => Array.from({length:40},(_,i)=>({key:String(start+i/2),s:s+1,p:start+i/2})));
function defaultSegment(){return window.innerWidth<=1366?1:0;}
let records = readRecords(), state = fresh(), step = 0, sheet = 'line', segment = defaultSegment(), reviewSegment = defaultSegment(), layer = 'surface';
let timer, storageFailed = false, lastCell = null, saving = false;
function fresh(meta = {}) {
  return {id:globalThis.crypto?.randomUUID?.() || `draft-${Date.now()}-${Math.random().toString(36).slice(2)}`,methods:[],meta:{site:'',date:'',time:'',depth:'',temperature:'',visibility:'',leader:'',scientist:'',...meta},recorders:{line:'',fish:'',invert:''},line:{},down:{},mud:false,bleaching:false,bleach:{},fish:{},invert:{},rare:{},impact:{},custom:{fish:[],invert:[],rare:[]},trashMode:'level',notes:'',missingReason:'',status:'draft',step:0,updated:''};
}
function readRecords(){try{const data=JSON.parse(localStorage.getItem(KEY)||'[]');return Array.isArray(data)?data.filter(x=>x?.id&&Array.isArray(x.methods)&&x.meta&&x.line&&x.custom):[];}catch{return [];}}
function persist(change=true){
  if(change){state.status='draft';state.ack=false;delete state.simulatedSave;}
  state.step=step;state.updated=new Date().toISOString();
  const i=records.findIndex(x=>x.id===state.id);
  if(i<0)records.push(structuredClone(state));else records[i]=structuredClone(state);
  try{localStorage.setItem(KEY,JSON.stringify(records));storageFailed=false;$('#save-status').textContent=state.simulatedSave?'模擬儲存成功 · 本機留存':'草稿已儲存在此裝置';$('#save-status').classList.remove('storage-warning');}
  catch{storageFailed=true;$('#save-status').textContent='儲存失敗，請下載草稿';$('#save-status').classList.add('storage-warning');toast('此瀏覽器無法儲存，請下載草稿備份。');}
  $('#draft-count').textContent=records.length;
}
function toast(text){$('#toast').textContent=text;$('#toast').hidden=false;clearTimeout(timer);timer=setTimeout(()=>$('#toast').hidden=true,4200);}
function methodList(){return ['line','fish','invert'].filter(k=>state.methods.includes(k));}
function sheetList(){return methodList().flatMap(k=>k==='invert'?['invert','rare','impact']:[k]);}
function sheetName(k){return {line:'底質',fish:'魚類',invert:'無脊椎',rare:'罕見生物',impact:'環境衝擊'}[k];}
function siteName(){return C.sites.find(s=>s[2]===state.meta.site)?.[1] || '尚未選擇樣點';}
function eventId(){const m=state.meta;return m.site&&m.date&&m.time&&m.depth!==''?`${m.site}_${m.date.replaceAll('-','_')}_${m.time.replace(':','-')}_${Number(m.depth)}m`:'';}
function eventKey(){const m=state.meta;return `${m.site}|${m.date}|${m.time.replace(':','-')}|${Number(m.depth)}m`;}
function duplicateRecord(){
  const currentIndex=records.findIndex(r=>r.id===state.id);
  return records.find((r,i)=>r.id!==state.id&&r.meta.site===state.meta.site&&r.meta.date===state.meta.date&&r.meta.time===state.meta.time&&Number(r.meta.depth)===Number(state.meta.depth)&&
    (i<currentIndex||['line','fish','invert','rare','impact'].some(k=>Object.values(r[k]||{}).some(v=>v!==''))));
}
function button(text,action,extra='',kind=''){return `<button type="button" class="button ${kind}" data-action="${action}" ${extra}>${text}</button>`;}
function intro(n,title,description){return `<section class="page-intro"><p class="eyebrow">REEF CHECK 志工填報</p><div class="intro-row"><h1>${title}</h1><span class="badge">步驟 ${String(Math.min(n,4)).padStart(2,'0')} / 04</span></div><p>${description}</p></section>`;}
function render(){
  lastCell=null;
  $('#draft-count').textContent=records.length;
  $('#workflow').innerHTML=[['選擇探查','拿出這次的手板'],['潛水資訊','確認同一支氣瓶'],['手板輸入','照著表格逐格記錄'],['檢查與完成','']].map(([a,b],i)=>`<button class="step ${step===i?'active':step>i?'done':''}" data-action="step" data-step="${i}" ${step===i?'aria-current="step"':''}><span class="step-num">${step>i?'✓':i+1}</span><span><b>${a}</b>${b?`<small>${b}</small>`:''}</span></button>`).join('');
  $('#main').innerHTML=step===0?chooseView():step===1?metaView():step===2?boardView():step===3?reviewView():completeView();
  $('#workflow').inert=saving;$('.topbar').inert=saving;
  updateCounters();
}
function go(n){
  if(n>0&&!state.methods.length){toast('先選擇至少一種探查。');return;}
  if(n>1){const errors=metaErrors();if(errors.length){step=1;render();$('#meta-error').textContent=errors.join('；');$('#metadata-form').reportValidity();return;}
    const duplicate=duplicateRecord();
    if(duplicate){modal('這支氣瓶已經有紀錄',`<p>樣點、日期、開始時間與深度都相同。請開啟原紀錄增加探查項目；若是另一支氣瓶，請先核對時間與深度。</p><div class="actions">${button('返回核對資訊','duplicate-back')}${button('開啟原紀錄','resume',`data-id="${esc(duplicate.id)}"`,'primary')}</div>`);return;}
  }
  step=n;if(step===2&&!sheetList().includes(sheet))sheet=sheetList()[0];
  persist(false);render();$('#main').focus();window.scrollTo({top:0});
}
function miniTable(k){return `<table class="mini-sheet" aria-hidden="true"><thead><tr><th>${k==='line'?'m':'類群'}</th><th>1 段</th><th>2 段</th><th>3 段</th><th>4 段</th></tr></thead><tbody>${[0,1,2].map((r)=>`<tr><td>${k==='line'?r*.5:k==='fish'?['蝶魚','石鱸','笛鯛'][r]:['蝦','海膽','海參'][r]}</td>${[0,1,2,3].map(()=>'<td>—</td>').join('')}</tr>`).join('')}</tbody></table>`;}
function chooseView(){
  const other=records.filter(r=>r.id!==state.id&&r.status==='draft');
  return intro(1,'這次，要記錄哪一種探查？','拿出這支氣瓶的手板，選擇你要輸入的項目。同一支氣瓶可以一次填多種。')+
    (other.length?`<div class="saved-banner"><span>還有 ${other.length} 份草稿，隨時可以接著填。</span>${button('繼續草稿 →','drafts','','quiet')}</div>`:'')+
    `<div class="method-grid">${Object.entries(C.methods).map(([k,m],i)=>`<label class="method-card"><input type="checkbox" name="method" value="${k}" ${state.methods.includes(k)?'checked':''} aria-label="${m.name}"><span class="method-symbol" aria-hidden="true">0${i+1}</span><h2>${m.name}</h2><span class="eyebrow">${m.en}</span><p>${m.description}</p>${miniTable(k)}<span class="mini-caption">${m.caption}</span></label>`).join('')}</div>
    <div class="callout"><span class="callout-icon">↳</span><div><b>共用欄位</b><p>樣點、日期、氣瓶開始時間與深度只填一次；不同氣瓶請建立另一份紀錄。</p></div></div>
    <div class="actions"><span id="selection-count" class="selection-count">已選 ${state.methods.length} 種探查</span>${button('下一步：潛水資訊 <span class="arrow">→</span>','step','data-step="1"','primary')}</div>`;
}
function field(label,key,type='text',required=false,hint='',attrs=''){
  return `<label class="field">${label}${required?' <span class="required">必填</span>':''}<input name="${key}" data-meta="${key}" type="${type}" value="${esc(state.meta[key])}" ${required?'required':''} ${attrs}>${hint?`<small>${hint}</small>`:''}</label>`;
}
function depthChoice(){
  if(state.depthChoice==='other')return 'other';
  const v=state.meta.depth;
  return v===''?'':Number(v)===5?'5':Number(v)===10?'10':'other';
}
function depthControl(){const choice=depthChoice();return `<fieldset class="depth-control"><legend>調查深度（m） <span class="required">必填</span></legend><div class="depth-tags">${[['5','5m'],['10','10m'],['other','其他']].map(([v,label])=>`<label class="depth-tag"><input type="radio" name="depth-choice" value="${v}" ${choice===v?'checked':''} required><span>${label}</span></label>`).join('')}</div>${choice==='other'?`<label class="field depth-custom">實際深度（m）<input name="depth" data-meta="depth" type="number" inputmode="decimal" min="0.1" max="100" step="0.1" value="${esc(state.meta.depth)}" placeholder="例如 3.8" required></label>`:''}<small>依手板選擇實際深度；不是 5m 或 10m 時，請選「其他」。</small></fieldset>`;}
function selectDepth(choice){
  if(!['5','10','other'].includes(choice))return;
  const previous=depthChoice();
  state.meta.depth=choice==='other'?(previous==='other'?state.meta.depth:''):choice;
  state.depthChoice=choice;
  persist();
}
function metaView(){return intro(2,'先確認，這是哪一支氣瓶。','請抄錄手板上的資料。多種探查共用同一支氣瓶的開始時間，避免把一趟潛水拆成兩份。')+
  `<form id="metadata-form"><section class="panel"><div class="panel-heading"><div><h2>潛水與樣點</h2><p>本次包含：${methodList().map(k=>C.methods[k].name).join('、')}</p></div><span class="badge">01 / 基本資料</span></div><div class="fields"><label class="field wide">調查樣點 <span class="required">必填</span><select required name="site" data-meta="site"><option value="">選擇手板上的樣點</option>${[...new Set(C.sites.map(s=>s[0]))].map(region=>`<optgroup label="${region}">${C.sites.filter(s=>s[0]===region).map(s=>`<option value="${esc(s[2])}" ${state.meta.site===s[2]?'selected':''}>${esc(s[1])} · ${esc(s[2])}</option>`).join('')}</optgroup>`).join('')}</select><small>依現有樣點清單；找不到時請先存草稿，與活動窗口確認名稱。</small></label>
  ${field('調查日期','date','date',true,'以手板上的實際日期為準。')}${field('這支氣瓶的開始時間','time','time',true,'Line 與 Belt 請用同一支氣瓶的開始時間。')}<div id="depth-control">${depthControl()}</div>${field('水溫（°C）','temperature','number',false,'未記錄可留白。','min="0" max="45" step="0.1" placeholder="例如 27.5"')}${field('能見度（m）','visibility','number',false,'未記錄可留白。','min="0" max="100" step="0.1" placeholder="例如 8"')}</div><div class="event-preview" id="event-preview">${eventId()?`氣瓶識別：${esc(eventId())}`:'填好樣點、日期、時間與深度後，自動產生氣瓶識別。'}</div></section>
  <section class="panel"><div class="panel-heading"><div><h2>這份手板，是誰記錄的？</h2><p>每種探查可填不同記錄者；多人請以「、」分隔。</p></div><span class="badge">02 / 記錄人員</span></div><div class="fields">${methodList().map(k=>`<label class="field">${C.methods[k].short}記錄者 <span class="required">必填</span><input name="recorder-${k}" data-recorder="${k}" value="${esc(state.recorders[k])}" required placeholder="姓名，可填多人"></label>`).join('')}${field('隊長','leader')}${field('科學指導員','scientist')}</div></section><p class="form-error" id="meta-error" role="alert"></p><div class="actions">${button('← 選擇探查','step','data-step="0"')}<button class="button primary" type="submit">開始填寫手板 <span class="arrow">→</span></button></div></form>`;}
function metaErrors(){const m=state.meta,e=[];if(!C.sites.some(s=>s[2]===m.site))e.push('請選擇樣點');if(!/^\d{4}-\d{2}-\d{2}$/.test(m.date))e.push('請填調查日期');if(!/^([01]\d|2[0-3]):[0-5]\d$/.test(m.time))e.push('請填氣瓶開始時間');if(m.depth===''||!Number.isFinite(Number(m.depth))||Number(m.depth)<=0||Number(m.depth)>100)e.push('深度請填 0.1–100 m');for(const [k,n,max] of [['temperature','水溫',45],['visibility','能見度',100]])if(m[k]!==''&&(!Number.isFinite(Number(m[k]))||Number(m[k])<0||Number(m[k])>max))e.push(`${n}請填 0–${max}`);methodList().forEach(k=>{if(!state.recorders[k].trim())e.push(`請填${C.methods[k].short}記錄者`);});return e;}
function strip(){return `<section class="survey-strip" aria-label="本支氣瓶"><div><small>樣點</small><b>${esc(siteName())}</b></div><div><small>日期</small><b>${esc(state.meta.date)}</b></div><div><small>氣瓶開始</small><b>${esc(state.meta.time)}</b></div><div><small>深度</small><b>${esc(state.meta.depth)} m</b></div>${button('修改資訊','step','data-step="1"','quiet small')}</section>`;}
function normalizeCode(value){const t=String(value).trim().toUpperCase().replaceAll('（','(').replaceAll('）',')');if(t==='-')return 'NA';if(t==='SI(SI)')return 'SI';if(/^(?:[1-9]|10)$/.test(t))return C.substrates[Number(t)-1][0];if(/^9[1-8]$/.test(t))return `SI(${C.substrates[Number(t)-91][0]})`;return t;}
function validCode(v,down=false){const t=normalizeCode(v);return down?C.substrates.slice(0,9).some(s=>s[0]===t)||t==='NA':C.substrates.some(s=>s[0]===t)||t==='NA'||/^SI\((HC|SC|RKC|NIA|SP|RC|RB|SD)\)$/.test(t);}
function normalizeNumber(v){const t=String(v).trim();return /^(na|-)$/i.test(t)?'NA':t;}
function validNumber(v,type='count'){const t=normalizeNumber(v);return t==='NA'||t!==''&&/^\d+(?:\.\d+)?$/.test(t)&&Number.isFinite(Number(t))&&Number(t)>=0&&(type==='percent'?Number(t)<=100:Number.isSafeInteger(Number(t)))&&(type!=='level'||Number(t)<=3);}
function rowsFor(k){return [...C[k].map((r,i)=>({id:String(i),name:r[0],zh:r[1],size:k==='impact'?'':r[2],type:k==='impact'?(r[3]==='trash'?state.trashMode:r[2]):'count',group:k==='impact'?r[3]:k})),...(state.custom[k]||[]).map(r=>({...r,size:'',type:'count',group:k}))];}
function activeMudPoints(){return points.filter(p=>normalizeCode(state.line[p.key]||'')==='SI');}
function sheetStats(k){
  const cells=k==='line'?points.map(p=>({v:state.line[p.key]||'',valid:v=>validCode(v)})):rowsFor(k).flatMap(r=>starts.map((_,i)=>({v:state[k][`${r.id}:${i+1}`]||'',valid:v=>validNumber(v,r.type)})));
  return {total:cells.length,filled:cells.filter(c=>c.v!=='').length,valid:cells.filter(c=>c.valid(c.v)).length,missing:cells.filter(c=>c.v==='').length,invalid:cells.filter(c=>c.v!==''&&!c.valid(c.v)).length,na:cells.filter(c=>normalizeNumber(c.v)==='NA').length};
}
function boardView(){if(!sheetList().includes(sheet))sheet=sheetList()[0];return intro(3,'輸入手板資料','白色格為輸入欄。可以分段填寫；每次輸入都會自動儲存。')+strip()+
  `<nav class="sheets-nav" aria-label="探查手板">${sheetList().map(k=>`<button class="sheet-tab ${sheet===k?'active':''}" data-action="sheet" data-sheet="${k}" ${sheet===k?'aria-current="page"':''}>${sheetName(k)}<small data-tab-count="${k}"></small></button>`).join('')}</nav><section class="board"><div class="board-head"><div><p class="eyebrow">${sheet==='line'?'LINE TRANSECT · SUBSTRATE':'BELT TRANSECT · '+sheet.toUpperCase()}</p><h2>${sheetName(sheet)}記錄手板</h2><p>${sheet==='line'?'四段 × 每段 40 點，每 0.5 m 一格':'每一列一個類群，每一欄一個段次'}</p></div><span class="progress"><strong id="filled-count"></strong> / <span id="total-count"></span> <small>已填</small></span></div>${sheet==='line'?lineBoard():beltBoard(sheet)}<div class="field-focus" id="field-focus" aria-live="polite">點選一格開始輸入；位置與欄位提示會顯示在這裡。</div></section>
  <label class="field sheet-note">本支氣瓶的補充備註<textarea data-state="notes" placeholder="例如：浪大、第三段視線較差、其他生物說明…">${esc(state.notes)}</textarea></label>
  <div class="actions sticky-actions">${button('← 潛水資訊','step','data-step="1"')}<div class="right">${button('下載草稿','export-draft','','quiet')}${button(sheetList().indexOf(sheet)<sheetList().length-1?'下一張手板 →':'檢查這份紀錄 →','next-sheet','','primary')}</div></div>`;}
function lineBoard(){
  if(!state.mud)layer='surface';const isDown=layer==='down';const displayed=segment?[segment]:[1,2,3,4];
  return `<div class="board-tools"><div class="seg-filters">${[0,1,2,3,4].map(n=>`<button type="button" class="${segment===n?'active':''}" data-action="segment" data-segment="${n}">${n?'第 '+n+' 段':'整張手板'}</button>`).join('')}</div><span>Tab / Enter：沿同欄往下</span></div>
  <div class="line-legend">${C.substrates.map((s,i)=>`<span><b>${i+1} ${s[0]}</b> ${s[1]}</span>`).join('')}<span><b>NA</b> 未記錄（需說明）</span></div>
  ${state.mud?`<div class="board-tools"><div class="seg-filters">${button('上：表面底質','layer','data-layer="surface"',layer==='surface'?'primary small':'small')}${button('下：泥下底質','layer','data-layer="down"',isDown?'primary small':'small')}</div><span>${isDown?'只填上層為 SI 的同一位置；其餘位置不需填寫。':'先填上層，再切到「下」逐格對照。'}</span></div>`:''}
  <div class="table-scroll"><table class="slate line-table ${segment?'single':''}"><caption>${isDown?'下：泥下底質':'上：表面底質'} · 先左欄由上往下，再右欄由上往下。輸入 1–10、HC 等代號；91–98 也可直接填。<span class="mobile-note">手機可切換「第 1–4 段」放大填寫；整張手板可左右捲動。</span></caption><colgroup>${displayed.map(()=>'<col class="pos"><col class="code"><col class="pos"><col class="code">').join('')}</colgroup><thead><tr>${displayed.map(s=>`<th colspan="4" scope="colgroup">SEGMENT ${s} · 第 ${s} 段<small>${starts[s-1]}–${starts[s-1]+19.5} m</small></th>`).join('')}</tr><tr>${displayed.map(()=>'<th scope="col">m</th><th scope="col">底質 ↓</th><th scope="col">m</th><th scope="col">底質 ↓</th>').join('')}</tr></thead><tbody>${Array.from({length:20},(_,r)=>'<tr>'+displayed.map(s=>[0,1].map(l=>{const p=starts[s-1]+l*10+r/2,key=String(p),v=(isDown?state.down:state.line)[key]||'',enabled=!isDown||normalizeCode(state.line[key]||'')==='SI';return `<th class="position ${l===0&&s>1?'segment-edge':''}" scope="row">${p}</th><td><input type="text" autocapitalize="characters" autocomplete="off" spellcheck="false" data-cell="${isDown?'down':'line'}" data-key="${key}" data-order="${(s-1)*40+l*20+r}" data-segment="${s}" data-position="${p}" aria-label="第 ${s} 段 ${p} m ${isDown?'泥下':'表面'}底質" value="${esc(v)}" placeholder="${enabled?'·':'—'}" ${enabled?'':'disabled'} class="${v&&!validCode(v,isDown)?'invalid':''}" ${v&&!validCode(v,isDown)?'aria-invalid="true"':''}></td>`;}).join('')).join('')+'</tr>').join('')}</tbody></table></div>
  <div class="line-bottom"><label class="toggle-row"><input type="checkbox" data-toggle="mud" ${state.mud?'checked':''}><span>這份手板有「泥下底質」<small>含分開的「上／下」手板或 XLS 分頁。SI + RC → SI(RC)；SI + SI → SI。</small></span></label><p id="mud-progress" class="inline-note"></p><label class="toggle-row"><input type="checkbox" data-toggle="bleaching" ${state.bleaching?'checked':''}><span>另外填寫 HC／SC 白化點數<small>手板有分段白化計數時勾選；白化點數不得超過該段對應的珊瑚點數。</small></span></label>${state.bleaching?bleachTable():''}</div>`;
}
function bleachTable(){return `<div class="table-scroll"><table class="slate belt-table"><caption>分段白化點數（不是百分比）</caption><thead><tr><th>底質類型</th>${starts.map((_,s)=>`<th>第 ${s+1} 段</th>`).join('')}</tr></thead><tbody>${['HC','SC'].map(code=>`<tr><th class="row-label">${code} 白化點數</th>${starts.map((_,s)=>`<td><input data-cell="bleach" data-key="${code}:${s+1}" data-order="${s}" aria-label="${code} 第 ${s+1} 段白化點數" value="${esc(state.bleach[`${code}:${s+1}`]||'')}" placeholder="·" inputmode="numeric"></td>`).join('')}</tr>`).join('')}</tbody></table></div>`;}
function beltBoard(k){const rows=rowsFor(k);return `<div class="board-tools"><span>${k==='impact'?'分級填 0–3；百分比填 0–100；未記錄填 NA。':'沒看到請填 0；未記錄填 NA；空白表示還沒填。'}</span>${button(k==='impact'?'這張手板怎麼填？':'已確認：空白格皆為 0','zero','data-sheet="'+k+'"','small')}${k==='impact'?`<label>垃圾記錄方式 <select data-trash-mode aria-label="垃圾記錄方式"><option value="level" ${state.trashMode==='level'?'selected':''}>0–3 分級（英文表）</option><option value="count" ${state.trashMode==='count'?'selected':''}>原始件數（中文手板）</option></select></label>`:''}</div>
  <div class="table-scroll"><table class="slate belt-table"><caption>${k==='impact'?'分級：0 無 · 1 低（1 件）· 2 中（2–4 件）· 3 高（5 件以上）。':k==='rare'?'罕見生物獨立記錄；其他生物請新增具名列。':'依原手板類群與體長分列；合計會自動計算。'}<span class="mobile-note">表格可左右捲動；類群名稱固定在左側。</span></caption><colgroup><col class="row-name"><col><col><col><col><col class="sum-col"></colgroup><thead><tr><th scope="col">${k==='impact'?'項目／單位':'類群／體長'}</th>${starts.map((s,i)=>`<th scope="col">第 ${i+1} 段<small>${s}–${s+20} m</small></th>`).join('')}<th scope="col">${k==='impact'?'平均':'合計'}<small>自動計算</small></th></tr></thead><tbody>${rows.map((r,ri)=>`<tr><th class="row-label" scope="row">${esc(r.zh)} ${r.size?`<span class="mini-badge">${esc(r.size)}</span>`:''}<small>${esc(r.name)}${k==='impact'?' · '+{level:'分級 0–3',count:'件',percent:'%'}[r.type]:''}</small></th>${starts.map((_,s)=>{const key=`${r.id}:${s+1}`,v=state[k][key]||'';return `<td><input type="text" inputmode="${r.type==='percent'?'decimal':'numeric'}" autocomplete="off" data-cell="${k}" data-key="${key}" data-type="${r.type}" data-order="${ri*4+s}" aria-label="${esc(r.zh+' '+r.size)} 第 ${s+1} 段${k==='impact'?' '+{level:'分級',count:'件數',percent:'百分比'}[r.type]:'數量'}" value="${esc(v)}" placeholder="·" class="${v&&!validNumber(v,r.type)?'invalid':''}"></td>`;}).join('')}<td class="row-total" data-total-sheet="${k}" data-row-id="${r.id}">—</td></tr>`).join('')}</tbody></table></div>
  ${k!=='impact'?`<div class="custom-row"><input id="custom-name" aria-label="其他${sheetName(k)}名稱" placeholder="其他${sheetName(k)}：填寫實際名稱" maxlength="80">${button('＋ 新增其他類群','add-custom','data-sheet="'+k+'"','small')}<small>每種生物各佔一列，避免都記成 Other。</small></div>${state.custom[k].map(r=>`<div class="custom-row"><span>其他：${esc(r.zh)}</span>${button('修改名稱／移除','edit-custom',`data-sheet="${k}" data-id="${r.id}"`,'small quiet')}</div>`).join('')}`:''}`;}
function updateCounters(){
  if(step!==2)return;
  for(const k of sheetList()){const s=sheetStats(k),el=$(`[data-tab-count="${k}"]`);if(el)el.textContent=`${s.filled}/${s.total}`;}
  const st=sheetStats(sheet);$('#filled-count').textContent=st.filled;$('#total-count').textContent=st.total;
  document.querySelectorAll('[data-total-sheet]').forEach(el=>{const k=el.dataset.totalSheet,r=rowsFor(k).find(r=>r.id===el.dataset.rowId);const vals=starts.map((_,i)=>state[k][`${r.id}:${i+1}`]||'');el.textContent=vals.every(v=>validNumber(v,r.type)&&v!=='NA')?String(Math.round(vals.reduce((a,b)=>a+Number(b),0)/(k==='impact'?4:1)*100)/100):'—';});
  if($('#mud-progress')){const ps=activeMudPoints(),n=ps.filter(p=>validCode(state.down[p.key]||'',true)).length;$('#mud-progress').textContent=state.mud?`上層有 ${ps.length} 個 SI 位置，泥下已填 ${n} / ${ps.length}；其餘位置沿用上層。`:'';}
}
function issueLocations(k,kind){
  const entries=k==='line'?points.map(p=>({v:state.line[p.key]||'',valid:v=>validCode(v),label:`第 ${p.s} 段 ${p.p} m`})):
    rowsFor(k).flatMap(r=>starts.map((_,s)=>({v:state[k][`${r.id}:${s+1}`]||'',valid:v=>validNumber(v,r.type),label:`${r.zh}${r.size?' '+r.size:''} 第 ${s+1} 段`})));
  const found=entries.filter(x=>kind==='missing'?x.v==='':x.v!==''&&!x.valid(x.v));return found.slice(0,3).map(x=>x.label).join('、')+(found.length>3?'等':'');
}
function issues(){const result=metaErrors().map(text=>({text,step:1}));for(const k of sheetList()){
  const st=sheetStats(k);if(st.missing)result.push({text:`${sheetName(k)}：還有 ${st.missing} 格未填（${issueLocations(k,'missing')}）。點此回填。`,sheet:k,kind:'missing'});if(st.invalid)result.push({text:`${sheetName(k)}：${st.invalid} 格無效（${issueLocations(k,'invalid')}）。點此修正。`,sheet:k,kind:'invalid'});
  if(k==='line'&&state.mud){const ps=activeMudPoints(),bad=ps.filter(p=>!validCode(state.down[p.key]||'',true));if(bad.length)result.push({text:`泥下底質：${bad.length} 個 SI 位置待填或代碼無效。`,sheet:'line',layer:'down',key:bad[0].key});}
  if(k==='line'&&state.bleaching)for(const code of ['HC','SC'])for(let s=1;s<=4;s++){const v=state.bleach[`${code}:${s}`]||'',max=points.filter(p=>p.s===s&&normalizeCode(state.line[p.key]||'')===code).length;if(!validNumber(v)||v!=='NA'&&Number(v)>max)result.push({text:`${code} 第 ${s} 段白化點数：請填 0–${max}，不可超過已記錄的 ${code} 點數。`,sheet:'line',key:`${code}:${s}`,cell:'bleach'});}
  }
  if(unknownCount()&&!state.missingReason.trim())result.push({text:'有 NA 未記錄值，請補上未記錄原因。',reason:true});
  return result;
}
function unknownCount(){let n=sheetList().reduce((n,k)=>n+sheetStats(k).na,0);if(state.methods.includes('line')){if(state.mud)n+=activeMudPoints().filter(p=>state.down[p.key]==='NA').length;if(state.bleaching)n+=Object.values(state.bleach).filter(v=>v==='NA').length;}return n;}
function reviewView(){const list=issues(),total=sheetList().reduce((n,k)=>n+sheetStats(k).total,0),filled=sheetList().reduce((n,k)=>n+sheetStats(k).valid,0);return intro(4,'檢查紀錄','確認氣瓶資訊、段次與每張手板。若原始紀錄缺值，可標記 NA 並說明，交由窗口確認。')+strip()+
  `<div class="review-grid"><div class="stat"><b>${sheetList().length}</b><small>張記錄表</small></div><div class="stat"><b>${filled}<small>/ ${total} 格通過檢查</small></b></div><div class="stat"><b>${unknownCount()}</b><small>格 NA · 未記錄</small></div></div><section class="panel"><h2>這支氣瓶的記錄</h2>${sheetList().map(k=>{const st=sheetStats(k);return `<div class="review-row"><div><b>${sheetName(k)} <span class="badge">${st.valid}/${st.total} 格</span></b><p>${k==='line'?'4 段 × 40 點'+(state.mud?' · 含泥下底質':''):rowsFor(k).length+' 個項目 × 4 段'} · 記錄者：${esc(state.recorders[k==='rare'||k==='impact'?'invert':k])}${st.na?' · '+st.na+' 格 NA':''}</p></div>${button('回手板檢查','sheet',`data-sheet="${k}"`,'small')}</div>`;}).join('')}<div class="event-preview">氣瓶識別：${esc(eventId())}</div><details class="review-detail"><summary>展開逐格對照（含最終泥下代碼）</summary>${reviewTables()}</details></section>
  ${unknownCount()?`<label class="field panel">未記錄原因 <span class="required">必填</span><textarea data-state="missingReason" placeholder="請寫明手板哪一段、哪些位置為何未記錄。">${esc(state.missingReason)}</textarea><small>NA 保留為未知；不當成 0，也不列入底質有效觀測分母。</small></label>`:''}
  <section id="review-issues">${issueMarkup(list)}</section><label class="check-ack"><input id="acknowledge" type="checkbox" ${state.ack?'checked':''}><span>我已對照原始手板，確認樣點、日期、氣瓶時間、深度與各段數值。<br><small>下一步將展示資料庫儲存流程。此為模擬模式，實際資料仍只留在本機。</small></span></label><div class="actions">${button('← 返回手板','step','data-step="2"')}<div class="right">${button('下載草稿','export-draft','','quiet')}${button(unknownCount()?'送出儲存・標示待確認':'送出儲存','complete','','primary')}</div></div><p id="complete-error" class="form-error" role="alert"></p>`;}
function issueMarkup(list){return list.length?`<div class="callout warning"><div><b>有 ${list.length} 項需要補充</b><ul class="issue-list">${list.map((x,i)=>`<li><button class="issue-link" data-action="issue" data-index="${i}">${esc(x.text)}</button></li>`).join('')}</ul></div></div>`:`<div class="review-ok">✓ 所選手板的必填格與範圍檢查通過${unknownCount()?'；NA 已附原因，仍需窗口確認':''}。</div>`;}
function canonical(p){const up=normalizeCode(state.line[p.key]||'');if(up!=='SI'||!state.mud)return up||null;const d=normalizeCode(state.down[p.key]||'');if(d==='SI')return 'SI';if(d==='NA')return 'NA';return validCode(d,true)?`SI(${d})`:null;}
function reviewValue(value,valid=true){const empty=value===null||value===undefined||value==='';return `<span class="review-value ${empty||!valid?'review-invalid':value==='NA'?'review-na':''}">${esc(empty?'未填':value)}</span>`;}
function reviewLineTable(){return `<div class="table-scroll" tabindex="0" role="region" aria-label="底質最終記錄手板，可左右捲動"><table class="slate line-table review-line"><caption>底質手板 · 最終記錄值（唯讀）<small>與輸入手板相同：每段先左欄由上往下，再右欄由上往下。SI(RC) 表示泥下為 RC；SI(SI) 記為 SI。未填與 NA 不會當成有效底質。表格可左右捲動。</small></caption><colgroup>${starts.map(()=>'<col class="pos"><col class="code"><col class="pos"><col class="code">').join('')}</colgroup><thead><tr>${starts.map((start,s)=>`<th colspan="4" scope="colgroup">SEGMENT ${s+1} · 第 ${s+1} 段<small>${start}–${start+19.5} m</small></th>`).join('')}</tr><tr>${starts.map(()=>'<th scope="col">m</th><th scope="col">底質 ↓</th><th scope="col">m</th><th scope="col">底質 ↓</th>').join('')}</tr></thead><tbody>${Array.from({length:20},(_,r)=>'<tr>'+starts.map((start,s)=>[0,1].map(l=>{const pos=start+l*10+r/2,p=points.find(p=>p.p===pos),v=canonical(p);return `<th class="position ${l===0&&s>0?'segment-edge':''}" scope="row">${pos}</th><td data-review-position="${pos}">${reviewValue(v,validCode(v??''))}</td>`;}).join('')).join('')+'</tr>').join('')}</tbody></table></div>`;}
function reviewBeltTable(k){return `<div class="table-scroll" tabindex="0" role="region" aria-label="${sheetName(k)}對照手板，可左右捲動"><table class="slate belt-table"><caption>${sheetName(k)}手板 · 記錄值（唯讀）</caption><colgroup><col class="row-name"><col><col><col><col><col class="sum-col"></colgroup><thead><tr><th scope="col">${k==='impact'?'項目／單位':'類群／體長'}</th>${starts.map((start,s)=>`<th scope="col">第 ${s+1} 段<small>${start}–${start+20} m</small></th>`).join('')}<th scope="col">${k==='impact'?'平均':'合計'}<small>自動計算</small></th></tr></thead><tbody>${rowsFor(k).map(r=>{const vals=starts.map((_,s)=>state[k][`${r.id}:${s+1}`]??''),total=vals.every(v=>validNumber(v,r.type)&&normalizeNumber(v)!=='NA')?String(Math.round(vals.reduce((n,v)=>n+Number(v),0)/(k==='impact'?4:1)*100)/100):'—';return `<tr><th class="row-label" scope="row">${esc(r.zh)} ${r.size?`<span class="mini-badge">${esc(r.size)}</span>`:''}<small>${esc(r.name)}${k==='impact'?' · '+{level:'分級 0–3',count:'件',percent:'%'}[r.type]:''}</small></th>${vals.map(v=>`<td>${reviewValue(v,validNumber(v,r.type))}</td>`).join('')}<td class="row-total">${total}</td></tr>`;}).join('')}</tbody></table></div>`;}
function reviewLinePanel(){
  const full=reviewLineTable();
  // Keep the same handboard coordinates while narrowing the view, never the data.
  let table=full;
  if(reviewSegment){
    const start=starts[reviewSegment-1];
    const head=`<thead><tr><th colspan="4" scope="colgroup">SEGMENT ${reviewSegment} · 第 ${reviewSegment} 段<small>${start}–${start+19.5} m</small></th></tr><tr><th scope="col">m</th><th scope="col">底質 ↓</th><th scope="col">m</th><th scope="col">底質 ↓</th></tr></thead>`;
    const body='<tbody>'+Array.from({length:20},(_,r)=>'<tr>'+[0,1].map(l=>{const pos=start+l*10+r/2,v=canonical(points.find(p=>p.p===pos));return `<th class="position" scope="row">${pos}</th><td data-review-position="${pos}">${reviewValue(v,validCode(v??''))}</td>`;}).join('')+'</tr>').join('')+'</tbody>';
    table=full.replace('review-line','review-line single').replace(/<colgroup>[\s\S]*?<\/tbody>/,`<colgroup><col class="pos"><col class="code"><col class="pos"><col class="code"></colgroup>${head}${body}`);
  }
  return `<div class="board-tools"><div class="seg-filters" aria-label="逐格對照段次">${[1,2,3,4,0].map(n=>`<button type="button" class="${reviewSegment===n?'active':''}" data-action="review-segment" data-segment="${n}" aria-pressed="${reviewSegment===n}">${n?'第 '+n+' 段':'整張手板'}</button>`).join('')}</div><span>逐段放大核對；切換不會更動資料。</span></div>${table}`;
}
function reviewTables(){return sheetList().map(k=>k==='line'?`<div id="review-line-panel">${reviewLinePanel()}</div>`:reviewBeltTable(k)).join('');}
async function submitRecord(delay=1200){
  if(saving)return;
  if(issues().length){step=3;render();$('#complete-error').textContent='請先處理待補充項目；可先下載草稿。';return;}
  if(!state.ack){step=3;render();$('#complete-error').textContent='請勾選已對照原始手板的確認。';$('#acknowledge').focus();return;}
  // Persist the draft first; reloading during the simulated request is recoverable.
  state.status='draft';delete state.simulatedSave;persist(false);
  saving=true;step=4;render();$('#main').focus();window.scrollTo({top:0});
  await new Promise(resolve=>setTimeout(resolve,delay));
  state.status=unknownCount()?'needs_review':'complete_local';
  state.simulatedSave={mode:'simulation',receipt_id:`DEMO-${state.id}`,saved_at:new Date().toISOString(),review_status:unknownCount()?'needs_review':'pending_review'};
  persist(false);
  if(storageFailed){state.status='draft';delete state.simulatedSave;const i=records.findIndex(r=>r.id===state.id);if(i>=0)records[i]=structuredClone(state);}
  saving=false;render();$('#main').focus();
}
function completeView(){
  if(saving)return intro(4,'正在儲存這支氣瓶的記錄…','正在模擬送出與接收儲存回覆，請稍候。')+`<section class="panel success-panel" aria-busy="true"><div class="save-spinner" aria-hidden="true"></div><h2 role="status">儲存中</h2><p>整理氣瓶資訊、各段觀測與最終泥下代碼。<br>請勿關閉頁面或重複送出。</p><span class="badge">模擬模式 · 未連接資料庫</span></section>`;
  if(storageFailed)return intro(4,'這次還沒有儲存成功。','本機儲存空間或瀏覽器設定可能阻止了儲存。')+`<section class="panel success-panel"><h2 role="alert">請重試，或先下載備份</h2><p>記錄仍在目前頁面中，請先不要關閉。此原型沒有資料庫副本。</p><div class="actions">${button('重新儲存','complete','','primary')}${button('下載草稿備份','export-draft')}${button('返回檢查','step','data-step="3"')}</div></section>`;
  const receipt=state.simulatedSave;
  return intro(4,'紀錄已儲存','請確認儲存狀態，並視需要下載備份。')+`<section class="panel success-panel"><div class="success-icon" aria-hidden="true">✓</div><h2 role="status">${receipt?'儲存成功':'已完成本機記錄'}</h2><p>${receipt?'這支氣瓶的所有探查已一併儲存。':'這是先前保留的本機紀錄。'}</p><span class="badge">${receipt?'模擬儲存成功 · 尚未寫入資料庫':'本機紀錄 · 尚未上傳'}</span><dl class="save-receipt"><div><dt>調查樣點</dt><dd>${esc(siteName())}</dd></div><div><dt>日期／氣瓶開始時間</dt><dd>${esc(state.meta.date)} · ${esc(state.meta.time)}</dd></div><div><dt>深度／探查</dt><dd>${esc(state.meta.depth)} m · ${methodList().map(k=>C.methods[k].short).join('、')}</dd></div>${receipt?`<div><dt>模擬儲存時間</dt><dd>${esc(new Date(receipt.saved_at).toLocaleString('zh-TW',{hour12:false}))}</dd></div><div><dt>模擬回執編號</dt><dd>${esc(receipt.receipt_id)}</dd></div>`:''}</dl><div class="event-preview">氣瓶識別：${esc(eventId())}</div><div class="callout ${unknownCount()?'warning':''}"><div><b>${unknownCount()?'已保留未記錄值，待窗口確認':'已完成填寫，待窗口審核'}</b><p>${unknownCount()?'NA 與原因已隨紀錄保留，不會改成 0。':'儲存成功不代表資料已審核通過。'}目前僅展示流程，未通知窗口；資料仍只存於此瀏覽器。</p></div></div><div class="actions">${button('檢視這份紀錄','view-saved','','primary')}${button('下载備份 JSON','export','','quiet')}</div></section><div class="actions">${button('我的紀錄','drafts')}${button('記錄下一支氣瓶 →','new-same','','primary')}</div>`;
}
function payload(draft=false){const event=eventKey(),id=eventId(),m=state.meta;const key=k=>`${event}|${{line:'line',fish:'belt_fish',invert:'belt_invert'}[k]}`;
  const data={format:'reefcheck-web-prototype-v2',status:draft?'draft':unknownCount()?'needs_review':'complete_local',uploaded:false,exported_at:new Date().toISOString(),event:{event_key:event,event_id:id,site_name_en__lookup:m.site,site_name_zh:siteName(),survey_date:m.date,event_time:m.time.replace(':','-'),depth_m:m.depth===''?null:Number(m.depth)},transects:methodList().map(k=>({transect_key:key(k),event_id:id,method:{line:'line',fish:'belt_fish',invert:'belt_invert'}[k],start_time:m.time,water_temp_c:m.temperature===''?null:Number(m.temperature),visibility_m:m.visibility===''?null:Number(m.visibility),recorders:state.recorders[k].split(/[、,，]/).map(x=>x.trim()).filter(Boolean),team_leader:m.leader,team_scientist:m.scientist,comments:state.notes})),substrate_points:[],substrate_bleaching:[],belt_observations:[],impact_observations:[],mud_audit:[],missing_reason:state.missingReason,validation_issues:issues().map(x=>x.text)};
  if(state.methods.includes('line')){data.substrate_points=points.map(p=>({transect_key:key('line'),segment:p.s,position_m:p.p,substrate_code:canonical(p),substrate_layer:'surface'}));if(state.mud)data.mud_audit=activeMudPoints().map(p=>({position_m:p.p,segment:p.s,surface:state.line[p.key],down:state.down[p.key]||null,canonical:canonical(p)}));if(state.bleaching)data.substrate_bleaching=['HC','SC'].flatMap(code=>starts.map((_,s)=>({transect_key:key('line'),substrate_code:code,segment:s+1,bleached_points:asNumber(state.bleach[`${code}:${s+1}`])})));}
  for(const k of ['fish','invert','rare'])if(state.methods.includes(k==='rare'?'invert':k))data.belt_observations.push(...rowsFor(k).flatMap(r=>starts.map((_,s)=>({transect_key:key(k==='rare'?'invert':k),taxon_group:k,taxon_name_en__lookup:r.name,taxon_size_class__lookup:r.size,taxon_local_row_id:r.id,segment:s+1,count:asNumber(state[k][`${r.id}:${s+1}`]),record_status:state[k][`${r.id}:${s+1}`]==='NA'?'not_recorded':state[k][`${r.id}:${s+1}`]===undefined||state[k][`${r.id}:${s+1}`]===''?'blank':'recorded'}))));
  if(state.methods.includes('invert'))data.impact_observations=rowsFor('impact').flatMap(r=>starts.map((_,s)=>({transect_key:key('invert'),impact_group:r.group,impact_name_en__lookup:r.name,impact_value_type:r.type,segment:s+1,raw_value:asNumber(state.impact[`${r.id}:${s+1}`])})));
  if(state.simulatedSave)data.simulated_save=structuredClone(state.simulatedSave);
  if(draft)data.draft_state=structuredClone(state);return data;
}
function asNumber(v){return v!==undefined&&v!==''&&v!=='NA'&&Number.isFinite(Number(v))?Number(v):null;}
function download(draft){const blob=new Blob([JSON.stringify(payload(draft),null,2)],{type:'application/json'}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`${(eventId()||'reefcheck').replace(/[^\w.\-]/g,'_')}-${draft?'draft':'record'}.json`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);toast('已啟動下載，請查看瀏覽器下載項目。');}
function modal(title,body){$('#modal-content').innerHTML=`<div class="modal-title"><h2>${title}</h2><button data-action="close" aria-label="關閉對話框">×</button></div>${body}`;if(!$('#modal').open)$('#modal').showModal();}
function openDrafts(){modal('我的紀錄',`<p>只顯示此瀏覽器儲存的紀錄。不同瀏覽器或裝置不會自動同步。</p>${records.length?records.slice().reverse().map(r=>`<div class="draft-item"><div><b>${esc(C.sites.find(s=>s[2]===r.meta.site)?.[1]||'尚未選擇樣點')}</b><small>${esc(r.meta.date||'日期未填')} · ${esc(r.meta.time||'時間未填')} · ${esc(r.meta.depth||'—')} m</small><small>${r.methods.map(k=>C.methods[k].short).join('、')} · ${r.status==='draft'?'草稿':r.status==='needs_review'?'已完成・待確認':'已完成・本機'}</small></div>${button('開啟','resume',`data-id="${esc(r.id)}"`,'small')}</div>`).join(''):'<p>還沒有儲存的紀錄。</p>'}<div class="actions">${button('新增一支氣瓶','new','','primary')}</div>`);}
function help(){modal('填寫說明',`<ol><li><b>先選探查，再填氣瓶資訊。</b>同一支氣瓶選多種探查，日期、時間與深度會共用。</li><li><b>底質照原表：</b>四段、每段左右兩欄。Tab 或 Enter 沿欄向下，方向鍵移到鄰格；可貼上一整欄 Excel 代碼。數字 1–10 與 91–98 都能辨識，0 不代表其他。</li><li><b>有上下手板：</b>勾選「泥下底質」，切換上／下，依同位置配對。SI(SI) 會記為 SI。</li><li><b>Belt 照列填四段。</b>0 表示看過但沒看到；NA 或 - 表示未記錄；空白留待補填。其他生物請逐種新增名称。</li><li><b>檢查後完成本機記錄。</b>不完整仍可下載草稿；NA 需附原因。此原型不會自動上傳資料庫。</li></ol>`);}
function setFieldFocus(html){
  const panel=$('#field-focus');if(!panel)return;
  if(!$('#focus-description'))panel.innerHTML=`<span id="focus-description"></span>${button('此格未記錄 NA','mark-na','','small')}`;
  $('#focus-description').innerHTML=html;
}
function handleCell(el){
  const k=el.dataset.cell,key=el.dataset.key,old=el.dataset.previous??state[k][key]??'';
  const explicitSilt=k==='line'&&el.value.trim().toUpperCase().replaceAll('（','(').replaceAll('）',')')==='SI(SI)';
  const v=k==='line'||k==='down'?normalizeCode(el.value):normalizeNumber(el.value);
  state[k][key]=v;el.value=v;el.dataset.previous=v;
  const valid=k==='line'||k==='down'?validCode(v,k==='down'):validNumber(v,el.dataset.type||'count');
  el.classList.toggle('invalid',v!==''&&!valid);el.setAttribute('aria-invalid',String(v!==''&&!valid));
  if(k==='line'&&normalizeCode(old)!==v&&state.down[key]){delete state.down[key];toast('上層已更改，該位置泥下值已清除，請重新核對。');}
  if(explicitSilt)state.down[key]='SI';persist();updateCounters();
  setFieldFocus(`<b>${esc(el.getAttribute('aria-label'))}</b> ${v&&!valid?'· 無效值，請依代碼／範圍修正。':v==='NA'?'· 未記錄，完成前請說明原因。':explicitSilt?'· SI(SI) 已記成 SI，並保留泥下 SI。':k==='line'||k==='down'?'· '+esc(v||'尚未填寫'):''}`);
}
document.addEventListener('change',e=>{
  const el=e.target;
  if(el.name==='depth-choice'){
    selectDepth(el.value);$('#depth-control').innerHTML=depthControl();
    $('#event-preview').textContent=eventId()?`氣瓶識別：${eventId()}`:'填好樣點、日期、時間與深度後，自動產生氣瓶識別。';
    $(el.value==='other'?'[data-meta="depth"]':`[name="depth-choice"][value="${el.value}"]`).focus();
  }
  if(el.name==='method'){state.methods=[...document.querySelectorAll('[name="method"]:checked')].map(x=>x.value);persist();$('#selection-count').textContent=`已選 ${state.methods.length} 種探查`;}
  if(el.dataset.cell)handleCell(el);
  if(el.dataset.toggle){const k=el.dataset.toggle;if(!el.checked&&(k==='mud'?Object.keys(state.down).length:Object.keys(state.bleach).length)){el.checked=true;modal('關閉這張附表？',`<p>已填的資料會保留在草稿，但關閉期間不會放進完成記錄。重新勾選後可以繼續核對。</p><div class="actions">${button('保留附表','close')}${button('確認關閉','disable-extra',`data-extra="${k}"`,'primary')}</div>`);}else{state[k]=el.checked;persist();render();}}
  if(el.hasAttribute('data-trash-mode')){const next=el.value;el.value=state.trashMode;if(Object.keys(state.impact).some(k=>['3','4'].includes(k.split(':')[0]))){modal('更換垃圾記錄方式',`<p>分級與件數的單位不同。確認後會清除這兩列垃圾數值，請依手板重新填寫。</p><div class="actions">${button('取消','close')}${button('更換並重新填寫','trash-mode',`data-mode="${next}"`,'primary')}</div>`);}else{state.trashMode=next;persist();render();}}
  if(el.id==='acknowledge'){state.ack=el.checked;persist(false);}
});
document.addEventListener('input',e=>{const el=e.target;if(el.dataset.meta){state.meta[el.dataset.meta]=el.value;persist();$('#event-preview').textContent=eventId()?`氣瓶識別：${eventId()}`:'填好樣點、日期、時間與深度後，自動產生氣瓶識別。';}if(el.dataset.recorder){state.recorders[el.dataset.recorder]=el.value;persist();}if(el.dataset.state){state[el.dataset.state]=el.value;persist();if(step===3){$('#review-issues').innerHTML=issueMarkup(issues());$('#acknowledge').checked=false;}}if(el.dataset.cell){const k=el.dataset.cell,key=el.dataset.key,old=state[k][key]||'',v=k==='line'||k==='down'?normalizeCode(el.value):normalizeNumber(el.value);state[k][key]=v;if(k==='line'&&validCode(v)&&v!==normalizeCode(old)&&state.down[key])delete state.down[key];if(k==='line'&&/^SI[（(]SI[）)]$/i.test(el.value.trim()))state.down[key]='SI';persist();updateCounters();}});
document.addEventListener('focusin',e=>{const el=e.target;if(el.dataset.cell){lastCell=el;el.dataset.previous=state[el.dataset.cell][el.dataset.key]||'';setFieldFocus(`<b>${esc(el.getAttribute('aria-label'))}</b> · ${el.dataset.cell==='down'?'對照上層 SI 的同一位置。':el.dataset.cell==='line'?'可輸入代碼或數字；Tab 沿此欄往下。':'填 0 表示未發現，NA 表示未記錄。'}`);}});
document.addEventListener('keydown',e=>{
  const el=e.target;if(!el.dataset.cell||!['Tab','Enter','ArrowUp','ArrowDown'].includes(e.key))return;
  const cells=[...document.querySelectorAll(`[data-cell="${el.dataset.cell}"]:not(:disabled)`)].sort((a,b)=>Number(a.dataset.order)-Number(b.dataset.order));const index=cells.indexOf(el),delta=e.key==='ArrowUp'||e.shiftKey?-1:1,next=cells[index+delta];
  if(next){e.preventDefault();handleCell(el);next.focus();next.select();}else if(e.key==='Enter'){e.preventDefault();handleCell(el);toast('已到這張表的最後一格，可換段或檢查紀錄。');}
});
document.addEventListener('paste',e=>{
  const el=e.target;if(!el.dataset.cell)return;const text=e.clipboardData.getData('text');if(!/[\n\t]/.test(text))return;
  if(['line','down'].includes(el.dataset.cell)&&text.includes('\t')){e.preventDefault();toast('底質請只複製單欄代碼，不包含公尺或其他欄，避免對錯位置。');return;}
  const tokens=text.trim().split(/\r?\n|\t/),cells=[...document.querySelectorAll(`[data-cell="${el.dataset.cell}"]:not(:disabled)`)].sort((a,b)=>Number(a.dataset.order)-Number(b.dataset.order)),start=cells.indexOf(el);
  e.preventDefault();if(tokens.length>cells.length-start){toast(`貼上有 ${tokens.length} 格，但目前剩 ${cells.length-start} 格。請減少範圍後再貼。`);return;}
  tokens.forEach((v,i)=>{cells[start+i].value=v;handleCell(cells[start+i]);});toast(`已依填寫順序貼上 ${tokens.length} 格，請核對位置。`);
});
document.addEventListener('submit',e=>{if(e.target.id==='metadata-form'){e.preventDefault();go(2);}});
document.addEventListener('click',e=>{
  const el=e.target.closest('[data-action]');if(!el)return;const a=el.dataset.action;e.preventDefault();
  if(saving)return;
  if(a==='home'||a==='step')go(a==='home'?0:Number(el.dataset.step));
  else if(a==='duplicate-back'){$('#modal').close();go(1);}
  else if(a==='mark-na'){if(lastCell?.isConnected){lastCell.value='NA';handleCell(lastCell);lastCell.focus();}else toast('請先選取要標記的格子。');}
  else if(a==='help')help();else if(a==='drafts')openDrafts();else if(a==='close'){$('#modal').close();$('#modal-content').innerHTML='';}
  else if(a==='sheet'){sheet=el.dataset.sheet;go(2);}
  else if(a==='segment'){segment=Number(el.dataset.segment);render();}
  else if(a==='review-segment'){reviewSegment=Number(el.dataset.segment);$('#review-line-panel').innerHTML=reviewLinePanel();$(`[data-action="review-segment"][data-segment="${reviewSegment}"]`).focus({preventScroll:true});}
  else if(a==='layer'){layer=el.dataset.layer;render();}
  else if(a==='next-sheet'){const idx=sheetList().indexOf(sheet);if(idx<sheetList().length-1){sheet=sheetList()[idx+1];go(2);}else go(3);}
  else if(a==='export-draft')download(true);
  else if(a==='export'){if(issues().length){go(3);toast('還有需補充項目，請先檢查或下載草稿。');}else download(false);}
  else if(a==='complete'){void submitRecord();}
  else if(a==='view-saved'){modal('已儲存的紀錄（本機／模擬）',`<p>${esc(siteName())} · ${esc(state.meta.date)} · ${esc(state.meta.time)} · ${esc(state.meta.depth)} m</p>${reviewTables()}<div class="actions">${button('關閉','close')}${button('修改這份紀錄','edit-saved','','primary')}</div>`);}
  else if(a==='edit-saved'){$('#modal').close();$('#modal-content').innerHTML='';go(3);}
  else if(a==='zero'){if(sheet==='impact'){help();return;}const k=el.dataset.sheet;const n=rowsFor(k).reduce((n,r)=>n+starts.filter((_,s)=>!state[k][`${r.id}:${s+1}`]).length,0);modal('確認空白格都是「未發現」',`<p>將${sheetName(k)}手板的 ${n} 個空白格填成 0。只適用於確實調查過、但沒有看到的項目。未記錄請填 NA。</p><div class="actions">${button('返回核對','close')}${button('確認填 0','confirm-zero',`data-sheet="${k}"`,'primary')}</div>`);}
  else if(a==='confirm-zero'){const k=el.dataset.sheet;rowsFor(k).forEach(r=>starts.forEach((_,s)=>{const key=`${r.id}:${s+1}`;if(!state[k][key])state[k][key]='0';}));persist();$('#modal').close();render();}
  else if(a==='add-custom'){const name=$('#custom-name').value.trim(),k=el.dataset.sheet;if(!name){toast('先填其他生物的實際名稱。');$('#custom-name').focus();return;}if(rowsFor(k).some(r=>r.zh.toLowerCase()===name.toLowerCase())){toast('這個類群已在表格內，請直接填寫原列。');return;}state.custom[k].push({id:'custom-'+Date.now().toString(36),name:`Other: ${name}`,zh:name});persist();render();toast('已新增一列，請填入四段數量。');}
  else if(a==='edit-custom'){const r=state.custom[el.dataset.sheet].find(r=>r.id===el.dataset.id);modal('修改其他類群',`<label class="field">生物名稱<input id="rename-custom" value="${esc(r.zh)}" maxlength="80"></label><p>修改名稱會保留已填數值；移除會一併刪除這一列的四段數值。</p><div class="actions">${button('移除此列','remove-custom',`data-sheet="${el.dataset.sheet}" data-id="${r.id}"`,'danger')}${button('儲存名稱','rename-custom',`data-sheet="${el.dataset.sheet}" data-id="${r.id}"`,'primary')}</div><p id="rename-error" class="form-error" role="alert"></p>`);}
  else if(a==='rename-custom'){const k=el.dataset.sheet,id=el.dataset.id,name=$('#rename-custom').value.trim();if(!name||rowsFor(k).some(r=>r.id!==id&&r.zh.toLowerCase()===name.toLowerCase())){$('#rename-error').textContent='請填寫不重複的實際生物名稱。';return;}const r=state.custom[k].find(r=>r.id===id);r.zh=name;r.name=`Other: ${name}`;persist();$('#modal').close();render();}
  else if(a==='remove-custom'){const k=el.dataset.sheet,id=el.dataset.id;state.custom[k]=state.custom[k].filter(r=>r.id!==id);starts.forEach((_,s)=>delete state[k][`${id}:${s+1}`]);persist();$('#modal').close();render();toast('已移除該自訂類群與四段數值。');}
  else if(a==='disable-extra'){state[el.dataset.extra]=false;persist();$('#modal').close();render();}
  else if(a==='trash-mode'){state.trashMode=el.dataset.mode;Object.keys(state.impact).filter(k=>['3','4'].includes(k.split(':')[0])).forEach(k=>delete state.impact[k]);persist();$('#modal').close();render();}
  else if(a==='issue'){const x=issues()[Number(el.dataset.index)];if(!x)return;if(x.reason){$('[data-state="missingReason"]').focus();return;}if(x.step===1){go(1);return;}sheet=x.sheet;layer=x.layer||'surface';segment=0;go(2);let input;if(x.key)input=document.querySelector(`[data-cell="${x.cell||x.layer==='down'&&'down'||sheet}"][data-key="${CSS.escape(x.key)}"]`);else input=[...document.querySelectorAll(`[data-cell="${sheet}"]`)].find(el=>x.kind==='missing'?!el.value:el.classList.contains('invalid'));input?.focus();input?.scrollIntoView({block:'center'});}
  else if(a==='resume'){const saved=records.find(r=>r.id===el.dataset.id);if(!saved)return;state=structuredClone(saved);step=state.status==='draft'?Math.min(state.step,3):4;sheet=sheetList()[0];$('#modal').close();render();window.scrollTo({top:0});}
  else if(a==='new'||a==='new-same'){const m=a==='new-same'?{site:state.meta.site,date:state.meta.date,leader:state.meta.leader,scientist:state.meta.scientist}:{};if(state.methods.length)persist(false);state=fresh(m);step=0;sheet='line';layer='surface';$('#modal').close();render();window.scrollTo({top:0});}
});
// Keep v1 drafts intact. Never overwrite another tab's newly created record.
window.addEventListener('storage',e=>{if(e.key===KEY){records=readRecords();toast('另一個分頁更新了紀錄；繼續填寫前請確認使用的是同一份草稿。');}});
render();
