// Exercise actual event handlers without a browser or persistent user data.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const v8 = require('node:v8');
const assert = require('node:assert/strict');

function form(debug) {
  const handlers = {}, nodes = new Map();
  const node = key => {
    if (!nodes.has(key)) nodes.set(key, {
      dataset: {}, classList: {add() {}, remove() {}, toggle() {}},
      focus() {}, setAttribute() {}, scrollIntoView() {}
    });
    return nodes.get(key);
  };
  const context = vm.createContext({
    URLSearchParams, structuredClone: x => v8.deserialize(v8.serialize(x)),
    setTimeout: () => 1, clearTimeout() {}, CSS: {escape: x => x},
    localStorage: {getItem: () => null, setItem() {}},
    window: {location: {search: debug ? '?debug=1' : '', protocol: 'file:'},
      innerWidth: 820, addEventListener() {}, scrollTo() {}},
    document: {querySelector: node, querySelectorAll: () => [],
      addEventListener: (name, fn) => { handlers[name] = fn; }}
  });
  for (const file of ['catalog.js', 'app.js']) {
    const source = fs.readFileSync(path.join(__dirname, '..', file), 'utf8');
    vm.runInContext(source.replace(/\nrender\(\);\ninitApp\(\);/, '\n'), context);
  }
  const run = source => vm.runInContext(source, context);
  run('render=()=>{};state.methods=["line"];step=2;');
  const click = (action, data = {}) => handlers.click({
    preventDefault() {}, target: {closest: () => ({dataset: {action, ...data}})}
  });
  return {run, click};
}

const debug = form(true);
for (const phase of ['entry', 'number-review', 'bleaching', 'complete']) {
  debug.click('debug-line-phase', {phase});
  assert.equal(debug.run('state.linePhase'), phase);
  assert.equal(debug.run('Object.keys(state.line).length'), 0);
}
assert.equal(debug.run('step'), 3);
debug.run('step=2;state.linePhase="entry";');
debug.click('review-line-numbers');
assert.equal(debug.run('state.linePhase'), 'number-review');
debug.click('confirm-line-numbers');
assert.equal(debug.run('state.linePhase'), 'bleaching');
debug.click('confirm-line-bleaching');
assert.equal(debug.run('step'), 2);
assert.equal(debug.run('state.linePhase'), 'complete');
debug.run('step=2;state.linePhase="entry";linePage=0;');
debug.click('line-page', {group: '1'});
assert.equal(debug.run('linePage'), 1);

const normal = form(false);
normal.click('debug-line-phase', {phase: 'complete'});
assert.equal(normal.run('state.linePhase'), 'entry');
normal.click('line-page', {group: '1'});
assert.equal(normal.run('linePage'), 0);
normal.run('pointGroups[0].forEach(p=>state.line[p.key]="1");');
normal.click('line-page', {group: '1'});
assert.equal(normal.run('linePage'), 1);
assert.equal(normal.run('state.line["0"]'), '1');

normal.run('linePage=31;state.line["94.5"]="98";');
const map = normal.run('lineLocator()');
assert.equal((map.match(/data-map-key=/g) || []).length, 20);
assert.equal(new Set([...map.matchAll(/data-map-key="([^"]+)"/g)].map(m=>m[1])).size, 20);
assert.equal((map.match(/aria-current="location"/g) || []).length, 1);
assert.ok(map.includes('子樣區 4'));
assert.ok(map.includes('85–94.5 m'));
assert.ok(map.includes('<div class="locator-column-head"><span>m</span><span>數字</span></div>'));
assert.ok(map.includes('data-map-key="94.5">98</i>'));
normal.run('layer="down";state.down["94.5"]="6";');
assert.ok(normal.run('lineLocator()').includes('data-map-key="94.5">6</i>'));
normal.click('locate-line-group', {group: '4'});
assert.equal(normal.run('linePage'), 4);
assert.equal(normal.run('layer'), 'down');
normal.click('locate-line-group', {group: '100'});
assert.equal(normal.run('linePage'), 4);
for(let page=0;page<32;page++){
  normal.run(`linePage=${page};`);
  const html=normal.run('lineLocator()');
  const keys=[...html.matchAll(/data-map-key="([^"]+)"/g)].map(m=>Number(m[1]));
  const start=[0,25,50,75][Math.floor(page/8)]+(page%8<4?0:10);
  assert.deepEqual(keys,Array.from({length:20},(_,i)=>start+i/2));
  assert.ok(html.includes(`data-group="${page}" aria-current="location"`));
}
normal.run('state.linePhase="number-review";state.mud=false;state.line["10"]="1";');
const review=normal.run('lineNumberReview()');
assert.ok(review.includes('<table class="raw-review-board">'));
assert.equal((review.match(/data-review-key=/g)||[]).length,160);
assert.equal(new Set([...review.matchAll(/data-review-key="([^"]+)"/g)].map(m=>m[1])).size,160);
const firstRow=[...review.slice(review.indexOf('<tbody>')).split('</tr>')[0].matchAll(/data-review-key="([^"]+)"/g)].map(m=>Number(m[1]));
assert.deepEqual(firstRow,[0,10,25,35,50,60,75,85]);
assert.ok(review.includes('data-group="4" data-layer="surface" data-review-key="10"'));
assert.ok(review.includes('原始值 1，點選修改">1</button>'));
normal.run('state.mud=true;layer="down";state.line["10"]="9";state.down["10"]="6";');
assert.ok(normal.run('lineNumberReview()').includes('data-group="4" data-layer="down" data-review-key="10"'));
normal.click('edit-line-group',{group:'4',layer:'down'});
assert.equal(normal.run('layer'),'down');
assert.equal(normal.run('linePage'),4);
console.log('PASS: debug/normal navigation; locator boundaries; raw review table positions and original values; mud-layer editing.');
module.exports={form};
