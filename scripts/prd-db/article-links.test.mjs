import {test} from 'node:test'
import assert from 'node:assert/strict'
import {JSDOM} from 'jsdom'
import {createPublicContent} from '../../web/admin/public-content.js'

test('article management lists reports with linked site names and category filters', async () => {
 const dom=new JSDOM('<main></main>');const panel=dom.window.document.querySelector('main')
 const app=createPublicContent({apiFetch:async path=>path==='/admin/sites'?[{id:1,name_zh:'合界'}]:[
  {id:1,publication_status:'published',related_site_ids:[1],data:{title:'調查報告',category:'observation_report'}},
  {id:2,publication_status:'draft',data:{title:'新消息',category:'news'}}],drawer:{open(){}},notify(){}})
 app.render(panel,'article');await new Promise(resolve=>setTimeout(resolve,0))
 assert.match(panel.textContent,/關聯樣點：合界/)
 panel.querySelector('[data-category="observation_report"]').click()
 assert.match(panel.querySelector('tbody').textContent,/調查報告/)
 assert.doesNotMatch(panel.querySelector('tbody').textContent,/新消息/)
 app.reset();dom.window.close()
})
