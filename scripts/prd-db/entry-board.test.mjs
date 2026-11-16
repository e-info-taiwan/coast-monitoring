import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import vm from 'node:vm'
import {PGlite} from '@electric-sql/pglite'
import {pgcrypto} from '@electric-sql/pglite/contrib/pgcrypto'
import {citext} from '@electric-sql/pglite/contrib/citext'

test('template row keys match existing browser drafts even if masters were reordered before upgrade',async()=>{
 const db=new PGlite({extensions:{pgcrypto,citext}});await db.waitReady
 const dir=new URL('../../migrations/',import.meta.url),sandbox={window:{}}
 vm.runInNewContext(await fs.readFile(new URL('../../web/public/reef-check/catalog.js',import.meta.url),'utf8'),sandbox)
 const catalog=sandbox.window.REEF
 let before
 try{
  for(const file of (await fs.readdir(dir)).filter(f=>f.endsWith('.sql')).sort()){
   if(file.startsWith('000023')){
    await db.exec('UPDATE taxon SET sort_order=1000-sort_order; UPDATE impact_type SET sort_order=1000-sort_order')
    before={taxa:(await db.query('SELECT * FROM taxon ORDER BY id')).rows,impacts:(await db.query('SELECT * FROM impact_type ORDER BY id')).rows}
   }
   await db.exec('BEGIN;'+await fs.readFile(new URL(file,dir),'utf8')+';COMMIT;')
  }
  const taxa=(await db.query('SELECT x.taxon_group,x.name_en,COALESCE(x.size_class,\'\') AS size_class,r.row_id FROM reef_entry_required_taxon r JOIN taxon x ON x.id=r.taxon_id')).rows
  assert.equal(taxa.length,31)
  for(const row of taxa){const expected=catalog[row.taxon_group].findIndex(r=>r[0]===row.name_en&&(r[2]||'')===row.size_class);assert.notEqual(expected,-1);assert.equal(row.row_id,String(expected))}
  const impacts=(await db.query('SELECT x.impact_group,x.name_en,r.row_id FROM reef_entry_required_impact r JOIN impact_type x ON x.id=r.impact_type_id')).rows
  assert.equal(impacts.length,9)
  for(const row of impacts)assert.equal(row.row_id,String(catalog.impact.findIndex(r=>r[0]===row.name_en&&r[3]===row.impact_group)))
  assert.deepEqual((await db.query('SELECT * FROM taxon ORDER BY id')).rows,before.taxa)
  assert.deepEqual((await db.query('SELECT * FROM impact_type ORDER BY id')).rows,before.impacts)
 }finally{await db.close()}
})
