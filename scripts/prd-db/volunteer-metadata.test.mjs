import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {JSDOM} from 'jsdom'

const base=new URL('../../web/public/reef-check/',import.meta.url)
const siteFixtures=[
 {id:1,is_active:true,region:'小琉球',name_zh:'美人洞',name_en:'MeiRenDong',latitude:22.35,longitude:120.38},
 {id:2,is_active:true,region:'北海岸',name_zh:'另一樣點',name_en:'OtherSite',latitude:25.1,longitude:121.2},
 {id:3,is_active:true,region:'其他',name_zh:'缺座標樣點',name_en:'NoCoordinates',latitude:null,longitude:null}
]
function setup({debug=false,local=false}={}){
 const dom=new JSDOM(fs.readFileSync(new URL('index.html',base),'utf8'),{url:(local?'file:///Users/mac/coral/coast-monitoring/web/public/reef-check/index.html':'https://example.invalid/public/reef-check/')+(debug?'?debug=1':''),runScripts:'outside-only'})
 const w=dom.window;w.structuredClone=structuredClone;w.scrollTo=()=>{};w.HTMLElement.prototype.scrollIntoView=()=>{};const requests=[]
 w.fetch=async(path,options)=>{
  if(path==='/api/session')return {ok:false}
  if(path==='/api/public/reef-check/entry-sites')return {ok:true,json:async()=>siteFixtures}
  if(path==='/api/public/reef-check/entry-environment')return new Promise(resolve=>requests.push({input:JSON.parse(options.body),resolve}))
  throw Error('Unexpected request '+path)
 }
 w.eval(fs.readFileSync(new URL('catalog.js',base),'utf8'))
 w.eval(fs.readFileSync(new URL('app.js',base),'utf8').replace(/\nrender\(\);\ninitApp\(\);/,'\n')+'\nwindow.__entryRun=source=>eval(source);')
 const run=source=>w.__entryRun(source)
 run(`state.activityCode='RC-FIXTURE';state.activity={name:'測試活動',site_ids:[1,2,3],start_date:'2026-11-06',end_date:'2026-11-08'};state.methods=['line'];state.meta.date='2026-11-06';step=1;render();`)
 const input=(selector,value,event='input')=>{const el=w.document.querySelector(selector);assert.ok(el,selector);el.value=value;el.dispatchEvent(new w.Event(event,{bubbles:true}));return el}
 const selectSite=value=>{input('[data-meta="site"]',value);input('[data-meta="site"]',value,'change')}
 return {dom,w,requests,run,input,selectSite}
}
const flush=()=>new Promise(resolve=>setTimeout(resolve,0))

test('24-hour numeric fields preserve focus and produce canonical 15:40 without AM/PM',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong')
  assert.equal(f.w.document.querySelector('input[type="time"]'),null)
  const hour=f.w.document.querySelector('[data-time-part="hour"]');assert.equal(hour.value,'');assert.equal(hour.placeholder,'時');assert.equal(f.w.document.querySelector('[data-time-part="minute"]').placeholder,'分');hour.focus()
  f.input('[data-time-part="hour"]','15');assert.equal(f.w.document.activeElement,hour)
  f.input('[data-time-part="minute"]','40');f.input('[data-time-part="minute"]','40','change')
  assert.equal(f.run('state.meta.time'),'15:40');assert.equal(f.requests.length,1)
  assert.ok(f.w.document.querySelector('.time-control').textContent.includes('24 小時制'))
  f.input('[data-time-part="hour"]','24');assert.equal(f.run('state.meta.time'),'')
  f.input('[data-time-part="hour"]','0');f.input('[data-time-part="hour"]','0','change');f.input('[data-time-part="minute"]','5');f.input('[data-time-part="minute"]','5','change')
  assert.equal(f.run('state.meta.time'),'00:05')
 }finally{f.w.close()}
})

