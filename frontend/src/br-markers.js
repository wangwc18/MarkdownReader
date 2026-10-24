// Decorate Vditor's existing HTML token; generated CSS supplies the visual break.
// Keep its original code text intact so serialization retains the user's tag.
export function decorateBreakMarkers(root) {
  if (!root?.querySelectorAll) return;
  const nodes = [...root.querySelectorAll('[data-type="html-inline"]')];
  if (root.matches?.('[data-type="html-inline"]')) nodes.push(root);
  for (const node of nodes) {
    const code = node.querySelector(':scope > code.vditor-ir__marker');
    node.classList.toggle('markdownreader-br', !!code && /^<br\s*\/?\s*>$/i.test(code.textContent.trim()));
  }
}
