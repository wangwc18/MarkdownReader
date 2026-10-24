// Apply changes between two editor serializations to the corresponding original
// source. The baseline contains the editor's existing normalization, so it does
// not become a new edit merely because another paragraph changed.
export function preserveRenderedSource(original, before, after, diff) {
  if (before === after) return original;
  if (original === before) return after;
  const mapping = diff.diff_main(before, original), delta = diff.diff_main(before, after), edits = [];
  let offset = 0, pending = null;
  const finish = () => {
    if (!pending) return;
    edits.push({start:diff.diff_xIndex(mapping,pending.start),end:diff.diff_xIndex(mapping,pending.end),value:pending.value});
    pending = null;
  };
  for (const [operation,text] of delta) {
    if (operation === 0) { finish(); offset += text.length; continue; }
    pending ??= {start:offset,end:offset,value:''};
    if (operation < 0) {offset += text.length;pending.end = offset;}
    else pending.value += text;
  }
  finish();
  let value = original;
  for (const edit of edits.reverse()) value = value.slice(0,edit.start)+edit.value+value.slice(edit.end);
  return value;
}
