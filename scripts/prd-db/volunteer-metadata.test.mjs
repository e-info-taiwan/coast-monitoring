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
function setup(){
 const dom=new JSDOM(fs.readFileSync(new URL('index.html',base),'utf8'),{url:'https://example.invalid/public/reef-check/',runScripts:'outside-only'})
 const w=dom.window;w.structuredClone=structuredClone;w.scrollTo=()=>{};const requests=[]
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
  const hour=f.w.document.querySelector('[data-time-part="hour"]');hour.focus()
  f.input('[data-time-part="hour"]','15');assert.equal(f.w.document.activeElement,hour)
  f.input('[data-time-part="minute"]','40');f.input('[data-time-part="minute"]','40','change')
  assert.equal(f.run('state.meta.time'),'15:40');assert.equal(f.requests[0].input.event_time,'15:40')
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

test('CWA reference never overwrites manual temperature and ignores late responses from another site',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong');f.input('[data-meta="temperature"]','25.5')
  f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40');f.input('[data-time-part="minute"]','40','change')
  f.selectSite('OtherSite');assert.equal(f.requests.length,2)
  f.requests[1].resolve({ok:true,json:async()=>({site_id:2,station_name:'新測站',sea_temp_c:27,observed_at:'2026-11-06T07:30:00Z'})});await flush()
  f.requests[0].resolve({ok:true,json:async()=>({site_id:1,station_name:'舊測站',sea_temp_c:24,observed_at:'2026-11-06T01:00:00Z'})});await flush()
  const text=f.w.document.querySelector('#environment-reference').textContent
  assert.ok(text.includes('新測站'));assert.ok(!text.includes('舊測站'));assert.ok(text.includes('15:30'))
  assert.equal(f.run('state.meta.temperature'),'25.5');assert.equal(f.run('payload().transects[0].water_temp_c'),25.5)
  f.input('[data-meta="date"]','2026-11-07');f.input('[data-meta="date"]','2026-11-07','change')
  f.requests[2].resolve({ok:false});await flush()
  assert.ok(f.w.document.querySelector('#environment-reference').textContent.includes('暫時無法取得'))
  assert.equal(f.run('state.meta.temperature'),'25.5')
 }finally{f.w.close()}
})

test('switching away and resuming a draft does not leave its reference stuck loading',async()=>{
 const f=setup();try{
  await f.run('initApp()');f.selectSite('MeiRenDong')
  f.input('[data-time-part="hour"]','15');f.input('[data-time-part="minute"]','40');f.input('[data-time-part="minute"]','40','change')
  f.run('globalThis.savedEntry=structuredClone(state);state=fresh();step=0;render()')
  f.requests[0].resolve({ok:true,json:async()=>({station_name:'舊請求',sea_temp_c:24})});await flush()
  f.run('state=globalThis.savedEntry;step=1;render()');assert.equal(f.requests.length,2)
  f.requests[1].resolve({ok:true,json:async()=>({station_name:'重新查詢',sea_temp_c:27})});await flush()
  assert.ok(f.w.document.querySelector('#environment-reference').textContent.includes('重新查詢'))
 }finally{f.w.close()}
})
