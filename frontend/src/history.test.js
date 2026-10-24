import {test} from "node:test";
import assert from "node:assert/strict";
import {recordHistory} from "./history.js";
const edit=(before,after,time=0,type='insertText',extra={})=>({before:[before,before],after:[after,after],time,type,mode:'source',...extra});
test('continuous typing is one undo unit and preserves both cursor endpoints',()=>{
  const doc={};recordHistory(doc,'','a',edit(0,1));recordHistory(doc,'a','ab',edit(1,2,100));recordHistory(doc,'ab','abc',edit(2,3,200));
  assert.deepEqual(doc.history,['','abc']);assert.deepEqual(doc.historySelections,[[0,0],[3,3]]);
});
test('pause, cursor relocation, paragraph insertion, paste and commands separate undo units',()=>{
  const doc={};recordHistory(doc,'','a',edit(0,1));recordHistory(doc,'a','ab',edit(1,2,1500));
  recordHistory(doc,'ab','axb',edit(1,2,1600));recordHistory(doc,'axb','ax\nb',edit(2,3,1700,'insertLineBreak'));
  recordHistory(doc,'ax\nb','ax\nPASTEb',edit(3,8,1800,'insertFromPaste'));recordHistory(doc,'ax\nPASTEb','# ax\nPASTEb');
  assert.equal(doc.history.length,7);
});
test('backspace groups separately from typing and forward deletion',()=>{
  const doc={};recordHistory(doc,'abc','ab',edit(3,2,0,'deleteContentBackward'));recordHistory(doc,'ab','a',edit(2,1,100,'deleteContentBackward'));
  recordHistory(doc,'a','',edit(0,0,200,'deleteContentForward'));assert.deepEqual(doc.history,['abc','a','']);
});
test('composition updates form one undo unit, later compositions remain separate',()=>{
  const doc={};recordHistory(doc,'','n',edit(0,1,0,'insertCompositionText',{composition:1}));recordHistory(doc,'n','你',edit(0,1,2100,'insertCompositionText',{composition:1}));
  recordHistory(doc,'你','你好',edit(1,2,2300,'insertCompositionText',{composition:2}));assert.deepEqual(doc.history,['','你','你好']);
});
test('typing after undo discards redo branch and mode changes break grouping',()=>{
  const doc={};recordHistory(doc,'','a',edit(0,1));recordHistory(doc,'a','ab',edit(1,2,20,'insertText',{mode:'ir'}));
  doc.historyIndex=1;doc.historyGroup=null;recordHistory(doc,'a','ac',edit(1,2,30));assert.deepEqual(doc.history,['','a','ac']);
});
test('snapshot cap also bounds cursor history',()=>{
  const doc={};for(let i=0;i<300;i++)recordHistory(doc,String(i),String(i+1),edit(0,1,i*2000));
  assert.equal(doc.history.length,250);assert.equal(doc.historySelections.length,250);assert.equal(doc.historyIndex,249);
});
test('file effects track history branches and snapshot eviction',()=>{
  const doc={};recordHistory(doc,'','copy');doc.historyEffects[1]={imageCopy:'token'};
  recordHistory(doc,'copy','typed');assert.deepEqual(doc.historyEffects,[null,{imageCopy:'token'},null]);
  doc.historyIndex=0;recordHistory(doc,'','new branch');assert.deepEqual(doc.historyEffects,[null,null]);
  for(let i=0;i<300;i++)recordHistory(doc,String(i),String(i+1));
  assert.equal(doc.historyEffects.length,doc.history.length);assert.equal(doc.historyEffects[0],null);
});
