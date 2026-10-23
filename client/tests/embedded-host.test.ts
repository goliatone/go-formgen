import { afterEach, describe, it, expect } from 'vitest';
import * as source from '../src/index';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
const shipped=new Function(`${readFileSync(resolve('../pkg/runtime/assets/formgen-relationships.min.js'),'utf8')};return FormgenRelationships;`)() as typeof source;
import { compileConditions } from '../src/embedded-conditions';
import type { EmbeddedModel } from '../src/embedded-contract';
for(const [name,{attachFormController,captureSnapshotEnvelope,restoreSnapshotEnvelope,fixedRowFieldPath}] of [['source',source],['shipped',shipped]] as const)describe(`host definition and row revision guards ${name}`, () => {
 afterEach(()=>document.body.innerHTML='');
 const context={definitionRevision:'questionnaire-7',rows:{rows:['row-a','row-b']}};
 function controller(){document.body.innerHTML='<form><input name="answer" value="saved" disabled></form>';return attachFormController(document.querySelector('form')!,{model:{fields:[{name:'answer',type:'string',required:false}]}});}
 it('round trips independent locked state only with matching context',()=>{
  const c=controller();const saved=captureSnapshotEnvelope(c,context);
  c.setValues({answer:'changed'},{notify:false});
  expect(restoreSnapshotEnvelope(c,saved,context,{notify:false})).toEqual([]);
  expect(c.snapshot().values.answer).toBe('saved');
  saved.rows.rows.reverse();expect(context.rows.rows).toEqual(['row-a','row-b']);c.destroy();
 });
 it('rejects missing, mismatched and legacy contexts before mutation or callbacks',()=>{
  const c=controller();const saved=captureSnapshotEnvelope(c,context);const before=c.snapshot();let calls=0;c.onChange(()=>calls++);
  for(const bad of [saved.snapshot,{...saved,definitionRevision:''},{...saved,definitionRevision:'other'},{...saved,rows:{rows:['row-b','row-a']}},{...saved,rows:{rows:['row-a','row-a']}},null]) {
   expect(restoreSnapshotEnvelope(c,bad,context)[0].code).toBe('snapshotContextMismatch');expect(c.snapshot()).toEqual(before);
  }
  expect(calls).toBe(0);c.destroy();
 });
 it('rejects sparse, inherited and non-plain mappings before capture or restore',()=>{
  const c=controller();const saved=captureSnapshotEnvelope(c,context);const before=c.snapshot();let calls=0;c.onChange(()=>calls++);
  const sparse=new Array(2);sparse[1]='row-a';
  const inherited=new Array(2);Object.setPrototypeOf(inherited,Object.assign(Object.create(Array.prototype),{0:'row-b'}));inherited[1]='row-a';
  for(const rows of [{rows:sparse},{rows:inherited},new Map(),new Date(),Object.create({rows:['row-a']})]) {
   const bad={definitionRevision:context.definitionRevision,rows} as typeof context;
   expect(()=>captureSnapshotEnvelope(c,bad)).toThrow(/valid row mapping/);
   expect(restoreSnapshotEnvelope(c,saved,bad)[0].code).toBe('snapshotContextMismatch');
   expect(restoreSnapshotEnvelope(c,{...saved,rows},context)[0].code).toBe('snapshotContextMismatch');
   expect(c.snapshot()).toEqual(before);
  }
  expect(calls).toBe(0);c.destroy();
 });
 it('capture and JSON persistence preserve valid empty and null-prototype mappings',()=>{
  const c=controller();
  for(const rows of [{}, {rows:[]},Object.assign(Object.create(null),{rows:['row-a']})]) {
   const current={definitionRevision:'v1',rows};
   const saved=JSON.parse(JSON.stringify(captureSnapshotEnvelope(c,current)));
   expect(restoreSnapshotEnvelope(c,saved,current,{notify:false})).toEqual([]);
  }
  c.destroy();
 });
 it('rebuilds fixed conditions by stable row identity after reorder/delete',()=>{
  const rows={rows:['row-a','row-b']};
  expect(fixedRowFieldPath(rows,'rows','row-b','answer')).toBe('rows[1].answer');
  rows.rows.reverse();expect(fixedRowFieldPath(rows,'rows','row-b','answer')).toBe('rows[0].answer');
  rows.rows.splice(1,1);expect(()=>fixedRowFieldPath(rows,'rows','row-a','answer')).toThrow();
  expect(()=>fixedRowFieldPath(rows,'rows','row-b','__proto__.x')).toThrow();
  const model:EmbeddedModel={fields:[{name:'gate',type:'boolean',required:false},{name:'rows',type:'array',required:false,items:{name:'row',type:'object',required:false,nested:[{name:'answer',type:'string',required:false}]}}],conditions:{version:'formgen.conditions/v1',rules:[{path:fixedRowFieldPath(rows,'rows','row-b','answer'),requiredWhen:{op:'eq',path:'gate',value:true}}]}};
  expect(compileConditions(model).diagnostics).toEqual([]);
 });
});
