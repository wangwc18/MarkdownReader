export function searchPattern(query, {regex = false, caseSensitive = false, wholeWord = false} = {}) {
  const pattern = regex ? query : query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  return new RegExp(wholeWord ? `\\b(?:${pattern})\\b` : pattern, caseSensitive ? 'g' : 'gi');
}
