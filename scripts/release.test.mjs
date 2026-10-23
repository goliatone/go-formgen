import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, writeFile, readFile, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, dirname } from 'node:path';
import { prepare, publish, verify, validateEvidence, run, manualRelease, releaseVersion } from './release.mjs';
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
  await mkdir(resolve(root,'examples/nested/.gomodcache/dependency'),{recursive:true});
  await writeFile(resolve(root,'examples/nested/.gomodcache/dependency/noise.go'),'cached Go module');
  await checkManifest(root);
  await writeFile(resolve(root,'client/src/change.ts'),'changed');await assert.rejects(checkManifest(root),/manifest differs/);await writeManifest(root);
  await writeFile(resolve(root,'client/dist/esm/index.js'),'stale');await assert.rejects(checkManifest(root),/manifest differs/);await writeManifest(root);
  await writeFile(resolve(root,'pkg/runtime/assets/formgen-relationships.min.js'),'drift');await assert.rejects(checkManifest(root),/Stale embedded/);
  await rm(resolve(root,'client/dist/browser/formgen-relationships.min.js'));await assert.rejects(checkManifest(root));
 }finally{await rm(root,{recursive:true,force:true});}
});

test('Taskfile wrappers preserve failures and forward manual release arguments',async()=>{
 const {chmod}=await import('node:fs/promises');const {spawnSync}=await import('node:child_process');
 const root=resolve(import.meta.dirname,'..');const dir=await mkdtemp(resolve(tmpdir(),'formgen-taskfile-'));
 try{
  await writeFile(resolve(dir,'npm'),'#!/bin/sh\nexit 42\n');await chmod(resolve(dir,'npm'),0o755);
  for(const task of ['client:build','client:ci']){
   const r=spawnSync('bash',['taskfile',task],{cwd:root,env:{...process.env,PATH:dir+':'+process.env.PATH},encoding:'utf8'});
   assert.equal(r.status,42,r.stderr);
  }
  for(const task of ['client:version:bump','version:set','version:bump']){
   const r=spawnSync('bash',['taskfile',task,'patch'],{cwd:root,env:{...process.env,PATH:dir+':'+process.env.PATH},encoding:'utf8'});
   assert.notEqual(r.status,0);
  }
  await writeFile(resolve(dir,'node'),'#!/bin/sh\nprintf "%s\\n" "$@"\nexit 42\n');await chmod(resolve(dir,'node'),0o755);
  const manual=spawnSync('bash',['taskfile','release','minor','Manual release'],{cwd:root,env:{...process.env,PATH:dir+':'+process.env.PATH},encoding:'utf8'});
  assert.equal(manual.status,42,manual.stderr);
  assert.equal(manual.stdout,'scripts/release.mjs\nmanual\nminor\nManual release\n');
 }finally{await rm(dir,{recursive:true,force:true});}
});

test('committed artifact hashing streams source maps larger than exec buffers',async()=>{
 const {execFileSync}=await import('node:child_process');
 const {gitBlobHash}=await import('./release.mjs');const {sha256}=await import('../client/scripts/artifacts.mjs');
 const root=await mkdtemp(resolve(tmpdir(),'formgen-blob-'));
 try{
  execFileSync('git',['init','-q',root]);
  const data=Buffer.alloc(2*1024*1024,65);
  const object=execFileSync('git',['hash-object','-w','--stdin'],{cwd:root,input:data,encoding:'utf8'}).trim();
  assert.equal(await gitBlobHash(root,object),sha256(data));
 }finally{await rm(root,{recursive:true,force:true});}
});

