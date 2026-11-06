import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {JSDOM} from 'jsdom'

const base=new URL('../../web/public/reef-check/',import.meta.url)
function setup(debug=false){
 const dom=new JSDOM(fs.readFileSync(new URL('index.html',base),'utf8'),{url:'https://example.invalid/public/reef-check/'+(debug?'?debug=1':''),runScripts:'outside-only'})
 const w=dom.window;w.structuredClone=structuredClone;w.scrollTo=()=>{};let requests=0
 w.fetch=()=>{requests++;throw Error('Local deletion must never contact the backend')}
 w.HTMLDialogElement.prototype.showModal=function(){this.setAttribute('open','')}
 w.HTMLDialogElement.prototype.close=function(){this.removeAttribute('open')}
 w.eval(fs.readFileSync(new URL('catalog.js',base),'utf8'))
 w.eval(fs.readFileSync(new URL('app.js',base),'utf8').replace(/\nrender\(\);\ninitApp\(\);/,'\n')+'\nwindow.__localRun=source=>eval(source);')
 const run=source=>w.__localRun(source)
 run(`C.sites=[['小琉球','美人洞','MeiRenDong',1]];records=[Object.assign(fresh({site:'MeiRenDong',date:'2026-11-06',time:'15:40',depth:'5'}),{id:'draft-a',methods:['line'],line:{'0':'1'},step:1}),Object.assign(fresh({site:'MeiRenDong',date:'2026-11-05',time:'09:30',depth:'10'}),{id:'saved-b',methods:['line'],status:'saved',uploaded:true,dbSave:{receipt_id:'RC-FIXTURE'}})];localStorage.setItem(KEY,JSON.stringify(records));state=structuredClone(records[0]);step=0;render();openDrafts();`)
 const click=(action,id)=>{const selector=`[data-action="${action}"]${id?`[data-id="${id}"]`:''}`;const el=w.document.querySelector(selector);assert.ok(el,selector);el.click()}
 const stored=()=>JSON.parse(w.localStorage.getItem(run('KEY')))
 return {w,run,click,stored,requests:()=>requests}
}

test('local records show delete controls; cancel leaves all records unchanged',()=>{
 const f=setup();try{
  assert.equal(f.w.document.querySelectorAll('[data-action="delete-local-record"]').length,2)
  f.click('delete-local-record','draft-a')
  assert.ok(f.w.document.querySelector('#modal-content').textContent.includes('刪除後無法復原'))
  assert.ok(f.w.document.querySelector('#modal-content').textContent.includes('2026-11-06'))
  assert.equal(f.stored().length,2)
  f.click('drafts');assert.equal(f.stored().length,2);assert.equal(f.run('state.id'),'draft-a')
  assert.equal(f.requests(),0)
 }finally{f.w.close()}
})

test('confirmed deletion resets an active draft, survives reload and preserves other records',()=>{
 const f=setup();try{
  const original=f.stored().find(r=>r.id==='saved-b')
  f.click('delete-local-record','draft-a');f.click('confirm-delete-local-record','draft-a')
  assert.deepEqual(f.stored(),[original]);assert.notEqual(f.run('state.id'),'draft-a')
  assert.equal(f.w.document.querySelector('#draft-count').textContent,'1')
  f.run('records=readRecords();render();openDrafts()')
  assert.equal(f.w.document.querySelector('[data-action="delete-local-record"][data-id="draft-a"]'),null)
  f.run('go(0)');assert.ok(f.stored().every(r=>r.id!=='draft-a'))
  assert.equal(f.requests(),0)
 }finally{f.w.close()}
})

test('submitted copies remain deletable with explicit database warning; deleting all stays empty',()=>{
 const f=setup();try{
  f.click('delete-local-record','saved-b')
  assert.ok(f.w.document.querySelector('#modal-content').textContent.includes('不會刪除資料庫中的觀測資料'))
  f.click('confirm-delete-local-record','saved-b');f.click('delete-local-record','draft-a');f.click('confirm-delete-local-record','draft-a')
  assert.deepEqual(f.stored(),[]);assert.equal(f.w.document.querySelector('#draft-count').textContent,'0')
  assert.ok(f.w.document.querySelector('#modal-content').textContent.includes('還沒有儲存的紀錄'))
  assert.equal(f.requests(),0)
 }finally{f.w.close()}
})

test('deletion preserves records newly saved by another tab and keeps test storage separate',()=>{
 const f=setup(true);try{
  const realKey='teia-reefcheck-records-v2',sentinel='[{"id":"real-data"}]';f.w.localStorage.setItem(realKey,sentinel)
  f.click('delete-local-record','draft-a')
  f.run(`const latest=JSON.parse(localStorage.getItem(KEY));latest.push(Object.assign(fresh(),{id:'another-tab'}));localStorage.setItem(KEY,JSON.stringify(latest))`)
  f.click('confirm-delete-local-record','draft-a')
  assert.deepEqual(f.stored().map(r=>r.id),['saved-b','another-tab'])
  assert.equal(f.w.localStorage.getItem(realKey),sentinel);assert.equal(f.requests(),0)
 }finally{f.w.close()}
})

test('storage failure or malformed storage reports failure without deleting the in-memory record',()=>{
 const f=setup();try{
  f.click('delete-local-record','draft-a');const original=f.w.localStorage.getItem(f.run('KEY'))
  const set=f.w.Storage.prototype.setItem;f.w.Storage.prototype.setItem=()=>{throw new Error('storage blocked')}
  f.click('confirm-delete-local-record','draft-a')
  assert.equal(f.w.localStorage.getItem(f.run('KEY')),original);assert.equal(f.run('records.length'),2);assert.equal(f.run('state.id'),'draft-a')
  assert.ok(f.w.document.querySelector('#delete-local-error').textContent.includes('刪除失敗'))
  f.w.Storage.prototype.setItem=set;f.w.localStorage.setItem(f.run('KEY'),'{}')
  f.click('confirm-delete-local-record','draft-a');assert.equal(f.w.localStorage.getItem(f.run('KEY')),'{}');assert.equal(f.run('records.length'),2)
 }finally{f.w.close()}
})

test('another tab deleting the active record prevents later auto-save from resurrecting it',()=>{
 for(const saving of [false,true]){
  const f=setup();try{
   f.run(`saving=${saving};localStorage.setItem(KEY,JSON.stringify(records.filter(r=>r.id!=='draft-a')))`)
   f.w.dispatchEvent(new f.w.StorageEvent('storage',{key:f.run('KEY')}))
   f.run('persist(false)');assert.ok(f.stored().every(r=>r.id!=='draft-a'))
   assert.equal(f.requests(),0)
  }finally{f.w.close()}
 }
})
