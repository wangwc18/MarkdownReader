import test from 'node:test';
import assert from 'node:assert/strict';
import {renderPunctuation} from './render-punctuation.js';
test('observed native rendered punctuation sample converts display glyphs',()=>{
 assert.equal(renderPunctuation('"Hello", \'world\'. It\'s 5 -- 7 --- 9...'),'“Hello”, ‘world’. It’s 5 – 7 — 9…');
});
test('render punctuation context survives inline text-node boundaries',()=>{
 const state={previous:''};
 assert.equal(['"','Hello','", \'','world',"'. It's ",'5 -- 7'].map(text=>renderPunctuation(text,state)).join(''),'“Hello”, ‘world’. It’s 5 – 7');
 assert.equal(renderPunctuation('Already “curved” and it’s fine — …'),'Already “curved” and it’s fine — …');
});
