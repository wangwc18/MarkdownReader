import {markdownLinks} from './markdown-links.js';
import {restoreHTMLImageReferences} from './html-media-references.js';

export function restoreMediaReferences(value, references = {}) {
  const edits=new Map();
  for(const link of markdownLinks(value)){
    if(!Object.prototype.hasOwnProperty.call(references,link.destination))continue;
    const reference=references[link.destination];
    if(typeof reference!=='string')continue;
    const angle=value[link.destinationStart-1]==='<'&&value[link.destinationEnd]==='>';
    const safe=reference.replace(/[<>]/g,c=>encodeURIComponent(c));
    edits.set(link.destinationStart,{start:link.destinationStart,end:link.destinationEnd,
      value:!angle&&/[\s()]/.test(reference)?'<'+safe+'>':safe});
  }
  for(const edit of [...edits.values()].sort((a,b)=>b.start-a.start))
    value=value.slice(0,edit.start)+edit.value+value.slice(edit.end);
  return restoreHTMLImageReferences(value,references);
}
