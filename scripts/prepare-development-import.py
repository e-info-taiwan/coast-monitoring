#!/usr/bin/env python3
"""Generate an additive, source-keyed draft import; never overwrite editor changes."""
import argparse
import csv
import hashlib
import json
from pathlib import Path


def prepare(directory):
    files = ['development_cases.csv', 'development_case_timeline.csv', 'development_case_timeline_source.csv',
             'development_case_media.csv', 'development_case_source.csv', 'development_case_discrepancy.csv']
    tables = {name: list(csv.DictReader((directory / name).open(encoding='utf-8-sig'))) for name in files}
    hashes = {name: hashlib.sha256((directory / name).read_bytes()).hexdigest() for name in files}
    result = []
    seen = set()
    for case in tables[files[0]]:
        key = case['source_feature_id']
        if not key or key in seen:
            raise ValueError('Missing or duplicate source feature ID')
        seen.add(key)
        related = {name: [r for r in rows if r.get('source_feature_id') == key] for name, rows in tables.items() if name != files[0]}
        differences = related['development_case_discrepancy.csv']
        summary = case['detail_summary_text'] or case['summary_text']
        for diff in differences:
            summary += f"\n待核對（{diff['field']}）：詳細頁 {diff['detail_tab_value']}；總表 {diff['master_table_value']}。"
        data = dict(title=case['name'], summary=summary, county=case['county'], authority=case['competent_authority'],
                    status=case['detail_status_raw'] or case['status_raw'], source_url=case['detail_sheet_url'] or case['source_url'],
                    geometry=json.loads(case['geometry_geojson']), sort_order=int(case['source_order']), timeline=[], media=[])
        if data['geometry']['type'] not in ['Point', 'Polygon', 'MultiPolygon']:
            raise ValueError('Unsupported geometry')
        if case['area_ha'] and not any(x['field'] == 'area' for x in differences):
            data['area_m2'] = float(case['area_ha']) * 10000
        if case['investment_twd'] and not any(x['field'].startswith('investment') for x in differences):
            data['investment_ntd'] = float(case['investment_twd'])
        for row in related['development_case_timeline.csv']:
            precision = row['date_precision']
            date = row['event_date_start']
            if precision in ('year', 'month', 'day'):
                date = date[:{'year': 4, 'month': 7, 'day': 10}[precision]]
            else:
                precision, date = 'unknown', ''
            sources = [s for s in related['development_case_timeline_source.csv'] if s['event_key'] == row['event_key']]
            description = row['event_text'] + ('\n' + row['note_text'] if row['note_text'] else '')
            if precision == 'unknown':
                description = f"原始日期：{row['event_date_raw']}\n" + description
            description += ''.join(f"\n{s['source_label']}：{s['source_url']}" for s in sources)
            data['timeline'].append(dict(title=row['phase_label'] or row['event_text'][:80], description=description,
                date=date, date_precision=precision, source_url=sources[0]['source_url'] if sources else case['detail_sheet_url'],
                sort_order=int(row['source_order']), status=row['phase_label'], is_published=False))
        for row in related['development_case_media.csv']:
            data['media'].append(dict(url=row['source_url'], alt=row['alt_text'] or case['name']+'（來源圖片，待確認授權）',
                                     is_valid=False, license_confirmed=False, is_published=False))
        result.append(dict(data=data, snapshot=dict(source_feature_id=key, source_format='development-handoff-2026-11-04',
                          source_sha256=hashes, case=case, related=related)))
    return result


def sql_payload(records):
    payload = json.dumps(records, ensure_ascii=False, separators=(',', ':'))
    assert '$development_import$' not in payload
    return """-- Import delivered coastal development cases as reviewable drafts.
-- Preserve full source rows, ambiguous dates, discrepancies and all geometry.
-- Re-execution skips source IDs, preserving subsequent manual edits.
CREATE UNIQUE INDEX IF NOT EXISTS development_source_feature_unique
 ON public_content ((source_snapshot->>'source_feature_id'))
 WHERE kind='development' AND source_snapshot ? 'source_feature_id';
DO $import$
DECLARE item jsonb; cid bigint;
BEGIN
 PERFORM pg_advisory_xact_lock(76104,1);
 FOR item IN SELECT value FROM jsonb_array_elements($development_import$""" + payload + """$development_import$::jsonb) LOOP
  INSERT INTO public_content(kind,publication_status,data,source_snapshot)
  VALUES('development','draft',item->'data',item->'snapshot')
  ON CONFLICT DO NOTHING RETURNING id INTO cid;
  IF cid IS NULL THEN CONTINUE; END IF;
  INSERT INTO development_case(id,data) VALUES(cid,(item->'data')-'timeline'-'media');
  INSERT INTO development_case_timeline(case_id,sort_order,data)
    SELECT cid,ordinality-1,value FROM jsonb_array_elements(item->'data'->'timeline') WITH ORDINALITY;
  INSERT INTO development_case_media(case_id,sort_order,data)
    SELECT cid,ordinality-1,value FROM jsonb_array_elements(item->'data'->'media') WITH ORDINALITY;
  INSERT INTO reef_prd_migration_review(kind,source_id,target_id,reason,source_data)
    VALUES('development_import',cid,cid,'Imported source as draft; verify discrepancies, dates and image licenses before publication.',item->'snapshot');
 END LOOP;
END $import$;
"""


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('directory', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    records = prepare(args.directory)
    args.output.write_text(sql_payload(records), encoding='utf-8')
    print(json.dumps(dict(cases=len(records), timeline=sum(len(r['data']['timeline']) for r in records),
                         media=sum(len(r['data']['media']) for r in records)), ensure_ascii=False))