test('publication pins verification and tag to the captured commit and blocks checkout drift', async () => {
 const {writeFileSync,readFileSync}=await import('node:fs');
 const {sha256,manifestPath}=await import('../client/scripts/artifacts.mjs');
 const root=resolve(import.meta.dirname,'..');
 const manifestHash=sha256(readFileSync(resolve(root,manifestPath)));
 for(const drift of ['none','head','dirty']) {
  let head='verified-sha',dirty=false;const calls=[];
  const exec=(_root,cmd,args)=>{
   calls.push([cmd,...args]);
   if(args[0]==='status')return dirty?' M tracked':'';
   if(args[0]==='rev-parse')return head;
   if(args[0]==='tag'||args[0]==='ls-remote'||args[0]==='push')return '';
   if(cmd==='gh'&&args[1]==='view')return JSON.stringify({headSha:head,conclusion:'success',url:'https://github.com/run/1'});
   if(cmd==='gh'&&args[1]==='download') {writeFileSync(resolve(args[args.indexOf('--dir')+1],'evidence.json'),JSON.stringify({format:'formgen.release-evidence/v1',sourceCommit:head,manifestHash,status:'passed',provider:'github-actions',runUrl:'https://github.com/run/1'}));return '';}
   throw Error('Unexpected command');
  };
  const verifying=async(_root,ref)=>{assert.equal(ref,'verified-sha');if(drift==='head')head='other-sha';if(drift==='dirty')dirty=true;};
  if(drift==='none') {
   await publish(root,'123',exec,verifying);
   assert.ok(calls.some(c=>c[1]==='tag'&&c[2]==='-a'&&c[4]==='verified-sha'));
   assert.ok(calls.some(c=>c[1]==='push'&&c[3].startsWith('verified-sha:refs/tags/v')));
  }else{
   await assert.rejects(publish(root,'123',exec,verifying),/Candidate changed/);
   assert.equal(calls.some(c=>c[1]==='push'||c[1]==='tag'&&c[2]==='-a'),false);
  }
 }
});

test('fingerprints include imported hidden modules and hidden configuration while ignoring named caches',async()=>{
 const {buildInfo}=await import('../client/scripts/artifacts.mjs');
 const {build}=await import('../client/node_modules/esbuild/lib/main.js');
 const root=await mkdtemp(resolve(tmpdir(),'formgen-hidden-input-'));
 try{
  for(const dir of ['client/src','pkg','examples','scripts'])await mkdir(resolve(root,dir),{recursive:true});
  await writeFile(resolve(root,'.version'),'1.2.3');await writeFile(resolve(root,'.node-version'),process.versions.node);
  await writeFile(resolve(root,'client/package.json'),'{"version":"1.2.3"}');
  await writeFile(resolve(root,'client/package-lock.json'),'{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}');
  await writeFile(resolve(root,'client/src/index.ts'),"export {answer} from './.answer';");
  const hidden=resolve(root,'client/src/.answer.ts');await writeFile(hidden,'export const answer=1;');
  const bundle=async()=> (await build({entryPoints:[resolve(root,'client/src/index.ts')],bundle:true,write:false})).outputFiles[0].text;
  const before=await buildInfo(root), bytes=await bundle();await writeFile(hidden,'export const answer=2;');
  assert.notEqual(await bundle(),bytes);assert.notEqual((await buildInfo(root)).fingerprint,before.fingerprint);
  const changed=await buildInfo(root);await writeFile(resolve(root,'client/.browserslistrc'),'defaults');
  assert.notEqual((await buildInfo(root)).fingerprint,changed.fingerprint);
  const hiddenConfig=await buildInfo(root);await writeFile(resolve(root,'client/src/.loader-input.txt'),'first');
  assert.notEqual((await buildInfo(root)).fingerprint,hiddenConfig.fingerprint);
  const loaded=await buildInfo(root);await writeFile(resolve(root,'client/src/.loader-input.txt'),'second');
  assert.notEqual((await buildInfo(root)).fingerprint,loaded.fingerprint);
  const configured=await buildInfo(root);await mkdir(resolve(root,'examples/.gomodcache'),{recursive:true});
  await writeFile(resolve(root,'examples/.gomodcache/noise.go'),'noise');
  assert.equal((await buildInfo(root)).fingerprint,configured.fingerprint);
  await mkdir(resolve(root,'client/dev'));
  const stylesheet=resolve(root,'client/dev/formgen-vanilla.css');
  await symlink('../../pkg/renderers/vanilla/assets/formgen-vanilla.css',stylesheet);
  assert.equal((await buildInfo(root)).fingerprint,configured.fingerprint);
  await rm(stylesheet);await symlink('../../outside.css',stylesheet);
  await assert.rejects(buildInfo(root),/Symbolic input paths/);
  await rm(stylesheet);await symlink('index.ts',resolve(root,'client/src/alias.ts'));
  await assert.rejects(buildInfo(root),/Symbolic input paths/);
 }finally{await rm(root,{recursive:true,force:true});}
});