test('site coordinates autofill, switching clears missing coordinates, reload preserves manual draft',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong')
  assert.equal(f.w.document.querySelector('[data-meta="latitude"]').value,'22.35')
  assert.equal(f.w.document.querySelector('[data-meta="longitude"]').value,'120.38')
  f.input('[data-meta="latitude"]','22.351');f.input('[data-meta="longitude"]','120.381')
  await f.run('initApp()');assert.equal(f.run('state.meta.latitude'),'22.351');assert.equal(f.run('state.meta.longitude'),'120.381')
  f.run('state=readRecords().find(r=>r.id===state.id);render()')
  assert.equal(f.w.document.querySelector('[data-meta="latitude"]').value,'22.351')
  f.selectSite('OtherSite');assert.equal(f.run('state.meta.latitude'),'25.1');assert.equal(f.run('state.meta.longitude'),'121.2')
  f.selectSite('NoCoordinates');assert.equal(f.run('state.meta.latitude'),'');assert.equal(f.run('state.meta.longitude'),'')
  assert.ok(f.w.document.querySelector('#coordinate-hint').textContent.includes('沒有完整座標'))
 }finally{f.w.close()}
})

test('temperature preview shows station, Taiwan time and reference without substituting manual values',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong')
  f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40');f.input('[data-time-part="minute"]','40','change')
  assert.equal(f.requests.length,1);assert.deepEqual(f.requests[0].input,{activity_code:'RC-FIXTURE',site_id:1,survey_date:'2026-11-06',event_time:'15:40'})
  f.requests[0].resolve({ok:true,json:async()=>({site_id:1,station_id:'ST01',station_name:'測試海溫站',sea_temp_c:27.2,observed_at:'2026-11-06T07:00:00Z'})});await flush()
  const panel=f.w.document.querySelector('#environment-reference')
  assert.ok(panel.textContent.includes('27.2 °C'));assert.ok(panel.textContent.includes('測試海溫站'));assert.ok(panel.textContent.includes('15:00:00'))
  assert.equal(f.w.document.querySelector('[data-meta="temperature"]').value,'')
  assert.equal(f.run('payload().transects[0].water_temp_c'),null)
  f.input('[data-meta="temperature"]','25.5');f.selectSite('OtherSite')
  assert.equal(f.run('payload().transects[0].water_temp_c'),25.5);assert.equal(f.requests.length,2)
  const css=fs.readFileSync(new URL('styles.css',base),'utf8')
  assert.ok(css.includes('.time-parts input::placeholder'))
  assert.ok(css.includes('.time-parts input:placeholder-shown'))
 }finally{f.w.close()}
})

test('missing station and missing same-day observation show the requested backend reminder, not a fake value',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong');f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40','change')
  const panel=()=>f.w.document.querySelector('#environment-reference'),reminder='若鄰近有測站但未抓到資料，請至後台確認樣點附近測站。'
  f.requests[0].resolve({ok:true,json:async()=>({site_id:1,station_id:null,sea_temp_c:null,message:'舊版說明'})});await flush()
  assert.ok(panel().textContent.includes('尚未對應氣象署測站'));assert.ok(panel().textContent.includes(reminder));assert.ok(!panel().textContent.includes('°C'))
  const next=f.run('refreshEnvironment(true)');f.requests[1].resolve({ok:true,json:async()=>({site_id:1,station_id:'ST01',station_name:'測試海溫站',sea_temp_c:null})});await next
  assert.ok(panel().textContent.includes('調查當日沒有可用海溫'));assert.ok(panel().textContent.includes('不改用其他日期'));assert.ok(panel().textContent.includes(reminder))
  assert.equal(f.requests[1].input.survey_date,'2026-11-06');assert.equal(f.run('state.meta.temperature'),'')
  const failed=f.run('refreshEnvironment(true)');f.requests[2].resolve({ok:false});await failed
  assert.ok(panel().textContent.includes('暫時無法取得'));assert.ok(!panel().textContent.includes('尚未對應'))
  panel().querySelector('[data-action="refresh-environment"]').click();assert.equal(f.requests.length,4)
  f.requests[3].resolve({ok:true,json:async()=>({site_id:1,station_id:'ST01',sea_temp_c:0})});await flush()
  assert.ok(panel().textContent.includes('0 °C'));assert.equal(f.run('state.meta.temperature'),'')
 }finally{f.w.close()}
})

