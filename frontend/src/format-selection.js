import {markdownLinks} from './markdown-links.js';

export function formatRange(text, start, end=start) {
  const spans=markdownLinks(text).map(link=>[link.start,link.end]);
  const markers=new Map(),tags=new Map();
  let fence=null,offset=0;
  const blocked=[];
  for(const line of text.split('\n')){
    const match=/^(?: {0,3}> ?)* {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
    if(fence||match||/^(?: {4}|\t)/.test(line.replace(/^(?: {0,3}> ?)+/,'')))blocked.push([offset,offset+line.length+1]);
    if(match){if(!fence)fence=match[1];else if(match[1][0]===fence[0]&&match[1].length>=fence.length&&!match[2].trim())fence=null;}
    offset+=line.length+1;
  }
  for(let i=0;i<text.length;){
    const block=blocked.find(([a,b])=>i>=a&&i<b);
    if(block){markers.clear();tags.clear();i=block[1];continue;}
    if(text.startsWith('<!--',i)){const close=text.indexOf('-->',i+4);i=close<0?text.length:close+3;continue;}
    if(text[i]==='\\'){i+=2;continue;}
    if(text.startsWith('\n\n',i)){markers.clear();tags.clear();i+=2;continue;}
    if(text[i]==='`'){
      const run=/^`+/.exec(text.slice(i))[0];let close=i+run.length;
      while((close=text.indexOf(run,close))>=0){if(text[close-1]!=='`'&&text[close+run.length]!=='`')break;close+=run.length;}
      if(close>=0){spans.push([i,close+run.length]);i=close+run.length;}else i+=run.length;
      continue;
    }
    const tag=/^<(\/?)\s*(u|s|del|mark|sup|sub|strong|em|code|span)\b(?:"[^"]*"|'[^']*'|[^'">])*>/i.exec(text.slice(i));
    if(tag){
      const name=tag[2].toLowerCase(),stack=tags.get(name)||[];
      if(tag[1]){if(stack.length)spans.push([stack.pop(),i+tag[0].length]);}
      else if(!/\/\s*>$/.test(tag[0]))stack.push(i);
      tags.set(name,stack);i+=tag[0].length;continue;
    }
    const delimiter=/^(\*{1,3}|_{1,3}|~~|==|~|\^|\${1,2})/.exec(text.slice(i));
    if(!delimiter){i++;continue;}
    const marker=delimiter[0],before=text[i-1]||'',after=text[i+marker.length]||'';
    if(marker.startsWith('_')&&/[\p{L}\p{N}]/u.test(before)&&/[\p{L}\p{N}]/u.test(after)){i+=marker.length;continue;}
    const stack=markers.get(marker)||[];
    if(stack.length&&before&&!/\s/.test(before))spans.push([stack.pop(),i+marker.length]);
    else if(after&&!/\s/.test(after))stack.push(i);
    markers.set(marker,stack);i+=marker.length;
  }
  return spans.filter(([a,b])=>a<=start&&end<=b&&(start!==end||start<b)).sort((a,b)=>(a[1]-a[0])-(b[1]-b[0]))[0]||null;
}
