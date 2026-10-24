import test from 'node:test';
import assert from 'node:assert/strict';
import { searchPattern } from './search-pattern.js';
test('observed Typora word boundary excludes Chinese and English substrings', () => {
  assert.equal([...('![透明色块](透明色块.svg)').matchAll(searchPattern('透明色块', {wholeWord:true}))].length, 0);
  assert.deepEqual([...('images typora-data-image.png IMAGE').matchAll(searchPattern('image', {wholeWord:true}))].map(m=>m[0]), ['image','IMAGE']);
});
test('word boundary keeps regex capture numbering and literal punctuation', () => {
  assert.equal('foo-12 foobar-12'.replace(searchPattern('(foo)-(\\d+)', {regex:true,wholeWord:true}), '$2/$1'), '12/foo foobar-12');
  assert.equal([...('a.b axb A.B').matchAll(searchPattern('a.b',{wholeWord:true,caseSensitive:true}))].length,1);
});
