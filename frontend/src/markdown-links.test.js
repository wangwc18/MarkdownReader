import {test} from 'node:test';
import assert from 'node:assert/strict';
import {markdownLinks,linkAt,writeLink} from './markdown-links.js';
test('inline links handle Chinese paths, balanced parentheses, titles and escapes',()=>{
 const text='前文 [**中文**](<目录/图 (1).png> "标题")，![图\\]像](assets/a\\(b\\).png) 后文';
 const links=markdownLinks(text);assert.equal(links.length,2);assert.equal(links[0].destination,'目录/图 (1).png');assert.equal(links[0].title,'标题');assert.equal(links[1].destination,'assets/a(b).png');assert.equal(links[1].image,true);
 assert.equal(linkAt(text,text.indexOf('中文')).label,'**中文**');
});
test('reference links resolve case and whitespace while definitions and code are excluded',()=>{
 const text='[参考][ A  B ] 和 [快捷]，![参考图][a b]\n\n[a b]: ../中文.md "说明"\n[快捷]: https://example.com\n\n`[代码](bad.md)`\n```md\n[围栏](bad.md)\n```\n<!-- [注释](bad.md) -->';
 const links=markdownLinks(text);assert.equal(links.length,3);assert.equal(links[0].destination,'../中文.md');assert.equal(links[1].destination,'https://example.com');assert.equal(links[2].image,true);
});
test('writing links round trips URL, escaped label and title without interpreting HTML',()=>{
 const md=writeLink('中文 [图]','../目录/a (1).png#图','说明 "引号"',true);
 const link=markdownLinks(md)[0];assert.equal(link.destination,'../目录/a%20(1).png#图');assert.equal(link.title,'说明 "引号"');assert.equal(link.label,'中文 \\[图\\]');assert.equal(link.image,true);
});

test('destination source offsets point to inline or shared definition URI only',()=>{
 const text='![图](<a\\(b\\).png> "title") ![参考][id]\n\n[id]: <目录/a.png> "说明"';
 const links=markdownLinks(text);
 assert.equal(text.slice(links[0].destinationStart,links[0].destinationEnd),'a\\(b\\).png');
 assert.equal(text.slice(links[1].destinationStart,links[1].destinationEnd),'目录/a.png');
});
