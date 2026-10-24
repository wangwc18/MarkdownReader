const local = reference => !/^(https?:|data:|blob:)/i.test(reference);
const pathKey = path => path.replace(/\//g, '\\').toLowerCase();
export async function imageMoveAliases(references, selected, resolve) {
  const source = pathKey(await resolve(selected)), aliases = [];
  const unique = [...new Set([selected, ...references])];
  if (unique.length > 2000) throw Error('单次移动引用数量超过2000');
  for (const reference of unique) {
    if (!local(reference)) continue;
    try { if (pathKey(await resolve(reference)) === source) aliases.push(reference); }
    catch (error) { if (reference === selected) throw error; }
  }
  return aliases;
}
export function movedImageReferences(aliases, destination) {
  return Object.fromEntries(aliases.map(reference => {
    const suffix = reference.search(/[?#]/);
    return [reference, destination + (suffix < 0 ? '' : reference.slice(suffix))];
  }));
}
