import {markdownLinks} from './markdown-links.js';
const key = value => value.replace(/\\([!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~])/g,'$1').trim().replace(/\s+/g,' ').toLowerCase();

export function restoreReferenceImageTitles(markdown,titles=new Map()) {
  const edits=new Map();
  for(const link of markdownLinks(markdown)){
    if(!link.image||!link.reference)continue;
    const token=titles.get(key(link.reference));
    const parsed=token?markdownLinks(`![sample][id]\n\n[id]: image.png ${token}`)[0]:null;
    const artifact=parsed&&markdown.startsWith(token,link.end)&&(!link.title||link.title===parsed.title);
    if(artifact)edits.set(link.end,{start:link.end,end:link.end+token.length,value:''});
    const start=link.destinationStart,end=link.destinationEnd;
    const raw=markdown.slice(start,end),angle=markdown[start-1]==='<'&&markdown[end]==='>';
    let destination=angle||/[()\s]/.test(link.destination)?'<'+raw+'>':raw;
    if(artifact&&!link.title)destination+=' '+token;
    const a=angle?start-1:start,b=angle?end+1:end;
    if(destination!==markdown.slice(a,b))edits.set(a,{start:a,end:b,value:destination});
  }
  for(const edit of [...edits.values()].sort((a,b)=>b.start-a.start))
    markdown=markdown.slice(0,edit.start)+edit.value+markdown.slice(edit.end);
  return markdown;
}

// Repair only generated title tokens identified by the current IR metadata.
// The live DOM and its selection remain untouched.
export function serializeReferenceImages(root,lute) {
  if(!root)return null;
  const candidates=new Map();let references=false;
  for(const image of root.querySelectorAll('[data-type="img"]')){
    const marker=image.querySelector(':scope > .vditor-ir__marker--link');
    const explicit=/^\[([^\]]*)\]$/.exec(marker?.textContent||'');
    if(marker&&!explicit)continue;
    if(!marker&&image.querySelector(':scope > .vditor-ir__marker--paren'))continue;
    const label=[...image.querySelectorAll(':scope > .vditor-ir__marker--bracket')].map(node=>node.textContent).join('').slice(1,-1);
    const reference=explicit?.[1]||label;
    if(!reference)continue;references=true;
    const title=image.querySelector(':scope > .vditor-ir__marker--title');
    if(title){const id=key(reference),values=candidates.get(id)||new Set();values.add(title.textContent);candidates.set(id,values);}
  }
  if(!references)return null;
  const titles=new Map([...candidates].filter(([,values])=>values.size===1).map(([id,values])=>[id,[...values][0]]));
  const markdown=lute.VditorIRDOM2Md(root.innerHTML).replace(/\u00a0/g,' ');
  return restoreReferenceImageTitles(markdown,titles);
}