test('publication validates and pushes the same captured tag object',async()=>{
 const {writeFileSync,readFileSync}=await import('node:fs');
 const {sha256,manifestPath}=await import('../client/scripts/artifacts.mjs');
 const root=resolve(import.meta.dirname,'..'),manifestHash=sha256(readFileSync(resolve(root,manifestPath)));
 for(const wrongObject of [false,true]){
  const calls=[];
  const execute=(_root,cmd,args)=>{
   calls.push([cmd,...args]);
   if(cmd==='git'){
    if(args[0]==='rev-parse'){
     if(args[1]==='HEAD')return 'verified-commit';
     if(args[1].startsWith('refs/tags/'))return args[1].endsWith('^{commit}')?'verified-commit':'captured-tag-object';
     if(args[1]==='captured-tag-object^{commit}')return wrongObject?'other-commit':'verified-commit';
     throw Error('Unexpected revision');
    }
    return '';
   }
   if(args[1]==='view')return JSON.stringify({headSha:'verified-commit',conclusion:'success',url:'https://github.com/run/1'});
   writeFileSync(resolve(args[args.indexOf('--dir')+1],'evidence.json'),JSON.stringify({format:'formgen.release-evidence/v1',sourceCommit:'verified-commit',manifestHash,status:'passed',provider:'github-actions',runUrl:'https://github.com/run/1'}));return '';
  };
  if(wrongObject){
   await assert.rejects(publish(root,'123',execute,async()=>{}),/tag target differs/);
   assert.equal(calls.some(call=>call[1]==='push'),false);
  }else{
   await publish(root,'123',execute,async()=>{});
   assert.ok(calls.some(call=>call[1]==='push'&&call[3].startsWith('captured-tag-object:refs/tags/v')));
  }
 }
});

// A complete, tiny candidate exercises real Git archives, npm packaging and retained files.
async function releaseFixture() {

 const {copies,buildInfo,writeManifest}=await import('../client/scripts/artifacts.mjs');
 const root=await mkdtemp(resolve(tmpdir(),'formgen-final-artifacts-'));
 for(const dir of ['client/src','pkg','examples','scripts','testdata/embedded','client/dist/esm','client/dist/types','client/dist/browser/frameworks'])await mkdir(resolve(root,dir),{recursive:true});
 for(const dir of ['examples','scripts'])await writeFile(resolve(root,dir,'.keep'),'');
 await writeFile(resolve(root,'.version'),'1.2.3');await writeFile(resolve(root,'.node-version'),process.versions.node);
 const entries={'.':'index','./behaviors':'behaviors/index','./frameworks/preact':'frameworks/preact'};
 const exports=Object.fromEntries(Object.entries(entries).map(([key,path])=>[key,{import:`./dist/esm/${path}.js`,types:`./dist/types/${path}.d.ts`}]));
 await writeFile(resolve(root,'client/package.json'),JSON.stringify({name:'fixture-runtime',version:'1.2.3',type:'module',exports,module:'./dist/esm/index.js',browser:'./dist/browser/formgen-relationships.min.js',types:'./dist/types/index.d.ts',files:['dist','runtime-manifest.json']}));
 await writeFile(resolve(root,'client/package-lock.json'),'{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}');
 const info=await buildInfo(root);
 for(const path of Object.values(entries))for(const ext of ['esm/'+path+'.js','types/'+path+'.d.ts']) {
  const target=resolve(root,'client/dist',ext);await mkdir(dirname(target),{recursive:true});
  await writeFile(target,`export const RUNTIME_VERSION=${JSON.stringify(info.version)};export const RUNTIME_BUILD=${JSON.stringify(info.fingerprint)};`);
 }
 for(const [source,targets] of Object.entries(copies)) {
  const code=source.endsWith('.js')?`var ${source.includes('behaviors')?'FormgenBehaviors':'FormgenRelationships'}={RUNTIME_VERSION:'${info.version}',RUNTIME_BUILD:'${info.fingerprint}'};console.info('version ${info.version} build ${info.fingerprint.slice(0,12)}');`:'output';
  for(const path of [source,...targets]){await mkdir(dirname(resolve(root,path)),{recursive:true});await writeFile(resolve(root,path),code);}
 }
 await writeFile(resolve(root,'client/dist/browser/frameworks/preact.min.js'),`var FormgenRelationshipsPreact={RUNTIME_VERSION:'${info.version}',RUNTIME_BUILD:'${info.fingerprint}'};console.info('version ${info.version} build ${info.fingerprint.slice(0,12)}');`);
 for(const name of ['v1','definition-shapes','portable-definitions','authoring'])await writeFile(resolve(root,`testdata/embedded/${name}.json`),'{}');
 await writeManifest(root);
 run(root,'git',['init','-q']);run(root,'git',['add','.']);run(root,'git',['-c','user.name=Fixture','-c','user.email=fixture@example.test','commit','-qm','candidate']);
 return root;
}