test('preview ignores late responses after site, incomplete time or draft changes',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong');f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40','change')
  f.selectSite('OtherSite');assert.equal(f.requests.length,2)
  f.requests[1].resolve({ok:true,json:async()=>({site_id:2,station_id:'ST02',station_name:'目前測站',sea_temp_c:28})});await flush()
  f.requests[0].resolve({ok:true,json:async()=>({site_id:1,station_id:'ST01',station_name:'舊測站',sea_temp_c:21})});await flush()
  assert.ok(f.w.document.querySelector('#environment-reference').textContent.includes('目前測站'));assert.ok(!f.w.document.querySelector('#environment-reference').textContent.includes('舊測站'))
  f.input('[data-time-part="minute"]','');assert.ok(!f.w.document.querySelector('#environment-reference').textContent.includes('28 °C'))
  f.input('[data-time-part="minute"]','41','change');assert.equal(f.requests.length,3)
  f.input('[data-time-part="minute"]','');f.requests[2].resolve({ok:true,json:async()=>({site_id:2,station_id:'ST02',sea_temp_c:29})});await flush()
  assert.ok(!f.w.document.querySelector('#environment-reference').textContent.includes('29 °C'))
  f.input('[data-time-part="minute"]','42','change');assert.equal(f.requests.length,4)
  f.run('state={...state,id:"another-draft",meta:{...state.meta}};render()');assert.equal(f.requests.length,5)
  f.requests[3].resolve({ok:true,json:async()=>({site_id:2,station_id:'ST02',sea_temp_c:19})});await flush()
  assert.ok(!f.w.document.querySelector('#environment-reference').textContent.includes('19 °C'))
  f.requests[4].resolve({ok:true,json:async()=>({site_id:2,station_id:'ST02',sea_temp_c:26})});await flush()
  assert.ok(f.w.document.querySelector('#environment-reference').textContent.includes('26 °C'))
 }finally{f.w.close()}
})

test('local and debug pages show an explanation but never request official reference temperatures',async()=>{
 for(const options of [{debug:true},{local:true}]){
  const f=setup(options);try{
   await f.run('initApp()');f.selectSite('MeiRenDong');f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40','change')
   await f.run('refreshEnvironment(true)');assert.equal(f.requests.length,0)
   assert.ok(f.w.document.querySelector('#environment-reference').textContent.includes('本機／測試模式不查詢正式海溫'))
  }finally{f.w.close()}
 }
})

test('next group is directly below the five input rows, and numeric 10 stays raw but submits as OT',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="general";points.slice(0,5).forEach(p=>state.line[p.key]="10");render()')
  const right=f.w.document.querySelector('.line-entry-input'),list=right.querySelector('.line-zoom-list'),nav=right.querySelector('.zoom-nav')
  assert.ok(list.compareDocumentPosition(nav)&f.w.Node.DOCUMENT_POSITION_FOLLOWING)
  assert.equal(list.querySelectorAll('input').length,5);assert.equal(list.querySelectorAll('input.invalid').length,0)
  assert.equal(f.run('linePage'),0);assert.equal(f.run('state.line["0"]'),'10');assert.equal(f.run('canonical(points[0])'),'OT')
  assert.equal(f.run('payload().substrate_points[0].substrate_code'),'OT')
  nav.querySelector('[data-action="line-page"][data-group="1"]').click();assert.equal(f.run('linePage'),1)
  f.run('linePage=0;state.line["0"]="11";render()');assert.equal(f.w.document.querySelectorAll('.line-zoom-list input.invalid').length,1)
  f.input('[data-cell="line"][data-key="0"]','10','change');assert.equal(f.w.document.querySelectorAll('.line-zoom-list input.invalid').length,0)
  assert.equal(f.run('state.line["0"]'),'10')
 }finally{f.w.close()}
})

