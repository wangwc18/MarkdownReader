import test from 'node:test';
import assert from 'node:assert/strict';
import { documentImages, htmlImages, scaledImage, imageScales, replaceImageDestination, replaceImageDestinations } from './image-scaling.js';

test('Markdown scaling follows native percentages and leaves original 100% untouched',()=>{
  const text='前文\n![色块](assets/a.png "标题")\n后文',img=documentImages(text)[0];
  assert.equal(scaledImage(text,img,100),text.slice(img.start,img.end));
  for(const n of imageScales.filter(n=>n!==100))assert.equal(scaledImage(text,img,n),`<img src="assets/a.png" alt="色块" title="标题" style="zoom:${n}%;" />`);
  assert.throws(()=>scaledImage(text,img,42));
});
test('repeat HTML scaling preserves source URL and independent attributes',()=>{
  let text='<img src="a&amp;copy.png" alt="色块" width="32" style="color:red;zoom:50%;background:url(\'a;b\');" />';
  const original=documentImages(text)[0];assert.equal(original.destination,'a&copy.png');
  text=scaledImage(text,original,200);
  assert.match(text,/width="32"/);assert.match(text,/color:red;background:url\('a;b'\);zoom:200%;/);
  text=scaledImage(text,documentImages(text)[0],100);
  assert.doesNotMatch(text,/zoom/);assert.match(text,/color:red/);assert.match(text,/src="a&amp;copy.png"/);
  assert.equal(scaledImage('<img src="a" style="zoom:50%;" />',documentImages('<img src="a" style="zoom:50%;" />')[0],100),'<img src="a"  />');
});
test('HTML parsing excludes code, comments and raw text bodies',()=>{
  const text='`<img src="inline">`\n```html\n<img src="fenced">\n```\n    <img src="indent">\n<!-- <img src="comment"> -->\n<script><img src="script"></script>\n<textarea><img src="textarea"></textarea>\n<img alt="a > b" src=real.png>\n![md](real.png)';
  assert.deepEqual(htmlImages(text).map(i=>i.destination),['real.png']);
  assert.deepEqual(documentImages(text).filter(i=>i.destination==='real.png').map(i=>i.html||false),[true,false]);
});
test('reference occurrences scale independently and attributes are escaped',()=>{
  const text='![A][shared]\n![B][shared]\n\n[shared]: a.png "a&b"';
  const images=documentImages(text);assert.equal(images.length,2);
  const replacement=scaledImage(text,images[1],50);
  const next=text.slice(0,images[1].start)+replacement+text.slice(images[1].end);
  assert.match(next,/^!\[A\]\[shared\]/);assert.match(next,/alt="B" title="a&amp;b"/);assert.match(next,/\[shared\]: a.png/);
});
test('copy destination edit preserves HTML attributes and shared reference siblings',()=>{
 const html='<img src="old.png" alt="A" style="zoom:50%;" title="title" />';
 assert.equal(replaceImageDestination(html,documentImages(html)[0],'new&amp.png'),'<img src="new&amp;amp.png" alt="A" style="zoom:50%;" title="title" />');
 const md='![A](<old.png> "title")';assert.equal(replaceImageDestination(md,documentImages(md)[0],'new.png'),'![A](<new.png> "title")');
 const refs='![A][shared]\n![B][shared]\n\n[shared]: old.png';const second=documentImages(refs)[1];
 assert.equal(replaceImageDestination(refs,second,'new.png'),'![B](new.png)');
});
test('bulk destination replacement preserves code, ordinary links, titles and shared definitions',()=>{
 const text='![甲][shared]\n![乙][shared]\n\n[ordinary](old.png)\n\n<img src="old.png" alt="HTML" style="zoom:50%;"/>\n\n`![literal](old.png)`\n\n[shared]: old.png "标题"';
 const result=replaceImageDestinations(text,{'old.png':'/new.png'});
 assert.match(result,/!\[甲\]\(\/new.png "标题"\)/);assert.match(result,/!\[乙\]\(\/new.png "标题"\)/);
 assert.match(result,/<img src="\/new.png" alt="HTML" style="zoom:50%;"\/>/);
 assert.ok(result.includes('[ordinary](old.png)'));assert.ok(result.includes('`![literal](old.png)`'));assert.ok(result.includes('[shared]: old.png "标题"'));
});