test('passing test and packaging stages cannot silently change the verified candidate',async()=>{
 const {appendFileSync}=await import('node:fs');
 const {access}=await import('node:fs/promises');
 for(const mutation of ['client-test','release-test','go-test','tagged-go','pack','manifest','fixture']) {
  const root=await releaseFixture();
  try{
   await mkdir(resolve(root,'.tmp/release'),{recursive:true});
   await writeFile(resolve(root,'.tmp/release/evidence.json'),'{"status":"passed","sourceCommit":"old"}');
   let changed=false;
   const exec=(cwd,cmd,args,capture)=>{
    const stage=cmd==='npm'&&args[0]==='test'?'client-test':cmd==='node'?'release-test':cmd==='go'?(args.includes('-tags')?'tagged-go':'go-test'):cmd==='npm'&&args[0]==='pack'?'pack':null;
    if(stage===mutation || ['manifest','fixture'].includes(mutation)&&stage==='client-test') {
     appendFileSync(resolve(cwd,mutation==='fixture'?'../testdata/embedded/v1.json':mutation==='manifest'?'runtime-manifest.json':cmd==='npm'?'dist/esm/index.js':'client/dist/esm/index.js'),mutation==='manifest'?' ':'\n// mutation after passing stage\n');changed=true;
    }
    if(cmd==='npm'&&args[0]==='pack')return run(cwd,cmd,args,capture);
    if(cmd==='npm'||cmd==='go'||cmd==='node')return '';
    return run(cwd,cmd,args,capture);
   };
   await assert.rejects(verify(root,'HEAD',exec),/Artifact manifest differs|Candidate manifest changed|Candidate fixture changed/);
   assert.equal(changed,true,mutation);
   await assert.rejects(access(resolve(root,'.tmp/release/evidence.json')));
  }finally{await rm(root,{recursive:true,force:true});}
 }
});

