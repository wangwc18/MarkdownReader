import {test} from "node:test";
import assert from "node:assert/strict";
import {markdownTables, tableAt, transformTable, splitTableRow} from "./table-editing.js";

const fixture = '前文😀\n\n| 名称 | 数值 |\n| :--- | ---: |\n| **甲** | `a\\|b` |\n| 乙 | 2 |\n\n后文\n';
const context = () => tableAt(fixture, fixture.indexOf('**甲**') + 2);
const apply = edit => fixture.slice(0, edit.start) + edit.text + fixture.slice(edit.end);
test('table location excludes fences and respects escaped pipes and optional outer bars', () => {
  assert.deepEqual(splitTableRow('甲 | a\\|b').cells, ['甲','a\\|b']);
  assert.deepEqual(splitTableRow('| 甲 | a\\\\| b |').cells, ['甲','a\\\\','b']);
  assert.equal(markdownTables('```md\n| a | b |\n| - | - |\n```\n').length, 0);
  assert.equal(markdownTables('    | a |\n    | --- |').length, 0);
  assert.equal(markdownTables('>     | a |\n>     | --- |').length, 0);
  assert.throws(()=>transformTable(tableAt('| a |\n| --- |\n| b | extra |',2),'column-right'),/超出表头/);
  assert.equal(context().row, 1); assert.equal(context().column, 0);
  assert.equal(fixture.slice(context().start, context().end).startsWith('| 名称'), true);
});
test('row insertion and movement preserve cells, surrounding document, and column alignment', () => {
  const edit = transformTable(context(), 'row-above');
  const table = markdownTables(apply(edit))[0];
  assert.deepEqual(table.rows, [['名称','数值'],['',''],['**甲**','`a\\|b`'],['乙','2']]);
  assert.ok(apply(edit).startsWith('前文😀\n\n')); assert.ok(apply(edit).endsWith('\n\n后文\n'));
  const moved = markdownTables(transformTable(context(),'row-down').text)[0];
  assert.deepEqual(moved.rows[2], ['**甲**','`a\\|b`']);
  assert.deepEqual(moved.align, [':---','---:']);
});
test('column movement carries header, body and alignment together', () => {
  const moved = markdownTables(transformTable(context(),'column-rightward').text)[0];
  assert.deepEqual(moved.rows, [['数值','名称'],['`a\\|b`','**甲**'],['2','乙']]);
  assert.deepEqual(moved.align, ['---:',':---']);
  const inserted = markdownTables(transformTable(context(),'column-left').text)[0];
  assert.deepEqual(inserted.rows[1], ['', '**甲**','`a\\|b`']);
});
test('delete, resize, format and align have defined structural results', () => {
  assert.equal(markdownTables(transformTable(context(),'delete-row').text)[0].rows.length, 2);
  assert.equal(markdownTables(transformTable(context(),'delete-column').text)[0].rows[1][0], '`a\\|b`');
  assert.equal(transformTable(context(),'delete-table').text, '');
  assert.deepEqual(markdownTables(transformTable(context(),'resize',{rows:4,cols:3}).text)[0].rows[3], ['','','']);
  assert.throws(()=>transformTable(context(),'resize',{rows:0,cols:2}));
  assert.deepEqual(markdownTables(transformTable(context(),'format').text)[0].rows, context().rows);
  assert.equal(transformTable(context(),'format').text.split('\n')[0], '| 名称   | 数值   |');
  assert.equal(markdownTables(transformTable(context(),'align-center').text)[0].align[0], ':---:');
  const one = tableAt('| 甲 |\n| --- |\n| 乙 |', 2);
  assert.equal(transformTable(one,'delete-column').text,'');
});
test('quoted tables retain their container and cursor points into the resulting cell', () => {
  const text = '> | 名称 | 数值 |\n> | --- | --- |\n> | 甲 | 1 |';
  const c = tableAt(text, text.indexOf('甲'));
  const edit = transformTable(c, 'column-right');
  assert.ok(edit.text.split('\n').every(l=>l.startsWith('> |')));
  const after = tableAt(edit.text, edit.cursor);
  assert.equal(after.column, 1); assert.equal(after.row, 1);
});
