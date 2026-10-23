import { afterEach, describe, it, expect } from 'vitest';
import { attachFormController, captureSnapshotEnvelope, restoreSnapshotEnvelope, fixedRowFieldPath } from '../src/index';
import { compileConditions } from '../src/embedded-conditions';
import type { EmbeddedModel } from '../src/embedded-contract';
describe('host definition and row revision guards', () => {
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