test('entry code mappings do not guess unknown codes or confuse empty, NA and OT',()=>{
 const f=setup();try{
  f.run('state.coralMorphology="classified"')
  const base=['HC','SC','RKC','NIA','SP','RC','RB','SD','SI','OT']
  base.forEach((code,i)=>{assert.equal(f.run(`normalizeCode("${i+1}")`),code);assert.equal(f.run(`validCode("${i+1}")`),true)})
  base.slice(0,8).forEach((code,i)=>assert.equal(f.run(`normalizeCode("${91+i}")`),`SI(${code})`))
  assert.equal(f.run('normalizeCode("0")'),'OT');assert.equal(f.run('normalizeCode("-")'),'NA')
  assert.equal(f.run('substrateIssue("").kind'),'missing');assert.equal(f.run('normalizeCode("")'),'')
  for(const raw of ['NI','RD','SL','HCA','101','11','SI(RD)'])assert.equal(f.run(`substrateIssue(${JSON.stringify(raw)}).kind`),'unknown')
  for(const raw of ['HC-a','HC-b','HC-c','a','b','c'])assert.equal(f.run(`validCode(${JSON.stringify(raw)})`),true)
  for(const raw of ['SI(OT)','SI(NA)','SI(HC-a)'])assert.equal(f.run(`substrateIssue(${JSON.stringify(raw)}).kind`),'combination')
 }finally{f.w.close()}
})

test('known morphology codes show an actionable mode warning across entry, review and navigation',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="general";points.forEach(p=>state.line[p.key]="6");render()')
  const selector='[data-cell="line"][data-key="0"]',el=f.w.document.querySelector(selector);el.focus()
  f.input(selector,'a')
  assert.equal(f.w.document.activeElement,el);assert.equal(el.value,'a');assert.equal(f.run('state.line["0"]'),'a')
  assert.equal(el.classList.contains('code-warning'),true);assert.equal(el.classList.contains('invalid'),false);assert.equal(el.getAttribute('aria-invalid'),'true')
  const feedback=f.w.document.getElementById(el.getAttribute('aria-describedby'))
  assert.equal(feedback.hidden,false);assert.ok(feedback.textContent.includes('模式不符'));assert.ok(feedback.textContent.includes('有效'))
  f.w.document.querySelector('[data-action="line-page"][data-group="1"]').click()
  assert.equal(f.run('linePage'),0);assert.ok(f.w.document.querySelector('#toast').textContent.includes('有效的硬珊瑚型態碼'))
  assert.ok(f.run('substrateIssues()[0].text').includes('模式不符'))
  const table=new JSDOM(f.run('substrateHandboard()'));try{const cell=table.window.document.querySelector('[data-review-key="0"]');assert.ok(cell.textContent.includes('模式不符'));assert.equal(cell.closest('td').classList.contains('raw-warning'),true)}finally{table.window.close()}
  f.run('state.coralMorphology="classified";render()')
  assert.equal(f.w.document.querySelector(selector).value,'a');assert.equal(f.w.document.querySelector(selector).getAttribute('aria-invalid'),null)
  assert.equal(f.run('payload().substrate_points[0].substrate_code'),'HC-a')
  f.input(selector,'RD');assert.equal(f.w.document.querySelector(selector).classList.contains('invalid'),true);assert.equal(f.w.document.querySelector(selector).classList.contains('code-warning'),false)
  assert.ok(f.w.document.querySelector('#substrate-feedback-line-0').textContent.includes('未知代碼'))
  f.input(selector,'10');assert.equal(f.w.document.querySelector(selector).getAttribute('aria-invalid'),'false');assert.equal(f.w.document.querySelector('#substrate-feedback-line-0').hidden,true)
  f.run('C.sites=[["小琉球","美人洞","MeiRenDong",1]];state.line["0"]="a";state.coralMorphology="general";Object.assign(state.meta,{site:"MeiRenDong",time:"09:30",depth:"5"});state.recorders.line="測試志工";step=3;render()')
  assert.equal(f.run('metaErrors().length'),0)
  const issueIndex=f.run('issues().findIndex(x=>x.kind==="mode")')
  assert.ok(issueIndex>=0);f.w.document.querySelector(`[data-action="issue"][data-index="${issueIndex}"]`).click()
  assert.equal(f.run('step'),2);assert.equal(f.run('linePage'),0);assert.equal(f.w.document.activeElement.dataset.key,'0')
 }finally{f.w.close()}
})

