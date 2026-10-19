'use strict';
/* Offline, framework-free prototype. All persisted values are escaped on render. */
const C = window.REEF;
const DEBUG = new URLSearchParams(window.location.search).get('debug')==='1';
const KEY = DEBUG?'teia-reefcheck-debug-records-v1':'teia-reefcheck-records-v2';
const $ = s => document.querySelector(s);
const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const starts = [0,25,50,75];
const points = starts.flatMap((start,s) => Array.from({length:40},(_,i)=>({key:String(start+i/2),s:s+1,p:start+i/2})));
const pointGroups = starts.flatMap((start)=>[0,10].flatMap(column=>[0,2.5,5,7.5].map(offset=>Array.from({length:5},(_,i)=>points.find(p=>p.p===start+column+offset+i/2)))));
function defaultSegment(){return window.innerWidth<=1366?1:0;}
let records = readRecords(), state = fresh(), step = 0, sheet = 'line', segment = defaultSegment(), reviewSegment = defaultSegment(), layer = 'surface', linePage = 0, reviewGroup = 0, reviewMode = 'raw';
let timer, storageFailed = false, lastCell = null, saving = false;
let currentUser = null, csrfToken = '', submitError = null;
let activityBusy=false;
function fresh(meta = {}) {
  return {id:globalThis.crypto?.randomUUID?.() || `draft-${Date.now()}-${Math.random().toString(36).slice(2)}`,methods:[],activityCode:'',activity:null,meta:{site:'',date:'',time:'',depth:'',temperature:'',visibilityMin:'',visibilityMax:'',leader:'',scientist:'',weather:'',airTemperature:'',latitude:'',longitude:'',...meta},recorders:{line:'',fish:'',invert:''},fishLengthMode:'',coralMorphology:'',line:{},down:{},bleached:{},linePhase:'entry',mud:false,bleaching:false,bleach:{},fish:{},invert:{},rare:{},impact:{},custom:{fish:[],invert:[],rare:[]},rareDetails:{},notApplicable:{fish:[],invert:[],rare:[],impact:[]},rkcNote:'',otherDamageNote:'',notes:'',missingReason:'',status:'draft',step:0,updated:''};
}
function migrateRecord(record){
  if(!record?.meta)return record;
  if(record.meta.visibilityMin===undefined)record.meta.visibilityMin=record.meta.visibility??'';
  if(record.meta.visibilityMax===undefined)record.meta.visibilityMax=record.meta.visibility??'';
  delete record.meta.visibility;
  if(record.fishLengthMode!=='split'&&record.fishLengthMode!=='combined')record.fishLengthMode='';
  if(!['entry','number-review','bleaching','complete'].includes(record.linePhase))record.linePhase='entry';
  record.bleached={...(record.bleached||{})};
  record.activityCode=record.activityCode||'';record.activity=record.activity||null;
  record.coralMorphology=record.coralMorphology||'';record.rareDetails=record.rareDetails||{};
  record.rkcNote=record.rkcNote||'';record.otherDamageNote=record.otherDamageNote||'';
  for(const k of ['weather','airTemperature','latitude','longitude'])if(record.meta[k]===undefined)record.meta[k]='';
  record.notApplicable={fish:[],invert:[],rare:[],impact:[],...(record.notApplicable||{})};
  for(const k of ['fish','invert','rare','impact'])if(!Array.isArray(record.notApplicable[k]))record.notApplicable[k]=[];
  return record;
}
function readRecords(){try{const data=JSON.parse(localStorage.getItem(KEY)||'[]');return Array.isArray(data)?data.filter(x=>x?.id&&Array.isArray(x.methods)&&x.meta&&x.line&&x.custom).map(migrateRecord):[];}catch{return [];}}
function persist(change=true){
  if(change){state.status='draft';state.ack=false;delete state.simulatedSave;delete state.dbSave;}
  state.step=step;state.updated=new Date().toISOString();
  const i=records.findIndex(x=>x.id===state.id);
  if(i<0)records.push(structuredClone(state));else records[i]=structuredClone(state);
  try{
    localStorage.setItem(KEY,JSON.stringify(records));
    storageFailed=false;
    const saveInfo = (state.dbSave || state.simulatedSave) ? '已存入資料庫 · 本機留存' : (currentUser ? `草稿（${currentUser.name || currentUser.email}）` : '草稿已儲存在此裝置');
    $('#save-status').textContent=saveInfo;
    $('#save-status').classList.remove('storage-warning');
  }catch{
    storageFailed=true;
    $('#save-status').textContent='儲存失敗，請下載草稿';
    $('#save-status').classList.add('storage-warning');
    toast('此瀏覽器無法儲存，請下載草稿備份。');
  }
  $('#draft-count').textContent=records.length;
}
function toast(text){$('#toast').textContent=text;$('#toast').hidden=false;clearTimeout(timer);timer=setTimeout(()=>$('#toast').hidden=true,4200);}
function methodList(){return ['line','fish','invert'].filter(k=>state.methods.includes(k));}
function sheetList(){return methodList().flatMap(k=>k==='invert'?['invert','rare','impact']:[k]);}
function sheetName(k){return {line:'底質',fish:'魚類',invert:'無脊椎',rare:'罕見生物',impact:'環境衝擊'}[k];}
function siteName(){const s=C.sites.find(s=>s[2]===state.meta.site||s[1]===state.meta.site);return s?.[1] || '尚未選擇樣點';}
function siteId(){const s=C.sites.find(s=>s[2]===state.meta.site||s[1]===state.meta.site);return s&&s[3]?s[3]:0;}
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
  const debugLink=$('#debug-mode-link');
  debugLink.hidden=window.location.protocol!=='file:'&&!DEBUG;
  debugLink.href=DEBUG?'?':'?debug=1';
  debugLink.textContent=DEBUG?'離開測試模式':'開啟測試模式';
  $('#draft-count').textContent=records.length;
  $('#workflow').innerHTML=[['選擇探查','拿出這次的手板'],['潛水資訊','確認同一支氣瓶'],['手板輸入','照著表格逐格記錄'],['檢查與完成','']].map(([a,b],i)=>`<button class="step ${step===i?'active':step>i?'done':''}" data-action="step" data-step="${i}" ${step===i?'aria-current="step"':''}><span class="step-num">${step>i?'✓':i+1}</span><span><b>${a}</b>${b?`<small>${b}</small>`:''}</span></button>`).join('');
  const page=step===0?chooseView():step===1?metaView():step===2?boardView():step===3?reviewView():completeView();
  $('#main').innerHTML=(DEBUG?'<aside class="debug-banner" role="status"><b>測試模式</b><span>可略過必填欄位瀏覽流程；不會送出或寫入資料庫。</span></aside>':'')+page
    .replace('垃圾分級由件數自動換算', '珊瑚損害與垃圾均由件數自動分級')
    .replace('垃圾顯示原始件數', '珊瑚損害與垃圾顯示原始件數');
  $('#workflow').inert=saving;$('.topbar').inert=saving;
  updateCounters();
}
function go(n){
  if(DEBUG&&n>0&&!state.methods.length){state.methods=['line'];sheet='line';}
  if(!DEBUG&&n>0&&!state.methods.length){toast('先選擇至少一種探查。');return;}
  if(!DEBUG&&n>0&&!state.activity){step=0;render();toast('請先確認活動代碼。');return;}
  if(!DEBUG&&n>1){const errors=metaErrors();if(errors.length){step=1;render();$('#meta-error').textContent=errors.join('；');$('#metadata-form').reportValidity();return;}
    const duplicate=duplicateRecord();
    if(duplicate){modal('這支氣瓶已經有紀錄',`<p>樣點、日期、開始時間與深度都相同。請開啟原紀錄增加探查項目；若是另一支氣瓶，請先核對時間與深度。</p><div class="actions">${button('返回核對資訊','duplicate-back')}${button('開啟原紀錄','resume',`data-id="${esc(duplicate.id)}"`,'primary')}</div>`);return;}
  }
  if(!DEBUG&&n===3&&state.methods.includes('line')&&state.linePhase!=='complete'){step=2;sheet='line';render();toast('請先確認全部原始數字並完成白化標記。');return;}
  step=n;if(step===2&&!sheetList().includes(sheet))sheet=sheetList()[0];
  persist(false);render();$('#main').focus();window.scrollTo({top:0});
}
function miniTable(k){return `<table class="mini-sheet" aria-hidden="true"><thead><tr><th>${k==='line'?'m':'類群'}</th><th>1 段</th><th>2 段</th><th>3 段</th><th>4 段</th></tr></thead><tbody>${[0,1,2].map((r)=>`<tr><td>${k==='line'?r*.5:k==='fish'?['蝶魚','石鱸','笛鯛'][r]:['蝦','海膽','海參'][r]}</td>${[0,1,2,3].map(()=>'<td>—</td>').join('')}</tr>`).join('')}</tbody></table>`;}
function chooseView(){
  const other=records.filter(r=>r.id!==state.id&&r.status==='draft');
  return intro(1,'這次，要記錄哪一種探查？','拿出這支氣瓶的手板，選擇你要輸入的項目。同一支氣瓶可以一次填多種。')+
    activityView()+(other.length?`<div class="saved-banner"><span>還有 ${other.length} 份草稿，隨時可以接著填。</span>${button('繼續草稿 →','drafts','','quiet')}</div>`:'')+
    `<div class="method-grid">${Object.entries(C.methods).map(([k,m],i)=>`<label class="method-card"><input type="checkbox" name="method" value="${k}" ${state.methods.includes(k)?'checked':''} aria-label="${m.name}"><span class="method-symbol" aria-hidden="true">0${i+1}</span><h2>${m.name}</h2><span class="eyebrow">${m.en}</span><p>${m.description}</p>${miniTable(k)}<span class="mini-caption">${m.caption}</span></label>`).join('')}</div>
    <div class="callout"><span class="callout-icon">↳</span><div><b>共用欄位</b><p>樣點、日期、氣瓶開始時間與深度只填一次；不同氣瓶請建立另一份紀錄。</p></div></div>
    <div class="actions"><span id="selection-count" class="selection-count">已選 ${state.methods.length} 種探查</span>${button('下一步：潛水資訊 <span class="arrow">→</span>','step','data-step="1"','primary')}</div>`;
}
function activityView(){return `<section class="panel activity-panel"><h2>這次的活動代碼</h2><p>輸入活動窗口提供的代碼，確認調查樣點與日期。</p><div class="activity-code-row"><label class="field">活動代碼<input data-activity-code value="${esc(state.activityCode)}" autocomplete="off" autocapitalize="characters" placeholder="活動窗口提供的代碼" ${activityBusy?'disabled':''}></label>${button(activityBusy?'確認中…':'確認活動','verify-activity',activityBusy?'disabled':'','primary')}</div>${state.activity?`<p class="review-ok">✓ ${esc(state.activity.name)} · ${esc(state.activity.start_date)}–${esc(state.activity.end_date)}${state.activity.demo?' · 測試活動':''}</p>`:''}<p id="activity-error" class="form-error" role="alert"></p>${DEBUG?button('帶入測試活動','demo-activity','','quiet small'):''}</section>`;}
function allowedSites(){return state.activity&&!state.activity.demo?C.sites.filter(s=>state.activity.site_ids.includes(s[3])):C.sites;}
async function verifyActivity(){
  if(activityBusy)return;const code=state.activityCode.trim();if(!code){$('#activity-error').textContent='請輸入活動代碼。';return;}
  activityBusy=true;render();
  try{const res=await fetch('/api/public/reef-check/activities/verify',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({code}),credentials:'include'});const data=await res.json();if(!res.ok)throw new Error(data.error||'活動代碼無效或已停用。');state.activity=data;persist();}
  catch(err){state.activity=null;activityBusy=false;render();$('#activity-error').textContent=window.location.protocol==='file:'?'本機預覽無法驗證正式活動；請開啟測試模式查看，正式填報需在網站上驗證。':err.message;return;}
  activityBusy=false;render();
}
function entrySettings(){return `${state.methods.includes('line')?`<fieldset class="entry-mode"><legend>這份手板有分類硬珊瑚型態嗎？ <span class="required">必填</span></legend>${[['general','沒有，只記 HC／1'],['classified','有，記 a／b／c 型態']].map(([v,label])=>`<label><input type="radio" name="coral-morphology" value="${v}" ${state.coralMorphology===v?'checked':''}>${label}</label>`).join('')}</fieldset>`:''}${state.methods.includes('fish')?`<fieldset class="entry-mode"><legend>魚類手板有分開記錄體長嗎？ <span class="required">必填</span></legend>${[['split','有，依體長分列'],['combined','沒有，記同類群總數']].map(([v,label])=>`<label><input type="radio" name="fish-length-mode" value="${v}" ${state.fishLengthMode===v?'checked':''}>${label}</label>`).join('')}</fieldset>`:''}`;}
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
  `<form id="metadata-form" ${DEBUG?'novalidate':''}><section class="panel"><div class="panel-heading"><div><h2>潛水與樣點</h2><p>本次包含：${methodList().map(k=>C.methods[k].name).join('、')}</p></div><span class="badge">01 / 基本資料</span></div><div class="fields"><label class="field wide">調查樣點 <span class="required">必填</span><select required name="site" data-meta="site"><option value="">選擇手板上的樣點</option>${[...new Set(allowedSites().map(s=>s[0]))].map(region=>`<optgroup label="${region}">${allowedSites().filter(s=>s[0]===region).map(s=>`<option value="${esc(s[2])}" ${state.meta.site===s[2]?'selected':''}>${esc(s[1])} · ${esc(s[2])}</option>`).join('')}</optgroup>`).join('')}</select><small>依現有樣點清單；找不到時請先存草稿，與活動窗口確認名稱。</small></label>
  ${field('調查日期','date','date',true,'以手板上的實際日期為準。',state.activity?`min="${esc(state.activity.start_date)}" max="${esc(state.activity.end_date)}"`:'')}${field('這支氣瓶的開始時間','time','time',true,'Line 與 Belt 請用同一支氣瓶的開始時間。')}<div id="depth-control">${depthControl()}</div>${field('水溫（°C）','temperature','number',false,'每支氣瓶一個水溫；未記錄可留白。','min="0" max="45" step="0.1" placeholder="例如 27.5"')}<div class="visibility-range"><p>能見度範圍（m）</p><div class="range-fields">${field('最低能見度','visibilityMin','number',false,'','min="0" max="100" step="0.1" placeholder="例如 8"')}${field('最高能見度','visibilityMax','number',false,'','min="0" max="100" step="0.1" placeholder="例如 12"')}</div><small>只記一個數值時填任一格即可，系統會視為最低＝最高；未記錄可兩格留白。</small>${field('天氣','weather','text',false,'未記錄可留白。')}${field('氣溫（°C）','airTemperature','number',false,'未記錄可留白。','min="-20" max="60" step="0.1"')}${field('調查緯度','latitude','number',false,'與經度一起填，未記錄可兩格留白。','min="-90" max="90" step="any"')}${field('調查經度','longitude','number',false,'請使用十進位座標。','min="-180" max="180" step="any"')}</div></div><div class="event-preview" id="event-preview">${eventId()?`氣瓶識別：${esc(eventId())}`:'填好樣點、日期、時間與深度後，自動產生氣瓶識別。'}</div></section>
  <section class="panel"><div class="panel-heading"><div><h2>這份手板，是誰記錄的？</h2><p>每種探查可填不同記錄者；多人請以「、」分隔。</p></div><span class="badge">02 / 記錄人員</span></div><div class="fields">${methodList().map(k=>`<label class="field">${C.methods[k].short}記錄者 <span class="required">必填</span><input name="recorder-${k}" data-recorder="${k}" value="${esc(state.recorders[k])}" required placeholder="姓名，可填多人"></label>`).join('')}${field('隊長','leader')}${field('科學指導員','scientist')}</div></section>${entrySettings()}<p class="form-error" id="meta-error" role="alert"></p><div class="actions">${button('← 選擇探查','step','data-step="0"')}<button class="button primary" type="submit">開始填寫手板 <span class="arrow">→</span></button></div></form>`;}
