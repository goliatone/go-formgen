import { describe,it,expect } from 'vitest';
import fixture from '../../testdata/embedded/portable-definitions.json';
import { isPortablePattern, preflightEmbeddedDefinition } from '../src/index';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
const shipped=new Function(`${readFileSync(resolve('../pkg/runtime/assets/formgen-relationships.min.js'),'utf8')};return FormgenRelationships;`)() as typeof import('../src/index');
for(const [name,runtime] of [['source',{isPortablePattern,preflightEmbeddedDefinition}],['shipped',shipped]] as const){
 describe(`portable definitions ${name}`,()=>{
  for(const c of fixture.patterns) it(c.pattern,()=>{expect(runtime.isPortablePattern(c.pattern)).toBe(c.accepted);if(c.accepted){for(const value of c.pass??[])expect(new RegExp(c.pattern).test(value)).toBe(true);for(const value of c.fail??[])expect(new RegExp(c.pattern).test(value)).toBe(false);}});
  for(const c of fixture.definitions.filter(c=>!('sourceOnly' in c)))it(c.name,()=>expect(runtime.preflightEmbeddedDefinition(JSON.parse(c.source)).length===0).toBe(c.accepted));
 });
}