test('mud input distinguishes final composites from lower-layer values and has a scoped legend',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="general";state.mud=true;layer="down";points.slice(0,5).forEach(p=>state.line[p.key]="9");render()')
  const selector='[data-cell="down"][data-key="0"]'
  f.input(selector,'94')
  assert.equal(f.run('substrateIssue(state.down["0"],true).kind'),'layer')
  assert.ok(f.w.document.querySelector('#substrate-feedback-down-0').textContent.includes('有效的上下層合成代碼'))
  assert.equal(f.w.document.querySelector(selector).classList.contains('invalid'),false)
  const legend=f.w.document.querySelector('.code-help .line-legend').textContent
  assert.ok(legend.includes('1 HC'));assert.ok(legend.includes('9 SI'));assert.ok(!legend.includes('OT'));assert.ok(!legend.includes('91–98'))
  f.input(selector,'10');assert.equal(f.run('substrateIssue(state.down["0"],true).kind'),'combination')
  assert.ok(f.w.document.querySelector('#substrate-feedback-down-0').textContent.includes('SI(OT)'))
  f.input(selector,'4');assert.equal(f.run('canonical(points[0])'),'SI(NIA)');assert.equal(f.run('state.down["0"]'),'4')
  const audit=f.run('payload().mud_audit[0]');assert.equal(audit.surface,'SI');assert.equal(audit.down,'NIA');assert.equal(audit.canonical,'SI(NIA)')
  f.input(selector,'SI （ SI ）','change');assert.equal(f.run('canonical(points[0])'),'SI');assert.equal(f.run('state.down["0"]'),'SI （ SI ）');assert.equal(f.run('payload().mud_audit[0].down'),'SI')
  f.input(selector,'NA');assert.equal(f.run('canonical(points[0])'),'NA');assert.equal(f.run('unknownCount()'),1)
  f.run('layer="surface";render()');assert.ok(f.w.document.querySelector('.line-legend').textContent.includes('10／0 OT'));assert.ok(f.w.document.querySelector('.line-legend').textContent.includes('91–98'))
 }finally{f.w.close()}
})

test('known spacing and parentheses are normalized for payloads but original input remains untouched',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="classified";render()')
  const selector='[data-cell="line"][data-key="0"]'
  for(const [raw,code] of [['si （ hc ）','SI(HC)'],['SI (SC)','SI(SC)'],['HC - a','HC-a']]){
   f.input(selector,raw,'change')
   assert.equal(f.w.document.querySelector(selector).value,raw);assert.equal(f.run('state.line["0"]'),raw)
   assert.equal(f.run('payload().substrate_points[0].substrate_code'),code)
   assert.equal(f.w.document.querySelector(selector).classList.contains('invalid'),false)
  }
  f.input(selector,'SI （ SI ）','change');assert.equal(f.run('state.line["0"]'),'SI （ SI ）');assert.equal(f.run('state.down["0"]'),'SI');assert.equal(f.run('canonical(points[0])'),'SI')
 }finally{f.w.close()}
})