function metaErrors(){const m=state.meta,e=[];if(!C.sites.some(s=>s[2]===m.site||s[1]===m.site))e.push('請選擇樣點');if(!/^\d{4}-\d{2}-\d{2}$/.test(m.date))e.push('請填調查日期');if(!/^([01]\d|2[0-3]):[0-5]\d$/.test(m.time))e.push('請填氣瓶開始時間');if(m.depth===''||!Number.isFinite(Number(m.depth))||Number(m.depth)<=0||Number(m.depth)>100)e.push('深度請填 0.1–100 m');for(const [k,n,max] of [['temperature','水溫',45],['visibilityMin','最低能見度',100],['visibilityMax','最高能見度',100]])if(m[k]!==''&&(!Number.isFinite(Number(m[k]))||Number(m[k])<0||Number(m[k])>max))e.push(`${n}請填 0–${max}`);if(m.visibilityMin!==''&&m.visibilityMax!==''&&Number(m.visibilityMin)>Number(m.visibilityMax))e.push('最低能見度不能大於最高能見度');methodList().forEach(k=>{if(!state.recorders[k].trim())e.push(`請填${C.methods[k].short}記錄者`);});return [...e,...extraMetaErrors()];}
function strip(){return `<section class="survey-strip" aria-label="本支氣瓶"><div><small>樣點</small><b>${esc(siteName())}</b></div><div><small>日期</small><b>${esc(state.meta.date)}</b></div><div><small>氣瓶開始</small><b>${esc(state.meta.time)}</b></div><div><small>深度</small><b>${esc(state.meta.depth)} m</b></div>${button('修改資訊','step','data-step="1"','quiet small')}</section>`;}
function normalizeCode(value){const t=String(value).trim().toUpperCase().replaceAll('（','(').replaceAll('）',')');if(t==='-')return 'NA';if(t==='SI(SI)')return 'SI';if(t==='0')return 'OT';if(/^[1-9]$/.test(t))return C.substrates[Number(t)-1][0];if(/^[ABC]$/.test(t))return `HC-${t.toLowerCase()}`;if(/^HC-[ABC]$/.test(t))return `HC-${t.slice(-1).toLowerCase()}`;if(/^9[1-8]$/.test(t))return `SI(${C.substrates[Number(t)-91][0]})`;return t;}
function validCode(v,down=false){const t=normalizeCode(v);if(!down&&state.coralMorphology==='general'&&/^HC-[abc]$/.test(t))return false;return down?C.substrates.slice(0,9).some(s=>s[0]===t)||t==='NA':C.substrates.some(s=>s[0]===t)||t==='NA'||/^SI\((HC|SC|RKC|NIA|SP|RC|RB|SD)\)$/.test(t);}
function normalizeNumber(v){const t=String(v).trim();return /^(na|-)$/i.test(t)?'NA':t;}
function validNumber(v,type='count'){const t=normalizeNumber(v);return t==='NA'||t!==''&&/^\d+(?:\.\d+)?$/.test(t)&&Number.isFinite(Number(t))&&Number(t)>=0&&(type==='percent'?Number(t)<=100:Number.isSafeInteger(Number(t)))&&(type!=='level'||Number(t)<=3);}
function rowsFor(k){const base=k==='fish'?fishRows():C[k].map((r,i)=>({id:String(i),name:r[0],zh:r[1],size:k==='impact'?'':r[2],type:k==='impact'?r[2]:'count',group:k==='impact'?r[3]:k}));return [...base,...(state.custom[k]||[]).map(r=>({...r,size:'',type:'count',group:k}))];}
function fishRows(){
  const rows=C.fish.map((r,i)=>({id:String(i),name:r[0],zh:r[1],size:r[2],type:'count',group:'fish'}));
  if(state.fishLengthMode==='combined')return [...rows.filter(r=>r.name!=='Grouper'),{id:'grouper-combined',name:'Grouper',zh:'石斑魚',size:'',type:'count',group:'fish'}];
  return rows;
}
function rowApplicable(k,id){return !(state.notApplicable[k]||[]).includes(id);}
function applicableRows(k){return rowsFor(k).filter(r=>rowApplicable(k,r.id));}
function excludedRows(k){return rowsFor(k).filter(r=>!rowApplicable(k,r.id));}
function trashLevel(value){const n=Number(value);return !Number.isFinite(n)||n<0?null:n===0?0:n===1?1:n<=4?2:3;}
function extraMetaErrors(){
  const e=[],m=state.meta;
  if(!state.activity)e.push('請先驗證活動代碼');
  else if(!state.activity.demo){if(!allowedSites().some(s=>s[2]===m.site))e.push('樣點不在本活動範圍');if(m.date<state.activity.start_date||m.date>state.activity.end_date)e.push('日期不在本活動範圍');}
  if(state.methods.includes('fish')&&!['split','combined'].includes(state.fishLengthMode))e.push('請選擇魚類體長模式');
  if(state.methods.includes('line')&&!['general','classified'].includes(state.coralMorphology))e.push('請選擇硬珊瑚型態記錄方式');
  for(const [key,min,max,label] of [['airTemperature',-20,60,'氣溫'],['latitude',-90,90,'緯度'],['longitude',-180,180,'經度']])if(m[key]!==''&&m[key]!==undefined&&(!Number.isFinite(Number(m[key]))||Number(m[key])<min||Number(m[key])>max))e.push(`${label}格式或範圍錯誤`);
  if(Boolean(m.latitude)!==Boolean(m.longitude))e.push('經緯度請一起填寫或一起留白');return e;
}
function rkcStats(){const codes=points.map(canonical).filter(c=>c&&c!=='NA'&&validCode(c)),count=codes.filter(c=>c==='RKC').length;return {count,total:codes.length,percent:codes.length?count/codes.length*100:null,required:codes.length>0&&count/codes.length>=0.1};}
function needsOtherDamageNote(){return rowsFor('impact').filter(r=>r.name==='Coral damage: other').some(r=>starts.some((_,s)=>Number(state.impact[`${r.id}:${s+1}`])>0));}
function investigationNotes(){if(step===2&&sheet==='line'&&state.linePhase!=='complete')return '';const rkc=rkcStats();return `<section class="panel investigation-notes" id="investigation-notes"><h2>現場補充資訊</h2>${state.methods.includes('line')?`<p>新死珊瑚 RKC：${rkc.percent===null?'尚無有效資料':`${rkc.count} / ${rkc.total} 個有效點 · ${Number(rkc.percent.toFixed(2))}%`}</p>${rkc.required?`<label class="field">新死珊瑚達 10%，請填可能原因 <span class="required">必填</span><textarea data-state="rkcNote" placeholder="例如觀察到白化、物理損害等；無法判斷可寫明無法判斷。">${esc(state.rkcNote)}</textarea></label>`:''}`:''}${state.methods.includes('invert')&&needsOtherDamageNote()?`<label class="field">其他珊瑚損害說明 <span class="required">必填</span><textarea data-state="otherDamageNote" placeholder="請說明手板中「珊瑚損害：其他」的種類或可能原因。">${esc(state.otherDamageNote)}</textarea></label>`:''}<label class="field">其他備註<textarea data-state="notes" placeholder="其他現場資訊，可留白。">${esc(state.notes)}</textarea></label></section>`;}
function bleachingSummary(){
  return `<section class="bleaching-summary" data-bleaching-summary><h3>白化數量 · 自動計算</h3><p>依標記位置分別加總 HC（含 a／b／c）與 SC，不需重複輸入。</p><table><thead><tr><th>底質</th>${starts.map((_,s)=>`<th>子樣區 ${s+1}</th>`).join('')}</tr></thead><tbody>${['HC','SC'].map(code=>`<tr><th>${code} 白化</th>${starts.map((_,s)=>{const ps=points.filter(p=>p.s===s+1&&(code==='HC'?/^HC(?:-[abc])?$/.test(canonical(p)||''):canonical(p)==='SC')),ready=points.filter(p=>p.s===s+1).every(p=>validCode(canonical(p)||''))&&ps.every(p=>typeof state.bleached[p.key]==='boolean'),n=ps.filter(p=>state.bleached[p.key]===true).length;return `<td>${ready?n:'待確認'}</td>`;}).join('')}</tr>`).join('')}</tbody></table></section>`;
}
function extraEntryIssues(){
  const list=[];
  if(state.methods.includes('line')){
    if(state.linePhase!=='complete')list.push({text:'請先完成原始數字核對與白化標記。',sheet:'line'});
    const unmarked=points.find(p=>isBleachableCode(canonical(p))&&typeof state.bleached[p.key]!=='boolean');
    if(unmarked)list.push({text:`${unmarked.p} m 珊瑚點尚未核對白化。`,sheet:'line',key:unmarked.key});
    if(rkcStats().required&&!state.rkcNote.trim())list.push({text:'RKC 已達 10%，請填寫可能原因。',field:'rkcNote'});
  }
  if(state.methods.includes('invert')&&needsOtherDamageNote()&&!state.otherDamageNote.trim())list.push({text:'已記錄其他珊瑚損害，請填寫獨立說明。',field:'otherDamageNote'});
  return list;
}
function rareDetailFields(){return state.custom.rare.length?`<section class="rare-details"><h3>其他罕見生物 · 尺寸與備註</h3>${state.custom.rare.map(r=>`<div class="fields"><label class="field">${esc(r.zh)}：尺寸（選填）<input data-rare-detail="size" data-row-id="${r.id}" value="${esc(state.rareDetails[r.id]?.size||'')}" placeholder="例如約 1 m，照手板填寫"></label><label class="field">${esc(r.zh)}：備註（選填）<input data-rare-detail="notes" data-row-id="${r.id}" value="${esc(state.rareDetails[r.id]?.notes||'')}" placeholder="外觀、數量記錄方式等"></label></div>`).join('')}</section>`:'';}
function activeMudPoints(){return points.filter(p=>normalizeCode(state.line[p.key]||'')==='SI');}
function groupProblem(index,currentLayer=layer){
  const group=pointGroups[index]||[];
  if(currentLayer==='down')return group.find(p=>normalizeCode(state.line[p.key]||'')==='SI'&&!validCode(state.down[p.key]||'',true));
  return group.find(p=>!validCode(state.line[p.key]||''));
}
function sheetStats(k){
  const cells=k==='line'?points.map(p=>({v:state.line[p.key]||'',valid:v=>validCode(v)})):applicableRows(k).flatMap(r=>starts.map((_,i)=>({v:state[k][`${r.id}:${i+1}`]||'',valid:v=>validNumber(v,r.type)})));
  return {total:cells.length,filled:cells.filter(c=>c.v!=='').length,valid:cells.filter(c=>c.valid(c.v)).length,missing:cells.filter(c=>c.v==='').length,invalid:cells.filter(c=>c.v!==''&&!c.valid(c.v)).length,na:cells.filter(c=>normalizeNumber(c.v)==='NA').length};
}
function boardAdvanceButton(){
  if(sheet==='line'){
    if(state.linePhase==='entry')return button('檢查全部數字 →','review-line-numbers','','primary');
    if(state.linePhase==='number-review')return button('數字正確，開始標記白化 →','confirm-line-numbers','','primary');
    if(state.linePhase==='bleaching')return button('完成白化標記 →','confirm-line-bleaching','','primary');
  }
  return button(sheetList().indexOf(sheet)<sheetList().length-1?'下一張手板 →':'檢查這份紀錄 →','next-sheet','','primary');
}
function boardView(){if(!sheetList().includes(sheet))sheet=sheetList()[0];return intro(3,'輸入手板資料','依照手板逐組輸入；每組 5 點，完成後再統一確認與轉換。')+strip()+
  `<nav class="sheets-nav" aria-label="探查手板">${sheetList().map(k=>`<button class="sheet-tab ${sheet===k?'active':''}" data-action="sheet" data-sheet="${k}" ${sheet===k?'aria-current="page"':''}>${sheetName(k)}<small data-tab-count="${k}"></small></button>`).join('')}</nav><section class="board"><div class="board-head"><div><p class="eyebrow">${sheet==='line'?'LINE TRANSECT · SUBSTRATE':'BELT TRANSECT · '+sheet.toUpperCase()}</p><h2>${sheetName(sheet)}記錄手板</h2><p>${sheet==='line'?'四段 × 每段 40 點，每 0.5 m 一格':'每一列一個類群，每一欄一個段次'}</p></div><span class="progress"><strong id="filled-count"></strong> / <span id="total-count"></span> <small>已填</small></span></div>${sheet==='line'?lineBoard():beltBoard(sheet)}${sheet==='rare'?rareDetailFields():''}<div class="field-focus" id="field-focus" aria-live="polite">點選一格開始輸入；位置與欄位提示會顯示在這裡。</div></section>
  ${investigationNotes()}
  <div class="actions sticky-actions">${button('← 潛水資訊','step','data-step="1"')}<div class="right">${button('下載草稿','export-draft','','quiet')}${boardAdvanceButton()}</div></div>`;}
