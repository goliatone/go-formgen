import { afterEach,describe,it,expect } from 'vitest';
import fixture from '../../testdata/embedded/authoring.json';
import { attachFormController, encodeEnumValue, preflightEmbeddedDefinition, validateConditional, captureSnapshotEnvelope, restoreSnapshotEnvelope } from '../src/index';
import type { EmbeddedModel } from '../src/embedded-contract';
const model=fixture.form as EmbeddedModel;
describe('synthetic host authoring contract',()=>{
 afterEach(()=>document.body.innerHTML='');
 for(const c of fixture.cases)it(c.id,()=>{
  const result=validateConditional(model,c.values);
  expect(result.issues.map(i=>`${i.code}:${i.path}`).sort()).toEqual([...c.issues].sort());
 });
 it('passes portable preflight and validates a section without unrelated questions',()=>{
  expect(preflightEmbeddedDefinition(model)).toEqual([]);
  const result=validateConditional(model,{q_enabled:false,q_choices:[0],q_scale:null},{scope:{paths:['q_choices']}});
  expect(result.issues).toEqual([]);
 });
 it('preserves unanswered choice, clear/retain, readonly, cancel and explicit presence',()=>{
  document.body.innerHTML=`<form><fieldset><input type="radio" name="q_enabled" value="${encodeEnumValue(false)}"><input type="radio" name="q_enabled" value="${encodeEnumValue(true)}"></fieldset><input name="q_clear"><input name="q_retain"><input type="checkbox" name="q_choices[]" value="${encodeEnumValue(0)}"><input type="checkbox" name="q_choices[]" value="${encodeEnumValue(2)}"><input name="q_scale" type="number"><input name="q_readonly" value="reference" readonly></form>`;
  const c=attachFormController(document.querySelector('form')!,{model});
  expect(document.querySelectorAll('input[name=q_enabled]:checked')).toHaveLength(0);
  expect(c.snapshot().presence.q_enabled).toBe('absent');
  expect(c.validate().some(i=>i.code==='required'&&i.path==='q_enabled')).toBe(true);
  c.setValues({q_enabled:true,q_clear:'followup',q_retain:'history',q_choices:[0],q_scale:0},{notify:false});
  const context={definitionRevision:fixture.definitionRevision,rows:{}};
  const saved=captureSnapshotEnvelope(c,context);
  c.setValues({q_enabled:false},{notify:false});
  expect(c.snapshot().values.q_clear).toBeUndefined();expect(c.snapshot().retained.q_retain).toBe('history');
  c.setValues({q_enabled:true},{notify:false});expect(c.snapshot().values.q_retain).toBe('history');expect(c.snapshot().presence.q_clear).toBe('absent');
  expect(restoreSnapshotEnvelope(c,saved,context,{notify:false})).toEqual([]);expect(c.snapshot()).toEqual(saved.snapshot);
  c.setValues({q_clear:null,q_choices:[]},{notify:false});expect(c.snapshot().presence.q_clear).toBe('null');expect(c.snapshot().values.q_choices).toEqual([]);
  c.setValues({q_clear:''},{notify:false});expect(c.snapshot().presence.q_clear).toBe('present');expect(c.snapshot().values.q_clear).toBe('');
  (document.querySelector('[name=q_readonly]') as HTMLInputElement).disabled=true;
  expect(c.snapshot().values.q_readonly).toBe('reference');expect(c.snapshot().locked).toContain('q_readonly');c.destroy();
 });
});
