const {test} = require('node:test');
const assert = require('node:assert/strict');
const {Service} = require('../invites.js');
function harness() {
  const cfg = {baseUrl:'https://mail.example.test',email:'alice@example.test',token:'access'};
  const entry = {id:'invite',etag:'version-1',partstat:'NEEDS-ACTION',summary:'Meeting',can_reply:true};
  const calls = [];
  const client = {
    generation:0,
    settings:async()=>({...cfg}),
    json:(body)=>({method:'POST',body:JSON.stringify(body)}),
    api:async(path, opts={})=>{
      calls.push({path,...opts});
      if (opts.method!=='POST') return {invites:[{...entry}]};
      entry.partstat={accept:'ACCEPTED',tentative:'TENTATIVE',decline:'DECLINED'}[JSON.parse(opts.body).action];
      entry.etag+='-updated'; entry.proposed_start=null;
      return {id:entry.id,partstat:entry.partstat};
    },
  };
  const service = new Service(client);
  const request = (action='accept')=>({scope:service.scope(cfg),generation:client.generation,id:entry.id,etag:entry.etag,action});
  return {cfg,entry,calls,client,service,request};
}
for (const [action,status] of [['accept','ACCEPTED'],['tentative','TENTATIVE'],['decline','DECLINED']]) {
  test(`invitation ${action} posts the current ETag through the shared session`,async()=>{
    const h=harness(); const before=h.entry.etag;
    const data=await h.service.load();
    assert.equal(data.scope,h.service.scope(h.cfg)); assert.equal(data.generation,0);
    const result=await h.service.reply(h.request(action));
    assert.equal(result.partstat,status);
    const writes=h.calls.filter(call=>call.method==='POST');
    assert.equal(writes.length,1);
    assert.equal(writes[0].path,'/calendar/invites/invite/reply');
    assert.deepEqual(JSON.parse(writes[0].body),{action,etag:before});
    assert.equal(h.calls.some(call=>call.path.includes('auth/login')),false);
  });
}
test('stale, cancelled and unavailable invitations never send a reply',async()=>{
  for (const mode of ['stale','cancelled','unavailable']) {
    const h=harness(),request=h.request();
    if(mode==='stale')h.entry.etag='new-version';
    if(mode==='cancelled')h.entry.cancelled=true;
    if(mode==='unavailable')h.entry.can_reply=false;
    await assert.rejects(h.service.reply(request),/изменилось или отменено/);
    assert.equal(h.calls.some(call=>call.method==='POST'),false);
  }
});
test('account switch and same-account re-login invalidate the visible invitation snapshot',async()=>{
  for(const mode of ['account','generation']) {
    const h=harness(),request=h.request();
    if(mode==='account')h.cfg.email='bob@example.test';else h.client.generation++;
    await assert.rejects(h.service.reply(request),/Учётная запись изменилась/);
    assert.equal(h.calls.length,0);
  }
});
test('double click and concurrent tabs cannot submit two answers',async()=>{
  const h=harness(),api=h.client.api;
  let release;
  h.client.api=async(...args)=>{await new Promise(resolve=>{release=resolve});return api(...args)};
  const first=h.service.reply(h.request());
  while(!release)await new Promise(resolve=>setImmediate(resolve));
  await assert.rejects(h.service.reply(h.request()),/предыдущего ответа/);
  h.client.api=api; release();
  await first;
  assert.equal(h.calls.filter(call=>call.method==='POST').length,1);
});
test('a lost reply response is not replayed; reload recognizes the saved answer without a second mail',async()=>{
  const h=harness(),api=h.client.api;
  h.client.api=async(path,opts)=>{
    const result=await api(path,opts);
    if(opts?.method==='POST')throw new TypeError('Lost response after commit');
    return result;
  };
  await assert.rejects(h.service.reply(h.request()),/Lost response/);
  h.client.api=api;
  await h.service.load();
  const result=await h.service.reply(h.request());
  assert.equal(result.already,true);
  assert.equal(h.calls.filter(call=>call.method==='POST').length,1);
});
test('already answered invitations do not resend mail, but a time proposal requires an explicit new response',async()=>{
  const h=harness(); h.entry.partstat='TENTATIVE';
  assert.equal((await h.service.reply(h.request('tentative'))).already,true);
  h.entry.proposed_start='2026-11-10T09:00:00Z';
  await h.service.reply(h.request('tentative'));
  assert.equal(h.calls.filter(call=>call.method==='POST').length,1);
});
test('missing versions and unknown actions are rejected before any request',async()=>{
  for(const change of [{etag:''},{action:'counter'},{action:'__proto__'},{action:'constructor'},{id:''}]) {
    const h=harness();
    await assert.rejects(h.service.reply({...h.request(),...change}));
    assert.equal(h.calls.length,0);
  }
});
test('a late invitation list is discarded after logout',async()=>{
  const h=harness(),api=h.client.api;
  h.client.api=async(...args)=>{h.cfg.token='';return api(...args)};
  await assert.rejects(h.service.load(),/Учётная запись изменилась/);
  const result=await h.service.load();
  assert.equal(result.signedIn,false);assert.deepEqual(result.invites,[]);
});

test('an account switch during preflight is rejected before the POST',async()=>{
  const h=harness(),api=h.client.api;
  h.client.api=async(...args)=>{const result=await api(...args);h.cfg.email='other@example.test';return result};
  await assert.rejects(h.service.reply(h.request()),/Учётная запись изменилась/);
  assert.equal(h.calls.some(call=>call.method==='POST'),false);
});
test('a success-shaped but unconfirmed server response is not reported as a saved answer',async()=>{
  const h=harness(),api=h.client.api;
  h.client.api=async(path,opts)=>opts?.method==='POST'?{}:api(path,opts);
  await assert.rejects(h.service.reply(h.request()),/не подтвердил ответ/);
});
