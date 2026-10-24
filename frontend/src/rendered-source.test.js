import test from 'node:test';
import assert from 'node:assert/strict';
import DiffMatchPatch from 'diff-match-patch';
import {preserveRenderedSource} from './rendered-source.js';
const diff=new DiffMatchPatch();
test('rendered text edits preserve unrelated heading, list and whitespace syntax',()=>{
 const source='##标题\n\n10. 项目\n  - 子项\n\n 一段正文\n',baseline='## 标题\n\n1. 项目\n\n   - 子项\n\n一段正文\n';
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('正文','正文编辑'),diff),source.replace('正文','正文编辑'));
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('标题','新标题').replace('子项','新子项'),diff),source.replace('标题','新标题').replace('子项','新子项'));
 assert.equal(preserveRenderedSource(source,baseline,baseline,diff),source);
});
test('intentional formatting, replacement and deletion are retained',()=>{
 const source='##标题\n\n正文\n',baseline='## 标题\n\n正文\n';
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('正文','**正文**'),diff),'##标题\n\n**正文**\n');
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('正文','内容'),diff),'##标题\n\n内容\n');
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('正文',''),diff),'##标题\n\n\n');
 assert.equal(preserveRenderedSource(source,baseline,baseline.replace('## 标题','### 标题'),diff),'###标题\n\n正文\n');
});
