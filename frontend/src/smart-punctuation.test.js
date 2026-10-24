import {test} from 'node:test';
import assert from 'node:assert/strict';
import {canConvertSmartDash} from './smart-punctuation.js';

test('smart dashes convert ordinary prose and respect selected replacement text',()=>{
  for(const text of ['正文 --','English prose --','[label](target) prose --','`code` prose --','<code>x</code> prose --','$$\nx\n$$\n\nprose --'])
    assert.equal(canConvertSmartDash(text,text.length),true,text);
  assert.equal(canConvertSmartDash('prose --selection',8,17),false);
});

test('smart dashes preserve Markdown structural separators',()=>{
  for(const text of ['--','  --','> --','- --','| --','| :--','---\ntitle: --','---\ntitle: abc\nvalue: --','正文 \\--','https://example.test/path--','[ref]: path--'])
    assert.equal(canConvertSmartDash(text,text.length),false,text);
});

test('smart dashes protect incomplete and complete code and math contexts',()=>{
  for(const text of ['```js\nlet x = --','~~~\n--','> ```\n> code --','- ```js\n  code --','    code --','`code --','``code ` --','$x--','$$\nx--','<code>code --','<pre>code --','[link](https://host/path--','![image](path--','<a title="value --'])
    assert.equal(canConvertSmartDash(text,text.length),false,text);
});

test('smart dash context closes after literals and front matter',()=>{
  for(const text of ['```\ncode\n```\n\nprose --','---\ntitle: test\n---\n\nprose --','---\ntitle: test\n...\n\nprose --','[link](a(b)c) prose --','<a title="value">prose --','$x$ prose --'])
    assert.equal(canConvertSmartDash(text,text.length),true,text);
});
