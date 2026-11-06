import {test} from 'node:test'
import assert from 'node:assert/strict'
import {JSDOM} from 'jsdom'
import {createPublicContent} from '../../web/admin/public-content.js'
import {createReefData} from '../../web/admin/reef-data.js'

test('newly generated activity code remains visible above form after list refresh and later saves', async () => {
 const dom=new JSDOM('<main></main><aside></aside>',{url:'https://example.invalid/admin/'})
 const old={window:globalThis.window,document:globalThis.document,FormData:globalThis.FormData}
 Object.assign(globalThis,{window:dom.window,document:dom.window.document,FormData:dom.window.FormData})
 const panel=document.querySelector('main'),body=document.querySelector('aside');let saved=null
 const app=createPublicContent({notify(){},drawer:{open(o){body.innerHTML=o.html;o.bind(body)}},apiFetch:async(path,opts)=>{
  if(path==='/admin/sites')return [{id:1,name_zh:'合界'}]
  if(opts?.method){saved={id:77,entry_config_id:9,publication_status:'draft',updated_at:'v1',data:opts.body.data};return {...saved,...(opts.method==='POST'?{entry_code:'TEST-CODE-77'}:{})}}
  return saved?[saved]:[]
 }})
 try {
  app.render(panel,'activity');await new Promise(resolve=>setTimeout(resolve,0))
  panel.querySelector('[data-pc="new"]').click();await new Promise(resolve=>setTimeout(resolve,0))
  let form=body.querySelector('form');form.elements.title.value='測試活動';form.elements.entry_enabled.value='true';form.querySelector('[name=site_ids]').checked=true
  form.elements.start_at.value='2026-11-06T09:00';form.elements.end_at.value='2026-11-06T12:00'
  await form.onsubmit({preventDefault(){}});await new Promise(resolve=>setTimeout(resolve,0))
  assert.equal(body.querySelector('.pc-entry-code output').textContent,'TEST-CODE-77')
  assert.ok(body.querySelector('.pc-entry-code').compareDocumentPosition(body.querySelector('form')) & dom.window.Node.DOCUMENT_POSITION_FOLLOWING)
  form=body.querySelector('form');await form.onsubmit({preventDefault(){}});await new Promise(resolve=>setTimeout(resolve,0))
  assert.equal(body.querySelector('.pc-entry-code output').textContent,'TEST-CODE-77')
  form=body.querySelector('form');form.elements.entry_enabled.value='false';await form.onsubmit({preventDefault(){}});await new Promise(resolve=>setTimeout(resolve,0))
  assert.equal(body.querySelector('.pc-entry-code output'),null)
 }finally{app.reset();Object.assign(globalThis,old);dom.window.close()}
})
test('observation activity button triggers actual application navigation',async()=>{
 const dom=new JSDOM('<main></main>');let calls=0
 const previous=globalThis.window;globalThis.window=dom.window
 try {
 const app=createReefData({apiFetch:async()=>[],onCount(){},onManageActivities(){calls++}})
 await app.load();app.render(dom.window.document.querySelector('main'))
 const button=dom.window.document.querySelector('#rd-manage-activities')
 button.dispatchEvent(new dom.window.MouseEvent('click',{bubbles:true,cancelable:true}))
 assert.equal(calls,1)
 } finally {globalThis.window=previous;dom.window.close()}
})
