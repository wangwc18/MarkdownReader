let layer;
export function clearSourceSearchHighlights() {
  if (layer) { layer.hidden = true; layer.replaceChildren(); }
}
export function paintSourceSearchHighlights(field, matches, current) {
  layer ??= Object.assign(document.createElement('div'), {id:'source-search-highlights'});
  if (!layer.isConnected) { layer.setAttribute('aria-hidden','true'); layer.inert = true; field.parentElement.append(layer); }
  const style = getComputedStyle(field);
  for (const property of ['font','letterSpacing','wordSpacing','textIndent','tabSize','padding',
    'boxSizing','wordBreak','overflowWrap','textAlign','direction']) layer.style[property] = style[property];
  Object.assign(layer.style,{left:field.offsetLeft+'px',top:field.offsetTop+'px',
    width:field.clientWidth+'px',height:field.clientHeight+'px',whiteSpace:field.wrap==='off'?'pre':'pre-wrap'});
  layer.replaceChildren();let end = 0;
  for (const match of matches) {
    if (!match[0].length) continue;
    layer.append(document.createTextNode(field.value.slice(end,match.index)));
    const mark = document.createElement('span');mark.textContent = match[0];
    mark.className = match.index === current ? 'search-current' : 'search-match';
    mark.dataset.offset = match.index;layer.append(mark);end = match.index + match[0].length;
  }
  layer.append(document.createTextNode(field.value.slice(end)+'\u200b'));
  layer.hidden = false;syncSourceSearchScroll(field);
}
export function syncSourceSearchScroll(field) {
  if(layer&&!layer.hidden){layer.scrollTop=field.scrollTop;layer.scrollLeft=field.scrollLeft;}
}