function groupLabel(group){return `第 ${group[0].s} 段 · ${group[0].p}–${group[4].p} m`;}
function groupPosition(index){return {segment:Math.floor(index/8)+1,within:index%8+1};}
function linePhaseSteps(){const phase=state.linePhase||'entry',current={entry:1,'number-review':2,bleaching:3,complete:4}[phase];return `${DEBUG?'<p class="debug-phase-hint">測試模式：可直接點選下方關卡，空白也能繼續。</p>':''}<div class="line-phase" aria-label="底質填寫流程">${['填數字','確認數字','標記白化','最後確認'].map((label,i)=>{const cls=current===i+1?'active':current>i+1?'done':'',content=`<b>${current>i+1?'✓':i+1}</b>${label}`;return DEBUG||i+1<current?`<button type="button" class="${cls}" data-action="${DEBUG?'debug-line-phase':'line-return-phase'}" data-phase="${['entry','number-review','bleaching','complete'][i]}" ${current===i+1?'aria-current="step"':''}>${content}</button>`:`<span class="${cls}">${content}</span>`;}).join('')}</div>`;}
function lineLocator(){
  const group=pointGroups[linePage],values=layer==='down'?state.down:state.line,first=Math.floor(linePage/4)*4,columnStart=pointGroups[first][0].p;
  return `<section class="line-locator" aria-label="目前手板欄位對照"><div class="locator-heading"><b>子樣區 ${group[0].s}</b><span>${columnStart}–${columnStart+9.5} m</span><small>${linePage%8<4?'左':'右'}欄 · ${layer==='down'?'泥下':'表面'}</small></div><div class="locator-column"><div class="locator-column-head"><span>m</span><span>數字</span></div>${[0,1,2,3].map(row=>{const index=first+row,ps=pointGroups[index];return `<button type="button" class="locator-group ${index===linePage?'active':''}" data-action="locate-line-group" data-group="${index}" ${index===linePage?'aria-current="location"':''} aria-label="子樣區 ${group[0].s}，${ps[0].p} 至 ${ps[4].p} 公尺${index===linePage?'，目前輸入位置':''}">${ps.map(p=>`<span class="locator-cell"><small>${p.p}</small><i data-map-key="${p.key}">${esc(values[p.key]||'·')}</i></span>`).join('')}</button>`;}).join('')}</div><p>橘框：目前 5 格</p></section>`;
}
function lineGroupNav(groupIndexes=pointGroups.map((_,i)=>i),action='line-page'){
  const cursor=Math.max(0,groupIndexes.indexOf(linePage)),prev=groupIndexes[cursor-1],next=groupIndexes[cursor+1];
  return `<div class="zoom-nav">${button('← 上一組',action,prev===undefined?'disabled':`data-group="${prev}"`,'small')}${next===undefined?'':button('確認這 5 格，下一組 →',action,`data-group="${next}"`,'primary small')}</div>`;
}
function lineEntryBoard(){
  if(!state.mud)layer='surface';const isDown=layer==='down',group=pointGroups[linePage]||pointGroups[0],values=isDown?state.down:state.line,pos=groupPosition(linePage);
  return `${linePhaseSteps()}<div class="line-zoom-head"><div><p class="eyebrow">${isDown?'泥下底質':'表面底質'} · 每次 5 點</p><h3>${groupLabel(group)}</h3></div><strong>${linePage+1}<small>/ ${pointGroups.length} 組</small></strong></div>
  <p class="subsample-progress">子樣區 ${pos.segment} · 第 ${pos.within} / 8 組</p><div class="zoom-progress" aria-hidden="true"><i style="width:${pos.within/8*100}%"></i></div>
  ${state.mud?`<div class="layer-switch">${button('上：表面數字','layer','data-layer="surface"',layer==='surface'?'primary':'')}${button('下：泥下數字','layer','data-layer="down"',isDown?'primary':'')}</div>`:''}
  <div class="line-entry-workspace">${lineLocator()}
  <div class="line-zoom-list">${group.map((p,i)=>{const v=values[p.key]||'',enabled=!isDown||normalizeCode(state.line[p.key]||'')==='SI';return `<label class="line-zoom-row ${enabled?'':'disabled'}"><span><b>${p.p} m</b><small>第 ${p.s} 段</small></span><input type="text" inputmode="text" autocapitalize="characters" autocomplete="off" spellcheck="false" data-cell="${isDown?'down':'line'}" data-key="${p.key}" data-order="${linePage*5+i}" data-segment="${p.s}" data-position="${p.p}" aria-label="第 ${p.s} 段 ${p.p} m ${isDown?'泥下':'表面'}底質原始數字" value="${esc(v)}" placeholder="${enabled?'填手板數字':'不適用'}" ${enabled?'':'disabled'} class="${v&&!validCode(v,isDown)?'invalid':''}" ${v&&!validCode(v,isDown)?'aria-invalid="true"':''}></label>`;}).join('')}</div>
  </div><p class="zoom-hint">先照手板填原始數字；此階段不會轉成 HC、SC 等代碼。填完第 5 格不會自動跳轉，請核對後自行按「下一組」。</p>${lineGroupNav()}
  <details class="code-help"><summary>查看手板數字說明</summary><div class="line-legend">${C.substrates.map(s=>`<span><b>${s[2]} ${s[0]}</b> ${s[1]}</span>`).join('')}<span><b>NA</b> 未記錄</span></div></details>
  <div class="line-bottom"><label class="toggle-row"><input type="checkbox" data-toggle="mud" ${state.mud?'checked':''}><span>這份手板有「泥下底質」<small>開啟後使用相同的 5 點一組方式輸入「下」。</small></span></label><p id="mud-progress" class="inline-note"></p></div>`;
}
function substrateHandboard(mode='raw',marking=false){
  const down=state.mud&&layer==='down'&&mode==='raw'&&!marking,values=down?state.down:state.line;
  return `<div class="raw-board-scroll" tabindex="0" role="region" aria-label="${down?'泥下':'表面'}底質手板，可左右滑動"><table class="raw-review-board"><caption>${down?'泥下':'表面'}底質 · ${mode==='raw'?'原始數字':'轉換代碼'}${marking?'與白化標記':''}</caption><thead><tr>${starts.map((start,s)=>`<th colspan="4" scope="colgroup">子樣區 ${s+1}<small>${start}–${start+19.5} m</small></th>`).join('')}</tr><tr>${starts.map(start=>`<th colspan="2" scope="colgroup">左欄 · ${start}–${start+9.5} m</th><th colspan="2" scope="colgroup">右欄 · ${start+10}–${start+19.5} m</th>`).join('')}</tr><tr>${starts.map(()=>`<th scope="col">m</th><th scope="col">${mode==='raw'?'數字':'代碼'}</th><th scope="col">m</th><th scope="col">${mode==='raw'?'數字':'代碼'}</th>`).join('')}</tr></thead><tbody>${Array.from({length:20},(_,row)=>`<tr class="${row%5===4?'group-end':''}">${starts.map((start,s)=>[0,1].map(col=>{
    const key=String(start+col*10+row/2),p=points.find(p=>p.key===key),code=canonical(p),v=mode==='raw'?values[key]||'':code||'',applicable=!down||normalizeCode(state.line[key]||'')==='SI',bad=applicable&&!validCode(v,down),index=s*8+col*4+Math.floor(row/5);
    const content=marking?`<div class="raw-marking-cell" data-review-key="${key}"><b>${esc(v||'未填')}</b>${mode==='raw'&&state.mud&&normalizeCode(state.line[key]||'')==='SI'?`<small>下：${esc(state.down[key]||'未填')}</small>`:''}${isBleachableCode(code)?bleachMarker(p,code):'<small>—</small>'}</div>`:`<button type="button" data-action="edit-line-group" data-group="${index}" data-layer="${down?'down':'surface'}" data-review-key="${key}" aria-label="子樣區 ${s+1}，${key} 公尺，${down?'泥下':'表面'}原始值 ${esc(v||'未填')}，點選修改">${esc(v||'未填')}${v&&bad?'<small>無效</small>':''}</button>`;
    return `<th scope="row" class="raw-position ${col===0?'segment-start':''}">${key}</th><td class="${applicable?(bad?'raw-error':''):'raw-inactive'}">${applicable?content:'<span aria-label="不適用">—</span>'}</td>`;
  }).join('')).join('')}</tr>`).join('')}</tbody></table></div>`;
}
function lineNumberReview(){
  const complete=state.linePhase==='complete';if(!state.mud)layer='surface';
  return `${linePhaseSteps()}<div class="number-review-head"><div><p class="eyebrow">${complete?'已完成白化標記':'步驟 2'}</p><h3>${complete?'回看原始數字':'確認所有原始數字'}</h3></div><p>按照手板排列；點數字可放大修改該組 5 格。</p></div>${state.mud?`<div class="layer-switch">${button('上：表面數字','layer','data-layer="surface"',layer==='surface'?'primary':'')}${button('下：泥下數字','layer','data-layer="down"',layer==='down'?'primary':'')}</div>`:''}<p class="raw-board-hint">保留手板原始數字，尚未轉換。每 5 列以粗線分隔；空白顯示「未填」。表格可左右滑動查看四個子樣區。</p>${substrateHandboard()}${complete?bleachingSummary():''}`;
}
function bleachingGroupIndexes(){return pointGroups.map((group,i)=>group.some(p=>isBleachableCode(canonical(p)))?i:-1).filter(i=>i>=0);}
function lineBleachingBoard(){
  return `${linePhaseSteps()}<div class="number-review-head"><h3>點選白化位置</h3><p>空心＝無白化；點成實心＝有白化。</p></div>${bleachingSummary()}${substrateHandboard('raw',true)}`;
}
function lineBoard(){
  if(state.linePhase==='number-review'||state.linePhase==='complete')return lineNumberReview();
  if(state.linePhase==='bleaching')return lineBleachingBoard();
  return lineEntryBoard();
}
function bleachTable(){return `<div class="table-scroll"><table class="slate belt-table"><caption>分段白化點數（不是百分比）</caption><thead><tr><th>底質類型</th>${starts.map((_,s)=>`<th>第 ${s+1} 段</th>`).join('')}</tr></thead><tbody>${['HC','SC'].map((code,ci)=>`<tr><th class="row-label">${code} 白化點數</th>${starts.map((_,s)=>`<td><input data-cell="bleach" data-key="${code}:${s+1}" data-order="${ci*4+s}" aria-label="${code} 第 ${s+1} 段白化點數" value="${esc(state.bleach[`${code}:${s+1}`]||'')}" placeholder="·" inputmode="numeric"></td>`).join('')}</tr>`).join('')}</tbody></table></div>`;}
function beltBoard(k){if(k==='fish'&&!state.fishLengthMode)return `<div class="empty-bleaching"><h3>先選擇魚類體長模式</h3><p>依手板決定分體長或不分體長，才能顯示正確列。</p>${button('返回選擇體長模式','step','data-step="1"','primary')}</div>`;const rows=rowsFor(k),excluded=excludedRows(k).length;return `<div class="board-tools"><span>${k==='impact'?'珊瑚損害與垃圾請填原始件數，系統會換算 0–3 級；百分比填 0–100；未記錄填 NA。':'沒看到請填 0；未記錄填 NA；若這版手板沒有整列，請勾「本手板未列」。'}</span><div class="tool-actions">${k==='impact'?'':button('已確認：空白格皆為 0','zero','data-sheet="'+k+'"','small')}${k==='impact'?button('查看系統分級','impact-help','','small quiet'):''}</div></div>
  ${excluded?`<div class="excluded-summary">已標記 ${excluded} 列「本手板未列」；這些列不會產生觀測資料。</div>`:''}
  <div class="table-scroll"><table class="slate belt-table"><caption>${k==='impact'?'垃圾分級由件數自動換算：0 件＝0、1 件＝1、2–4 件＝2、5 件以上＝3。':k==='rare'?'罕見生物獨立記錄；其他生物請新增具名列。':'依原手板類群與體長分列；合計會自動計算。'}<span class="mobile-note">表格可左右捲動；類群名稱固定在左側。</span></caption><colgroup><col class="row-name"><col><col><col><col><col class="sum-col"></colgroup><thead><tr><th scope="col">${k==='impact'?'項目／單位':'類群／體長'}</th>${starts.map((s,i)=>`<th scope="col">第 ${i+1} 段<small>${s}–${s+20} m</small></th>`).join('')}<th scope="col">${k==='impact'?'合計／換算':'合計'}<small>自動計算</small></th></tr></thead><tbody>${rows.map((r,ri)=>{const applicable=rowApplicable(k,r.id);return `<tr class="${applicable?'':'row-not-applicable'}"><th class="row-label" scope="row">${esc(r.zh)} ${r.size?`<span class="mini-badge">${esc(r.size)}</span>`:''}<small>${esc(r.name)}${k==='impact'?' · '+(r.type==='count'?'原始件數':r.type==='percent'?'%':'分級'):''}</small><label class="row-applicability"><input type="checkbox" data-row-applicability data-sheet="${k}" data-row-id="${r.id}" ${applicable?'':'checked'}> 本手板未列</label></th>${starts.map((_,s)=>{const key=`${r.id}:${s+1}`,v=state[k][key]||'';return `<td><input type="text" inputmode="${r.type==='percent'?'decimal':'numeric'}" autocomplete="off" data-cell="${k}" data-key="${key}" data-type="${r.type}" data-order="${ri*4+s}" aria-label="${esc(r.zh+' '+r.size)} 第 ${s+1} 段${k==='impact'?(r.type==='count'?'原始件數':'百分比'):'數量'}" value="${esc(v)}" placeholder="${applicable?'·':'—'}" ${applicable?'':'disabled'} class="${v&&!validNumber(v,r.type)?'invalid':''}"></td>`;}).join('')}<td class="row-total" data-total-sheet="${k}" data-row-id="${r.id}">—</td></tr>`;}).join('')}</tbody></table></div>
  ${k!=='impact'?`<div class="custom-row"><input id="custom-name" aria-label="其他${sheetName(k)}名稱" placeholder="其他${sheetName(k)}：填寫實際名稱" maxlength="80">${button('＋ 新增其他類群','add-custom','data-sheet="'+k+'"','small')}${!rows.some(r=>r.name==='Other')?button('原表只寫 Other','add-unspecified-other','data-sheet="'+k+'"','small quiet'):''}<small>新資料請填實際名稱；只有歷史手板未註明名稱時，才使用 Other。</small></div>${state.custom[k].map(r=>`<div class="custom-row"><span>${r.name==='Other'?'歷史資料：':'其他：'}${esc(r.zh)}</span>${button('修改名稱／移除','edit-custom',`data-sheet="${k}" data-id="${r.id}"`,'small quiet')}</div>`).join('')}`:''}`;}
