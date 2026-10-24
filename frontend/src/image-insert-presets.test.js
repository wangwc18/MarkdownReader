import test from 'node:test';
import assert from 'node:assert/strict';
import {selectedImageInsertPreset,imageInsertPresetSettings} from './image-insert-presets.js';
test('native copy presets retain paths and distinguish explicit custom folders',()=>{
 for(const preset of ['copy-current','copy-assets','copy-filename']){
  const settings=imageInsertPresetSettings(preset);
  assert.equal(selectedImageInsertPreset(settings),preset);
  assert.equal(settings.imageFolderCustom,false);
 }
 assert.equal(selectedImageInsertPreset({copyImages:true,imageFolder:'assets'}),'copy-assets');
 assert.equal(selectedImageInsertPreset({imageInsertAction:'copy',imageFolder:'assets',imageFolderCustom:true}),'copy');
 assert.equal(selectedImageInsertPreset({imageInsertAction:'upload',imageFolder:'assets'}),'upload');
 assert.equal(imageInsertPresetSettings('copy').imageFolderCustom,true);
});
