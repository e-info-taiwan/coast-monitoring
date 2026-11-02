import test from 'node:test'
import assert from 'node:assert/strict'
import {readFile} from 'node:fs/promises'
const src=await readFile(new URL('../reef-publication-review.js',import.meta.url),'utf8')
const {renderPublicationReview:render}=await import(`data:text/javascript;base64,${Buffer.from(src).toString('base64')}`)
test('review preserves positions, zero, NA, missing cells and escaped names',()=>{
 const html=render({event:{site_name:'<script>'},transects:[{method:'line',points:[{segment:4,position_m:94.5,substrate_layer:'surface',substrate_code:'HC',is_bleached:true}],belt:[{taxon_id:1,name_zh:'魚',segment:1,count:0},{taxon_id:1,name_zh:'魚',segment:2,count:0,record_status:'not_recorded'}]}]})
 assert.match(html,/&lt;script&gt;/)
 assert.doesNotMatch(html,/<script>/)
 assert.match(html,/>94.5<\/th><td>1<small>HC · 白化/)
 assert.match(html,/<td>0<\/td><td>NA（未記錄）<\/td><td><span class="pr-missing">未填/)
 assert.match(html,/子樣區 4/)
})

test('point-derived bleaching uses API fields and keeps unknown distinct from zero',()=>{
 const html=render({transects:[{method:'line',derived_bleaching:[{segment:1,derived_hc_bleached_point_count:0,derived_sc_bleached_point_count:null}]}]})
 assert.match(html,/逐格白化統計/)
 assert.match(html,/<th>1<\/th><td>0<\/td><td><span class="pr-missing">未填/)
})