function rowSummary(k,r,vals){
  if(!vals.every(v=>validNumber(v,r.type)&&normalizeNumber(v)!=='NA'))return '—';
  const numbers=vals.map(Number),sum=numbers.reduce((a,b)=>a+b,0);
  if(k==='impact'&&(r.group==='trash'||r.group==='coral_damage'))return `${sum} 件 · ${numbers.map(trashLevel).join('/')} 級`;
  if(k==='impact')return `平均 ${Math.round(sum/4*100)/100}${r.type==='percent'?'%':''}`;
  return String(sum);
}
function updateCounters(){
  if(step!==2)return;
  document.querySelectorAll('[data-map-key]').forEach(el=>{el.textContent=(layer==='down'?state.down:state.line)[el.dataset.mapKey]||'·';});
  for(const k of sheetList()){const s=sheetStats(k),el=$(`[data-tab-count="${k}"]`);if(el)el.textContent=`${s.filled}/${s.total}`;}
  const st=sheetStats(sheet);$('#filled-count').textContent=st.filled;$('#total-count').textContent=st.total;
  document.querySelectorAll('[data-total-sheet]').forEach(el=>{const k=el.dataset.totalSheet,r=rowsFor(k).find(r=>r.id===el.dataset.rowId);if(!rowApplicable(k,r.id)){el.textContent='未列';return;}const vals=starts.map((_,i)=>state[k][`${r.id}:${i+1}`]||'');el.textContent=rowSummary(k,r,vals);});
  if($('#mud-progress')){const ps=activeMudPoints(),n=ps.filter(p=>validCode(state.down[p.key]||'',true)).length;$('#mud-progress').textContent=state.mud?`上層有 ${ps.length} 個 SI 位置，泥下已填 ${n} / ${ps.length}；其餘位置沿用上層。`:'';}
}
function issueLocations(k,kind){
  const entries=k==='line'?points.map(p=>({v:state.line[p.key]||'',valid:v=>validCode(v),label:`第 ${p.s} 段 ${p.p} m`})):
    applicableRows(k).flatMap(r=>starts.map((_,s)=>({v:state[k][`${r.id}:${s+1}`]||'',valid:v=>validNumber(v,r.type),label:`${r.zh}${r.size?' '+r.size:''} 第 ${s+1} 段`})));
  const found=entries.filter(x=>kind==='missing'?x.v==='':x.v!==''&&!x.valid(x.v));return found.slice(0,3).map(x=>x.label).join('、')+(found.length>3?'等':'');
}
function issues(){const result=metaErrors().map(text=>({text,step:1}));for(const k of sheetList()){
  const st=sheetStats(k);if(st.missing)result.push({text:`${sheetName(k)}：還有 ${st.missing} 格未填（${issueLocations(k,'missing')}）。點此回填。`,sheet:k,kind:'missing'});if(st.invalid)result.push({text:`${sheetName(k)}：${st.invalid} 格無效（${issueLocations(k,'invalid')}）。點此修正。`,sheet:k,kind:'invalid'});
  if(k==='line'&&state.mud){const ps=activeMudPoints(),bad=ps.filter(p=>!validCode(state.down[p.key]||'',true));if(bad.length)result.push({text:`泥下底質：${bad.length} 個 SI 位置待填或代碼無效。`,sheet:'line',layer:'down',key:bad[0].key});}
  if(k==='line'&&state.bleaching)for(const code of ['HC','SC'])for(let s=1;s<=4;s++){const v=state.bleach[`${code}:${s}`]||'',max=points.filter(p=>{const c=normalizeCode(state.line[p.key]||'');return p.s===s&&(code==='HC'?c==='HC'||/^HC-[abc]$/.test(c):c==='SC');}).length;if(!validNumber(v)||v!=='NA'&&Number(v)>max)result.push({text:`${code} 第 ${s} 段白化點数：請填 0–${max}，不可超過已記錄的 ${code} 點數。`,sheet:'line',key:`${code}:${s}`,cell:'bleach'});}
  }
  result.push(...extraEntryIssues());
  if(unknownCount()&&!state.missingReason.trim())result.push({text:'有 NA 未記錄值，請補上未記錄原因。',reason:true});
  return result;
}
function unknownCount(){let n=sheetList().reduce((n,k)=>n+sheetStats(k).na,0);if(state.methods.includes('line')){if(state.mud)n+=activeMudPoints().filter(p=>normalizeCode(state.down[p.key]||'')==='NA').length;if(state.bleaching)n+=Object.values(state.bleach).filter(v=>v==='NA').length;}return n;}
function reviewView(){const list=issues(),total=sheetList().reduce((n,k)=>n+sheetStats(k).total,0),filled=sheetList().reduce((n,k)=>n+sheetStats(k).valid,0);return intro(4,'檢查紀錄','確認氣瓶資訊、段次與每張手板。若原始紀錄缺值，可標記 NA 並說明，交由窗口確認。')+strip()+investigationNotes()+
  `<div class="review-grid"><div class="stat"><b>${methodList().length}</b><small>種探查 · ${sheetList().length} 個手板區段</small></div><div class="stat"><b>${filled}<small>/ ${total} 格通過檢查</small></b></div><div class="stat"><b>${unknownCount()}</b><small>格 NA · 未記錄</small></div></div><section class="panel"><h2>這支氣瓶的記錄</h2>${sheetList().map(k=>{const st=sheetStats(k),excluded=k==='line'?0:excludedRows(k).length;return `<div class="review-row"><div><b>${sheetName(k)} <span class="badge">${st.valid}/${st.total} 格</span></b><p>${k==='line'?'4 段 × 40 點'+(state.mud?' · 含泥下底質':''):applicableRows(k).length+' 個適用項目 × 4 段'}${excluded?' · '+excluded+' 列本手板未列':''} · 記錄者：${esc(state.recorders[k==='rare'||k==='impact'?'invert':k])}${st.na?' · '+st.na+' 格 NA':''}</p></div>${button('回手板檢查','sheet',`data-sheet="${k}"`,'small')}</div>`;}).join('')}<div class="event-preview">氣瓶識別：${esc(eventId())}</div><details class="review-detail"><summary>展開最後逐組確認（原始數字／轉換代碼／白化）</summary>${reviewTables()}</details></section>
  ${unknownCount()?`<label class="field panel">未記錄原因 <span class="required">必填</span><textarea data-state="missingReason" placeholder="請寫明手板哪一段、哪些位置為何未記錄。">${esc(state.missingReason)}</textarea><small>NA 保留為未知；不當成 0，也不列入底質有效觀測分母。</small></label>`:''}
  ${state.methods.includes('line')?bleachingSummary():''}<section id="review-issues">${issueMarkup(list)}</section><label class="check-ack"><input id="acknowledge" type="checkbox" ${state.ack?'checked':''}><span>我已對照原始手板，確認樣點、日期、氣瓶時間、深度、各段數值與逐格白化標記。<br><small>送出後資料會先存為草稿，待活動窗口審核。</small></span></label><div class="actions">${button('← 返回手板','step','data-step="2"')}<div class="right">${button('下載草稿','export-draft','','quiet')}${button(DEBUG?'預覽完成頁':unknownCount()?'送出儲存・標示待確認':'送出儲存','complete','','primary')}</div></div><p id="complete-error" class="form-error" role="alert"></p>`;}
