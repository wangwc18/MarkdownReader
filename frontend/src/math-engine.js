// MathJax is shared with Vditor's SVG export. Recreate its input components
// when physics changes, so a previous export cannot make the switch sticky.
import { mathWithLineBreaks } from './math-line-breaks.js';
let pending = Promise.resolve();
let appliedPhysics;
let appliedLineBreaks;
let basePackages;
let renderSequence = 0;
const renderedRoots = new WeakMap();
const generations = new WeakMap();

function enqueue(operation) {
  const task=pending.then(operation);
  pending=task.catch(()=>{});
  return task;
}

export function prepareMathJax(cdn, physics, lineBreaks = true) {
  return enqueue(()=>configure(cdn,physics,lineBreaks));
}

async function configure(cdn, physics, lineBreaks) {
    if (!window.MathJax?.startup?.document) {
      window.MathJax = {
        loader:{paths:{mathjax:cdn+'/dist/js/mathjax'}},
        startup:{typeset:false},svg:{fontCache:'none'},
      };
      await new Promise((resolve,reject) => {
        const script=document.createElement('script');
        script.id='protyleMathJaxScript';script.src=cdn+'/dist/js/mathjax/tex-svg-full.js';
        script.onload=resolve;script.onerror=()=>{script.remove();reject(Error('本地公式资源加载失败，请重新解压完整程序'));};
        document.head.append(script);
      });
      await window.MathJax.startup.promise;
    }
    const engine=window.MathJax;
    if (!basePackages) basePackages=[...engine.config.tex.packages].filter(value=>value!=='physics');
    if (appliedPhysics !== !!physics || appliedLineBreaks !== !!lineBreaks) {
      if (physics) await engine.loader.load('[tex]/physics');
      engine.config.tex.packages=physics?[...basePackages,'physics']:[...basePackages];
      engine.startup.getComponents();
      engine.startup.document.inputJax[0].preFilters.add(({math})=>{
        math.math=mathWithLineBreaks(math.math,!!lineBreaks);
      });
      appliedPhysics=!!physics;
      appliedLineBreaks=!!lineBreaks;
    }
    return engine;
}

function inputs(root, editor) {
  return [...root.querySelectorAll(editor ? '.vditor-ir__preview .language-math' : '.language-math')]
    .map(node=>({node,tex:(node.getAttribute('data-math') ?? node.textContent).replace(/\u00a0/g,' ').trim(),display:node.tagName==='DIV'}));
}
const equalInputs=(a,b)=>a.length===b.length&&a.every((entry,i)=>entry.node===b[i].node&&entry.tex===b[i].tex&&entry.display===b[i].display);

// A complete MathDocument resolves forward references and owns the numbering
// transaction. Old render jobs never commit into replaced or edited nodes.
export function renderDocumentMath(root, options, {editor=false,isCurrent=()=>true,force=false}={}) {
  const entries=inputs(root,editor),key=JSON.stringify([!!options.physics,options.lineBreaks!==false,!!options.numbering]);
  const old=renderedRoots.get(root);
  if(!force&&old?.key===key&&equalInputs(old.entries,entries))return Promise.resolve(false);
  const generation=(generations.get(root)||0)+1;generations.set(root,generation);
  const valid=()=>root.isConnected&&generations.get(root)===generation&&isCurrent()&&equalInputs(entries,inputs(root,editor));
  return enqueue(async()=>{
    if(!valid())return false;
    if(!entries.length){renderedRoots.set(root,{key,entries});return false;}
    const engine=await configure(options.cdn,!!options.physics,options.lineBreaks!==false);
    let serial,delimiters;
    do {serial=++renderSequence;delimiters=['\uE100'+serial+'i','\uE101'+serial+'i','\uE100'+serial+'d','\uE101'+serial+'d'];}
    while(entries.some(entry=>delimiters.some(token=>entry.tex.includes(token))));
    const prefix='mr-math-'+serial+'-';
    engine.config.tex.tags=options.numbering?'all':'none';
    engine.config.tex.inlineMath=[[delimiters[0],delimiters[1]]];
    engine.config.tex.displayMath=[[delimiters[2],delimiters[3]]];
    engine.startup.getComponents();
    engine.startup.document.inputJax[0].preFilters.add(({math})=>{math.math=mathWithLineBreaks(math.math,options.lineBreaks!==false);});
    engine.texReset();
    const stage=document.createElement('div'),style=getComputedStyle(root);
    stage.className='vditor-reset';
    Object.assign(stage.style,{position:'fixed',left:'-100000px',top:'0',width:Math.max(100,root.clientWidth||800)+'px',fontSize:style.fontSize,fontFamily:style.fontFamily,visibility:'hidden',pointerEvents:'none'});
    const holders=entries.map(entry=>{const node=document.createElement('div');node.textContent=(entry.display?delimiters[2]:delimiters[0])+entry.tex+(entry.display?delimiters[3]:delimiters[1]);stage.append(node);return node;});
    document.body.append(stage);
    let output;
    try {
      await engine.typesetPromise([stage]);
      const ids=new Map();
      for(const node of stage.querySelectorAll('[id]')){
        const id=node.id;ids.set(id,prefix+id);node.id=prefix+id;
      }
      for(const node of stage.querySelectorAll('a')){
        for(const attribute of ['href','xlink:href']){
          const href=node.getAttribute(attribute);if(!href)continue;
          const fragment=href.slice(href.lastIndexOf('#')+1);
          let id;try{id=decodeURIComponent(fragment);}catch{continue;}
          if(href.includes('#')&&ids.has(id))node.setAttribute(attribute,'#'+encodeURIComponent(ids.get(id)));
        }
      }
      output=holders.map(holder=>{
        const math=holder.querySelector('mjx-container');
        if(!math)throw Error('公式文档排版未产生结果');
        return math.cloneNode(true);
      });
    } finally {engine.typesetClear([stage]);stage.remove();}
    if(!valid())return false;
    for(let i=0;i<entries.length;i++){
      entries[i].node.setAttribute('data-math',entries[i].tex);
      entries[i].node.replaceChildren(output[i]);
    }
    renderedRoots.set(root,{key,entries});
    return true;
  });
}
