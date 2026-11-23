const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const {JSDOM}=require('jsdom');
function setup(){
 const dom=new JSDOM('<div id="app-mattermost"></div>',{runScripts:'outside-only',url:'https://mail.test'}),w=dom.window;
 const scope={tenant_id:'tenant-one',domain_id:'domain-one'},requests=[];
 Object.assign(w,{$:id=>w.document.getElementById(id),mmAdmin:{selected:scope,epoch:1},state:{me:{id:'admin'}},escapeHtml:x=>String(x).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c])),setMsg:(n,m)=>{if(n)n.textContent=m},mmQuery:s=>'?'+new URLSearchParams(s)});
 w.mmCurrentGuard=(epoch,account)=>()=>{if(epoch!==w.mmAdmin.epoch||account!==w.state.me?.id)throw new Error('stale scope')};
 const data={peers:[{id:'peer-one',name:'<img src=x onerror=alert(1)>',enabled:true,confirmed:true,mode:'native'}]};
 w.api=async(route,options={})=>{options._guard?.();requests.push({route,options});let result=options.method==='POST'?{peer:{id:'new-peer'},invite_code:'encrypted-invitation'}:data;if(options.method==='DELETE')result={ok:true};options._guard?.();return result};
 const source=fs.readFileSync(require.resolve('../src/mattermost-admin.js'),'utf8').split('/* Federation shares only channels explicitly selected in the current domain. */')[1];w.eval(source);
 return {w,dom,requests,data};
}
test('federation escapes remote labels and erases passwords and invitation on logout',async()=>{
 const {w,requests}=setup();await w.loadMattermostFederation();assert.equal(w.document.querySelectorAll('img').length,0);
 const form=w.document.getElementById('mm-fed-create');form.elements.name.value='partner';form.elements.site_url.value='https://chat.test';form.elements.password.value='never-render-this-password';
 await form.onsubmit({preventDefault(){},currentTarget:form});
 assert.equal(requests.find(r=>r.options.method==='POST').route,'/api/v1/admin/mattermost/federation?tenant_id=tenant-one&domain_id=domain-one');
 assert.equal(w.document.getElementById('mm-fed-result').querySelector('textarea').value,'encrypted-invitation');
 assert.equal(w.document.getElementById('mm-fed-create').elements.password.value,'');
 w.dispatchEvent(new w.Event('tms-session-ended'));assert.equal(w.document.getElementById('mm-federation').textContent,'');
});
test('late federation response cannot populate a different domain',async()=>{
 const {w}=setup();let resolve;w.api=async(route,options)=>{options._guard();const result=await new Promise(r=>resolve=r);options._guard();return result};
 const loading=w.loadMattermostFederation();w.mmAdmin.selected={tenant_id:'other',domain_id:'another'};w.mmAdmin.epoch++;
 resolve({peers:[{id:'old',name:'PRIVATE OLD DOMAIN',enabled:true}]});await loading;
 assert.equal(w.document.body.textContent.includes('PRIVATE OLD DOMAIN'),false);
});
