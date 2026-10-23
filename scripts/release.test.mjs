import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname } from 'node:path';
import { prepare, publish, verify, validateEvidence, run } from './release.mjs';
import { versionInfo } from '../client/scripts/artifacts.mjs';

test('production version and lock mismatch are fatal', async () => {
  const root = await mkdtemp(resolve(tmpdir(), 'formgen-version-'));
  try {
    await mkdir(resolve(root, 'client'));
    await writeFile(resolve(root, '.version'), '1.2.3\n');
    await writeFile(resolve(root, 'client/package.json'), '{"version":"1.2.3"}');
    await writeFile(resolve(root, 'client/package-lock.json'), '{"version":"1.2.2","packages":{"":{"version":"1.2.3"}}}');
    await assert.rejects(versionInfo(root), /versions differ/);
    await writeFile(resolve(root, '.version'), 'dev');
    await assert.rejects(versionInfo(root), /Invalid shared/);
  } finally { await rm(root, { recursive:true, force:true }); }
});
test('driver propagates failed child commands', () => {
  assert.throws(() => run(process.cwd(), process.execPath, ['-e', 'process.exit(42)']), /failed \(42\)/);
});
test('publication fails before side effects for dirty tree, existing tag, failed or wrong-SHA CI', async () => {
  // Real repository metadata supplies versionInfo; every command is stubbed.
  const root = resolve(import.meta.dirname, '..');
  for (const fail of ['dirty','tag','remote','ci-failed','ci-sha']) {
    const calls=[];
    const exec = (_root,cmd,args) => {
      calls.push([cmd,...args]);
      if (args[0]==='status') return fail==='dirty' ? ' M tracked' : '';
      if (args[0]==='rev-parse') return 'abc';
      if (args[0]==='tag' && args[1]==='--list') return fail==='tag' ? 'v1' : '';
      if (args[0]==='ls-remote') return fail==='remote' ? 'remote-tag' : '';
      if (cmd==='gh') return JSON.stringify({headSha:fail==='ci-sha'?'other':'abc',conclusion:fail==='ci-failed'?'failure':'success'});
      throw Error('Unexpected operation');
    };
    await assert.rejects(publish(root, '123', exec));
    assert.equal(calls.some(c=>c[0]==='git' && ['push','commit'].includes(c[1]) || c[1]==='tag' && c[2]==='-a'),false);
  }
});
test('failed verification prerequisite cannot reach publication', async () => {
  const root=resolve(import.meta.dirname,'..');
  for(const failure of ['archive','tar','ci','build','stale']) {
    const calls=[];
    const exec=(_root,cmd,args)=>{
      calls.push([cmd,...args]);
      if(args[0]==='status')return '';
      if(args[0]==='rev-parse')return 'abc';
      // First missing archive step or later missing candidate contents must fail.
      if(cmd==='git'&&args[0]==='archive'&&failure==='archive')throw Error('failed preflight');
      if(cmd==='tar')throw Error(`failed ${failure}`);
      return '';
    };
    await assert.rejects(verify(root,'HEAD',exec));
    assert.equal(calls.some(c=>['push','tag','commit'].includes(c[1])),false);
  }
});
test('evidence must match successful exact-SHA artifact and hosted CI', () => {
 const e={format:'formgen.release-evidence/v1',sourceCommit:'a',manifestHash:'b',status:'passed',provider:'github-actions',runUrl:'https://github.com/run/1'};
 validateEvidence(e,'a','b',true);
 for(const patch of [{sourceCommit:'wrong'},{manifestHash:'wrong'},{status:'failed'},{provider:'local'},{runUrl:null}])assert.throws(()=>validateEvidence({...e,...patch},'a','b',true));
});