function issueMarkup(list){return list.length?`<div class="callout warning"><div><b>有 ${list.length} 項需要補充</b><ul class="issue-list">${list.map((x,i)=>`<li><button class="issue-link" data-action="issue" data-index="${i}">${esc(x.text)}</button></li>`).join('')}</ul></div></div>`:`<div class="review-ok">✓ 所選手板的必填格與範圍檢查通過${unknownCount()?'；NA 已附原因，仍需窗口確認':''}。</div>`;}
function canonical(p){const up=normalizeCode(state.line[p.key]||'');if(up!=='SI'||!state.mud)return up||null;const d=normalizeCode(state.down[p.key]||'');if(d==='SI')return 'SI';if(d==='NA')return 'NA';return validCode(d,true)?`SI(${d})`:null;}
function reviewValue(value,valid=true){const empty=value===null||value===undefined||value==='';return `<span class="review-value ${empty||!valid?'review-invalid':value==='NA'?'review-na':''}">${esc(empty?'未填':value)}</span>`;}
function isBleachableCode(code){return code==='SC'||code==='HC'||/^HC-[abc]$/.test(code||'');}
function bleachMarker(p,code){if(!isBleachableCode(code))return '';const active=Boolean(state.bleached[p.key]);return `<button type="button" class="bleach-marker ${active?'active':''}" data-action="toggle-bleach-point" data-key="${p.key}" aria-pressed="${active}" aria-label="${p.p} m ${active?'有白化，點一下改為無白化':'無白化，點一下標記有白化'}"><span aria-hidden="true"></span><small>${active?'白化':'無白化'}</small></button>`;}
function reviewLineCell(p){const code=canonical(p),shown=reviewMode==='raw'?(state.line[p.key]||null):code;return `<div class="review-cell"><span><small>${reviewMode==='raw'?'原始數字':'轉換代碼'}</small>${reviewValue(shown,reviewMode==='raw'?validCode(shown??''):validCode(code??''))}</span>${bleachMarker(p,code)}</div>`;}
function reviewLineTable(){const group=pointGroups[reviewGroup]||pointGroups[0];return `<div class="final-zoom-list">${group.map(p=>`<div class="final-zoom-row"><b>${p.p} m</b>${reviewLineCell(p)}</div>`).join('')}</div>`;}
function reviewBeltTable(k){return `<div class="table-scroll" tabindex="0" role="region" aria-label="${sheetName(k)}對照手板，可左右捲動"><table class="slate belt-table"><caption>${sheetName(k)}手板 · 記錄值（唯讀）${k==='impact'?'<small>垃圾顯示原始件數；右欄顯示各段自動分級。</small>':''}</caption><colgroup><col class="row-name"><col><col><col><col><col class="sum-col"></colgroup><thead><tr><th scope="col">${k==='impact'?'項目／單位':'類群／體長'}</th>${starts.map((start,s)=>`<th scope="col">第 ${s+1} 段<small>${start}–${start+20} m</small></th>`).join('')}<th scope="col">${k==='impact'?'合計／換算':'合計'}<small>自動計算</small></th></tr></thead><tbody>${rowsFor(k).map(r=>{const applicable=rowApplicable(k,r.id),vals=starts.map((_,s)=>state[k][`${r.id}:${s+1}`]??''),total=applicable?rowSummary(k,r,vals):'未列';return `<tr class="${applicable?'':'row-not-applicable'}"><th class="row-label" scope="row">${esc(r.zh)} ${r.size?`<span class="mini-badge">${esc(r.size)}</span>`:''}<small>${esc(r.name)}${k==='impact'?' · '+(r.type==='count'?'原始件數':r.type==='percent'?'%':'分級'):''}</small></th>${applicable?vals.map(v=>`<td>${reviewValue(v,validNumber(v,r.type))}</td>`).join(''):`<td colspan="4"><span class="review-value">本手板未列</span></td>`}<td class="row-total">${total}</td></tr>`;}).join('')}</tbody></table></div>`;}
function reviewLinePanel(){
  return `<div class="review-mode-tabs" role="tablist" aria-label="底質顯示方式"><button role="tab" aria-selected="${reviewMode==='raw'}" class="${reviewMode==='raw'?'active':''}" data-action="review-mode" data-mode="raw">原始數字</button><button role="tab" aria-selected="${reviewMode==='canonical'}" class="${reviewMode==='canonical'?'active':''}" data-action="review-mode" data-mode="canonical">轉換代碼</button></div><p class="raw-board-hint">依手板表格核對所有位置；可切換原始數字與最終代碼，並修正白化標記。</p>${substrateHandboard(reviewMode,true)}<div class="bleach-legend"><span><i class="bleach-dot" aria-hidden="true"></i> 無白化</span><span><i class="bleach-dot active" aria-hidden="true"></i> 有白化</span></div>`;
}
function reviewTables(){return sheetList().map(k=>k==='line'?`<div id="review-line-panel">${reviewLinePanel()}</div>`:reviewBeltTable(k)).join('');}
async function submitRecord(){
  if(saving)return;
  if(DEBUG){step=4;render();$('#main').focus();window.scrollTo({top:0});return;}
  if(issues().length){step=3;render();$('#complete-error').textContent='請先處理待補充項目；可先下載草稿。';return;}
  if(!state.ack){step=3;render();$('#complete-error').textContent='請勾選已對照原始手板的確認。';$('#acknowledge').focus();return;}

  // Persist the draft first; reloading during request is recoverable.
  state.status='draft';delete state.simulatedSave;delete state.dbSave;persist(false);
  saving=true;submitError=null;step=4;render();$('#main').focus();window.scrollTo({top:0});

  try {
    const p = payload(false);
    const headers = { 'Content-Type': 'application/json' };
    if (csrfToken) headers['X-CSRF-Token'] = csrfToken;

    const res = await fetch('/api/public/reef-check/surveys', {
      method: 'POST',
      headers,
      credentials: 'include',
      body: JSON.stringify(p)
    });

    if (res.ok) {
      const result = await res.json();
      state.status = result.review_status || (unknownCount() ? 'needs_review' : 'submitted');
      state.dbSave = result;
      state.simulatedSave = result;
      state.uploaded = true;
      persist(false);
    } else {
      let errMsg = `伺服器回應錯誤 (${res.status})`;
      try {
        const errJson = await res.json();
        if (errJson.error) errMsg = errJson.error;
      } catch (_) {}
      submitError = errMsg;
      state.status = 'draft';
      toast(`儲存未成功：${errMsg}`);
    }
  } catch (err) {
    submitError = '網路連線失敗，請檢查網路連線後重試。草稿已安全保留在此裝置。';
    state.status = 'draft';
    toast('網路連線失敗，請先下載草稿或稍候重試。');
  } finally {
    if(storageFailed){state.status='draft';const i=records.findIndex(r=>r.id===state.id);if(i>=0)records[i]=structuredClone(state);}
    saving=false;
    render();
    $('#main').focus();
  }
}
function completeView(){
  if(DEBUG)return intro(4,'測試流程已走完','這是 Debug Mode 的完成頁預覽；沒有送出或寫入任何資料。')+`<section class="panel success-panel"><div class="success-icon debug-success" aria-hidden="true">✓</div><h2 role="status">完成頁預覽成功</h2><p>正式模式仍會檢查所有必填資料、逐格內容與核對聲明，通過後才會送入資料庫。</p><span class="badge debug-badge">DEBUG · 未儲存</span><div class="actions">${button('返回檢查頁','step','data-step="3"','primary')}${button('回到第一頁','step','data-step="0"')}</div></section>`;
  if(saving)return intro(4,'正在儲存這支氣瓶的記錄…','正在連線伺服器並寫入資料庫，請稍候。')+`<section class="panel success-panel" aria-busy="true"><div class="save-spinner" aria-hidden="true"></div><h2 role="status">資料庫儲存中</h2><p>整理氣瓶資訊、各段觀測與泥下底質代碼，寫入資料庫。<br>請勿關閉頁面或重複送出。</p><span class="badge">正在同步至資料庫</span></section>`;
  
  if(submitError)return intro(4,'儲存至資料庫未成功','手板資料已完整保存在此瀏覽器，不會遺失。')+`<section class="panel success-panel"><h2 role="alert">未能存入資料庫</h2><p class="form-error" style="margin:1rem 0;font-size:1.05rem;">${esc(submitError)}</p><p>請確認網路連線正常後重試，或先下載草稿 JSON 備份。</p><div class="actions">${button('重新儲存至資料庫','complete','','primary')}${button('下載草稿備份','export-draft')}${button('返回檢查','step','data-step="3"')}</div></section>`;

  if(storageFailed)return intro(4,'這次還沒有儲存成功。','本機儲存空間或瀏覽器設定可能阻止了儲存。')+`<section class="panel success-panel"><h2 role="alert">請重試，或先下載備份</h2><p>記錄仍在目前頁面中，請先不要關閉。</p><div class="actions">${button('重新儲存','complete','','primary')}${button('下載草稿備份','export-draft')}${button('返回檢查','step','data-step="3"')}</div></section>`;

  const receipt = state.dbSave || state.simulatedSave;
  const isDB = Boolean(state.dbSave);
  return intro(4,'紀錄已存入資料庫','感謝您的填報！調查數據已正式存檔，並視需要下載備份。')+
    `<section class="panel success-panel"><div class="success-icon" aria-hidden="true">✓</div><h2 role="status">${receipt ? '成功存入資料庫' : '已完成記錄'}</h2><p>${receipt ? '這支氣瓶的所有探查與觀測數據已成功寫入資料庫。' : '這是先前保留的本機紀錄。'}</p><span class="badge success-badge">${receipt ? (isDB ? '✓ 資料庫已存檔' : '本機留存') : '本機紀錄'} · 回執編號 ${esc(receipt?.receipt_id || state.id)}</span><dl class="save-receipt"><div><dt>調查樣點</dt><dd>${esc(siteName())}</dd></div><div><dt>日期／氣瓶開始時間</dt><dd>${esc(state.meta.date)} · ${esc(state.meta.time)}</dd></div><div><dt>深度／探查</dt><dd>${esc(state.meta.depth)} m · ${methodList().map(k=>C.methods[k].short).join('、')}</dd></div>${receipt?`<div><dt>資料庫儲存時間</dt><dd>${esc(new Date(receipt.saved_at).toLocaleString('zh-TW',{hour12:false}))}</dd></div><div><dt>官方回執編號</dt><dd>${esc(receipt.receipt_id)}</dd></div>`:''}</dl><div class="event-preview">氣瓶識別：${esc(receipt?.event_id || eventId())}</div><div class="callout ${unknownCount()?'warning':''}"><div><b>${unknownCount()?'已保留未記錄值 (NA)，待窗口確認':'已完成填報，待窗口審核'}</b><p>${unknownCount()?'NA 與原因已隨紀錄完整存入資料庫，不會改成 0。':'資料庫已收錄此筆調查紀錄。'}活動窗口將於後台進行核對與統計。</p></div></div><div class="actions">${button('檢視這份紀錄','view-saved','','primary')}${button('下載備份 JSON','export','','quiet')}</div></section><div class="actions">${button('我的紀錄','drafts')}${button('記錄下一支氣瓶 →','new-same','','primary')}</div>`;
}
function payload(draft=false){
  const event=eventKey(),id=eventId(),m=state.meta;
  const key=k=>`${event}|${{line:'line',fish:'belt_fish',invert:'belt_invert'}[k]}`;
  const visibilityMin=m.visibilityMin||m.visibilityMax,visibilityMax=m.visibilityMax||m.visibilityMin;
  const data={
    format:'reefcheck-web-v4',
    activity_code:state.activityCode,
    entry_metadata:{weather:m.weather,air_temperature_c:asNumber(m.airTemperature),latitude:asNumber(m.latitude),longitude:asNumber(m.longitude)},
    rkc_bleaching_note:state.rkcNote,
    other_damage_note:state.otherDamageNote,
    bleaching_mode:'point_marks',
    status:draft?'draft':unknownCount()?'needs_review':'complete_local',
    uploaded:!draft && Boolean(state.dbSave || state.simulatedSave),
    exported_at:new Date().toISOString(),
    event:{
      event_key:event,
      event_id:id,
      site_id:siteId(),
      site_name_en__lookup:m.site,
      site_name_zh:siteName(),
      survey_date:m.date,
      event_time:m.time,
      depth_m:m.depth===''?null:Number(m.depth)
    },
    transects:methodList().map(k=>({
      transect_key:key(k),
      event_id:id,
      method:{line:'line',fish:'belt_fish',invert:'belt_invert'}[k],
      fish_length_mode:k==='fish'&&state.fishLengthMode?state.fishLengthMode:undefined,
      coral_morphology:k==='line'?state.coralMorphology:undefined,
      start_time:m.time,
      water_temp_c:m.temperature===''?null:Number(m.temperature),
      visibility_min_m:visibilityMin===''?null:Number(visibilityMin),
      visibility_max_m:visibilityMax===''?null:Number(visibilityMax),
      recorders:state.recorders[k].split(/[、,，]/).map(x=>x.trim()).filter(Boolean),
      team_leader:m.leader,
      team_scientist:m.scientist,
      comments:state.notes
    })),
    substrate_points:[],
    substrate_bleaching:[],
    belt_observations:[],
    impact_observations:[],
    mud_audit:[],
    not_applicable_rows:sheetList().filter(k=>k!=='line').flatMap(k=>excludedRows(k).map(r=>({sheet:k,row_id:r.id,name:r.name,size:r.size||null,reason:'not_on_source_board'}))),
    missing_reason:state.missingReason,
    validation_issues:issues().map(x=>x.text)
  };
  if(state.methods.includes('line')){
    data.substrate_points=points.map(p=>({
      transect_key:key('line'),
      segment:p.s,
      position_m:p.p,
      substrate_code:canonical(p),
      substrate_layer:'surface',
      is_bleached:isBleachableCode(canonical(p))?(typeof state.bleached[p.key]==='boolean'?state.bleached[p.key]:null):null
    }));
    if(state.mud)data.mud_audit=activeMudPoints().map(p=>({
      position_m:p.p,
      segment:p.s,
      surface:state.line[p.key],
      down:state.down[p.key]||null,
      canonical:canonical(p)
    }));
    if(state.bleaching)data.substrate_bleaching=['HC','SC'].flatMap(code=>starts.map((_,s)=>({
      transect_key:key('line'),
      substrate_code:code,
      segment:s+1,
      bleached_points:asNumber(state.bleach[`${code}:${s+1}`])
    })));
  }
  for(const k of ['fish','invert','rare'])if(state.methods.includes(k==='rare'?'invert':k))
    data.belt_observations.push(...applicableRows(k).flatMap(r=>starts.map((_,s)=>({
      transect_key:key(k==='rare'?'invert':k),
      taxon_group:k,
      taxon_name_en__lookup:r.name,
      taxon_size_class__lookup:r.size,
      taxon_local_row_id:r.id,
      observed_size:k==='rare'?(state.rareDetails[r.id]?.size||''):undefined,
      notes:k==='rare'?(state.rareDetails[r.id]?.notes||''):undefined,
      segment:s+1,
      count:asNumber(state[k][`${r.id}:${s+1}`]),
      record_status:state[k][`${r.id}:${s+1}`]==='NA'?'not_recorded':state[k][`${r.id}:${s+1}`]===undefined||state[k][`${r.id}:${s+1}`]===''?'blank':'recorded'
    }))));
  if(state.methods.includes('invert'))
    data.impact_observations=applicableRows('impact').flatMap(r=>starts.map((_,s)=>{
      const raw=asNumber(state.impact[`${r.id}:${s+1}`]);
      return {
        transect_key:key('invert'),
        impact_group:r.group,
        impact_name_en__lookup:r.name,
        impact_value_type:r.type,
        segment:s+1,
        raw_value:raw
      };
    }));
  if(state.dbSave)data.db_save=structuredClone(state.dbSave);
  if(state.simulatedSave)data.simulated_save=structuredClone(state.simulatedSave);
  if(draft)data.draft_state=structuredClone(state);
  return data;
}
function asNumber(v){return v!==undefined&&v!==''&&v!=='NA'&&Number.isFinite(Number(v))?Number(v):null;}
function download(draft){const blob=new Blob([JSON.stringify(payload(draft),null,2)],{type:'application/json'}),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=`${(eventId()||'reefcheck').replace(/[^\w.\-]/g,'_')}-${draft?'draft':'record'}.json`;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);toast('已啟動下載，請查看瀏覽器下載項目。');}
function modal(title,body){$('#modal-content').innerHTML=`<div class="modal-title"><h2>${title}</h2><button data-action="close" aria-label="關閉對話框">×</button></div>${body}`;if(!$('#modal').open)$('#modal').showModal();}
function openDrafts(){modal('我的紀錄',`<p>只顯示此瀏覽器儲存的紀錄。不同瀏覽器或裝置不會自動同步。</p>${records.length?records.slice().reverse().map(r=>`<div class="draft-item"><div><b>${esc(C.sites.find(s=>s[2]===r.meta.site||s[1]===r.meta.site)?.[1]||'尚未選擇樣點')}</b><small>${esc(r.meta.date||'日期未填')} · ${esc(r.meta.time||'時間未填')} · ${esc(r.meta.depth||'—')} m</small><small>${r.methods.map(k=>C.methods[k].short).join('、')} · ${r.status==='draft'?'草稿':r.status==='needs_review'?'已存・待確認':r.uploaded||r.dbSave?'已存入資料庫':'已完成・本機'}</small></div>${button('開啟','resume',`data-id="${esc(r.id)}"`,'small')}</div>`).join(''):'<p>還沒有儲存的紀錄。</p>'}<div class="actions">${button('新增一支氣瓶','new','','primary')}</div>`);}
function help(){modal('填寫說明',`<ol><li><b>先選探查，再填氣瓶資訊。</b>同一支氣瓶選多種探查，日期、時間、深度與水溫共用；能見度有範圍時填最低與最高，只有一個值時填任一格即可。</li><li><b>底質每次填 5 格：</b>一個子樣區有 40 格。先照手板輸入原始數字，第 5 格填完後不會自動跳轉；請先核對，再按「確認這 5 格，下一組」。系統會先檢查空白與無效代碼。</li><li><b>數字確認後才轉代碼：</b>可用 1–9、0、a–c 及 91–98；0 會記為 OT，a–c 會保留為 HC-a–HC-c。最後確認可在「原始數字／轉換代碼」切換。</li><li><b>有上下手板：</b>勾選「泥下底質」，切換上／下，依同位置配對。SI(SI) 會記為 SI。</li><li><b>數字確認後逐格標記白化：</b>HC／SC 格會出現圓點；空心代表無白化，看到白化時點成實心。</li><li><b>Belt 照列填四段。</b>0 表示看過但沒看到；NA 或 - 表示未記錄；舊版手板沒有整列時勾「本手板未列」，不要補成 0。</li><li><b>珊瑚損害與垃圾一律填原始件數。</b>系統依每一段自動換算：0 件＝0 級、1 件＝1 級、2–4 件＝2 級、5 件以上＝3 級。</li><li><b>其他生物優先填實際名稱。</b>只有歷史手板真的只寫 Other 時，才使用「原表只寫 Other」。</li><li><b>檢查確認後存入系統資料庫。</b>若連線中斷仍可本機儲存與下載草稿；NA 需附原因。</li></ol>`);}
function setFieldFocus(html){
  const panel=$('#field-focus');if(!panel)return;
  if(!$('#focus-description'))panel.innerHTML=`<span id="focus-description"></span>${button('此格未記錄 NA','mark-na','','small')}`;
  $('#focus-description').innerHTML=html;
}
function handleCell(el){
  const k=el.dataset.cell,key=el.dataset.key,old=el.dataset.previous??state[k][key]??'';
  const explicitSilt=k==='line'&&el.value.trim().toUpperCase().replaceAll('（','(').replaceAll('）',')')==='SI(SI)';
  const v=k==='line'||k==='down'?el.value.trim():normalizeNumber(el.value);
  state[k][key]=v;el.value=v;el.dataset.previous=v;
  const valid=k==='line'||k==='down'?validCode(v,k==='down'):validNumber(v,el.dataset.type||'count');
  el.classList.toggle('invalid',v!==''&&!valid);el.setAttribute('aria-invalid',String(v!==''&&!valid));
  if(k==='line'&&normalizeCode(old)!==normalizeCode(v)&&state.down[key]){delete state.down[key];toast('上層已更改，該位置泥下值已清除，請重新核對。');}
  if(k==='line'&&normalizeCode(old)!==normalizeCode(v))delete state.bleached[key];
  if(k==='line'||k==='down')state.linePhase='entry';
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
  if(el.name==='fish-length-mode'){state.fishLengthMode=el.value;persist();toast('已切換體長模式；各模式原有數值仍保留於草稿。');}
  if(el.name==='coral-morphology'){state.coralMorphology=el.value;persist();}
  if(el.hasAttribute('data-row-applicability')){const k=el.dataset.sheet,id=el.dataset.rowId,list=new Set(state.notApplicable[k]||[]);if(el.checked)list.add(id);else list.delete(id);state.notApplicable[k]=[...list];persist();render();toast(el.checked?'已標記為本手板未列，不會產生觀測值。':'已恢復這一列，請填寫四段數值。');}
  if(el.dataset.cell)handleCell(el);
  if(el.dataset.toggle){const k=el.dataset.toggle;if(!el.checked&&(k==='mud'?Object.keys(state.down).length:Object.keys(state.bleach).length)){el.checked=true;modal('關閉這張附表？',`<p>已填的資料會保留在草稿，但關閉期間不會放進完成記錄。重新勾選後可以繼續核對。</p><div class="actions">${button('保留附表','close')}${button('確認關閉','disable-extra',`data-extra="${k}"`,'primary')}</div>`);}else{state[k]=el.checked;persist();render();}}
  if(el.id==='acknowledge'){state.ack=el.checked;persist(false);}
});
document.addEventListener('input',e=>{const el=e.target;if(el.hasAttribute('data-activity-code')){state.activityCode=el.value;state.activity=null;persist();}if(el.dataset.rareDetail){state.rareDetails[el.dataset.rowId]={...(state.rareDetails[el.dataset.rowId]||{}),[el.dataset.rareDetail]:el.value};persist();}if(el.dataset.meta){state.meta[el.dataset.meta]=el.value;persist();$('#event-preview').textContent=eventId()?`氣瓶識別：${eventId()}`:'填好樣點、日期、時間與深度後，自動產生氣瓶識別。';}if(el.dataset.recorder){state.recorders[el.dataset.recorder]=el.value;persist();}if(el.dataset.state){state[el.dataset.state]=el.value;persist();if(step===3){$('#review-issues').innerHTML=issueMarkup(issues());$('#acknowledge').checked=false;}}if(el.dataset.cell){const k=el.dataset.cell,key=el.dataset.key,old=state[k][key]||'',v=k==='line'||k==='down'?el.value.trim():normalizeNumber(el.value);state[k][key]=v;if(k==='line'&&validCode(v)&&normalizeCode(v)!==normalizeCode(old)&&state.down[key])delete state.down[key];if(k==='line'&&normalizeCode(old)!==normalizeCode(v))delete state.bleached[key];if(k==='line'||k==='down')state.linePhase='entry';if(k==='line'&&/^SI[（(]SI[）)]$/i.test(el.value.trim()))state.down[key]='SI';persist();updateCounters();}});
document.addEventListener('focusin',e=>{const el=e.target;if(el.dataset.cell){lastCell=el;el.dataset.previous=state[el.dataset.cell][el.dataset.key]||'';setFieldFocus(`<b>${esc(el.getAttribute('aria-label'))}</b> · ${el.dataset.cell==='down'?'對照上層 SI 的同一位置。':el.dataset.cell==='line'?'可輸入代碼或數字；Tab 沿此欄往下。':'填 0 表示未發現，NA 表示未記錄。'}`);}});
document.addEventListener('keydown',e=>{
  const el=e.target;if(!el.dataset.cell||!['Tab','Enter','ArrowUp','ArrowDown'].includes(e.key))return;
  const cells=[...document.querySelectorAll(`[data-cell="${el.dataset.cell}"]:not(:disabled)`)].sort((a,b)=>Number(a.dataset.order)-Number(b.dataset.order));const index=cells.indexOf(el),delta=e.key==='ArrowUp'||e.shiftKey?-1:1,next=cells[index+delta];
  if(next){e.preventDefault();handleCell(el);next.focus();next.select();}else if(e.key==='Enter'){e.preventDefault();handleCell(el);toast('這 5 格已填完；請先核對，再按「確認這 5 格，下一組」。');}
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
  if(a==='debug-line-phase'){
    if(!DEBUG||!['entry','number-review','bleaching','complete'].includes(el.dataset.phase))return;
    state.linePhase=el.dataset.phase;sheet='line';if(!state.methods.includes('line'))state.methods.push('line');
    if(state.linePhase==='complete'){reviewGroup=linePage;go(3);}else{step=2;persist(false);render();}
  }
  else if(a==='line-return-phase'){if(!['entry','number-review'].includes(el.dataset.phase))return;state.linePhase=el.dataset.phase;persist();render();}
  else if(a==='locate-line-group'){const index=Number(el.dataset.group);if(!Number.isInteger(index)||!pointGroups[index])return;linePage=index;render();}
  else if(a==='home'||a==='step')go(a==='home'?0:Number(el.dataset.step));
  else if(a==='duplicate-back'){$('#modal').close();go(1);}
  else if(a==='mark-na'){if(lastCell?.isConnected){lastCell.value='NA';handleCell(lastCell);lastCell.focus();}else toast('請先選取要標記的格子。');}
  else if(a==='verify-activity'){void verifyActivity();}
  else if(a==='demo-activity'){if(!DEBUG)return;state.activityCode='DEMO';state.activity={name:'測試活動',start_date:'2000-01-01',end_date:'2099-12-31',site_ids:C.sites.map(s=>s[3]).filter(Boolean),demo:true};persist();render();}
  else if(a==='help')help();else if(a==='drafts')openDrafts();else if(a==='close'){$('#modal').close();$('#modal-content').innerHTML='';}
  else if(a==='impact-help'){modal('原始件數與系統分級',`<p>珊瑚損害與垃圾都請直接抄錄手板的原始件數，不需要自己換算。</p><ul><li>0 件 → 0 級</li><li>1 件 → 1 級</li><li>2–4 件 → 2 級</li><li>5 件以上 → 3 級</li></ul><p>資料庫只保留志工填寫的原始件數；級別由後台隨時計算。</p><div class="actions">${button('我知道了','close','','primary')}</div>`);}
  else if(a==='sheet'){sheet=el.dataset.sheet;go(2);}
  else if(a==='line-page'){
    const destination=Number(el.dataset.group),bad=destination>linePage&&groupProblem(linePage);
    if(bad&&!DEBUG){const cell=document.querySelector(`[data-cell="${layer==='down'?'down':'line'}"][data-key="${CSS.escape(bad.key)}"]`);cell?.classList.add('invalid');cell?.setAttribute('aria-invalid','true');toast(`${layer==='down'?'泥下':'表面'} ${bad.p} m 尚未填寫或不是有效代碼，請修正後再前往下一組。`);cell?.focus();return;}
    linePage=destination;render();document.querySelector(`[data-cell="${layer==='down'?'down':'line'}"]`)?.focus();
  }
  else if(a==='line-bleach-page'){linePage=Number(el.dataset.group);render();}
  else if(a==='edit-line-group'){linePage=Number(el.dataset.group);state.linePhase='entry';layer=el.dataset.layer==='down'?'down':'surface';persist();render();document.querySelector(`[data-cell="${layer==='down'?'down':'line'}"]`)?.focus();}
  else if(a==='review-line-numbers'){state.linePhase='number-review';persist();render();window.scrollTo({top:0});}
  else if(a==='confirm-line-numbers'){
    const surfaceBad=points.find(p=>!validCode(state.line[p.key]||'')),downBad=state.mud&&activeMudPoints().find(p=>!validCode(state.down[p.key]||'',true)),bad=surfaceBad||downBad,isDownBad=!surfaceBad&&Boolean(downBad);
    if(bad&&!DEBUG){linePage=pointGroups.findIndex(group=>group.includes(bad));layer=isDownBad?'down':'surface';state.linePhase='entry';persist();render();toast(`${isDownBad?'泥下':'表面'} ${bad.p} m 尚未填寫或數字無效。`);document.querySelector(`[data-cell="${isDownBad?'down':'line'}"][data-key="${CSS.escape(bad.key)}"]`)?.focus();return;}
    points.forEach(p=>{if(isBleachableCode(canonical(p))){if(typeof state.bleached[p.key]!=='boolean')state.bleached[p.key]=false;}else delete state.bleached[p.key];});state.linePhase='bleaching';linePage=bleachingGroupIndexes()[0]||0;persist();render();window.scrollTo({top:0});
  }
  else if(a==='confirm-line-bleaching'){state.linePhase='complete';persist();render();$('#investigation-notes')?.scrollIntoView({block:'center'});}
  else if(a==='segment'){segment=Number(el.dataset.segment);render();}
  else if(a==='review-segment'){reviewSegment=Number(el.dataset.segment);$('#review-line-panel').innerHTML=reviewLinePanel();$(`[data-action="review-segment"][data-segment="${reviewSegment}"]`).focus({preventScroll:true});}
  else if(a==='review-group'){reviewGroup=Number(el.dataset.group);$('#review-line-panel').innerHTML=reviewLinePanel();$(`[data-action="review-group"][data-group="${reviewGroup}"]`)?.focus({preventScroll:true});}
  else if(a==='review-mode'){reviewMode=el.dataset.mode;$('#review-line-panel').innerHTML=reviewLinePanel();$(`[data-action="review-mode"][data-mode="${reviewMode}"]`).focus({preventScroll:true});}
  else if(a==='toggle-bleach-point'){const key=el.dataset.key,p=points.find(p=>p.key===key),code=p&&canonical(p);if(!p||!isBleachableCode(code))return;state.bleached[key]=!Boolean(state.bleached[key]);state.ack=false;persist();const active=state.bleached[key];el.classList.toggle('active',active);el.setAttribute('aria-pressed',String(active));el.setAttribute('aria-label',`${p.p} m ${active?'有白化，點一下改為無白化':'無白化，點一下標記有白化'}`);el.querySelector('small').textContent=active?'白化':'無白化';document.querySelectorAll('[data-bleaching-summary]').forEach(el=>el.outerHTML=bleachingSummary());const ack=$('#acknowledge');if(ack)ack.checked=false;}
  else if(a==='layer'){layer=el.dataset.layer;render();}
  else if(a==='next-sheet'){const idx=sheetList().indexOf(sheet);if(idx<sheetList().length-1){sheet=sheetList()[idx+1];go(2);}else go(3);}
  else if(a==='export-draft')download(true);
  else if(a==='export'){if(issues().length){go(3);toast('還有需補充項目，請先檢查或下載草稿。');}else download(false);}
  else if(a==='complete'){void submitRecord();}
  else if(a==='view-saved'){modal('已儲存的紀錄（資料庫／本機）',`<p>${esc(siteName())} · ${esc(state.meta.date)} · ${esc(state.meta.time)} · ${esc(state.meta.depth)} m</p>${reviewTables()}<div class="actions">${button('關閉','close')}${button('修改這份紀錄','edit-saved','','primary')}</div>`);}
  else if(a==='edit-saved'){$('#modal').close();$('#modal-content').innerHTML='';go(3);}
  else if(a==='zero'){const k=el.dataset.sheet;if(k==='impact')return;const n=applicableRows(k).reduce((n,r)=>n+starts.filter((_,s)=>!state[k][`${r.id}:${s+1}`]).length,0);modal('確認空白格都是「未發現」',`<p>將${sheetName(k)}手板中適用項目的 ${n} 個空白格填成 0。只適用於確實調查過、但沒有看到的項目。本版手板沒有的整列，請先勾「本手板未列」；未記錄請填 NA。</p><div class="actions">${button('返回核對','close')}${button('確認填 0','confirm-zero',`data-sheet="${k}"`,'primary')}</div>`);}
  else if(a==='confirm-zero'){const k=el.dataset.sheet;if(k==='impact')return;applicableRows(k).forEach(r=>starts.forEach((_,s)=>{const key=`${r.id}:${s+1}`;if(!state[k][key])state[k][key]='0';}));persist();$('#modal').close();render();}
  else if(a==='add-custom'){const name=$('#custom-name').value.trim(),k=el.dataset.sheet;if(!name){toast('先填其他生物的實際名稱。');$('#custom-name').focus();return;}if(rowsFor(k).some(r=>r.zh.toLowerCase()===name.toLowerCase())){toast('這個類群已在表格內，請直接填寫原列。');return;}state.custom[k].push({id:'custom-'+Date.now().toString(36),name:`Other: ${name}`,zh:name});persist();render();toast('已新增一列，請填入四段數量。');}
  else if(a==='add-unspecified-other'){const k=el.dataset.sheet;if(rowsFor(k).some(r=>r.name==='Other')){toast('這張手板已有未註明的 Other 列。');return;}state.custom[k].push({id:`legacy-other-${k}`,name:'Other',zh:'其他（原表未註明）',legacy:true});persist();render();toast('已新增歷史 Other 列；請填四段數量。');}
  else if(a==='edit-custom'){const r=state.custom[el.dataset.sheet].find(r=>r.id===el.dataset.id);modal('修改其他類群',`<label class="field">生物名稱<input id="rename-custom" value="${esc(r.zh)}" maxlength="80"></label><p>修改名稱會保留已填數值；移除會一併刪除這一列的四段數值。</p><div class="actions">${button('移除此列','remove-custom',`data-sheet="${el.dataset.sheet}" data-id="${r.id}"`,'danger')}${button('儲存名稱','rename-custom',`data-sheet="${el.dataset.sheet}" data-id="${r.id}"`,'primary')}</div><p id="rename-error" class="form-error" role="alert"></p>`);}
  else if(a==='rename-custom'){const k=el.dataset.sheet,id=el.dataset.id,name=$('#rename-custom').value.trim();if(!name||rowsFor(k).some(r=>r.id!==id&&r.zh.toLowerCase()===name.toLowerCase())){$('#rename-error').textContent='請填寫不重複的實際生物名稱。';return;}const r=state.custom[k].find(r=>r.id===id);r.zh=name;r.name=`Other: ${name}`;persist();$('#modal').close();render();}
  else if(a==='remove-custom'){const k=el.dataset.sheet,id=el.dataset.id;state.custom[k]=state.custom[k].filter(r=>r.id!==id);state.notApplicable[k]=(state.notApplicable[k]||[]).filter(x=>x!==id);starts.forEach((_,s)=>delete state[k][`${id}:${s+1}`]);persist();$('#modal').close();render();toast('已移除該自訂類群與四段數值。');}
  else if(a==='disable-extra'){state[el.dataset.extra]=false;persist();$('#modal').close();render();}
  else if(a==='issue'){const x=issues()[Number(el.dataset.index)];if(!x)return;if(x.field){document.querySelector(`[data-state="${x.field}"]`)?.focus();return;}if(x.reason){$('[data-state="missingReason"]').focus();return;}if(x.step===1){go(1);return;}sheet=x.sheet;layer=x.layer||'surface';segment=0;if(sheet==='line'){const point=x.key&&points.find(p=>p.key===x.key)||points.find(p=>x.kind==='missing'?!state.line[p.key]:state.line[p.key]&&!validCode(state.line[p.key]));if(point){linePage=pointGroups.findIndex(group=>group.includes(point));state.linePhase='entry';}}go(2);let input;if(x.key)input=document.querySelector(`[data-cell="${x.cell||x.layer==='down'&&'down'||sheet}"][data-key="${CSS.escape(x.key)}"]`);else input=[...document.querySelectorAll(`[data-cell="${sheet}"]`)].find(el=>x.kind==='missing'?!el.value:el.classList.contains('invalid'));input?.focus();input?.scrollIntoView({block:'center'});}
  else if(a==='resume'){const saved=records.find(r=>r.id===el.dataset.id);if(!saved)return;state=structuredClone(saved);step=state.status==='draft'?Math.min(state.step,3):4;sheet=sheetList()[0];$('#modal').close();render();window.scrollTo({top:0});}
  else if(a==='new'||a==='new-same'){const m=a==='new-same'?{site:state.meta.site,date:state.meta.date,leader:state.meta.leader,scientist:state.meta.scientist}:{};if(state.methods.length)persist(false);state=fresh(m);step=0;sheet='line';layer='surface';linePage=0;reviewGroup=0;$('#modal').close();render();window.scrollTo({top:0});}
});
// Keep v1 drafts intact. Never overwrite another tab's newly created record.
window.addEventListener('storage',e=>{if(e.key===KEY){records=readRecords();toast('另一個分頁更新了紀錄；繼續填寫前請確認使用的是同一份草稿。');}});
render();
initApp();

async function initApp(){
  try{
    const res=await fetch('/api/session',{credentials:'include'});
    if(res.ok){
      const data=await res.json();
      if(data.authenticated&&data.user){
        currentUser=data.user;
        csrfToken=data.csrf_token||'';
        persist(false);
      }
    }
  }catch(_){}

  try{
    const res=await fetch('/api/public/reef-check/sites',{credentials:'include'});
    if(res.ok){
      const dbSites=await res.json();
      if(Array.isArray(dbSites)&&dbSites.length){
        C.sites=dbSites.map(s=>[
          s.region||'其他樣點',
          s.name_zh,
          s.name_en||s.name_zh,
          s.id
        ]);
        if(step===1)render();
      }
    }
  }catch(_){}
}