test('packed inventory and retained output bytes must match before passing evidence is exposed',async()=>{
 const {readFile,appendFile,access}=await import('node:fs/promises');
 const {checkPackage,verifyPackageContents}=await import('./check-package.mjs');
 const {retainEvidence}=await import('./release.mjs');
 const {manifestPath}=await import('../client/scripts/artifacts.mjs');
 const root=await releaseFixture();
 try{
  const bytes=await readFile(resolve(root,manifestPath),'utf8'),manifest=JSON.parse(bytes);
  const output=resolve(root,'client/dist/esm/index.js');
  await verifyPackageContents(resolve(root,'client'),manifest);
  await appendFile(output,'\n// corrupt packed bytes\n');
  await assert.rejects(verifyPackageContents(resolve(root,'client'),manifest),/Packed artifacts differ/);
  const corruptPack=JSON.parse(run(resolve(root,'client'),'npm',['pack','--json','--pack-destination',root],true))[0];
  await assert.rejects(checkPackage(resolve(root,corruptPack.filename),manifest.version,manifest.fingerprint,bytes),/Packed artifacts differ/);
  run(root,'git',['restore','client/dist/esm/index.js']);
  const extra=resolve(root,'client/dist/unexpected.js');await writeFile(extra,'unexpected');
  await assert.rejects(verifyPackageContents(resolve(root,'client'),manifest),/Packed artifacts differ/);await rm(extra);
  const pack=JSON.parse(run(resolve(root,'client'),'npm',['pack','--json','--pack-destination',root],true))[0];
  const tarball=resolve(root,pack.filename);await checkPackage(tarball,manifest.version,manifest.fingerprint,bytes);
  await assert.rejects(checkPackage(tarball,manifest.version,manifest.fingerprint,bytes+' '),/Packed manifest differs/);
  await appendFile(output,'\n// changed after packaging\n');
  await assert.rejects(retainEvidence(root,root,tarball,bytes,{status:'passed'}),/Retained artifact differs/);
  await assert.rejects(access(resolve(root,'.tmp/release/evidence.json')));
  run(root,'git',['restore','client/dist/esm/index.js']);
  await retainEvidence(root,root,tarball,bytes,{status:'passed'});
  assert.equal(JSON.parse(await readFile(resolve(root,'.tmp/release/evidence.json'))).status,'passed');
  await verifyPackageContents(resolve(root,'.tmp/release/artifacts/client'),manifest);
 }finally{await rm(root,{recursive:true,force:true});}
});

test('manual version selection preserves patch/minor/major and explicit versions', () => {
 assert.equal(releaseVersion('0.34.1'),'0.34.2');
 assert.equal(releaseVersion('0.34.1','minor'),'0.35.0');
 assert.equal(releaseVersion('0.34.1','major'),'1.0.0');
 assert.equal(releaseVersion('1.3.0-rc.1','minor'),'1.3.0');
 assert.equal(releaseVersion('1.3.1-rc.1','patch'),'1.3.1');
 assert.equal(releaseVersion('1.3.1-rc.1','minor'),'1.4.0');
 assert.equal(releaseVersion('0.34.1','0.34.1'),'0.34.1');
 assert.equal(releaseVersion('999999999999999999.2.3','major'),'1000000000000000000.0.0');
 assert.throws(()=>releaseVersion('0.34.1','invalid'),/Use release/);
});

