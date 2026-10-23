// @vitest-environment node
import { it,expect } from 'vitest';
import { buildSync } from 'esbuild';
import { runInNewContext } from 'node:vm';
it('announces each runtime identity once and warns about mixed builds',()=>{
 const info:string[]=[],warn:unknown[]=[];
 const context={console:{info:(m:string)=>info.push(m),warn:(...m:unknown[])=>warn.push(m)}};
 for(const [version,fingerprint]of [['1.2.3','aaa'],['1.2.3','aaa'],['1.2.3','bbb'],['1.2.4','ccc']]){
  const built=buildSync({entryPoints:['src/version.ts'],bundle:true,write:false,format:'iife',globalName:'Identity',define:{__FORMGEN_RUNTIME_VERSION__:JSON.stringify(version),__FORMGEN_RUNTIME_BUILD__:JSON.stringify(fingerprint)}});
  runInNewContext(built.outputFiles[0].text,context);
 }
 expect(info).toHaveLength(3);expect(warn).toHaveLength(2);expect(info[2]).toContain('version 1.2.4 build ccc');
});
