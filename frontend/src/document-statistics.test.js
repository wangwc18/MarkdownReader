import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import {documentStatistics, statisticsText} from './document-statistics.js';

test('native Typora strict-mode sample: words, source characters and lines', () => {
  const source = fs.readFileSync(new URL('../../examples/严格模式验证.md', import.meta.url), 'utf8');
  assert.deepEqual(documentStatistics(source), {words:93, characters:197, lines:34, minutes:1});
});
test('native mixed-text samples keep hyphens, decimal points and apostrophes', () => {
  assert.equal(documentStatistics('hello-world').words, 1);
  assert.equal(documentStatistics('foo_bar').words, 2);
  assert.equal(documentStatistics("123 3.14 hello-world foo_bar don't 한국어 😀").words, 9);
  assert.equal(documentStatistics("中文 hello-world foo_bar 123 3.14 don't 😀 日本語 한국어").words, 14);
});
test('native ordered lists exclude both marker styles, ordinary numbers remain', () => {
  assert.equal(documentStatistics('1. apple\n10. banana\n\n3) cherry\n\n123 3.14').words, 5);
  assert.equal(documentStatistics('~~~md\n2. banana\n~~~').words, 3);
});
test('literal and escaped markers remain available for counting', () => {
  const code = '~~~md\n2. banana\n~~~\n\n    3. code\n\n\\4. escaped\n\n`code\n5. code`';
  assert.equal(statisticsText(code), code);
  assert.equal(documentStatistics('').words, 0);
  assert.equal(documentStatistics('one two three', 2).minutes, 2);
});
test('observed nested ordered markers are excluded but separate indented code stays literal', () => {
  const source='1. parent\n   2. child\n      3. deep\n\noutside\n\n    4. literal';
  assert.equal(documentStatistics(source).words,6);
  assert.equal(statisticsText(source),' parent\n    child\n       deep\n\noutside\n\n    4. literal');
  assert.equal(documentStatistics('1. parent\n   ~~~md\n   2. code\n   ~~~').words,4);
  assert.equal(documentStatistics('> 1. parent\n>    2. child\n>       3. deep').words,3);
});
test('source character counts include document CRLF without double counting existing CR', () => {
  const source='1. parent\n   2. child\n      3. deep\n\noutside\n\n    4. literal';
  assert.equal(documentStatistics(source,300,true,'LF').characters,60);
  assert.equal(documentStatistics(source,300,true,'CRLF').characters,66);
  assert.equal(documentStatistics(source.replaceAll('\n','\r\n'),300,true,'CRLF').characters,66);
});
