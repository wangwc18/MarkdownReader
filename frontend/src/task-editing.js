// Change the task marker, never bracket-like text inside the item's body.
export function taskStates(text, action) {
  let fence=null,comment=false;
  return text.split('\n').map(line=>{
    const body=line.replace(/^[ \t]*(?:>[ \t]*)*/,'').replace(/\r$/,'');
    if(fence){if(new RegExp('^'+fence[0]+'{'+fence.length+',}[ \\t]*$').test(body))fence=null;return line;}
    if(comment){if(line.includes('-->'))comment=false;return line;}
    const opening=/^(`{3,}|~{3,})/.exec(body);if(opening){fence=opening[1];return line;}
    const start=line.indexOf('<!--');if(start>=0&&line.indexOf('-->',start+4)<0)comment=true;
    return line.replace(/^([ \t]*(?:>[ \t]*)*(?:[-+*]|\d+[.)])[ \t]+)\[([ xX])\](?=[ \t]|$)/, (marker,prefix,state)=>{
      const next=action==='complete'?(state===' '?'x':state):action==='incomplete'?' ':state===' '?'x':' ';
      return prefix+'['+next+']';
    });
  }).join('\n');
}

// Vditor can split a mixed Markdown list into separate DOM lists. Inspect
// source siblings as well before enabling Typora's task-only commands.
export function pureTaskListAt(text, offset) {
  const lines=text.split('\n'),index=text.slice(0,offset).split('\n').length-1;
  const marker=/^([ \t]*(?:>[ \t]*)*)(?:[-+*]|\d+[.)])[ \t]+(.*)$/;
  const current=marker.exec(lines[index]||'');if(!current||!/^\[[ xX]\](?:[ \t]|$)/.test(current[2]))return false;
  for(const step of [-1,1])for(let i=index+step;i>=0&&i<lines.length;i+=step){
    const line=lines[i];if(!line.trim())continue;
    const match=marker.exec(line);
    if(match&&match[1]===current[1]){if(!/^\[[ xX]\](?:[ \t]|$)/.test(match[2]))return false;continue;}
    if(line.startsWith(current[1]+' ')||line.startsWith(current[1]+'\t'))continue;
    break;
  }
  return true;
}
