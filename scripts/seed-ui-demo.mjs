// Populate a disposable local server only. Never run against an existing mailbox.
import fs from 'node:fs';
const base=process.env.TMS_URL||'http://127.0.0.1:18080';
if(!['localhost','127.0.0.1','[::1]'].includes(new URL(base).hostname)||process.env.TMS_SEED_DEMO!=='1')throw Error('Use TMS_SEED_DEMO=1 with a disposable loopback server.');
async function request(path,token,body,method='GET'){
 const response=await fetch(base+path,{method,headers:{...(token?{Authorization:'Bearer '+token}:{}),...(body?{'Content-Type':'application/json'}:{})},...(body?{body:JSON.stringify(body)}:{})});
 const data=await response.json();if(!response.ok)throw Error(path+': '+JSON.stringify(data));return data;
}
const password=process.env.TMS_PASSWORD||'changeme';
async function login(email){const data=await request('/api/v1/auth/login','',{email,password},'POST');return data.access_token||data.tokens?.access_token;}
const token=await login('admin@example.com');if(!token)throw Error('No demo token');
const headers={Authorization:'Bearer '+token};
const users=[['anna','Анна Смирнова'],['mikhail','Михаил Петров'],['elena','Елена Волкова'],['design','Дизайн-команда'],['alexey','Алексей Иванов'],['maria','Мария Соколова']];
const existing=(await request('/api/v1/admin/users',token)).users||[];
for(const [local,name]of users){if(!existing.some(u=>u.email===local+'@example.com'))await request('/api/v1/admin/users',token,{email:local+'@example.com',password,display_name:name,quota_bytes:10737418240},'POST');}
const boxes=await request('/api/v1/mail/mailboxes',token);
const inbox=boxes.mailboxes.find(b=>/^inbox$/i.test(b.name));
if((await request('/api/v1/mail/mailboxes/'+inbox.id+'/messages',token)).messages.length===0){
 const subjects=['Встреча по проекту','Отчёт за неделю','Бюджет на следующий квартал','Новые макеты интерфейса','Согласование договора','План команды'];
 for(let i=users.length-1;i>=0;i--){const [local]=users[i],sender=await login(local+'@example.com');await request('/api/v1/mail/send',sender,{to:['admin@example.com'],subject:subjects[i],text:'Роман, привет!\n\nДавай обсудим новый интерфейс в четверг в 14:00. Подготовили материалы к встрече и предложения по улучшению навигации.\n\nБуду рада обратной связи.\n'+users[i][1],html:'<p>Роман, привет!</p><p>Давай обсудим новый интерфейс в четверг в <b>14:00</b>. Подготовили материалы к встрече и предложения по улучшению навигации.</p><p>Буду рада обратной связи.</p><p>'+users[i][1]+'</p>'},'POST');}
}
const books=(await request('/api/v1/contacts/books',token)).books;
const book=books.find(b=>b.id!=='gal');
if(!(await request('/api/v1/contacts/books/'+book.id+'/cards',token)).cards.length){for(const [local,name]of users.slice(0,3))await request('/api/v1/contacts/books/'+book.id+'/cards',token,{fn:name,email:local+'@example.com',tel:'+7 999 123-45-67',org:'Тайга',note:'Команда проекта Tayga Mail'},'POST');}
const calendars=(await request('/api/v1/calendar/calendars',token)).calendars;
const calendar=calendars.find(c=>c.name==='default')||calendars[0];
if(!(await request('/api/v1/calendar/calendars/'+calendar.id+'/events',token)).events.length){for(const [i,title]of ['Обсуждение интерфейса','Планирование команды','Демонстрация новой версии'].entries()){const start=new Date();start.setDate(start.getDate()+i);start.setHours(14,0,0,0);await request('/api/v1/calendar/calendars/'+calendar.id+'/events',token,{summary:title,description:'Обсудим новые макеты, приоритеты и следующий этап работы.',location:'Переговорная «Тайга»',timezone:'Europe/Moscow',start:start.toISOString(),end:new Date(+start+3600000).toISOString(),attendees:[]},'POST');}}
if(!(await request('/api/v1/tasks',token)).tasks?.length){for(const [i,title]of ['Посмотреть макеты интерфейса','Подготовить материалы к встрече','Обновить документацию','Согласовать план релиза'].entries())await request('/api/v1/tasks',token,{title,body:'Проверить материалы проекта и подготовить предложения для команды.',status:i===0?'InProgress':'NotStarted',percent_complete:i===0?40:0,due:new Date(Date.now()+i*86400000).toISOString()},'POST');}
const folders=(await request('/api/v1/notes/folders',token)).folders;const folder=folders.find(f=>!f.shared)||folders[0];
if(folder&&!(await request('/api/v1/notes?folder_id='+folder.id,token)).notes?.length){for(const title of ['Идеи для нового интерфейса','План релиза 1.0.0','Заметки со встречи'])await request('/api/v1/notes',token,{folder_id:folder.id,title,body_html:'<h2>Удобное рабочее пространство</h2><p>Собрали предложения команды по развитию Tayga Mail.</p><ul><li>Понятная навигация между разделами</li><li>Заметные основные действия</li><li>Удобная работа с вложениями</li></ul><p>Следующий шаг — проверить интерфейс на компьютере и телефоне.</p>'},'POST');}
for(const name of ['Дизайн','Документы']){try{await request('/api/v1/files/mkdir',token,{path:name},'POST')}catch(error){if(!/exists/i.test(error.message))throw error}}
for(const [name,body]of [['План релиза.txt',Buffer.from('Tayga Mail 1.0.0\nНовое рабочее пространство\nДокументация и скриншоты\n')],['Макеты интерфейса.pdf',fs.readFileSync('scripts/fixtures/preview.pdf')]]){const response=await fetch(base+'/api/v1/files/content?path='+encodeURIComponent(name),{method:'PUT',headers:{...headers,'Content-Type':'application/octet-stream'},body});if(!response.ok)throw Error('Demo file: '+await response.text());}
const teams=await request('/api/v4/users/me/teams',token);const team=teams[0];
if(team){const channels=await request('/api/v4/users/me/teams/'+team.id+'/channels',token);let channel=channels.find(c=>c.name==='interface');if(!channel)channel=await request('/api/v4/channels',token,{team_id:team.id,name:'interface',display_name:'Интерфейс',type:'O'},'POST');const posts=await request('/api/v4/channels/'+channel.id+'/posts',token);if(!posts.order?.length){for(const message of ['Команда, привет! Здесь обсуждаем интерфейс Tayga Mail.','Новые макеты готовы. Обратите внимание на навигацию и компактное меню.','Соберём обратную связь и обсудим её на встрече.'])await request('/api/v4/posts',token,{channel_id:channel.id,message},'POST');}}
console.log('Demo content created on the disposable local server.');
