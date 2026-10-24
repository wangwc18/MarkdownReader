import aff from '../node_modules/dictionary-en/index.aff?raw';
import dic from '../node_modules/dictionary-en/index.dic?raw';
import {createSpellchecker} from './offline-spelling.js';
export default createSpellchecker(aff,dic);
