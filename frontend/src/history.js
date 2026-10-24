function delta(before, after) {
  let start = 0, suffix = 0;
  while (start < before.length && start < after.length && before[start] === after[start]) start++;
  while (suffix < before.length - start && suffix < after.length - start && before[before.length - suffix - 1] === after[after.length - suffix - 1]) suffix++;
  return {removed: before.slice(start, before.length - suffix), inserted: after.slice(start, after.length - suffix)};
}

// Keep document snapshots independent of Vditor's DOM undo stack, including mode changes.
export function recordHistory(doc, before, after, transaction = null) {
  doc.history ??= [before];
  doc.historyIndex ??= 0;
  doc.historySelections ??= [];
  doc.historyEffects ??= Array(doc.history.length).fill(null);
  doc.historyEffects.length=doc.history.length;
  for(let i=0;i<doc.historyEffects.length;i++)doc.historyEffects[i]??=null;
  const change = delta(before, after);
  const inserted = before ? change.inserted : change.inserted.replace(/\n$/, "");
  let kind = null;
  if (transaction?.composition) kind = "composition:" + transaction.composition;
  else if (transaction && transaction.before[0] === transaction.before[1]) {
    if (transaction.type === "insertText" && !change.removed && !inserted.includes("\n")) kind = "typing";
    if (["deleteContentBackward", "deleteContentForward"].includes(transaction.type) && !change.inserted && !change.removed.includes("\n")) kind = transaction.type;
  }
  const previous = doc.historyGroup;
  const merge = !!(kind && previous && previous.kind === kind && previous.mode === transaction.mode &&
    (kind.startsWith("composition:") || transaction.time - previous.time <= 1000) && transaction.time >= previous.time &&
    previous.index === doc.historyIndex && doc.historyIndex === doc.history.length - 1 &&
    (kind.startsWith("composition:") || (previous.after[0] === transaction.before[0] && previous.after[1] === transaction.before[1])));
  if (merge) doc.history[doc.historyIndex] = after;
  else {
    if (transaction) doc.historySelections[doc.historyIndex] = transaction.before;
    doc.history.splice(doc.historyIndex + 1);
    doc.historySelections.splice(doc.historyIndex + 1);
    doc.historyEffects.splice(doc.historyIndex + 1);
    doc.history.push(after);
    doc.historyEffects.push(null);
    if (doc.history.length > 250) {doc.history.shift();doc.historySelections.shift();doc.historyEffects.shift();doc.historyEffects[0]=null;}
    doc.historyIndex = doc.history.length - 1;
  }
  if (transaction) doc.historySelections[doc.historyIndex] = transaction.after;
  doc.historyGroup = kind ? {kind, mode: transaction.mode, time: transaction.time, after: transaction.after, index: doc.historyIndex} : null;
}
