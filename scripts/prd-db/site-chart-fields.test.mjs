import { test } from 'node:test'
import assert from 'node:assert/strict'
import { JSDOM } from 'jsdom'
import { chartFields, bindChartFields, collectChartFields, stationDescription } from '../../web/admin/site-chart-fields.js'

test('chart editor shows database values, keeps automatic inheritance and distinguishes empty selections', () => {
  const e = {chart_key:'taxa_density',sort_order:1}
  const dom = new JSDOM(`<form>${chartFields({}, e, {years:[2019,2025],depths:[5,10],taxa:[{value:'3',label:'蝶魚'},{value:'7',label:'石斑魚'}]})}</form>`)
  try {
    const f = dom.window.document.querySelector('form')
    bindChartFields(f)
    assert.equal(f.elements.year_from.value,'2019')
    assert.equal(f.elements.year_to.value,'2025')
    assert.equal(f.elements.default_depth_m.value,'5')
    assert.match(f.textContent,/蝶魚/)
    const collect = () => collectChartFields(new dom.window.FormData(f),e)
    assert.equal(collect().year_from,null)
    assert.equal(collect().taxon_ids,null)
    const boxes = [...f.querySelectorAll('[name=taxon_ids]')]
    boxes[0].checked = false
    boxes[0].dispatchEvent(new dom.window.Event('change'))
    assert.deepEqual(collect().taxon_ids,[7])
    boxes[1].checked = false
    assert.deepEqual(collect().taxon_ids,[])
    f.elements.year_from.value='2020'
    f.elements.year_from.dispatchEvent(new dom.window.Event('input'))
    assert.equal(collect().year_from,2020)
    f.elements.auto_year_from.checked=true
    f.elements.auto_year_from.dispatchEvent(new dom.window.Event('change'))
    assert.equal(f.elements.year_from.value,'2019')
    assert.equal(collect().year_from,null)
  } finally { dom.window.close() }
})
test('explicit false, zero, empty and unavailable historic selections survive editing', () => {
  const c={chart_key:'impact_history',is_visible:false,sort_order:0,category_keys:['99:level']}
  const dom=new JSDOM(`<form>${chartFields(c,{sort_order:4,chart_key:c.chart_key},{categories:[]})}</form>`)
  const d=collectChartFields(new dom.window.FormData(dom.window.document.querySelector('form')),c)
  assert.equal(d.is_visible,false);assert.equal(d.sort_order,0);assert.deepEqual(d.category_keys,['99:level'])
  dom.window.close()
})
test('station labels distinguish absent observations from site and station coordinates', () => {
  const s={is_active:true,has_sea_temperature:true,latitude:25,longitude:121,availability:'missing_site_coordinates'}
  assert.match(stationDescription(s),/樣點尚未設定座標/)
  assert.doesNotMatch(stationDescription(s),/測站座標未提供/)
  assert.match(stationDescription({...s,has_sea_temperature:false}),/尚無海溫紀錄/)
  assert.match(stationDescription({...s,latitude:null}),/測站座標未提供/)
})