test('empty fields stay empty and block normal navigation with a missing-value message',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="general";render()')
  f.w.document.querySelector('[data-action="line-page"][data-group="1"]').click()
  assert.equal(f.run('linePage'),0);assert.equal(f.run('state.line["0"]??""'),'')
  const el=f.w.document.querySelector('[data-cell="line"][data-key="0"]')
  assert.equal(el.getAttribute('aria-invalid'),'true');assert.ok(f.w.document.querySelector('#substrate-feedback-line-0').textContent.includes('尚未填寫'))
  assert.ok(!f.w.document.querySelector('#toast').textContent.includes('未知代碼'))
 }finally{f.w.close()}
})

test('bleaching uses health_status and leaves points unknown until an explicit completion confirmation',()=>{
 const f=setup();try{
  f.run('step=2;state.coralMorphology="classified";points.forEach(p=>state.line[p.key]="6");Object.assign(state.line,{"0":"1","0.5":"2","1":"a","1.5":"91"});state.linePhase="number-review";render()')
  f.w.document.querySelector('[data-action="confirm-line-numbers"]').click()
  assert.equal(f.run('state.linePhase'),'bleaching');assert.equal(f.run('Object.keys(state.healthStatus).length'),0)
  const selector='[data-action="toggle-bleach-point"][data-key="0"]'
  assert.equal(f.w.document.querySelector(selector).textContent.trim(),'點選標記白化')
  assert.ok(f.w.document.querySelector(selector).getAttribute('aria-label').includes('尚未確認'))
  assert.equal(f.w.document.querySelector('[data-action="toggle-bleach-point"][data-key="1.5"]'),null)
  assert.equal(f.run('payload().substrate_points[0].health_status'),null)
  f.w.document.querySelector(selector).click();assert.equal(f.run('state.healthStatus["0"]'),'bleached')
  assert.equal(f.w.document.querySelector(selector).getAttribute('aria-pressed'),'true');assert.ok(f.w.document.querySelector('[data-bleaching-summary]').textContent.includes('待確認'))
  f.w.document.querySelector(selector).click();assert.equal(f.run('pointHealthStatus(points[0])'),null)
  f.w.document.querySelector(selector).click();f.w.document.querySelector('[data-action="confirm-line-bleaching"]').click()
  assert.equal(f.run('state.healthStatus["0"]'),'bleached');assert.equal(f.run('state.healthStatus["0.5"]'),'not_bleached');assert.equal(f.run('state.healthStatus["1"]'),'not_bleached')
  assert.equal(f.run('payload().substrate_points[3].health_status'),null);assert.equal(f.run('payload().substrate_points[4].health_status'),null)
  assert.equal(f.run('payload().substrate_points[0].is_bleached'),undefined)
  assert.ok(f.run('bleachingSummary()').includes('HC 白化標記數</th><td>1</td>'))
  f.run('step=3;state.ack=true;render()');f.w.document.querySelector(selector).click()
  assert.equal(f.run('state.healthStatus["0"]'),'not_bleached');assert.equal(f.run('state.ack'),false)
  f.run('step=2;state.linePhase="entry";render()');f.input('[data-cell="line"][data-key="0"]','2','change')
  assert.equal(f.run('pointHealthStatus(points[0])'),null)
 }finally{f.w.close()}
})

test('legacy local marks migrate without promoting unfinished default negatives to confirmed health status',()=>{
 const f=setup();try{
  f.run('state.linePhase="bleaching";state.bleached={"0":true,"0.5":false};state=migrateRecord(state)')
  assert.equal(f.run('state.healthStatus["0"]'),'bleached');assert.equal(f.run('state.healthStatus["0.5"]'),undefined);assert.equal(f.run('state.bleached'),undefined)
  f.run('state.linePhase="complete";state.bleached={"0.5":false};state=migrateRecord(state)')
  assert.equal(f.run('state.healthStatus["0.5"]'),'not_bleached')
 }finally{f.w.close()}
})

