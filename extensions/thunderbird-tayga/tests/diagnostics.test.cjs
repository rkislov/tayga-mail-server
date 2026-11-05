const {test} = require('node:test');
const assert = require('node:assert/strict');
const {Journal} = require('../diagnostics.js');
const key='diagnostics:v1';
function harness(initial={}, options={}) {
  const data=structuredClone(initial);
  const storage={get:async(name)=>structuredClone({[name]:data[name]}),set:async(values)=>Object.assign(data,structuredClone(values))};
  const journal=new Journal(storage,{now:()=>Date.parse('2026-11-04T12:00:00Z'),...options});
  return {data,storage,journal};
}
const secrets=['password:SecretPassword!', 'Bearer secret-access-token', 'secret-refresh-token', 'MFA-123456', 'alice@example.test', 'private-subject', 'confidential-file.pdf', 'https://mail.example.test/s/private-public-link', '/Users/private/profile', 'private-invitation-id'];
test('journal storage and export use a strict allowlist, not free-form message redaction',async()=>{
  const h=harness({token:'secret-access-token',refreshToken:'secret-refresh-token'});
  const payload=secrets.join(' ');
  await h.journal.record('sync','failure',{
    error:{name:'TypeError',status:503,code:payload,message:payload,stack:payload,cause:{password:payload}},
    message:payload,body:payload,headers:{Authorization:payload},url:payload,path:payload,email:payload,filename:payload,
    reason:payload,phase:payload,protocolCode:payload,errorCategory:payload,httpStatus:payload,durationMs:payload,
  });
  await h.journal.record(payload,'success');
  await h.journal.record('startup',payload);
  const view=await h.journal.view();
  assert.deepEqual(view.entries,[{time:'2026-11-04T12:00:00.000Z',event:'sync',result:'failure',httpStatus:503,errorCategory:'network'}]);
  const exported=await h.journal.export('2.3.0','159.0');
  assert.equal(exported.thunderbirdVersion,'159.0');
  for(const value of [h.data[key],exported]) for(const secret of secrets) assert.equal(JSON.stringify(value).includes(secret),false,secret);
  assert.equal(exported.pluginVersion,'2.3.0');
  assert.equal(JSON.stringify(exported).includes('secret-access-token'),false);
});
test('old or tampered persisted entries are sanitized before display and rewritten in storage',async()=>{
  const now='2026-11-04T12:00:00.000Z';
  const h=harness({[key]:{enabled:true,entries:[
    {time:now,event:'startup',result:'success',password:secrets[0],message:secrets[1],email:secrets[4]},
    {time:now,event:secrets[2],result:'failure'},
    {time:'not-a-date '+secrets[3],event:'sync',result:'failure'},
    {time:'2026-10-01T12:00:00Z',event:'sync',result:'success'},
    {time:'2099-01-01T12:00:00Z',event:'sync',result:'success'},
  ]}});
  const view=await h.journal.view();
  assert.equal(view.entries.length,1);
  assert.deepEqual(Object.keys(view.entries[0]).sort(),['event','result','time']);
  for(const secret of secrets) assert.equal(JSON.stringify(h.data[key]).includes(secret),false);
});
test('password or token-bearing return values and exception messages are never recorded',async()=>{
  const h=harness();
  const result=await h.journal.observe('auth.refresh',async()=>({access_token:secrets[1],refresh_token:secrets[2],password:secrets[0]}));
  assert.equal(result.password,secrets[0]);
  const error={name:'TimeoutError',status:504,get message(){throw new Error('logger must not read messages')},stack:secrets[0]};
  await assert.rejects(h.journal.observe('upload',async()=>{throw error}),err=>err===error);
  const report=await h.journal.export('2.3.0\n'+secrets[0], '159.0\n'+secrets[1]);
  assert.equal(report.thunderbirdVersion,'unknown');
  assert.equal(report.pluginVersion,'unknown');
  assert.equal(report.entries.at(-1).errorCategory,'timeout');
  for(const secret of secrets) assert.equal(JSON.stringify(report).includes(secret),false);
});
test('bounded history persists across a restart and expires after seven days',async()=>{
  let now=Date.parse('2026-11-04T12:00:00Z');
  const h=harness({}, {now:()=>now,limit:3});
  for(let i=0;i<8;i++){now++;await h.journal.record('sync','success',{durationMs:i})}
  let view=await h.journal.view();
  assert.deepEqual(view.entries.map(entry=>entry.durationMs),[5,6,7]);
  const restarted=new Journal(h.storage,{now:()=>now,limit:3});
  assert.equal((await restarted.view()).entries.length,3);
  now+=8*86400000;
  view=await restarted.view();
  assert.equal(view.entries.length,0);assert.equal(h.data[key].entries.length,0);
});
test('disable persists across restart, while clearing is ordered after queued writes',async()=>{
  const h=harness();
  const write=h.journal.record('startup','success');
  const clear=h.journal.clear();
  await Promise.all([write,clear]);
  assert.equal((await h.journal.view()).entries.length,0);
  await h.journal.setEnabled(false);
  await h.journal.record('auth.login','success');
  const restarted=new Journal(h.storage,{now:()=>Date.parse('2026-11-04T12:00:00Z')});
  assert.equal((await restarted.view()).enabled,false);
  await restarted.record('sync','success');
  assert.equal((await restarted.view()).entries.length,0);
  await restarted.setEnabled(true);await restarted.record('sync','success');
  assert.equal((await restarted.view()).entries.length,1);
});
test('storage failure does not interrupt login, sync or uploads',async()=>{
  const journal=new Journal({get:async()=>{throw new Error(secrets[0])},set:async()=>{throw new Error(secrets[1])}});
  assert.equal(await journal.observe('sync',async()=>42),42);
  const view=await journal.view();assert.equal(view.healthy,false);
  for(const secret of secrets)assert.equal(JSON.stringify(view).includes(secret),false);
});
test('queued records cannot retain subsequently mutated private error objects',async()=>{
  const h=harness();const details={error:{name:'TypeError'},phase:'retrying'};
  const write=h.journal.record('upload.phase','failure',details);
  details.error.name=secrets[0];details.phase=secrets[1];
  await write;
  const entry=(await h.journal.view()).entries[0];
  assert.equal(entry.phase,'retrying');assert.equal(entry.errorCategory,'network');
});
