import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import {resolve,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const context={console,setTimeout,clearTimeout,TextEncoder,TextDecoder};context.global=context;context.window=context;vm.createContext(context);
vm.runInContext(fs.readFileSync(resolve(root,'frontend/public/vendor/vditor/dist/js/lute/lute.min.js'),'utf8'),context);
const parse=(source,strict)=>{const lute=context.Lute.New();lute.SetMarkdownReaderStrictMode(strict);return lute.Md2HTML(source);};
test('parser modes match observed missing-space headings, quotes, and protected literals',()=>{
 for(const [source,strict,loose] of [
  ['##标题','<p>##标题</p>\n','<h2>标题</h2>\n'],
  ['## 标题','<h2>标题</h2>\n','<h2>标题</h2>\n'],
  ['\\##转义','<p>##转义</p>\n','<p>##转义</p>\n'],
  ['`##代码`','<p><code>##代码</code></p>\n','<p><code>##代码</code></p>\n'],
  ['> ##引用','<blockquote>\n<p>##引用</p>\n</blockquote>\n','<blockquote>\n<h2>引用</h2>\n</blockquote>\n'],
  ['~~~md\n##代码\n~~~','<pre><code class="language-md">##代码\n</code></pre>\n','<pre><code class="language-md">##代码\n</code></pre>\n'],
  ['#######过多标记','<p>#######过多标记</p>\n','<p>#######过多标记</p>\n'],
 ]){assert.equal(parse(source,true),strict,source+' strict');assert.equal(parse(source,false),loose,source+' loose');}
});
test('list containers use strict marker width or loose two-column continuation',()=>{
 for(const marker of ['1.','10.','100.']){
  const source=marker+' parent\n  - child';
  assert.match(parse(source,true),/<\/ol>\s*<ul>/);
  assert.match(parse(source,false),/<ol(?: start="\d+")?>\s*<li>parent\s*<ul>/);
 }
 for(const strict of [true,false]){
  assert.match(parse('- first\n\n  continued',strict),/<li>\s*<p>first<\/p>\s*<p>continued<\/p>/);
  assert.match(parse('- first\n\n continued',strict),/<\/ul>\s*<p>continued<\/p>/);
 }
});
test('strict and loose instances do not change each other; source remains untouched',()=>{
 const strict=context.Lute.New(),loose=context.Lute.New(),source=fs.readFileSync(resolve(root,'examples/严格模式验证.md'),'utf8');
 strict.SetMarkdownReaderStrictMode(true);loose.SetMarkdownReaderStrictMode(false);
 const before=Buffer.from(source),strictHTML=strict.Md2HTML(source),looseHTML=loose.Md2HTML(source);
 assert.match(strictHTML,/<p>##标题缺少空格<\/p>/);assert.match(looseHTML,/<h2>标题缺少空格<\/h2>/);
 assert.match(strictHTML,/<ol start="10">/);assert.doesNotMatch(looseHTML,/<ol start="10">/);
 assert.equal(strict.Md2HTML(source),strictHTML);assert.equal(loose.Md2HTML(source),looseHTML);assert.deepEqual(Buffer.from(source),before);
 context.Lute.SetMarkdownReaderDefaultStrictMode(false);assert.equal(context.Lute.New().Md2HTML('##标题'),'<h2>标题</h2>\n');
 context.Lute.SetMarkdownReaderDefaultStrictMode(true);assert.equal(context.Lute.New().Md2HTML('##标题'),'<p>##标题</p>\n');
});