test('both final review screens expose raw/canonical tabs without hiding them in a disclosure',()=>{
 const f=setup();try{
  f.run('state.coralMorphology="classified";state.linePhase="complete";points.forEach(p=>state.line[p.key]="6");state.line["0"]="1";state.healthStatus["0"]="bleached";state.ack=true;reviewMode="raw"')
  const initial=f.run('JSON.stringify({line:state.line,health:state.healthStatus,points:payload().substrate_points})')
  for(const screen of [2,3]){
   f.run(`step=${screen};reviewMode="raw";render()`)
   const panel=f.w.document.querySelector('[data-review-line-panel]')
   assert.ok(panel);assert.equal(panel.closest('details'),null)
   assert.equal(panel.querySelector('[data-review-key="0"]>b').textContent,'1')
   panel.querySelector('[data-action="review-mode"][data-mode="canonical"]').click()
   const current=f.w.document.querySelector('[data-review-line-panel]')
   assert.equal(current.querySelector('[data-review-key="0"]>b').textContent,'HC')
   assert.equal(current.querySelector('[data-mode="canonical"]').getAttribute('aria-selected'),'true')
   assert.equal(current.querySelector('[data-key="0"][data-action="toggle-bleach-point"]').getAttribute('aria-pressed'),'true')
   current.querySelector('[data-action="review-mode"][data-mode="raw"]').click()
   assert.equal(current.querySelector('[data-review-key="0"]>b').textContent,'1')
   assert.equal(f.run('state.ack'),true)
   assert.equal(f.run('JSON.stringify({line:state.line,health:state.healthStatus,points:payload().substrate_points})'),initial)
   assert.equal(f.w.document.querySelectorAll('[data-bleaching-summary]').length,1)
  }
 }finally{f.w.close()}
})

test('coral totals remain independent of bleaching marks, include morphology and exclude mud composites',()=>{
 const f=setup();try{
  f.run('step=3;state.coralMorphology="classified";state.linePhase="complete";state.mud=true;points.forEach(p=>state.line[p.key]="6");Object.assign(state.line,{"0":"1","0.5":"2","1":"a","1.5":"2","2":"9","2.5":"10","3":"NA","25":"1","35":"2"});state.down["2"]="1";points.forEach(p=>{if(isBleachableCode(canonical(p)))state.healthStatus[p.key]="not_bleached"});state.healthStatus["0"]="bleached";state.healthStatus["1.5"]="bleached";render()')
  const values=(group,kind)=>[...f.w.document.querySelector(`[data-coral-group="${group}"][data-summary-kind="${kind}"]`).querySelectorAll('td')].map(td=>td.textContent)
  assert.deepEqual(values('HC','total'),['2','1','0','0']);assert.deepEqual(values('SC','total'),['2','1','0','0'])
  assert.deepEqual(values('HC','bleached'),['1','0','0','0']);assert.deepEqual(values('SC','bleached'),['1','0','0','0'])
  f.w.document.querySelector('[data-action="toggle-bleach-point"][data-key="0"]').click()
  assert.deepEqual(values('HC','total'),['2','1','0','0']);assert.deepEqual(values('SC','total'),['2','1','0','0'])
  assert.deepEqual(values('HC','bleached'),['0','0','0','0']);assert.deepEqual(values('SC','bleached'),['1','0','0','0'])
  assert.equal(f.run('payload().substrate_points.find(p=>p.position_m===2).substrate_code'),'SI(HC)')
  f.run('delete state.line["0"];render()')
  assert.equal(values('HC','total')[0],'待確認底質');assert.equal(values('HC','bleached')[0],'待確認底質')
 }finally{f.w.close()}
})
