import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import {JSDOM} from 'jsdom'

const base=new URL('../../web/public/reef-check/',import.meta.url)
function setup(){
 const dom=new JSDOM(fs.readFileSync(new URL('index.html',base),'utf8'),{url:'file:///fixture/index.html',runScripts:'outside-only'})
 const w=dom.window;w.structuredClone=structuredClone;w.scrollTo=()=>{};w.HTMLElement.prototype.scrollIntoView=()=>{}
 w.eval(fs.readFileSync(new URL('catalog.js',base),'utf8'))
 w.eval(fs.readFileSync(new URL('app.js',base),'utf8').replace(/\nrender\(\);\ninitApp\(\);/,'\n')+'\nwindow.auditRun=s=>eval(s);')
 w.fixture=JSON.parse(fs.readFileSync(new URL('../../internal/repository/testdata/volunteer_board_draft.json',import.meta.url),'utf8'))
 const run=s=>w.auditRun(s)
 run(`state=migrateRecord({...fresh(),...fixture});C.sites=[['Fixture','測試樣點','Fixture',1]];state.meta={...fresh().meta,site:'Fixture',date:'2026-11-16',time:'09:30',depth:'5'};state.recorders={line:'記錄者',fish:'記錄者',invert:'記錄者'};state.activity={name:'測試活動',demo:true};state.activityCode='RC-FIXTURE';step=0;render()`)
 return {w,run,doc:w.document}
}

test('selection has one title and actual choices, without numbered decoration or repeated workflow badges',()=>{
 const f=setup();try{
  assert.equal(f.doc.querySelectorAll('.page-intro h1').length,1)
  assert.equal(f.doc.querySelectorAll('.page-intro .eyebrow,.page-intro .badge,.method-symbol,.method-card>.eyebrow').length,0)
  assert.equal(f.doc.querySelectorAll('.method-card input[name="method"]').length,3)
  assert.equal(f.doc.querySelectorAll('.method-card .mini-sheet').length,3)
  assert.ok(f.doc.querySelector('[data-activity-code]').labels[0].textContent.includes('活動代碼'))
 }finally{f.w.close()}
})

test('metadata retains required controls, clear 24-hour input, optional ranges and accessible record identifier',()=>{
 const f=setup();try{
  f.run('step=1;render()')
  for(const selector of ['[data-meta="site"]','[data-meta="date"]','[data-time-part="hour"]','[data-time-part="minute"]','[data-recorder="line"]','[data-recorder="fish"]','[data-recorder="invert"]'])assert.equal(f.doc.querySelector(selector).required,true,selector)
  assert.equal(f.doc.querySelector('[data-time-part="hour"]').placeholder,'時')
  assert.equal(f.doc.querySelector('[data-time-part="minute"]').placeholder,'分')
  assert.equal(f.doc.querySelectorAll('[data-meta="visibilityMin"],[data-meta="visibilityMax"]').length,2)
  assert.ok(f.doc.querySelector('.record-identifiers:not([open]) #event-preview'))
  assert.equal(f.doc.querySelectorAll('.panel-heading .badge').length,0)
  assert.ok(f.doc.querySelector('[data-meta="site"]').labels[0].querySelector('.field-label .required'))
 }finally{f.w.close()}
})

test('five-point entry retains locator and deliberate next button, with no competing primary action',()=>{
 const f=setup();try{
  f.run(`step=2;sheet='line';state.linePhase='entry';linePage=0;render()`)
  assert.equal(f.doc.querySelectorAll('.line-zoom-list input').length,5)
  assert.equal(f.doc.querySelectorAll('.locator-cell').length,20)
  assert.ok(f.doc.querySelector('.locator-group[aria-current="location"]'))
  assert.ok(f.doc.querySelector('.zoom-nav [data-action="line-page"].primary').textContent.includes('確認這 5 格'))
  assert.equal(f.doc.querySelector('[data-action="review-line-numbers"]').classList.contains('primary'),false)
  assert.equal(f.doc.querySelectorAll('.zoom-hint,.debug-phase-hint,.subsample-progress').length,0)
  assert.equal(f.doc.querySelector('#field-focus').hidden,true)
  f.doc.querySelector('[data-cell="line"]').focus()
  assert.equal(f.doc.querySelector('#field-focus').hidden,false)
  assert.ok(f.doc.querySelector('[data-action="mark-na"]'))
  f.run('linePage=31;render()')
  assert.equal(f.doc.querySelector('[data-action="review-line-numbers"]').classList.contains('primary'),true)
 }finally{f.w.close()}
})

