import test from 'node:test'
import assert from 'node:assert/strict'
import { statistics, impactValue, substrateSummary, bleachingPercent } from '../reef-data-math.mjs'

test('missing segments are not zero, sample SD uses observed segments', () => {
 assert.deepEqual(statistics([null, undefined]), {n:0,total:null,mean:null,sd:null})
 assert.deepEqual(statistics([0, null]), {n:1,total:0,mean:0,sd:null})
 assert.equal(statistics([1, 3, null, null]).mean,2)
 assert.equal(statistics([1, 3, null, null]).sd,Math.sqrt(2))
 assert.equal(statistics([10,14,17,16]).total,57)
})
test('v1.7 grades cap raw counts without rewriting percent values', () => {
 assert.equal(impactValue({raw_value:4,has_raw_count:true}),3)
 assert.equal(impactValue({raw_value:2,has_raw_count:true}),2)
 assert.equal(impactValue({raw_value:12.5,has_raw_count:false}),12.5)
})
test('NA and absent points excluded, OT and composites preserved, layers separate', () => {
 const points=[{segment:1,substrate_code:'HC',substrate_layer:'surface'}, {segment:1,substrate_code:'NA',substrate_layer:'surface'}, {segment:2,substrate_code:'OT',substrate_layer:'surface'}, {segment:2,substrate_code:'SI(HC)',substrate_layer:'surface'}, {segment:1,substrate_code:'HC',substrate_layer:'down'}]
 const result=substrateSummary(points)
 assert.equal(result.recorded,4);assert.equal(result.valid,3);assert.equal(result.unknown,1)
 assert.ok(Math.abs(result.rows.find(r=>r.code==='HC').cover - 100/3) < 1e-10)
 assert.deepEqual(result.rows.find(r=>r.code==='HC').counts,[1,0,null,null])
 assert.ok(result.rows.some(r=>r.code==='OT'));assert.ok(result.rows.some(r=>r.code==='SI(HC)'))
 assert.equal(bleachingPercent(points,1,'HC',1),100)
 assert.equal(bleachingPercent(points,1,'SC',0),null)
 assert.equal(substrateSummary([{segment:1,substrate_code:'NA',substrate_layer:'surface'}]).valid,0)
})
