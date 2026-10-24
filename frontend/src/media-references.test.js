import {test} from 'node:test';
import assert from 'node:assert/strict';
import {restoreMediaReferences} from './media-references.js';

test('Markdown image runtime references restore without changing other URLs',()=>{
  const url='/files/uuid/image.png';
  const text=`![图片](${url} "标题") [网页](https://example.com/image.png)`;
  assert.equal(restoreMediaReferences(text,{[url]:'../中文%20图像.png'}),'![图片](../中文%20图像.png "标题") [网页](https://example.com/image.png)');
});

test('HTML image references preserve quotes, dimensions and entity-safe original paths',()=>{
  const url='/files/uuid/image.png';
  const input=`<img SRC = '${url}' style="width:2in" alt="图像">`;
  assert.equal(restoreMediaReferences(input,{[url]:'图片/a&copy.png'}),'<img SRC = \'图片/a&amp;copy.png\' style="width:2in" alt="图像">');
});

test('entity escaped runtime URLs restore while data-src and longer URLs remain unchanged',()=>{
  const url='/files/uuid/a&b.png';
  const input='<img src="/files/uuid/a&amp;b.png" data-src="/files/uuid/a&b.png"><img src="/files/uuid/a&b.png-more">';
  assert.equal(restoreMediaReferences(input,{[url]:'../a&b.png'}),'<img src="../a&amp;b.png" data-src="/files/uuid/a&b.png"><img src="/files/uuid/a&b.png-more">');
});

test('decoded original src from editor serialization becomes entity safe again',()=>{
  assert.equal(restoreMediaReferences('<img src="assets/a&copy.png" style="width:128px">',{'/files/id/a&copy.png':'assets/a&copy.png'}),'<img src="assets/a&amp;copy.png" style="width:128px">');
});

test('angle destinations and shared reference definitions restore in place',()=>{
  const url='/files/uuid/image.png',reference='../目录/图 (1).png';
  const input=`![内联](<${url}> "标题") ![甲][image] ![乙][image]\n\n[image]: <${url}> '说明'\n`;
  const expected=`![内联](<${reference}> "标题") ![甲][image] ![乙][image]\n\n[image]: <${reference}> '说明'\n`;
  assert.equal(restoreMediaReferences(input,{[url]:reference}),expected);
});

test('restoration preserves Markdown code, labels, titles and longer URLs',()=>{
  const url='/files/id/image.png';
  const input=`![${url}](${url} "${url}")\n\n\`![代码](${url})\`\n\n~~~md\n![围栏](${url})\n~~~\n\n![其他](${url}-more)`;
  const expected=input.replace(`](${url} "`,'](assets/image.png "');
  assert.equal(restoreMediaReferences(input,{[url]:'assets/image.png'}),expected);
});

test('bare destinations use angle syntax when restored path contains spaces',()=>{
  assert.equal(restoreMediaReferences('![图](/files/id/image.png)',{'/files/id/image.png':'目录/图 (1).png'}),'![图](<目录/图 (1).png>)');
});
test('HTML restoration preserves fenced, indented and inline code examples',()=>{
 const url='/files/id/image.png',tag=`<img src="${url}">`;
 const literal=`\`\`${tag} \` suffix\`\`\n\n~~~html\n${tag}\n~~~\n\n    ${tag}\n\n> ~~~html\n> ${tag}\n> ~~~\n`;
 assert.equal(restoreMediaReferences(literal+'\n'+tag,{[url]:'assets/image.png'}),literal+'\n<img src="assets/image.png">');
});
test('only actual image src attributes are restored, not comments or quoted text',()=>{
 const url='/files/id/image.png',references={[url]:'a&b.png'};
 const literal=`<!-- <img src="${url}"> -->\n<span title='<img src="${url}">'>text</span>\n<script>const sample='<img src="${url}">';</script>\n正文 src="${url}"\n<img title='src="${url}"' data-src="${url}" src="${url}">`;
 assert.equal(restoreMediaReferences(literal,references),literal.replace(`data-src="${url}" src="${url}"`,`data-src="${url}" src="a&amp;b.png"`));
});
test('unquoted image src becomes a safe quoted attribute; escaped markup remains literal',()=>{
 const url='/files/id/image.png';
 assert.equal(restoreMediaReferences(`\\<img src="${url}"> <IMG src=${url} width=128>`,{[url]:'目录/a b.png'}),`\\<img src="${url}"> <IMG src="目录/a b.png" width=128>`);
});
test('fences and unmatched backticks in HTML comments do not hide a following image',()=>{
 const url='/files/id/image.png',prefix='<!--\n~~~html\n` sample\n-->\n';
 assert.equal(restoreMediaReferences(prefix+`<img src="${url}">`,{[url]:'assets/a.png'}),prefix+'<img src="assets/a.png">');
});