test('all line phases and review modes render without mutating values, mud or health answers',()=>{
 const f=setup();try{
  const snapshot=()=>f.run('(()=>{const p=payload();delete p.exported_at;return JSON.stringify(p)})()')
  for(const phase of ['entry','number-review','bleaching','complete']){
   f.run(`step=2;sheet='line';state.linePhase='${phase}';render()`)
   const before=snapshot();f.run('render()');assert.equal(snapshot(),before)
  }
  for(const mode of ['raw','canonical']){
   const before=snapshot();f.run(`reviewMode='${mode}';render()`);assert.equal(snapshot(),before)
   assert.equal(f.doc.querySelectorAll('[data-action="review-mode"]').length,2)
  }
  f.run(`state.linePhase='bleaching';render()`)
  assert.ok(f.doc.querySelector('.raw-board-scroll + .bleaching-summary'))
  assert.equal(f.doc.querySelectorAll('.bleaching-summary tr[data-summary-kind]').length,4)
  assert.ok(f.doc.querySelector('[data-action="toggle-bleach-point"]').textContent.includes('標記白化'))
 }finally{f.w.close()}
})

test('review preserves per-sheet counts and submit safeguards without duplicate statistic cards',()=>{
 const f=setup();try{
  f.run('step=3;render()')
  assert.equal(f.doc.querySelector('.review-grid'),null)
  assert.equal(f.doc.querySelectorAll('.review-row').length,5)
  assert.equal(f.doc.querySelectorAll('.review-count').length,5)
  assert.ok(f.doc.querySelector('#acknowledge'))
  assert.equal(f.doc.querySelector('[data-action="zero"]').nextElementSibling.dataset.action,'complete')
  assert.equal(f.doc.querySelector('.event-preview'),null)
  f.run(`points.forEach((p,i)=>state.line[p.key]=i<16?'3':'6');state.rkcNote='';render()`)
  assert.ok(f.doc.querySelector('[data-state="rkcNote"]'))
  assert.ok(f.doc.querySelector('[data-state="rkcNote"]').labels[0].textContent.includes('必填'))
  assert.ok(f.run('issues().some(i=>i.field==="rkcNote")'))
 }finally{f.w.close()}
})

test('saved, local, loading and failed results tell one accurate status and retain recovery actions',()=>{
 const f=setup();try{
  f.run(`step=4;state.dbSave={receipt_id:'RC-FIXTURE',saved_at:'2026-11-16T01:30:00Z'};render()`)
  assert.equal(f.doc.querySelector('h1').textContent,'已送出，待窗口審核')
  assert.equal((f.doc.querySelector('#main').textContent.match(/RC-FIXTURE/g)||[]).length,1)
  assert.equal(f.doc.querySelector('.success-panel h2'),null)
  assert.ok(f.doc.querySelector('[data-action="new-same"]'))
  f.run('state.dbSave=null;state.simulatedSave=null;render()')
  assert.equal(f.doc.querySelector('h1').textContent,'本機紀錄')
  assert.ok(!f.doc.querySelector('#main').textContent.includes('已存入資料庫'))
  f.run('saving=true;render()');assert.ok(f.doc.querySelector('[aria-busy="true"]'));assert.equal(f.doc.querySelectorAll('[data-action="complete"]').length,0)
  f.run(`saving=false;submitError='連線失敗';submitFailureKind='network';render()`)
  assert.ok(f.doc.querySelector('[role="alert"]'));assert.ok(f.doc.querySelector('[data-action="export-draft"]'));assert.ok(f.doc.querySelector('[data-action="complete"]'))
 }finally{f.w.close()}
})
