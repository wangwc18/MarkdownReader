const attributeEscape = value => value.replace(/&/g,'&amp;').replace(/"/g,'&quot;').replace(/'/g,'&#39;').replace(/</g,'&lt;').replace(/>/g,'&gt;');

// Ranges where apparent HTML belongs to Markdown code, not a rendered image.
export function codeRanges(text) {
  const ranges=[];let offset=0,fence=null,comment=false;
  for(const line of text.split('\n')){
    const content=line.replace(/^(?: {0,3}> ?)+/,'');
    if(!fence&&(comment||content.includes('<!--'))){
      const start=comment?0:content.indexOf('<!--')+4;
      comment=content.indexOf('-->',start)<0;offset+=line.length+1;continue;
    }
    const match=/^ {0,3}(`{3,}|~{3,})(.*)$/.exec(content);
    if(fence||match||/^(?: {4}|\t)/.test(content))ranges.push([offset,offset+line.length+1]);
    if(match){if(!fence)fence=match[1];else if(match[1][0]===fence[0]&&match[1].length>=fence.length&&!match[2].trim())fence=null;}
    offset+=line.length+1;
  }
  for(let i=0;i<text.length;i++){
    const range=ranges.find(([a,b])=>i>=a&&i<b);if(range){i=range[1]-1;continue;}
    if(text[i]==='\\'){i++;continue;}
    if(text.startsWith('<!--',i)){const end=text.indexOf('-->',i+4);i=end<0?text.length:end+2;continue;}
    if(text[i]!=='`')continue;
    const run=/^`+/.exec(text.slice(i))[0];let end=i+run.length;
    while((end=text.indexOf(run,end))>=0){if(text[end-1]!=='`'&&text[end+run.length]!=='`')break;end+=run.length;}
    if(end>=0){ranges.push([i,end+run.length]);i=end+run.length-1;}else i+=run.length-1;
  }
  return ranges;
}

export function restoreHTMLImageReferences(text,references={}) {
  const replacements=new Map();
  for(const [runtime,original] of Object.entries(references)){
    if(typeof original!=='string')continue;
    for(const key of [runtime,attributeEscape(runtime),original,attributeEscape(original)])replacements.set(key,original);
  }
  if(!replacements.size)return text;
  const ranges=codeRanges(text),edits=[];
  for(let i=0;i<text.length;i++){
    const range=ranges.find(([a,b])=>i>=a&&i<b);if(range){i=range[1]-1;continue;}
    if(text[i]==='\\'){i++;continue;}
    if(text.startsWith('<!--',i)){const end=text.indexOf('-->',i+4);i=end<0?text.length:end+2;continue;}
    if(text[i]!=='<')continue;
    const name=/^<([a-z][\w:-]*)(?=[\s/>])/i.exec(text.slice(i));if(!name)continue;
    let end=i+name[0].length,quote='';
    for(;end<text.length;end++){const c=text[end];if(quote){if(c===quote)quote='';}else if(c==='"'||c==="'")quote=c;else if(c==='>')break;}
    if(end===text.length)break;
    const tag=name[1].toLowerCase();
    if(tag==='img'){
      const start=i+name[0].length,attributes=text.slice(start,end);
      const restored=attributes.replace(/([^\s"'<>/=]+)(\s*=\s*)?(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+))?/g,(raw,key,equals,double,single,bare)=>{
        if(key.toLowerCase()!=='src'||!equals)return raw;
        const original=replacements.get(double??single??bare);if(original===undefined)return raw;
        const q=double!==undefined?'"':single!==undefined?"'":'"';
        return key+equals+q+attributeEscape(original)+q;
      });
      if(restored!==attributes)edits.push({start,end,value:restored});
    }
    i=end;
    if(['script','style','textarea','title'].includes(tag)){
      const closing=new RegExp('</'+tag+'\\s*>','gi');closing.lastIndex=end+1;const close=closing.exec(text);i=close?closing.lastIndex-1:text.length;
    }
  }
  for(const edit of edits.reverse())text=text.slice(0,edit.start)+edit.value+text.slice(edit.end);
  return text;
}
