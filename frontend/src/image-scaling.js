import { codeRanges } from './html-media-references.js';
import { markdownLinks, writeLink } from './markdown-links.js';

export const imageScales = [25,33,50,67,80,100,150,200];
const escape = value => value.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
const decode = value => value.replace(/&(#x[\da-f]+|#\d+|amp|quot|apos|lt|gt);/gi,(raw,key)=>{
  if(key[0]!=='#')return ({amp:'&',quot:'"',apos:"'",lt:'<',gt:'>'})[key.toLowerCase()]??raw;
  const n=key[1].toLowerCase()==='x'?parseInt(key.slice(2),16):Number(key.slice(1));
  return n>0&&n<=0x10ffff&&!(n>=0xd800&&n<=0xdfff)?String.fromCodePoint(n):'\ufffd';
});

export function htmlImages(text) {
  const ranges=codeRanges(text),images=[];
  for(let i=0;i<text.length;i++){
    const blocked=ranges.find(([a,b])=>i>=a&&i<b);if(blocked){i=blocked[1]-1;continue;}
    if(text[i]==='\\'){i++;continue;}
    if(text.startsWith('<!--',i)){const end=text.indexOf('-->',i+4);i=end<0?text.length:end+2;continue;}
    if(text[i]!=='<')continue;
    const name=/^<([a-z][\w:-]*)(?=[\s/>])/i.exec(text.slice(i));if(!name)continue;
    let end=i+name[0].length,quote='';
    for(;end<text.length;end++){const c=text[end];if(quote){if(c===quote)quote='';}else if(c==='"'||c==="'")quote=c;else if(c==='>')break;}
    if(end===text.length)break;
    const tag=name[1].toLowerCase();
    if(tag==='img'){
      const attrs=[];const start=i+name[0].length;
      for(const m of text.slice(start,end).matchAll(/([^\s"'<>/=]+)(\s*=\s*)?(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+))?/g)){
        attrs.push({key:m[1].toLowerCase(),value:decode(m[3]??m[4]??m[5]??''),start:start+m.index,end:start+m.index+m[0].length});
      }
      const attr=key=>attrs.find(a=>a.key===key);
      if(attr('src'))images.push({start:i,end:end+1,image:true,html:true,attrs,label:attr('alt')?.value||'',title:attr('title')?.value||'',destination:attr('src').value});
    }
    i=end;
    if(['script','style','textarea','title'].includes(tag)){
      const closing=new RegExp('</'+tag+'\\s*>','gi');closing.lastIndex=end+1;const close=closing.exec(text);i=close?closing.lastIndex-1:text.length;
    }
  }
  return images;
}
export function documentImages(text){return [...markdownLinks(text).filter(l=>l.image),...htmlImages(text)].sort((a,b)=>a.start-b.start);}

export function replaceImageDestination(text,image,destination){
  if(image.html){
    const attr=image.attrs.find(a=>a.key==='src');
    return text.slice(image.start,attr.start)+`src="${escape(destination)}"`+text.slice(attr.end,image.end);
  }
  if(image.reference)return writeLink(image.label,destination,image.title,true);
  return text.slice(image.start,image.destinationStart)+destination+text.slice(image.destinationEnd,image.end);
}

export function replaceImageDestinations(text,references){
  for(const image of documentImages(text).reverse()){
    if(!Object.hasOwn(references,image.destination))continue;
    text=text.slice(0,image.start)+replaceImageDestination(text,image,references[image.destination])+text.slice(image.end);
  }
  return text;
}

// Split declarations without treating quoted or parenthesized semicolons as separators.
function withoutZoom(style){
  const parts=[];let start=0,quote='',depth=0,comment=false;
  for(let i=0;i<=style.length;i++){
    const c=style[i];
    if(comment){if(c==='*'&&style[i+1]==='/'){comment=false;i++;}continue;}
    if(quote){if(c==='\\')i++;else if(c===quote)quote='';continue;}
    if(c==='/'&&style[i+1]==='*'){comment=true;i++;continue;}
    if(c==='"'||c==="'")quote=c;else if(c==='(')depth++;else if(c===')')depth=Math.max(0,depth-1);
    if(i===style.length||(c===';'&&!depth)){
      const part=style.slice(start,i);start=i+1;
      if(part.trim()&&!/^\s*(?:\/\*[\s\S]*?\*\/\s*)*zoom\s*:/i.test(part))parts.push(part.trim());
    }
  }
  return parts.length?parts.join(';')+';':'';
}
export function scaledImage(text,image,percent){
  if(!imageScales.includes(percent))throw Error('不支持的图片缩放比例');
  if(!image.html){
    if(percent===100)return text.slice(image.start,image.end);
    return `<img src="${escape(image.destination)}" alt="${escape(image.label)}"${image.title?` title="${escape(image.title)}"`:''} style="zoom:${percent}%;" />`;
  }
  const styles=image.attrs.filter(a=>a.key==='style');
  // HTML uses only the first occurrence of an attribute. Remove duplicate styles too.
  let style=withoutZoom(styles[0]?.value||'');
  if(percent!==100)style+=`zoom:${percent}%;`;
  let raw=text.slice(image.start,image.end);
  for(const attr of [...styles].reverse()){
    const value=attr===styles[0]&&style?`style="${escape(style)}"`:'';
    raw=raw.slice(0,attr.start-image.start)+value+raw.slice(attr.end-image.start);
  }
  if(!styles.length&&style)raw=raw.replace(/\s*\/?>(?=$)/,m=>` style="${escape(style)}"${m}`);
  return raw;
}