// Only these disposable repositories receive commits/tags/pushes. Production
// preparation and exact verification are already exercised above; these stage
// doubles exercise the orchestration and real Git publication transaction.
async function manualFixture() {
 const {copies,manifestPath,sha256}=await import('../client/scripts/artifacts.mjs');
 const {writeFileSync}=await import('node:fs');
 const parent=await mkdtemp(resolve(tmpdir(),'formgen-manual-'));
 const root=resolve(parent,'source'),remote=resolve(parent,'origin.git');
 await mkdir(resolve(root,'client'),{recursive:true});
 await writeFile(resolve(root,'.version'),'1.2.3\n');
 await writeFile(resolve(root,'client/package.json'),'{"version":"1.2.3"}');
 await writeFile(resolve(root,'client/package-lock.json'),'{"version":"1.2.3","packages":{"":{"version":"1.2.3"}}}');
 await writeFile(resolve(root,manifestPath),'{"version":"1.2.3"}');
 for(const path of Object.values(copies).flat()){await mkdir(dirname(resolve(root,path)),{recursive:true});await writeFile(resolve(root,path),'original');}
 await writeFile(resolve(root,'CHANGELOG.md'),'original changelog');
 await writeFile(resolve(root,'README.md'),'original source');
 run(root,'git',['init','-q','-b','main']);
 run(root,'git',['config','user.name','Fixture']);run(root,'git',['config','user.email','fixture@example.test']);
 run(root,'git',['add','.']);run(root,'git',['commit','-qm','initial']);
 run(parent,'git',['init','-q','--bare',remote]);run(root,'git',['remote','add','origin',remote]);
 run(root,'git',['push','-q','origin','main']);
 const initial=run(root,'git',['rev-parse','HEAD'],true),calls=[];
 const execute=(cwd,cmd,args,capture)=>{
  calls.push([cmd,...args]);
  if(cmd==='git'&&args[0]==='cliff'){
   if(args.includes('--output'))writeFileSync(resolve(cwd,'CHANGELOG.md'),`Changelog for ${args[args.indexOf('--tag')+1]}`);
   return 'git-cliff fixture';
  }
  return run(cwd,cmd,args,capture);
 };
 const preparing=async(cwd,version)=>{
  await writeFile(resolve(cwd,'.version'),version+'\n');
  await writeFile(resolve(cwd,'client/package.json'),JSON.stringify({version}));
  await writeFile(resolve(cwd,'client/package-lock.json'),JSON.stringify({version,packages:{'':{version}}}));
  await writeFile(resolve(cwd,manifestPath),JSON.stringify({version}));
 };
 const verifying=async(cwd,commit)=>{
  calls.push(['verify',commit]);
  assert.match(run(cwd,'git',['show',`${commit}:CHANGELOG.md`],true),/Changelog for v/);
  return {format:'formgen.release-evidence/v1',sourceCommit:commit,manifestHash:sha256(await readFile(resolve(cwd,manifestPath))),status:'passed',provider:'local'};
 };
 const noPublication=()=>assert.equal(calls.some(c=>c[0]==='git'&&(c[1]==='push'||c[1]==='tag'&&c[2]==='-a')),false);
 return {parent,root,remote,initial,calls,execute,preparing,verifying,noPublication};
}

test('manual minor release commits synchronized files and atomically pushes only its branch/tag',async()=>{
 const {readFile}=await import('node:fs/promises');
 const f=await manualFixture();
 try{
  await writeFile(resolve(f.root,'private.txt'),'unrelated');
  run(f.root,'git',['tag','v0.9.0']);
  const evidence=await manualRelease(f.root,'minor','My release',f.execute,f.preparing,f.verifying);
  const head=run(f.root,'git',['rev-parse','HEAD'],true);
  assert.equal((await versionInfo(f.root)).version,'1.3.0');assert.equal(evidence.sourceCommit,head);assert.equal(evidence.provider,'local');
  assert.equal(run(f.remote,'git',['rev-parse','refs/heads/main'],true),head);
  assert.equal(run(f.remote,'git',['rev-parse','v1.3.0^{commit}'],true),head);
  assert.equal(run(f.remote,'git',['tag','--list'],true),'v1.3.0');
  assert.equal(run(f.remote,'git',['for-each-ref','--format=%(contents:subject)','refs/tags/v1.3.0'],true),'My release');
  assert.equal(run(f.root,'git',['ls-files','private.txt'],true),'');assert.equal(await readFile(resolve(f.root,'private.txt'),'utf8'),'unrelated');
  assert.equal(run(f.root,'git',['status','--porcelain','--untracked-files=no'],true),'');
  const pushed=f.calls.find(c=>c[0]==='git'&&c[1]==='push');
  assert.deepEqual(pushed.slice(0,4),['git','push','--atomic','origin']);assert.equal(pushed.length,6);
  assert.equal(f.calls.some(c=>c[0]==='gh'),false);
 }finally{await rm(f.parent,{recursive:true,force:true});}
});

