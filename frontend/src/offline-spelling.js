import nspell from 'nspell';

export function englishWords(text){
 return [...text.matchAll(/[A-Za-z]+(?:['’][A-Za-z]+)*/g)].map(m=>({word:m[0],start:m.index,end:m.index+m[0].length}));
}
export function isPersonalWord(word,words){return words?.[word.replaceAll('’',"'")]===true;}
export function effectiveSpelling(document,settings){
 const choice=document?.spellLanguage||'';
 return {enabled:choice==='none'?false:choice?true:!!settings.spellcheck,language:choice==='en-GB'||(!choice&&settings.spellLanguage==='en-GB')?'en-GB':'en-US'};
}
export function createSpellchecker(aff,dic){
 const engine=nspell(aff,dic),cache=new Map();
 return {
  correct(word){const key=word.replaceAll('’',"'");if(!cache.has(key))cache.set(key,engine.correct(key));return cache.get(key);},
  suggest(word){return word.length>64?[]:engine.suggest(word.replaceAll('’',"'")).slice(0,8);},
 };
}
export const spellExcluded='pre:not(.vditor-reset),[data-type="code-block"],[data-type*="math"],mjx-container,script,style,textarea,.vditor-ir__marker,.vditor-ir__preview,[contenteditable="false"],a';
export function spellingTextNodes(root){
 if(!root)return [];
 const result=[],walker=root.ownerDocument.createTreeWalker(root,4);
 while(walker.nextNode())if(!walker.currentNode.parentElement?.closest(spellExcluded))result.push(walker.currentNode);
 return result;
}
