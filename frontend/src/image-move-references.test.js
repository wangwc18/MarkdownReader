import test from 'node:test';
import assert from 'node:assert/strict';
import { imageMoveAliases, movedImageReferences } from './image-move-references.js';
import { replaceImageDestinations } from './image-scaling.js';
test('move resolves local path aliases while preserving unrelated and unavailable references', async()=>{
 const calls=[],paths={'a.svg':'C:/图片/a.svg','./a.svg#red':'c:\\图片\\a.svg','file:///C:/图片/a.svg?x=1#blue':'C:/图片/a.svg','other.svg':'C:/图片/other.svg'};
 const aliases=await imageMoveAliases([...Object.keys(paths),'missing.svg','https://example.test/a.svg','data:image/png;base64,AA=='],'a.svg',async ref=>{calls.push(ref);if(!(ref in paths))throw Error('missing');return paths[ref];});
 assert.deepEqual(aliases,Object.keys(paths).slice(0,3));
 assert.ok(!calls.some(ref=>ref.startsWith('https:')||ref.startsWith('data:')));
 assert.deepEqual(movedImageReferences(aliases,'new%20name.svg'),{'a.svg':'new%20name.svg','./a.svg#red':'new%20name.svg#red','file:///C:/图片/a.svg?x=1#blue':'new%20name.svg?x=1#blue'});
});
test('move aliases update Markdown HTML and reference images without touching literal or ordinary links',()=>{
 const original='![a](a.svg#red "title")\n<img src="./a.svg?x=1#blue" alt="b" />\n![ref][r]\n\n[r]: a.svg#red\n\n[link](a.svg#red) `![code](a.svg#red)`';
 const result=replaceImageDestinations(original,movedImageReferences(['a.svg#red','./a.svg?x=1#blue'],'new.svg'));
 assert.ok(result.includes('![a](new.svg#red "title")'));assert.ok(result.includes('src="new.svg?x=1#blue"'));assert.ok(result.includes('![ref](new.svg#red)'));
 assert.ok(result.includes('[link](a.svg#red)'));assert.ok(result.includes('`![code](a.svg#red)`'));assert.ok(result.includes('[r]: a.svg#red'));
});
