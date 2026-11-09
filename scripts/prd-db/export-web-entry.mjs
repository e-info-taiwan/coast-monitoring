// Test-only: run the actual browser payload builder against an isolated config.
// Read a fixture over stdin; never contact a server or persist browser records.
import fs from 'node:fs'
import {JSDOM} from 'jsdom'
let input=''
for await(const chunk of process.stdin)input+=chunk
const fixture=JSON.parse(input),base=new URL('../../web/public/reef-check/',import.meta.url)
const dom=new JSDOM(fs.readFileSync(new URL('index.html',base),'utf8'),{url:'https://example.invalid/public/reef-check/',runScripts:'outside-only'})
const w=dom.window
w.structuredClone=structuredClone;w.scrollTo=()=>{}
w.fetch=()=>{throw Error('Web entry fixture must not access the network')}
try{
 w.eval(fs.readFileSync(new URL('catalog.js',base),'utf8'))
 w.eval(fs.readFileSync(new URL('app.js',base),'utf8').replace(/\nrender\(\);\ninitApp\(\);/,'\n')+'\nwindow.fixtureRun=source=>eval(source);')
 w.fixture=fixture
 const result=w.fixtureRun(`
  applyEntryBoard(fixture.config.entry_board);
  C.sites=[['Fixture',fixture.site.name_zh,fixture.site.name_en,fixture.site.id]];
  state=migrateRecord({...fresh(),...fixture.draft_state});
  state.activityCode=fixture.activity.code;
  state.activity={name:'Fixture',site_ids:[fixture.site.id],start_date:'2026-11-01',end_date:'2026-11-30'};
  state.meta={...fresh().meta,site:fixture.site.name_en,date:'2026-11-02',time:fixture.event_time||'09:30',depth:'5'};
  state.recorders={line:'Fixture recorder',fish:'Fixture recorder',invert:'Fixture recorder'};
  step=3;
  JSON.stringify({issues:issues().map(x=>x.text),payload:payload(false)});
 `)
 process.stdout.write(result)
}finally{w.close()}
