// Measure the actual wrapped source layout without changing its selection or value.
export function scrollSourceSelection(field) {
  const style = getComputedStyle(field);
  const mirror = document.createElement('div');
  for (const property of ['font', 'letterSpacing', 'wordSpacing', 'textIndent',
    'tabSize', 'padding', 'boxSizing', 'wordBreak', 'overflowWrap']) {
    mirror.style[property] = style[property];
  }
  Object.assign(mirror.style, {
    position: 'fixed', left: '-100000px', top: '0', visibility: 'hidden',
    whiteSpace: field.wrap === 'off' ? 'pre' : 'pre-wrap',
    width: `${field.clientWidth}px`, height: 'auto', border: '0',
  });
  const marker = document.createElement('span');
  marker.textContent = '\u200b';
  mirror.append(document.createTextNode(field.value.slice(0, field.selectionStart)), marker,
    document.createTextNode(field.value.slice(field.selectionStart) || '\u200b'));
  document.body.append(mirror);
  try {
    const top = marker.getBoundingClientRect().top - mirror.getBoundingClientRect().top;
    const lineHeight = parseFloat(style.lineHeight) || parseFloat(style.fontSize) * 1.75;
    field.focus({preventScroll: true});
    field.scrollTop = Math.max(0, top - (field.clientHeight - lineHeight) / 2);
  } finally {
    mirror.remove();
  }
}

export function scrollRenderedSelection(viewport, selection = getSelection()) {
  if (!viewport || !selection?.rangeCount || !viewport.contains(selection.anchorNode)) return;
  // A paragraph can be taller than the viewport. Its element bounds cannot locate
  // a caret inside a wrapped line; measure the range itself instead.
  const caret = selection.getRangeAt(0).cloneRange();
  caret.collapse(true);
  let rect = [...caret.getClientRects()].find(rect => rect.height > 0);
  if (!rect) rect = [...selection.getRangeAt(0).getClientRects()].find(rect => rect.height > 0);
  if (!rect) {
    selection.anchorNode.parentElement?.scrollIntoView({block: 'center'});
    return;
  }
  const bounds = viewport.getBoundingClientRect();
  viewport.scrollTop += rect.top - bounds.top - viewport.clientTop -
    (viewport.clientHeight - rect.height) / 2;
}