test('failed candidate verification blocks every publication operation', async () => {
 const { writeFileSync } = await import('node:fs');
 const { sha256,manifestPath } = await import('../client/scripts/artifacts.mjs');
 const root=resolve(import.meta.dirname,'..');
 const manifestHash=sha256(await (await import('node:fs/promises')).readFile(resolve(root,manifestPath)));
 for(const reason of ['build failed','test failed','preflight failed','stale asset','missing asset','wrong runtime export','wrong runtime log']){
  const calls=[];
  const exec=(_root,cmd,args)=>{
   calls.push([cmd,...args]);
   if(args[0]==='status')return '';
   if(args[0]==='rev-parse')return 'abc';
   if(args[0]==='tag'||args[0]==='ls-remote')return '';
   if(cmd==='gh'&&args[1]==='view')return JSON.stringify({headSha:'abc',conclusion:'success',url:'https://github.com/run/1'});
   if(cmd==='gh'&&args[1]==='download') { const dir=args[args.indexOf('--dir')+1];writeFileSync(resolve(dir,'evidence.json'),JSON.stringify({format:'formgen.release-evidence/v1',sourceCommit:'abc',manifestHash,status:'passed',provider:'github-actions',runUrl:'https://github.com/run/1'}));return ''; }
   throw Error('Unexpected command');
  };
  await assert.rejects(publish(root,'123',exec,async()=>{throw Error(reason)}),new RegExp(reason));
  assert.equal(calls.some(c=>c[1]==='push'||c[1]==='tag'&&c[2]==='-a'),false);
 }
});
test('preparation synchronizes all versions and stops on install/build failure', async()=>{
 for(const failedCommand of ['ci','build']){
  const root=await mkdtemp(resolve(tmpdir(),'formgen-prepare-'));
  try{
   await mkdir(resolve(root,'client'));
   await writeFile(resolve(root,'client/package.json'),'{"version":"0.1.0"}');
   await writeFile(resolve(root,'client/package-lock.json'),'{"version":"0.0.1","packages":{"":{"version":"0.0.1"}}}');
   const calls=[];
   const exec=(_root,cmd,args)=>{calls.push([cmd,...args]);if(cmd==='npm'&&(args[0]===failedCommand||args[1]===failedCommand))throw Error('injected failure');return '';};
   await assert.rejects(prepare(root,'0.2.0',exec),/injected failure/);
   assert.equal((await versionInfo(root)).version,'0.2.0');
   assert.equal(calls.some(c=>['push','commit'].includes(c[1])||c[1]==='tag'&&c[2]!=='--list'),false);
   if(failedCommand==='ci')assert.equal(calls.some(c=>c[2]==='build'),false);
  }finally{await rm(root,{recursive:true,force:true});}
 }
});
test('manifest detects stale inputs, outputs, missing files and embedded copy drift',async()=>{
 const {copies,writeManifest,checkManifest}=await import('../client/scripts/artifacts.mjs');
 const root=await mkdtemp(resolve(tmpdir(),'formgen-manifest-'));
 try{
  for(const dir of ['client/src','pkg','examples','scripts','client/dist/esm','client/dist/types'])await mkdir(resolve(root,dir),{recursive:true});
  await writeFile(resolve(root,'.version'),'1.2.3');await writeFile(resolve(root,'.node-version'),process.versions.node);
  await writeFile(resolve(root,'client/package.json'),'{"version":"1.2.3"}');
  await writeFile(resolve(root,'client/package-lock.json'),'{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}');
  await writeFile(resolve(root,'client/dist/esm/index.js'),'export{}');await writeFile(resolve(root,'client/dist/types/index.d.ts'),'export{}');
  for(const [source,targets]of Object.entries(copies))for(const path of [source,...targets]){await mkdir(dirname(resolve(root,path)),{recursive:true});await writeFile(resolve(root,path),'output');}
  await writeManifest(root);await checkManifest(root);
  await writeFile(resolve(root,'client/src/change.ts'),'changed');await assert.rejects(checkManifest(root),/manifest differs/);await writeManifest(root);
  await writeFile(resolve(root,'client/dist/esm/index.js'),'stale');await assert.rejects(checkManifest(root),/manifest differs/);await writeManifest(root);
  await writeFile(resolve(root,'pkg/runtime/assets/formgen-relationships.min.js'),'drift');await assert.rejects(checkManifest(root),/Stale embedded/);
  await rm(resolve(root,'client/dist/browser/formgen-relationships.min.js'));await assert.rejects(checkManifest(root));
 }finally{await rm(root,{recursive:true,force:true});}
});

test('Taskfile wrappers preserve npm failure and retire automatic release/version writes',async()=>{
 const {chmod}=await import('node:fs/promises');const {spawnSync}=await import('node:child_process');
 const root=resolve(import.meta.dirname,'..');const dir=await mkdtemp(resolve(tmpdir(),'formgen-taskfile-'));
 try{
  await writeFile(resolve(dir,'npm'),'#!/bin/sh\nexit 42\n');await chmod(resolve(dir,'npm'),0o755);
  for(const task of ['client:build','client:ci']){
   const r=spawnSync('bash',['taskfile',task],{cwd:root,env:{...process.env,PATH:dir+':'+process.env.PATH},encoding:'utf8'});
   assert.equal(r.status,42,r.stderr);
  }
  for(const task of ['release','client:version:bump','version:set','version:bump']){
   const r=spawnSync('bash',['taskfile',task,'patch'],{cwd:root,env:{...process.env,PATH:dir+':'+process.env.PATH},encoding:'utf8'});
   assert.notEqual(r.status,0);
  }
 }finally{await rm(dir,{recursive:true,force:true});}
});
