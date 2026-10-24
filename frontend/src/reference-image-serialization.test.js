import {test} from 'node:test';
import assert from 'node:assert/strict';
import {restoreReferenceImageTitles} from './reference-image-serialization.js';
test('shared image title artifacts return to one valid definition',()=>{
 const input='![甲][id]"标题"\n\n![乙][ID]"标题"\n\n[id]: images/a(1).png\n';
 const expected='![甲][id]\n\n![乙][ID]\n\n[id]: <images/a(1).png> "标题"\n';
 assert.equal(restoreReferenceImageTitles(input,new Map([['id','"标题"']])),expected);
});
test('title repair leaves code and user text after generated artifact intact',()=>{
 const input='![图][id]"Title""user text"\n\n[id]: image.png\n\n`![代码][id]"Title"`';
 const expected='![图][id]"user text"\n\n[id]: image.png "Title"\n\n`![代码][id]"Title"`';
 assert.equal(restoreReferenceImageTitles(input,new Map([['id','"Title"']])),expected);
});
test('unrelated text is retained and bare reference parentheses become valid',()=>{
 const input='![图][id]"正文"\n\n[id]: a(1).png\n';
 assert.equal(restoreReferenceImageTitles(input,new Map([['id','"标题"']])),input.replace('a(1).png','<a(1).png>'));
});
test('collapsed and shortcut image references retain their definition titles',()=>{
 const input='![折叠][]"标题甲"\n\n![简写]"标题乙"\n\n[折叠]: images/a(1).png\n[简写]: images/b(2).png\n';
 assert.equal(restoreReferenceImageTitles(input,new Map([['折叠','"标题甲"'],['简写','"标题乙"']])),
 '![折叠][]\n\n![简写]\n\n[折叠]: <images/a(1).png> "标题甲"\n[简写]: <images/b(2).png> "标题乙"\n');
});
