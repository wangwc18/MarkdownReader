const unescape = s => s.replace(/\\([!"#$%&'()*+,\-./:;<=>?@[\\\]^_`{|}~])/g, '$1');
const normalise = s => unescape(s).trim().replace(/\s+/g,' ').toLowerCase();
function closing(text, start, open, close) {
  let depth=1;
  for(let i=start+1;i<text.length;i++){
    if(text[i]==='\\'){i++;continue;}
    if(text[i]===open)depth++;
    if(text[i]===close&&!--depth)return i;
  }
  return -1;
}
function destination(text, start) {
  let i=start;while(/[ \t\n]/.test(text[i]||'!'))i++;
  let raw='', end=i;
  if(text[i]==='<'){
    const stop=text.indexOf('>',i+1);if(stop<0)return null;
    raw=text.slice(i+1,stop);end=stop+1;
  }else{
    let depth=0;
    for(;end<text.length;end++){
      const c=text[end];if(c==='\\'){end++;continue;}
      if(c==='(')depth++;
      if(c===')'){if(!depth)break;depth--;}
      if(/\s/.test(c)&&!depth)break;
    }
    raw=text.slice(i,end);
  }
  let tail=end;while(/[ \t\n]/.test(text[tail]||'!'))tail++;
  let title='';
  if(tail>end&&['"',"'",'('].includes(text[tail])){
    const quote=text[tail], close=quote==='('?')':quote;
    let stop=tail+1;for(;stop<text.length;stop++){if(text[stop]==='\\'){stop++;continue;}if(text[stop]===close)break;}
    if(stop>=text.length)return null;
    title=unescape(text.slice(tail+1,stop));tail=stop+1;
    while(/[ \t\n]/.test(text[tail]||'!'))tail++;
  }
  const destinationStart=i+(text[i]==='<'?1:0);
  return {destination:unescape(raw),title,end:tail,destinationStart,destinationEnd:destinationStart+raw.length};
}
export function markdownLinks(text) {
  const definitions=new Map(), blocked=[];
  let offset=0, fence=null, comment=false;
  for(const line of text.split('\n')){
    const code=/^(?: {0,3}> ?)* {0,3}(`{3,}|~{3,})(.*)$/.exec(line);
    let skip=!!fence||!!code||/^(?: {4}|\t)/.test(line.replace(/^(?: {0,3}> ?)+/,''));
    if(code){if(!fence)fence=code[1];else if(code[1][0]===fence[0]&&code[1].length>=fence.length&&!code[2].trim())fence=null;}
    if(comment||line.includes('<!--')){skip=true;comment=!line.includes('-->');}
    if(skip)blocked.push([offset,offset+line.length+1]);
    else {
      const def=/^ {0,3}\[([^\]]+)\]:[ \t]*(.*)$/.exec(line);
      if(def){
        const value=destination(def[2],0);
        if(value&&!definitions.has(normalise(def[1]))){
          const base=offset+line.length-def[2].length;
          value.destinationStart+=base;value.destinationEnd+=base;
          definitions.set(normalise(def[1]),value);
        }
        blocked.push([offset,offset+line.length+1]);
      }
    }
    offset+=line.length+1;
  }
  const links=[];
  for(let i=0;i<text.length;i++){
    const skip=blocked.find(([a,b])=>i>=a&&i<b);if(skip){i=skip[1]-1;continue;}
    if(text[i]==='\\'){i++;continue;}
    if(text[i]==='`'){
      const run=/^`+/.exec(text.slice(i))[0];const end=text.indexOf(run,i+run.length);
      i=end<0?i+run.length-1:end+run.length-1;continue;
    }
    const image=text[i]==='!'&&text[i+1]==='[';
    const start=i, bracket=image?i+1:i;
    if(text[bracket]!=='[')continue;
    const labelEnd=closing(text,bracket,'[',']');if(labelEnd<0)continue;
    const label=text.slice(bracket+1,labelEnd);
    if(label.startsWith('^'))continue;
    let end=labelEnd+1, info=null, reference=null;
    if(text[end]==='('){info=destination(text,end+1);if(!info||text[info.end]!==')')continue;end=info.end+1;}
    else{
      reference=label;
      if(text[end]==='['){const stop=closing(text,end,'[',']');if(stop<0)continue;reference=text.slice(end+1,stop)||label;end=stop+1;}
      info=definitions.get(normalise(reference));if(!info)continue;
    }
    links.push({start,end,image,label,destination:info.destination,title:info.title,reference,destinationStart:info.destinationStart,destinationEnd:info.destinationEnd});
    i=end-1;
  }
  return links;
}
export function linkAt(text, offset, image) {
  return markdownLinks(text).find(link=>offset>=link.start&&offset<=link.end&&(image===undefined||link.image===image))||null;
}
export function writeLink(label, destination, title='', image=false) {
  const url=destination.replace(/[\s<>]/g,c=>encodeURIComponent(c));
  const target=/[()]/.test(url)?'<'+url+'>':url;
  const safeLabel=label.replace(/\n/g,' ').replace(/(?<!\\)([\[\]])/g,'\\$1');
  return (image?'!':'')+'['+safeLabel+']('+target+(title?' "'+title.replace(/\\/g,'\\\\').replace(/"/g,'\\"').replace(/\n/g,' ')+'"':'')+')';
}
