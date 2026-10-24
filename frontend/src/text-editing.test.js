import { test } from "node:test";
import assert from "node:assert/strict";
import { lineBounds, sentenceBounds, replaceMatches } from "./text-editing.js";

test('rendered sentence selection follows observed Chinese and English boundaries', () => {
  const chinese='第一句中文。第二句包含 **加粗文字**！第三句结束？最后一段。';
  let range=sentenceBounds(chinese,chinese.indexOf('包含'));
  assert.equal(chinese.slice(...range),'第二句包含 **加粗文字**！');
  const english='English first sentence. Second sentence has words! Third sentence ends?';
  range=sentenceBounds(english,english.indexOf('has'));
  assert.equal(english.slice(...range),'Second sentence has words! ');
});

test('sentence selection respects soft line boundaries, ranges and empty lines',()=>{
  const text='第一句。第二句！\n软换行之后的句子。这里还有一句。';
  assert.equal(text.slice(...sentenceBounds(text,text.indexOf('这里'))),'这里还有一句。');
  assert.equal(text.slice(...sentenceBounds(text,1,7)),'第一句。第二句！');
  assert.deepEqual(sentenceBounds('\n中文',0),[0,0]);
});

test("line commands exclude a next line selected only at its start", () => {
  assert.deepEqual(lineBounds("第一行\n第二行\n第三行", 0, 4), [0, 3]);
  assert.deepEqual(lineBounds("\n中文", 0), [0, 0]);
  assert.deepEqual(lineBounds("中文\n", 3), [3, 3]);
  assert.deepEqual(lineBounds("甲\n乙", 2), [2, 3]);
});

test("regex replacement follows JavaScript capture and dollar semantics", () => {
  const input = "中文12, English34, 中文56";
  for (const replacement of ["$2-$1", "$$$&", "$`|$'", "$12/$20/$0", "$<word>-$2"] ) {
    const re = /(?<word>中文|English)(\d+)/g;
    assert.equal(replaceMatches(input, [...input.matchAll(re)], replacement, true), input.replace(re, replacement));
  }
});

test("literal replacements keep dollar signs; zero-width unicode matches terminate", () => {
  const input = "甲😀乙";
  const re = /(?=.)/gu;
  assert.equal(replaceMatches(input, [...input.matchAll(re)], "$&", false), "$&甲$&😀$&乙");
});
