"""Prepare the supplied Reef Check workbooks for embedding, preserving OOXML layout.
Usage: PYTHONPATH=<openpyxl> python3 scripts/prepare-report-templates.py line.xlsx belt.xlsx summary.xlsx
openpyxl is used only to translate shared formulas, never to re-save a workbook.
"""
import sys,re,zipfile,xml.etree.ElementTree as E,posixpath
from openpyxl.formula.translate import Translator
M='http://schemas.openxmlformats.org/spreadsheetml/2006/main'
N={'m':M}
CELL=re.compile(r'<c\b[^>]*?/>|<c\b[^>]*>.*?</c>',re.S)
ATTR=re.compile(r'\br="([A-Z]+)([0-9]+)"')

def prepare(src,kind):
 z=zipfile.ZipFile(src);parts={n:z.read(n) for n in z.namelist()}
 def text(n):return parts[n].decode('utf-8')
 for name in list(parts):
  if name.startswith('xl/worksheets/sheet')and name.endswith('.xml'):
   index=int(re.search(r'sheet(\d+)',name).group(1));raw=text(name)
   # Make shared formulas independent, so changing one cell cannot invalidate its siblings.
   root=E.fromstring(raw);masters={}
   for c in root.findall('.//m:c',N):
    f=c.find('m:f',N)
    if f is not None and f.get('t')=='shared' and f.text:masters[f.get('si')]=(c.get('r'),f.text)
   def patch(match):
    cell=match.group(0);a=ATTR.search(cell)
    if not a:return cell
    col,row=a.group(1),int(a.group(2));ref=col+str(row)
    fm=re.search(r'<f\b([^>]*?)/>|<f\b([^>]*)>(.*?)</f>',cell,re.S)
    if fm:
     attrs=fm.group(1) or fm.group(2);si=re.search(r'\bsi="([0-9]+)"',attrs)
     if 't="shared"' in attrs and si:
      origin,formula=masters[si.group(1)]
      formula=Translator('='+formula,origin=origin).translate_formula(ref)[1:]
      import html
      cell=cell[:fm.start()]+'<f>'+html.escape(formula,quote=False)+'</f>'+cell[fm.end():]
     cell=re.sub(r'<v>.*?</v>','',cell,flags=re.S)
     cell=re.sub(r'\s+t="[bestr]+"','',cell,count=1)
     return cell
    clear=False
    if kind=='line':
     if index==1:
      clear=ref in {'C1','C2','C3','C4','C5','C6','C7','C8','K2','L3','N3','L4','L5','L6','L7','I41','L41','C42'} or (15<=row<=34 and col in ['B','D','F','H','J','L','N','P']) or (37<=row<=39 and col in ['C','D','E','F'])
     if index==4 and row not in [1,16,17,27,41,42,43] and not (row>=44 and col in ['A','C','E','G','I','K','M','O','Q','R']):clear=True
    elif kind=='belt':
     if index==1:
      clear=ref in {'B1','B2','B3','B4','G2','G3','G4','I4','B7','E7','E8','B30','E30','B66'} or (row in list(range(10,22))+list(range(23,29))+list(range(33,42))+list(range(44,51))+list(range(55,60))+list(range(61,65))+[68,69,71,72] and col in ['B','C','D','E'])
     if index==3 and row!=1:clear=True
    else:
     if index==1:clear=col!='A'
     elif index in [2,4,6,7]:clear=col not in ['A','B'] or (index==2 and row in [29,30,145,146,147,148,149])
     elif index==8:clear=col!='A'
     else:clear=True
    if clear:
     opening=cell[:cell.index('>')+1] if '>' in cell else cell
     opening=re.sub(r'\s+t="[^"]*"','',opening).replace('/>','>')
     return opening+'</c>'
    return cell
   raw=CELL.sub(patch,raw)
   # All formula caches have been cleared. Leave layout and formatting tags byte-for-byte.
   parts[name]=raw.encode()
  if name.startswith('xl/charts/')and name.endswith('.xml'):
   raw=text(name)
   def empty_cache(m):
    kind=m.group(1);fmt=re.search(r'<c:formatCode>.*?</c:formatCode>',m.group(0),re.S)
    return '<c:'+kind+'>'+ (fmt.group(0) if fmt else '') +'<c:ptCount val="0"/></c:'+kind+'>'
   raw=re.sub(r'<c:(numCache|strCache)\b[^>]*>.*?</c:\1>',empty_cache,raw,flags=re.S)
   raw=re.sub(r'<c:externalData\b[^>]*(?:/>|>.*?</c:externalData>)','',raw,flags=re.S)
   parts[name]=raw.encode()
 # Exclude the two dedicated mud-underlayer charts, retaining morphology charts.
 if kind=='line':
  removed={'chart9.xml','chart11.xml'}
  relpath='xl/drawings/_rels/drawing2.xml.rels'
  if relpath in parts:
   relraw=text(relpath); ids=[]
   for match in re.finditer(r'<Relationship\b[^>]*/>',relraw):
    if any('Target="../charts/'+chart+'"' in match.group(0) for chart in removed):
     ids.append(re.search(r'Id="([^\"]+)"',match.group(0)).group(1));relraw=relraw.replace(match.group(0),'')
   parts[relpath]=relraw.encode()
   drawing=text('xl/drawings/drawing2.xml')
   drawing=re.sub(r'<xdr:(twoCellAnchor|oneCellAnchor)\b.*?</xdr:\1>',lambda m:'' if any('r:id="'+id+'"' in m.group(0)for id in ids)else m.group(0),drawing,flags=re.S)
   parts['xl/drawings/drawing2.xml']=drawing.encode()
  # The separate below-SI statistics section is not an output mode.
  raw=text('xl/worksheets/sheet3.xml')
  raw=re.sub(r'<row\b[^>]*r="(?:4[0-9]|5[0-3])"[^>]*>.*?</row>','',raw,flags=re.S)
  parts['xl/worksheets/sheet3.xml']=raw.encode()
 # Remove historical scratch sheets from the summary template.
 if kind=='summary':
  raw=text('xl/workbook.xml')
  raw=re.sub(r'<sheet\b[^>]*\br:id="rId(?:3|5|9|10)"[^>]*/>','',raw)
  parts['xl/workbook.xml']=raw.encode()
  raw=text('xl/_rels/workbook.xml.rels');raw=re.sub(r'<Relationship\b(?=[^>]*\bId="rId(?:3|5|9|10)")[^>]*/>','',raw);parts['xl/_rels/workbook.xml.rels']=raw.encode()
 # Discard old calculation chain; Excel rebuilds it after input changes.
 for name in ['xl/calcChain.xml','docProps/custom.xml']:
  parts.pop(name,None)
 for name in list(parts):
  if name.endswith('.rels'):
   raw=text(name);raw=re.sub(r'<Relationship\b(?=[^>]*(?:Target="[^"]*(?:calcChain.xml|custom.xml)"|Type="[^"]*/externalLink"))[^>]*/>','',raw);parts[name]=raw.encode()
  elif name in ['docProps/core.xml','docProps/app.xml']:
   raw=text(name)
   if name.endswith('core.xml'):
    raw=re.sub(r'<(dc:creator|cp:lastModifiedBy)>.*?</\1>',lambda m:'<'+m.group(1)+'></'+m.group(1)+'>',raw,flags=re.S)
   parts[name]=raw.encode()
 raw=text('xl/workbook.xml');raw=re.sub(r'<mc:AlternateContent\b.*?</mc:AlternateContent>','',raw,flags=re.S)
 raw=re.sub(r'activeTab="\d+"','activeTab="0"',raw)
 raw=re.sub(r'<calcPr\b[^>]*/>','<calcPr calcId="191029" fullCalcOnLoad="1" forceFullCalc="1"/>',raw)
 parts['xl/workbook.xml']=raw.encode()
 # Retain only reachable OOXML parts, including comments, charts and print layouts.
 reachable={'[Content_Types].xml'}
 def visit(part):
  if part in reachable:return
  if part not in parts:raise RuntimeError('Missing part '+part)
  reachable.add(part)
  rel=posixpath.join(posixpath.dirname(part),'_rels',posixpath.basename(part)+'.rels') if part else '_rels/.rels'
  if rel not in parts:return
  reachable.add(rel)
  root=E.fromstring(parts[rel])
  for r in root:
   if r.get('TargetMode')=='External':continue
   target=posixpath.normpath(posixpath.join(posixpath.dirname(part),r.get('Target'))).lstrip('/')
   visit(target)
 visit('') if '' in parts else None
 reachable.add('_rels/.rels')
 for r in E.fromstring(parts['_rels/.rels']):
  if r.get('TargetMode')!='External':visit(r.get('Target').lstrip('/'))
 parts={k:v for k,v in parts.items() if k in reachable}
 # Purge unused shared strings (old names and survey text must not remain in the ZIP).
 raw=text('xl/sharedStrings.xml');sis=re.findall(r'<si(?:\s[^>]*)?>.*?</si>',raw,re.S);used={}
 for name in list(parts):
  if not(name.startswith('xl/worksheets/')and name.endswith('.xml')):continue
  def remap(match):
   cell=match.group(0)
   if 't="s"'not in cell:return cell
   v=re.search(r'<v>(\d+)</v>',cell)
   if not v:return cell
   old=int(v.group(1))
   if old not in used:used[old]=len(used)
   return cell[:v.start()]+'<v>'+str(used[old])+'</v>'+cell[v.end():]
  parts[name]=CELL.sub(remap,text(name)).encode()
 parts['xl/sharedStrings.xml']=('<?xml version="1.0" encoding="UTF-8" standalone="yes"?><sst xmlns="'+M+'" count="'+str(len(used))+'" uniqueCount="'+str(len(used))+'">'+''.join(sis[i] for i in used)+'</sst>').encode()
 raw=text('[Content_Types].xml');raw=re.sub(r'<Override\b[^>]*PartName="/([^"]+)"[^>]*/>',lambda m:m.group(0) if m.group(1)in parts else '',raw);parts['[Content_Types].xml']=raw.encode()
 dst='internal/report/templates/'+kind+'.xlsx'
 with zipfile.ZipFile(dst,'w',zipfile.ZIP_DEFLATED)as out:
  for name,raw in parts.items():out.writestr(name,raw)
 print(dst,len(parts))
for src,kind in zip(sys.argv[1:],['line','belt','summary']):prepare(src,kind)