test('manual failures cannot tag or push before successful exact-candidate verification',async()=>{
 for(const failure of ['dirty','prepare','changelog','unexpected','verify','evidence','drift']){
  const f=await manualFixture();
  try{
   if(failure==='dirty')await writeFile(resolve(f.root,'README.md'),'dirty');
   const preparing=async(...args)=>{if(failure==='prepare')throw Error('injected prepare');await f.preparing(...args);if(failure==='unexpected')await writeFile(resolve(f.root,'README.md'),'unexpected source mutation');};
   const execute=(cwd,cmd,args,capture)=>{if(failure==='changelog'&&cmd==='git'&&args[0]==='cliff'&&args.includes('--output'))throw Error('injected changelog');return f.execute(cwd,cmd,args,capture);};
   const verifying=async(...args)=>{
    if(failure==='verify')throw Error('injected verify');
    const evidence=await f.verifying(...args);
    if(failure==='evidence')evidence.sourceCommit='wrong';
    if(failure==='drift')run(f.root,'git',['commit','--allow-empty','-qm','concurrent commit']);
    return evidence;
   };
   await assert.rejects(manualRelease(f.root,'minor',undefined,execute,preparing,verifying));f.noPublication();
   assert.equal(run(f.remote,'git',['rev-parse','refs/heads/main'],true),f.initial);
   assert.equal(run(f.remote,'git',['tag','--list'],true),'');
  }finally{await rm(f.parent,{recursive:true,force:true});}
 }
});

test('manual resume retries a failed atomic push without another bump or commit',async()=>{
 const f=await manualFixture();
 try{
  const execute=(cwd,cmd,args,capture)=>{if(cmd==='git'&&args[0]==='push')throw Error('injected push');return f.execute(cwd,cmd,args,capture);};
  await assert.rejects(manualRelease(f.root,'minor',undefined,execute,f.preparing,f.verifying),/release --resume/);
  const head=run(f.root,'git',['rev-parse','HEAD'],true);
  assert.equal(run(f.remote,'git',['rev-parse','refs/heads/main'],true),f.initial);
  assert.equal(run(f.remote,'git',['tag','--list'],true),'');
  await manualRelease(f.root,'--resume',undefined,f.execute,()=>{throw Error('must not prepare again');},f.verifying);
  assert.equal(run(f.root,'git',['rev-parse','HEAD'],true),head);
  assert.equal(run(f.remote,'git',['rev-parse','v1.3.0^{commit}'],true),head);
 }finally{await rm(f.parent,{recursive:true,force:true});}
});

test('atomic manual push rejects a diverged remote branch without publishing its tag',async()=>{
 const f=await manualFixture();
 try{
  const verifying=async(...args)=>{
   const evidence=await f.verifying(...args),peer=resolve(f.parent,'peer');
   run(f.parent,'git',['clone','-q','--branch','main',f.remote,peer]);
   run(peer,'git',['-c','user.name=Peer','-c','user.email=peer@example.test','commit','--allow-empty','-qm','remote advancement']);
   run(peer,'git',['push','-q','origin','main']);
   return evidence;
  };
  await assert.rejects(manualRelease(f.root,'minor',undefined,f.execute,f.preparing,verifying),/release --resume/);
  assert.equal(run(f.remote,'git',['tag','--list'],true),'');
  assert.notEqual(run(f.remote,'git',['rev-parse','refs/heads/main'],true),run(f.root,'git',['rev-parse','HEAD'],true));
 }finally{await rm(f.parent,{recursive:true,force:true});}
});

test('manual preflight preserves existing tags and rejects detached or invalid releases',async()=>{
 for(const kind of ['detached','local-tag','remote-tag','invalid']){
  const f=await manualFixture();
  try{
   if(kind==='detached')run(f.root,'git',['checkout','--detach','-q']);
   if(kind==='local-tag'||kind==='remote-tag')run(f.root,'git',['tag','-a','v1.3.0','-m','existing']);
   if(kind==='remote-tag')run(f.root,'git',['push','-q','origin','v1.3.0']);
   await assert.rejects(manualRelease(f.root,kind==='invalid'?'typo':'minor',undefined,f.execute,()=>{throw Error('must not prepare');},f.verifying));
   f.noPublication();assert.equal((await versionInfo(f.root)).version,'1.2.3');
   assert.equal(run(f.root,'git',['rev-parse','HEAD'],true),f.initial);
   if(kind.endsWith('tag'))assert.equal(run(f.root,'git',['rev-parse','v1.3.0^{commit}'],true),f.initial);
  }finally{await rm(f.parent,{recursive:true,force:true});}
 }
});
