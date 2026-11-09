import {test} from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import {PGlite} from '@electric-sql/pglite'
import {pgcrypto} from '@electric-sql/pglite/contrib/pgcrypto'
import {citext} from '@electric-sql/pglite/contrib/citext'

test('health migration preserves nulls, supports old writers and rejects conflicting answers',async()=>{
 const db=new PGlite({extensions:{pgcrypto,citext}});await db.waitReady
 const dir=new URL('../../migrations/',import.meta.url)
 try{
  for(const file of (await fs.readdir(dir)).filter(f=>f.endsWith('.sql')).sort()){
   if(file.startsWith('000022'))await db.exec(`
    INSERT INTO site(id,name_zh,name_en) VALUES(99001,'Health fixture','HealthFixture');
    INSERT INTO survey(id,site_id,start_date,end_date) VALUES(99001,99001,'2026-11-09','2026-11-09');
    INSERT INTO transect(id,survey_id,method,depth_m) VALUES(99001,99001,'line',5);
    INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,substrate_layer,is_bleached) VALUES
     (99001,1,0,'HC','surface',true),(99001,1,0.5,'HC','surface',false),
     (99001,1,1,'HC','surface',NULL),(99001,1,1.5,'RC','surface',NULL);
    INSERT INTO substrate_bleaching(transect_id,segment,hc_bleached_count) VALUES(99001,1,7);
   `)
   await db.exec('BEGIN;'+await fs.readFile(new URL(file,dir),'utf8')+';COMMIT;')
  }
  const rows=async q=>(await db.query(q)).rows
  assert.deepEqual((await rows('SELECT health_status FROM substrate_point WHERE transect_id=99001 ORDER BY position_m')).map(p=>p.health_status),['bleached','not_bleached',null,null])
  assert.equal((await rows('SELECT derived_hc_bleached_point_count AS n FROM substrate_bleaching_point_summary WHERE transect_id=99001'))[0].n,null)
  await db.exec("UPDATE substrate_point SET health_status='not_bleached' WHERE transect_id=99001 AND position_m=1")
  assert.equal((await rows('SELECT derived_hc_bleached_point_count AS n FROM substrate_bleaching_point_summary WHERE transect_id=99001'))[0].n,1)
  await db.exec('UPDATE substrate_point SET is_bleached=true WHERE transect_id=99001 AND position_m=0.5')
  assert.equal((await rows('SELECT derived_hc_bleached_point_count AS n FROM substrate_bleaching_point_summary WHERE transect_id=99001'))[0].n,2)
  await db.exec('UPDATE substrate_point SET health_status=NULL WHERE transect_id=99001 AND position_m=0')
  const cleared=(await rows('SELECT health_status,is_bleached FROM substrate_point WHERE transect_id=99001 AND position_m=0'))[0]
  assert.deepEqual(cleared,{health_status:null,is_bleached:null})
  await assert.rejects(db.exec("INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,substrate_layer,health_status,is_bleached) VALUES(99001,1,2,'HC','surface','bleached',false)"),/health_consistent|check constraint/)
  await assert.rejects(db.exec("UPDATE substrate_point SET health_status='healthy' WHERE transect_id=99001 AND position_m=1"),/check constraint/)
  await db.exec("INSERT INTO substrate_point(transect_id,segment,position_m,substrate_code,substrate_layer,health_status) VALUES(99001,1,2.5,'SC','surface','bleached')")
  assert.equal((await rows('SELECT is_bleached FROM substrate_point WHERE transect_id=99001 AND position_m=2.5'))[0].is_bleached,true)
  assert.equal((await rows('SELECT hc_bleached_count FROM substrate_bleaching WHERE transect_id=99001'))[0].hc_bleached_count,7)
  await db.exec("UPDATE substrate_point SET substrate_code='RC',is_bleached=NULL WHERE transect_id=99001 AND position_m=2.5")
  assert.deepEqual((await rows('SELECT health_status,is_bleached FROM substrate_point WHERE transect_id=99001 AND position_m=2.5'))[0],{health_status:null,is_bleached:null})
 }finally{await db.close()}
})
