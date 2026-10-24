import test from 'node:test';
import assert from 'node:assert/strict';
import {mathWithLineBreaks} from './math-line-breaks.js';

test('wrap raw double-backslash and newline expressions only for enabled rendering',()=>{
  for(const tex of ['a\\\\b','c\\newline d']){
    assert.equal(mathWithLineBreaks(tex,true),'\\displaylines{'+tex+'}');
    assert.equal(mathWithLineBreaks(tex,false),tex);
  }
});
test('existing environments remain unchanged and repeated render preparation is stable',()=>{
  for(const tex of ['\\begin{aligned}a&=b\\\\c&=d\\end{aligned}','\\displaylines{m\\\\n}'])assert.equal(mathWithLineBreaks(tex,true),tex);
  const once=mathWithLineBreaks('a\\\\b',true);assert.equal(mathWithLineBreaks(once,true),once);
});
test('comments and longer commands do not create compatibility line breaks',()=>{
  for(const tex of ['x+y','x % \\\\ ignored','\\newlines x'])assert.equal(mathWithLineBreaks(tex,true),tex);
  assert.equal(mathWithLineBreaks('x % comment\ny\\\\z',true),'\\displaylines{x % comment\ny\\\\z}');
  assert.equal(mathWithLineBreaks('x\\%y\\\\z',true),'\\displaylines{x\\%y\\\\z}');
});
