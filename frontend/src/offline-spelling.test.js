import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createSpellchecker,englishWords,isPersonalWord,effectiveSpelling} from './offline-spelling.js';
const spell=createSpellchecker(readFileSync(new URL('../node_modules/dictionary-en/index.aff',import.meta.url),'utf8'),readFileSync(new URL('../node_modules/dictionary-en/index.dic',import.meta.url),'utf8'));
test('bundled English dictionary detects native typo and suggests observed correction',()=>{
 assert.equal(spell.correct('quikc'),false);
 assert.ok(spell.suggest('quikc').includes('quick'));
 for(const word of ['The','quick','brown','fox','jumps','over','the','lazy','dog'])assert.equal(spell.correct(word),true,word);
 assert.equal(spell.correct('colour'),false);assert.equal(spell.correct('color'),true);
});
test('English word offsets retain Unicode and apostrophe source spelling',()=>{
 assert.deepEqual(englishWords('中文 quikc it’s 中文'),[{word:'quikc',start:3,end:8},{word:'it’s',start:9,end:13}]);
 assert.equal(spell.correct('it’s'),spell.correct("it's"));
});
test('portable personal words accept exact entries without changing base dictionary',()=>{
 const words={quikc:true,"it's":true};
 assert.ok(isPersonalWord('quikc',words));assert.ok(isPersonalWord('it’s',words));
 assert.equal(isPersonalWord('Quikc',words),false);assert.equal(isPersonalWord('quikc',{quikc:false}),false);
 assert.equal(isPersonalWord('quikc',null),false);assert.equal(spell.correct('quikc'),false);
});
test('British dictionary distinguishes regional spelling from American dictionary',()=>{
 const gb=createSpellchecker(readFileSync(new URL('../node_modules/dictionary-en-gb/index.aff',import.meta.url),'utf8'),readFileSync(new URL('../node_modules/dictionary-en-gb/index.dic',import.meta.url),'utf8'));
 assert.equal(gb.correct('colour'),true);assert.equal(gb.correct('color'),false);
 assert.equal(spell.correct('colour'),false);assert.equal(spell.correct('color'),true);
 assert.ok(gb.suggest('color').includes('colour'));
});
test('document language overrides default while inherit and none remain independent',()=>{
 assert.deepEqual(effectiveSpelling({}, {spellcheck:true,spellLanguage:'en-GB'}),{enabled:true,language:'en-GB'});
 assert.deepEqual(effectiveSpelling({spellLanguage:'en-US'}, {spellcheck:false,spellLanguage:'en-GB'}),{enabled:true,language:'en-US'});
 assert.equal(effectiveSpelling({spellLanguage:'none'}, {spellcheck:true}).enabled,false);
 assert.deepEqual(effectiveSpelling(null,{spellcheck:false}),{enabled:false,language:'en-US'});
});
