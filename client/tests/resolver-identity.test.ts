import {afterEach,describe,it,expect,vi} from 'vitest';
import * as source from '../src/index';
import {readFileSync} from 'node:fs';
import {resolve} from 'node:path';
const shipped=new Function(`${readFileSync(resolve('../pkg/runtime/assets/formgen-relationships.min.js'),'utf8')};return FormgenRelationships;`)() as typeof source;
for(const [name,runtime] of [['source',source],['shipped',shipped]] as const)describe(`resolver identity ${name}`,()=>{
 afterEach(()=>{document.body.innerHTML='';vi.unstubAllGlobals();});
 for(const [renderer,multiple] of [['default',false],['default',true],['typeahead',false],['chips',true]] as const)for(const restricted of [false,true])it(`round trips literal IDs with refresh, hydration, snapshots and native submission (${renderer}, ${restricted})`,async()=>{
  const a=runtime.encodeEnumValue(false),b=runtime.encodeEnumValue(a);
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,status:200,text:async()=>JSON.stringify([{id:a,label:'First'},{id:b,label:'Second'}])})));
  document.body.innerHTML=`<form><select name="code" ${multiple?'multiple':''} data-endpoint-renderer="${renderer}" data-endpoint-url="/options" data-endpoint-value-field="id" data-endpoint-label-field="label"></select></form>`;
  const root=document.querySelector('form')!,select=root.querySelector('select')!;
  const registry=await runtime.initFormgenRoot(root);
  const scalar={name:'code',type:'string' as const,required:true,...(restricted?{enum:[a,b]}:{})};
  const model={fields:[multiple?{name:'code',type:'array' as const,required:true,items:scalar}:scalar]};
  const c=runtime.attachFormController(root,{registry,model});
  const updates:unknown[]=[];
  select.addEventListener(runtime.RELATIONSHIP_UPDATE_EVENT,event=>updates.push((event as CustomEvent).detail.selectedValues));
  const first=multiple?[a]:a,second=multiple?[b]:b;const observed:unknown[]=[];
  c.onChange(()=>observed.push(c.snapshot().values.code));
  // Native selection and callback preserve the literal token-shaped identity.
  Array.from(select.options).forEach(o=>o.selected=o.textContent==='First');select.dispatchEvent(new Event('change',{bubbles:true}));
  expect(c.normalize().values.code).toEqual(first);expect(c.validate()).toEqual([]);
  expect(new FormData(root).getAll(multiple?'code[]':'code')).toEqual([runtime.encodeEnumValue(a)]);
  const saved=c.snapshot();
  c.setValues({code:second});expect(c.normalize().values.code).toEqual(second);expect(c.validate()).toEqual([]);
  expect(observed.at(-1)).toEqual(second);expect(select.selectedOptions[0].value).toBe(runtime.encodeEnumValue(b));
  expect(updates.at(-1)).toEqual([b]);
  await registry.get(select)!.resolve();expect(c.snapshot().values.code).toEqual(second);
  expect(c.restore(saved,{notify:false})).toEqual([]);expect(c.snapshot().values.code).toEqual(first);
  expect(select.selectedOptions[0].value).toBe(runtime.encodeEnumValue(a));expect(new FormData(root).getAll(multiple?'code[]':'code')).toEqual([runtime.encodeEnumValue(a)]);
  // Reset consumes native tokens, whereas hydration consumes literal IDs.
  Array.from(select.options).forEach(option=>option.defaultSelected=option.textContent==='Second');
  c.reset({notify:false});expect(c.normalize().values.code).toEqual(second);
  // Legacy hydration also consumes logical relationship IDs without ambiguity.
  runtime.hydrateFormValues(root,{values:{code:first}});expect(select.selectedOptions[0].value).toBe(runtime.encodeEnumValue(a));
  c.destroy();
 });

 it('keeps logical IDs in endpoint parameters and resolver callbacks',async()=>{
  const id=runtime.encodeEnumValue(false);
  const fetch=vi.fn(async(_url:string)=>({ok:true,status:200,text:async()=>JSON.stringify([{id,label:'Record'}])}));
  vi.stubGlobal('fetch',fetch);
  document.body.innerHTML='<form><select name="code" data-endpoint-url="/options" data-endpoint-value-field="id" data-endpoint-label-field="label" data-endpoint-dynamic-params-parent="{{self}}"></select></form>';
  const root=document.querySelector('form')!,select=root.querySelector('select')!;
  const registry=await runtime.initFormgenRoot(root);
  const c=runtime.attachFormController(root,{registry,model:{fields:[{name:'code',type:'string',required:false}]}});
  const results:unknown[]=[];
  select.addEventListener('formgen:relationship:success',event=>results.push((event as CustomEvent).detail.options));
  c.setValues({code:id});await registry.resolve(select);
  expect(new URL(fetch.mock.calls.at(-1)![0] as string,'https://example.test').searchParams.get('parent')).toBe(id);
  expect(results.at(-1)).toMatchObject([{value:id,label:'Record'}]);
  expect(c.snapshot().values.code).toBe(id);
  c.destroy();
 });

 it('encodes new hydrated relationship IDs before inserting native options',async()=>{
  const id=runtime.encodeEnumValue(false);
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,status:200,text:async()=>JSON.stringify([{value:id,label:'Record'}])})).mockResolvedValueOnce({ok:true,status:200,text:async()=>'[]'}));
  document.body.innerHTML='<form><select name="code" data-endpoint-url="/options"></select></form>';
  const root=document.querySelector('form')!,select=root.querySelector('select')!;
  const registry=await runtime.initFormgenRoot(root);
  const c=runtime.attachFormController(root,{registry,model:{fields:[{name:'code',type:'string',required:false}]}});
  c.setValues({code:id});
  expect(c.normalize()).toEqual({values:{code:id},issues:[]});
  expect(new FormData(root).get('code')).toBe(runtime.encodeEnumValue(id));
  await registry.resolve(select);
  expect(c.snapshot().values.code).toBe(id);
  c.destroy();
 });

 for(const renderer of ['typeahead','chips'] as const)it(`keeps literal IDs in ${renderer} user selections`,async()=>{
  const a=runtime.encodeEnumValue(false),b=runtime.encodeEnumValue(a),multiple=renderer==='chips';
  vi.stubGlobal('fetch',vi.fn(async()=>({ok:true,status:200,text:async()=>JSON.stringify([{value:a,label:'First'},{value:b,label:'Second'}])})));
  document.body.innerHTML=`<form><select name="code" ${multiple?'multiple':''} data-endpoint-renderer="${renderer}" data-endpoint-url="/options"></select></form>`;
  const root=document.querySelector('form')!,select=root.querySelector('select')!;
  const registry=await runtime.initFormgenRoot(root);
  const scalar={name:'code',type:'string' as const,required:false};
  const c=runtime.attachFormController(root,{registry,model:{fields:[multiple?{name:'code',type:'array',required:false,items:scalar}:scalar]}});
  const choice=Array.from(root.querySelectorAll<HTMLButtonElement>('button[data-value]')).find(button=>button.dataset.value===b)!;
  expect(choice).toBeDefined();choice.click();
  expect(c.snapshot().values.code).toEqual(multiple?[b]:b);
  expect(select.selectedOptions[0].value).toBe(runtime.encodeEnumValue(b));
  expect(c.validate()).toEqual([]);
  if(multiple){
   const remove=root.querySelector<HTMLButtonElement>('[data-fg-chip-value] button')!;remove.click();
   expect(select.selectedOptions.length).toBe(0);
  }else{
   expect(root.querySelector<HTMLInputElement>('[data-fg-typeahead-root] input')!.dataset.selectedValue).toBe(b);
  }
  c.destroy();
 });
});
