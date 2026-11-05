#!/usr/bin/env python3
"""Read-only end-to-end public API benchmark; includes response download and parsing."""
import argparse,csv,io,json,time,urllib.request,urllib.error

def main():
 p=argparse.ArgumentParser();p.add_argument('--base',required=True);p.add_argument('--runs',type=int,default=2);p.add_argument('--only-full',action='store_true');a=p.parse_args()
 paths=[('hechei_full','/sites/102/series'),('hechei_2024','/sites/102/series?from_year=2024&to_year=2024'),('compare','/compare?site_ids=102,151'),('csv','/sites/102/export.csv'),('compare_csv','/compare.csv?site_ids=102,151')]
 if a.only_full:paths=paths[:1]
 for run in range(1,a.runs+1):
  for case,path in paths:
   result={'case':case,'run':run,'path':path};start=time.perf_counter()
   try:
    with urllib.request.urlopen(a.base.rstrip('/')+'/api/public/reef-check'+path,timeout=30) as response:
     raw=response.read();result.update(status=response.status,bytes=len(raw),cache_control=response.headers.get('Cache-Control'))
    if 'csv' in case:
     table=list(csv.DictReader(io.StringIO(raw.decode('utf-8-sig'))));result.update(rows=len(table),columns=list(table[0]) if table else [])
    else:
     data=json.loads(raw);series=data['series'] if 'series' in data else [r for s in data['sites'] for r in s['series']]
     result.update(rows=len(series),events=len({r['event_id'] for r in series}),years=sorted({r['survey_date'][:4] for r in series}),sites=sorted({r['site_id'] for r in series}))
   except Exception as e:result.update(error=str(e),status=getattr(e,'code',None))
   result['seconds']=round(time.perf_counter()-start,3);print(json.dumps(result,ensure_ascii=False),flush=True)
if __name__=='__main__':main()
