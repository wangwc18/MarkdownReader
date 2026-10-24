import DiffMatchPatch from 'diff-match-patch';

export function renderPunctuation(text, state={previous:''}) {
  let out='';
  for(let i=0;i<text.length;i++){
    let value=text[i];
    if(text.startsWith('---',i)){value='—';i+=2;}
    else if(text.startsWith('--',i)){value='–';i++;}
    else if(text.startsWith('...',i)){value='…';i+=2;}
    else if(value==='"'||value==="'"){
      const previous=out.at(-1)||state.previous||'',next=text[i+1]||'';
      const opening=!previous||/[\s([{]/u.test(previous);
      if(value==='"')value=opening?'“':'”';
      else value=opening&&!/^\d/u.test(next)?'‘':'’';
    }
    out+=value;
  }
  if(out)state.previous=out.at(-1);
  return out;
}

const excluded='code,pre:not(.vditor-reset),kbd,samp,script,style,textarea,mjx-container,a,[data-type="a"],[data-type="img"],[data-type*="math"],[data-type="html-inline"],[data-type="html-block"],.vditor-ir__marker,.vditor-ir__preview,[contenteditable="false"]';
const blocks='p,h1,h2,h3,h4,h5,h6,li,td,th';
function selectionSnapshot(root){
  const selection=root?.ownerDocument?.defaultView?.getSelection();
  if(!selection||!root.contains(selection.anchorNode)||!root.contains(selection.focusNode))return null;
  return [selection.anchorNode,selection.anchorOffset,selection.focusNode,selection.focusOffset];
}
function restoreSelection(root,endpoints){
  if(!endpoints||!endpoints[0].isConnected||!endpoints[2].isConnected)return;
  const doc=root.ownerDocument,focused=doc.activeElement;
  try{
    doc.defaultView.getSelection().setBaseAndExtent(...endpoints);
    // Chromium may focus a contenteditable when restoring its selection.
    if(doc.activeElement!==focused){if(focused===doc.body)doc.activeElement?.blur();else focused?.focus?.({preventScroll:true});}
  }catch{}
}

// Display-only substitutions are kept outside the DOM's attributes/markup.
// Serialization and actual edits see the original text, never curved display
// glyphs substituted by this controller.
export class PunctuationView {
  records=new Map();
  diff=new DiffMatchPatch();
  root=null;
  enabled=false;

  restore(){
    if(!this.records.size)return;
    const endpoints=selectionSnapshot(this.root);
    if(endpoints)for(const index of [0,2]){
      const record=this.records.get(endpoints[index]);
      if(record&&endpoints[index].data===record.display)endpoints[index+1]=this.diff.diff_xIndex(this.diff.diff_main(record.display,record.original),endpoints[index+1]);
    }
    for(const [node,record] of this.records){if(node.isConnected&&node.data===record.display)node.data=record.original;}
    this.records.clear();
    restoreSelection(this.root,endpoints);
  }

  apply(root,enabled,{editing=true}={}){
    if(this.root!==root){this.restore();this.root=root;}
    this.enabled=!!enabled;
    if(!root||!enabled){this.restore();return;}
    const doc=root.ownerDocument,selection=doc.defaultView.getSelection();
    const anchor=selection?.anchorNode?.nodeType===1?selection.anchorNode:selection?.anchorNode?.parentElement;
    const active=editing&&root.contains(doc.activeElement)&&root.contains(anchor)?anchor.closest(blocks):null;
    // Restore an entered block before changing any further display text.
    if(active&&[...this.records.keys()].some(node=>active.contains(node)))this.restore();
    for(const [node,record] of this.records)if(!root.contains(node)||node.data!==record.display)this.records.delete(node);
    const endpoints=selectionSnapshot(root),changed=new Map();
    const states=new Map(),walker=doc.createTreeWalker(root,4);
    while(walker.nextNode()){
      const node=walker.currentNode,parent=node.parentElement,block=parent?.closest(blocks);
      if(!block||block===active||parent.closest(excluded))continue;
      const state=states.get(block)||{previous:''};states.set(block,state);
      const existing=this.records.get(node),original=existing?.original??node.data;
      const display=renderPunctuation(original,state);
      if(existing)continue;
      if(display!==original){const record={original,display};this.records.set(node,record);changed.set(node,record);node.data=display;}
    }
    if(endpoints&&changed.size){
      for(const index of [0,2]){const record=changed.get(endpoints[index]);if(record)endpoints[index+1]=this.diff.diff_xIndex(this.diff.diff_main(record.original,record.display),endpoints[index+1]);}
      restoreSelection(root,endpoints);
    }
  }

  originalText(callback){
    const root=this.root,enabled=this.enabled;
    this.restore();
    try{return callback();}finally{if(root&&enabled)this.apply(root,true);}
  }
}
