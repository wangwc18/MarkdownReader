import {test} from 'node:test';
import assert from 'node:assert/strict';
import {formatRange} from './format-selection.js';

test('format selection includes the Markdown or HTML markers observed in Typora',()=>{
  for(const span of ['~~删除线~~','==高亮==','<u>下划线</u>','**bold words**','*italic words*']){
    const text='前文 '+span+' 后文',position=4+span.indexOf('线');
    const range=formatRange(text,span.includes('线')?position:7);
    assert.equal(text.slice(...range),span);
  }
});
test('nested formatting selects the smallest enclosing format and code remains atomic',()=>{
  const text='**bold *inner words* rest** 和 `**literal**`';
  assert.equal(text.slice(...formatRange(text,text.indexOf('words'))),'*inner words*');
  assert.equal(text.slice(...formatRange(text,text.indexOf('literal'))),'`**literal**`');
  const html='<u class="a>b">多词 <em>inner</em> 内容</u>';
  assert.equal(html.slice(...formatRange(html,html.indexOf('内容'))),html);
});
test('fences, comments, intraword underscores and escaped markers are excluded',()=>{
  const text='```md\n**code**\n```\n<!-- ==comment== -->\nname_snake_case \\*escaped\\*';
  for(const word of ['code','comment','snake','escaped'])assert.equal(formatRange(text,text.indexOf(word)),null);
  assert.equal(formatRange('**bold** tail',8),null);
});
