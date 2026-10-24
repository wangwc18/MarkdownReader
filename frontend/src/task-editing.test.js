import test from 'node:test';
import assert from 'node:assert/strict';
import { taskStates, pureTaskListAt } from './task-editing.js';
test('task commands change markers and preserve bracketed body text',()=>{
  assert.equal(taskStates('- [ ] 文本 [x]、`[ ]` 和 [链接](x)\n- [x] 已办','toggle'),'- [x] 文本 [x]、`[ ]` 和 [链接](x)\n- [ ] 已办');
  assert.equal(taskStates('普通 [x]\n\\- [ ] 字面\n- [x]not-a-task','toggle'),'普通 [x]\n\\- [ ] 字面\n- [x]not-a-task');
});
test('explicit task states are idempotent and retain original marker spelling',()=>{
  const input='* [X] 大写\n+ [ ] 待办\n  - [x] 嵌套';
  const complete='* [X] 大写\n+ [x] 待办\n  - [x] 嵌套';
  assert.equal(taskStates(input,'complete'),complete);assert.equal(taskStates(complete,'complete'),complete);
  const undone='* [ ] 大写\n+ [ ] 待办\n  - [ ] 嵌套';assert.equal(taskStates(input,'incomplete'),undone);assert.equal(taskStates(undone,'incomplete'),undone);
});
test('task state updates preserve nested quote prefixes, numbering and CRLF',()=>{
  assert.equal(taskStates('> - [ ] 引用\r\n\t1. [x] 有序\r\n','toggle'),'> - [x] 引用\r\n\t1. [ ] 有序\r\n');
});
test('source context detects mixed list siblings despite separate DOM lists',()=>{
  const mixed='- 普通\n\n- [ ] 待办\n- [x] 已办';assert.equal(pureTaskListAt(mixed,mixed.indexOf('待办')),false);
  const pure='- [ ] 待办\n- [x] 已办';assert.equal(pureTaskListAt(pure,pure.indexOf('已办')),true);
  const separate='- 普通\n\n分隔段落\n\n- [ ] 待办';assert.equal(pureTaskListAt(separate,separate.indexOf('待办')),true);
});
test('task commands preserve nested fenced code and multiline comment examples',()=>{
  const text='- [ ] 任务\n\n  ```md\n  - [ ] 字面任务\n  ```\n  <!--\n  - [x] 注释任务\n  -->\n- [ ] 第二项';
  assert.equal(taskStates(text,'complete'),text.replace('- [ ] 任务','- [x] 任务').replace('- [ ] 第二项','- [x] 第二项'));
});
