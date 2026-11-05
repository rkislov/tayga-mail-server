// Frame playback uses office-rendered key states, never code from the document.
// A deterministic 500 ms pause advances click-triggered effects for both versions.
function fileMotionSchedule(slide,previous,events={}) {
  if(slide?.layers?.length)return fileMotionLayerSchedule(slide,previous,events);
  if(!slide)return {segments:[],duration:0,initial:0};
  if(slide.warning)return {segments:[],duration:500,initial:slide.states[0]};
  const segments=[];let cursor=0,current=slide.states[0];
  if(slide.transition.kind!=='cut'&&slide.transition.duration>0){segments.push({start:0,end:slide.transition.duration,from:previous?.states.at(-1)||0,to:current,kind:slide.transition.kind,direction:slide.transition.direction});cursor=slide.transition.duration}
  for(let index=0;index<slide.effects.length;index++){
    const effect=slide.effects[index];cursor+=effect.delay+(effect.trigger==='click'?500:0);
    segments.push({start:cursor,end:cursor+Math.max(1,effect.duration),from:current,to:slide.states[index+1],kind:effect.kind});cursor+=Math.max(1,effect.duration);current=slide.states[index+1];
  }
  return {segments,duration:cursor+500,initial:slide.states[0]};
}
function fileMotionFrame(ctx,cache,schedule,time) {
  if(schedule.layers){fileMotionLayerFrame(ctx,cache,schedule,time);return}
  const width=ctx.canvas.width,height=ctx.canvas.height;ctx.globalAlpha=1;ctx.clearRect(0,0,width,height);ctx.fillStyle='white';ctx.fillRect(0,0,width,height);
  let frame=schedule.initial,active=null;
  for(const segment of schedule.segments){if(time<segment.start)break;if(time>=segment.end){frame=segment.to;continue}active=segment;break}
  const draw=(id,x=0,y=0)=>{if(cache.has(id))ctx.drawImage(cache.get(id),x,y)};
  if(!active){draw(frame);return}
  const p=Math.max(0,Math.min(1,(time-active.start)/(active.end-active.start)));draw(active.from);
  if(active.kind==='fade'){ctx.globalAlpha=p;draw(active.to);ctx.globalAlpha=1;return}
  if(active.kind==='wipe'){
    ctx.save();ctx.beginPath();const direction=active.direction||'l';
    if(direction==='l')ctx.rect(width*(1-p),0,width*p,height);if(direction==='r')ctx.rect(0,0,width*p,height);if(direction==='u')ctx.rect(0,height*(1-p),width,height*p);if(direction==='d')ctx.rect(0,0,width,height*p);
    ctx.clip();draw(active.to);ctx.restore();return;
  }
  if(active.kind==='push'){
    const [x,y]=({l:[-width,0],r:[width,0],u:[0,-height],d:[0,height]})[active.direction||'l'];
    ctx.fillRect(0,0,width,height);draw(active.from,x*p,y*p);draw(active.to,x*(p-1),y*(p-1));return;
  }
  draw(active.to);
}
async function openFileMotionComparison(path,result,identity) {
  const view=fileLibraryDialog(mailImageText('Кадры и анимация','Frames and animation')+' · '+path,true);view.dialog.classList.add('file-render-dialog','file-motion-dialog');
  const abort=new AbortController(),loads=[],renderTasks=[],pdfs=[],decks=[];let closed=false,raf=0,playing=false,time=0,total=0,busy=false,sequence=0;
  const alive=()=>!closed&&view.dialog.open&&state.email===identity;
  const stop=()=>{playing=false;cancelAnimationFrame(raf);play.textContent=mailImageText('Воспроизвести','Play');play.setAttribute('aria-label',play.textContent)};
  const identityTimer=setInterval(()=>{if(state.email!==identity)view.dialog.close()},500);
  const visibility=()=>{if(document.hidden)stop()};document.addEventListener('visibilitychange',visibility);
  view.dialog.addEventListener('close',()=>{closed=true;sequence++;clearInterval(identityTimer);stop();abort.abort();document.removeEventListener('visibilitychange',visibility);for(const task of renderTasks)task.cancel();for(const load of loads)void load.destroy().catch(()=>{});caches=[];},{once:true});
  const toolbar=document.createElement('div');toolbar.className='file-render-toolbar';
  const slide=document.createElement('input');slide.type='number';slide.min='1';slide.value='1';slide.className='field-input';slide.setAttribute('aria-label',mailImageText('Слайд','Slide'));
  const mode=document.createElement('select');mode.className='field-input';mode.setAttribute('aria-label',mailImageText('Вид кадров','Frame view'));for(const [value,ru,en] of [['sides','Рядом','Side by side'],['overlay','Наложение 50%','Overlay 50%'],['diff','Разница кадров','Frame difference']])mode.add(new Option(mailImageText(ru,en),value));
  const previous=fileAction('‹',()=>navigate(-1)),next=fileAction('›',()=>navigate(1));previous.setAttribute('aria-label',mailImageText('Предыдущий слайд','Previous slide'));next.setAttribute('aria-label',mailImageText('Следующий слайд','Next slide'));
  const play=fileAction(mailImageText('Воспроизвести','Play'),()=>{if(playing){stop();return}if(time>=total)time=0;playing=true;play.textContent=mailImageText('Пауза','Pause');play.setAttribute('aria-label',play.textContent);let last=performance.now(),lastPaint=0;const tick=now=>{if(!alive()){stop();return}time=Math.min(total,time+now-last);last=now;if(now-lastPaint>=80||time>=total){paint();lastPaint=now}if(time>=total){stop();return}raf=requestAnimationFrame(tick)};raf=requestAnimationFrame(tick)});
  const back=fileAction('− 1',()=>step(-1)),forward=fileAction('+ 1',()=>step(1));back.setAttribute('aria-label',mailImageText('Предыдущий кадр','Previous frame'));forward.setAttribute('aria-label',mailImageText('Следующий кадр','Next frame'));
  const scrub=document.createElement('input');scrub.type='range';scrub.min='0';scrub.max='0';scrub.step='40';scrub.value='0';scrub.setAttribute('aria-label',mailImageText('Время кадра','Frame time'));scrub.className='file-motion-scrub';
  const label=document.createElement('output'),warning=document.createElement('p');warning.className='file-motion-warning';warning.setAttribute('role','note');
  toolbar.append(previous,slide,next,play,back,forward,mode);const timeline=document.createElement('div');timeline.className='file-render-toolbar';timeline.append(scrub,label);
  const content=document.createElement('div');content.className='file-render-content';const help=document.createElement('p');help.className='meta';help.textContent=mailImageText('Общая шкала времени. Обычные щелчки моделируются паузой 0,5 с. Интерактивные эффекты запускаются щелчком на объекте в режиме «Рядом» или кнопкой триггера; сброс очищает историю.','Shared timeline. Ordinary clicks advance after 0.5 s. Click an object or a trigger button for interactive effects; reset clears event history.');view.body.append(toolbar,timeline,help,warning,content);
  let caches=[],schedules=[],canvases=[],difference=null,currentSlides=[],eventMaps=[Object.create(null),Object.create(null)],previousSlides=[];
  const triggers=document.createElement('div');triggers.className='file-render-toolbar file-motion-triggers';triggers.setAttribute('aria-label',mailImageText('Интерактивные эффекты','Interactive effects'));view.body.insertBefore(triggers,content);
  const reset=fileAction(mailImageText('Сбросить триггеры','Reset triggers'),()=>{stop();eventMaps=[Object.create(null),Object.create(null)];time=0;refreshSchedules();paint()});
  function refreshSchedules(){schedules=currentSlides.map((s,i)=>fileMotionSchedule(s,previousSlides[i],eventMaps[i]));total=Math.max(...schedules.map(s=>s.duration));scrub.max=String(total)}
  function eventTargets(s){const ids=new Set((s?.effects||[]).map(e=>e.event).filter(Boolean));const ordered=(s?.hits||[]).filter(h=>ids.has(h.target)).map(h=>h.target);return [...ordered,...[...ids].filter(id=>!ordered.includes(id))]}
  function fireTrigger(index){if(busy||!alive())return;stop();let fired=false;currentSlides.forEach((s,i)=>{const target=eventTargets(s)[index];if(target&&eventMaps[i][target]===undefined){eventMaps[i][target]=time;fired=true}});refreshSchedules();paint();if(fired)play.click()}
  function triggerControls(){triggers.replaceChildren();const count=Math.max(...currentSlides.map(s=>eventTargets(s).length));for(let index=0;index<count;index++){const names=currentSlides.map(s=>{const id=eventTargets(s)[index];return s?.hits?.find(h=>h.target===id)?.name||id||'—'});const button=fileAction(mailImageText('Триггер ','Trigger ')+(index+1)+' · '+names.join(' / '),()=>fireTrigger(index));triggers.append(button)}if(count)triggers.append(reset)}
  const controls=[slide,previous,next,play,back,forward,mode,scrub];const enabled=ready=>{for(const control of controls)control.disabled=!ready;previous.disabled=!ready||Number(slide.value)<=1;next.disabled=!ready||Number(slide.value)>=Math.max(...decks.map(d=>d.slides.length),0)};enabled(false);
  const status=message=>{if(alive())view.status.textContent=message};
  function paint(){
    if(!alive()||busy||canvases.length!==2)return;
    for(let i=0;i<2;i++)fileMotionFrame(canvases[i].getContext('2d'),caches[i],schedules[i],time);
    scrub.value=String(Math.round(time));label.textContent=`${(time/1000).toFixed(2)} / ${(total/1000).toFixed(2)} ${mailImageText('с','s')}`;
    if(mode.value!=='diff'){status(mailImageText('Общий момент времени: ','Shared frame time: ')+(time/1000).toFixed(2)+' '+mailImageText('с','s'));if(mode.value==='overlay'){const ctx=difference.getContext('2d');ctx.globalAlpha=1;ctx.drawImage(canvases[0],0,0);ctx.globalAlpha=.5;ctx.drawImage(canvases[1],0,0);ctx.globalAlpha=1}return}
    const ctx=difference.getContext('2d'),w=difference.width,h=difference.height,a=canvases[0].getContext('2d').getImageData(0,0,w,h).data,b=canvases[1].getContext('2d').getImageData(0,0,w,h).data,out=ctx.createImageData(w,h);let count=0;
    for(let i=0;i<a.length;i+=4){const changed=Math.max(Math.abs(a[i]-b[i]),Math.abs(a[i+1]-b[i+1]),Math.abs(a[i+2]-b[i+2]))>24;count+=changed?1:0;out.data[i]=changed?220:Math.round((a[i]+a[i+1]+a[i+2])/3*.25+191);out.data[i+1]=changed?30:out.data[i];out.data[i+2]=changed?90:out.data[i];out.data[i+3]=255}ctx.putImageData(out,0,0);status(mailImageText('Отличается пикселей кадра: ','Different frame pixels: ')+(100*count/(w*h)).toFixed(2)+'%'+mailImageText(' · порог 24/255.',' · threshold 24/255.'));
  }
  function layout(){if(!canvases.length)return;content.replaceChildren();content.classList.toggle('file-render-sides',mode.value==='sides');if(mode.value==='sides'){for(let i=0;i<2;i++){const figure=document.createElement('figure'),caption=document.createElement('figcaption');caption.textContent=mailImageText(i?'Стало':'Было',i?'After':'Before');figure.append(caption,canvases[i]);content.append(figure)}}else{content.append(difference)}paint()}
  function step(delta){stop();time=Math.max(0,Math.min(total,time+delta*40));paint()}
  scrub.oninput=()=>{stop();time=Number(scrub.value);paint()};mode.onchange=layout;
  function navigate(delta){if(busy)return;slide.value=String(Number(slide.value)+delta);void renderSlide()};slide.onchange=()=>void renderSlide();
  async function renderSlide(){
    if(!alive()||busy||pdfs.length!==2)return;stop();busy=true;enabled(false);const generation=++sequence;content.replaceChildren();caches=[];canvases=[];status(mailImageText('Отрисовка ключевых состояний…','Rendering key states…'));
    try{
      const count=Math.max(...decks.map(d=>d.slides.length));let index=Math.trunc(Number(slide.value));if(!Number.isFinite(index))index=1;index=Math.max(1,Math.min(count,index));slide.value=String(index);const current=decks.map(d=>d.slides[index-1]);currentSlides=current;previousSlides=decks.map(d=>d.slides[index-2]);eventMaps=[Object.create(null),Object.create(null)];refreshSchedules();triggerControls();
      total=Math.max(...schedules.map(s=>s.duration));scrub.max=String(total);time=0;
      warning.textContent=current.map((s,i)=>mailImageText(i?'Стало: ':'Было: ',i?'After: ':'Before: ')+(!s?mailImageText('слайд отсутствует','slide absent'):s.warning?mailImageText('Статичный слайд — неподдерживаемая анимация или переход. ','Static slide — unsupported animation or transition. ')+s.warning:mailImageText('поддерживаемые эффекты','supported effects'))).join(' · ');
      const needed=current.map((s,i)=>[...new Set([...(s?.states||[]),...fileMotionPageIDs(s),...fileMotionPreviousIDs(decks[i].slides[index-2]),decks[i].slides[index-2]?.states.at(-1)||0])]);
      let width=1,height=1;const pages=[];
      for(let i=0;i<2;i++){const group=new Map();for(const id of needed[i]){if(!id)continue;const p=await pdfs[i].getPage(id);group.set(id,p);const viewport=p.getViewport({scale:1});width=Math.max(width,viewport.width);height=Math.max(height,viewport.height)}pages.push(group)}
      if(!Number.isFinite(width)||!Number.isFinite(height)||width<=0||height<=0)throw Error('Invalid frame size');const scale=Math.min(1.5,1000/width,1000/height,Math.sqrt((current.some(s=>s?.layers?.length)?150000:400000)/(width*height))),w=Math.max(1,Math.ceil(width*scale)),h=Math.max(1,Math.ceil(height*scale));
      for(let i=0;i<2;i++){const cache=new Map();for(const [id,p] of pages[i]){if(!alive()||sequence!==generation)return;const canvas=document.createElement('canvas');canvas.width=w;canvas.height=h;const ctx=canvas.getContext('2d');ctx.fillStyle='white';ctx.fillRect(0,0,w,h);const task=p.render({canvasContext:ctx,viewport:p.getViewport({scale}),background:'rgb(255,255,255)'});renderTasks.push(task);try{await task.promise}finally{renderTasks.splice(renderTasks.indexOf(task),1);p.cleanup()}canvas.motionSize=[p.getViewport({scale}).width,p.getViewport({scale}).height];cache.set(id,canvas)}
        if(current[i]?.layers?.length)fileMotionDecodeLayers(cache,current[i]);
        const prior=previousSlides[i];if(fileMotionNeedsFinal(prior)){const priorCache=new Map(cache);fileMotionDecodeLayers(priorCache,prior);const snapshot=document.createElement('canvas');snapshot.width=w;snapshot.height=h;const priorSchedule=fileMotionSchedule(prior,null,Object.create(null));fileMotionFrame(snapshot.getContext('2d'),priorCache,priorSchedule,priorSchedule.duration);cache.set(prior.states.at(-1),snapshot);for(const id of fileMotionPageIDs(prior)){if(id!==prior.states.at(-1))cache.delete(id)}}
        caches.push(cache);const canvas=document.createElement('canvas');canvas.width=w;canvas.height=h;canvas.setAttribute('aria-label',mailImageText(i?'Кадр: стало':'Кадр: было',i?'After frame':'Before frame'));canvas.onclick=event=>{if(!currentSlides[i]?.layers?.length)return;const rect=canvas.getBoundingClientRect(),size=caches[i].motionSize||[canvas.width,canvas.height],x=(event.clientX-rect.left)*canvas.width/rect.width/size[0],y=(event.clientY-rect.top)*canvas.height/rect.height/size[1];const targets=eventTargets(currentSlides[i]);for(const hit of [...(currentSlides[i].hits||[])].reverse()){if(!targets.includes(hit.target)||fileMotionTargetOpacity(schedules[i],hit.target,time)<=.01)continue;const tr=fileMotionObjectTransform(schedules[i],hit.target,time),cx=hit.box[0]+hit.box[2]/2,cy=hit.box[1]+hit.box[3]/2,angle=-((hit.rotation||0)+tr.angle)*Math.PI/180,rx=(x-cx-tr.offset[0])*size[0],ry=(y-cy-tr.offset[1])*size[1],ux=(rx*Math.cos(angle)-ry*Math.sin(angle))/tr.scale[0]/size[0],uy=(rx*Math.sin(angle)+ry*Math.cos(angle))/tr.scale[1]/size[1];if(Math.abs(ux)<=hit.box[2]/2&&Math.abs(uy)<=hit.box[3]/2){fireTrigger(targets.indexOf(hit.target));break}}};canvases.push(canvas)}
      if(!alive()||sequence!==generation)return;difference=document.createElement('canvas');difference.width=w;difference.height=h;difference.setAttribute('aria-label',mailImageText('Сравнение кадров','Frame comparison'));busy=false;layout();
    }catch(error){status(error.message);caches=[];canvases=[]}finally{busy=false;enabled(canvases.length===2)}
  }
  async function responseBytes(address,expected){const url=new URL(address,location.href);if(url.origin!==location.origin||url.pathname!==expected)throw Error('Invalid comparison address');const response=await fetch(url,{credentials:'same-origin',cache:'no-store',signal:abort.signal});if(!response.ok)throw Error(mailImageText('Сравнение недоступно. Откройте заново. Код: ','Comparison unavailable. Reopen. Status: ')+response.status);const reader=response.body.getReader(),chunks=[];let size=0;while(true){const {value,done}=await reader.read();if(done)break;size+=value.length;if(size>32*1024*1024){await reader.cancel();throw Error('Comparison > 32 MiB')}chunks.push(value)}const data=new Uint8Array(size);let offset=0;for(const chunk of chunks){data.set(chunk,offset);offset+=chunk.length}return data}
  try{
    status(mailImageText('Подготовка анимации…','Preparing animation…'));
    const pdfjs=await import('/assets/pdfjs/pdf.mjs');pdfjs.GlobalWorkerOptions.workerSrc='/assets/pdfjs/pdf.worker.mjs';
    for(const [metadata,address] of [[result.left_timeline,result.left_url],[result.right_timeline,result.right_url]]){
      const deck=JSON.parse(new TextDecoder().decode(await responseBytes(metadata,'/office/compare/timeline')));if(!alive())return;
      if(!Array.isArray(deck.slides)||!deck.slides.length||deck.slides.length>50||!Number.isInteger(deck.frames)||deck.frames<1||deck.frames>120)throw Error('Invalid storyboard');
      for(const s of deck.slides){if(!Array.isArray(s.effects)||s.effects.length>12||!Array.isArray(s.states)||s.states.length!==s.effects.length+1||s.states.some(id=>!Number.isInteger(id)||id<1||id>deck.frames)||!s.transition||!['cut','fade','wipe','push'].includes(s.transition.kind)||!Number.isInteger(s.transition.duration)||s.transition.duration<0||s.transition.duration>30000||s.effects.some(e=>!['cut','fade','path','rotate','scale','color','gradient'].includes(e.kind)||!['click','after','with'].includes(e.trigger)||!Number.isInteger(e.duration)||!Number.isInteger(e.delay)||e.duration<0||e.duration>30000||e.delay<0||e.delay>30000))throw Error('Invalid animation timeline');fileMotionValidateLayers(s,deck.frames)}
      const data=await responseBytes(address,'/office/compare/pdf');if(!alive())return;const load=pdfjs.getDocument({data,isEvalSupported:false,stopAtErrors:true,maxImageSize:16000000,canvasMaxAreaInBytes:16000000,cMapUrl:'/assets/pdfjs/cmaps/',cMapPacked:true,standardFontDataUrl:'/assets/pdfjs/standard_fonts/',wasmUrl:'/assets/pdfjs/wasm/'});loads.push(load);const pdf=await load.promise;if(!alive())return;if(pdf.numPages!==deck.frames)throw Error(mailImageText('Число отрисованных кадров не совпало с временной шкалой. Этот файл не поддерживается конвертером.','Rendered frame count does not match timeline. Converter does not support this file.'));pdfs.push(pdf);decks.push(deck);
    }
    slide.max=String(Math.max(...decks.map(d=>d.slides.length)));await renderSlide();
  }catch(error){stop();status(error.message);for(const load of loads)void load.destroy().catch(()=>{});enabled(false)}
}

function fileMotionLayerSchedule(slide,previous,events){
  const groups=new Map(),segments=[],transition=slide.transition.duration&&slide.transition.kind!=='cut'?slide.transition.duration:0;let duration=transition;
  for(const [effectIndex,effect] of slide.effects.entries()){const key=effect.event||'',base=key?(Object.hasOwn(events,key)?events[key]:undefined):transition;if(base===undefined)continue;const group=groups.get(key)||{start:base,end:base,max:base,count:0};let start;
    if(effect.trigger==='with')start=group.start+effect.delay;else if(effect.trigger==='after')start=group.end+effect.delay;else start=(key&&group.count===0?base:group.max+500)+effect.delay;
    const end=start+Math.max(1,effect.duration);segments.push({...effect,effectIndex,start,end});group.start=start;group.end=end;group.max=Math.max(group.max,end);group.count++;groups.set(key,group);duration=Math.max(duration,end);
  }
  return {segments,duration:duration+500,initial:slide.states[0],layers:slide.layers,background:slide.background,transition:slide.transition,previous:previous?.states.at(-1)||0,hits:slide.hits||[],colors:slide.colors||[]};
}
function fileMotionPoint(points,progress){
  let length=0;const lengths=[];for(let i=1;i<points.length;i++){const value=Math.hypot(points[i].x-points[i-1].x,points[i].y-points[i-1].y);lengths.push(value);length+=value}let remaining=length*Math.max(0,Math.min(1,progress));for(let i=0;i<lengths.length;i++){if(remaining<=lengths[i]&&lengths[i]>0){const t=remaining/lengths[i];return [points[i].x+(points[i+1].x-points[i].x)*t,points[i].y+(points[i+1].y-points[i].y)*t]}remaining-=lengths[i]}return [points.at(-1).x,points.at(-1).y];
}
function fileMotionTargetOffset(schedule,target,time){let offset=[0,0];for(const e of schedule.segments){if(e.target===target&&e.kind==='path'&&time>=e.start)offset=fileMotionPoint(e.path,(time-e.start)/(e.end-e.start))}return offset}
function fileMotionLayerFrame(ctx,cache,schedule,time){
  const w=ctx.canvas.width,h=ctx.canvas.height;let scratch=cache.motionScratch;if(!scratch){scratch=document.createElement('canvas');scratch.width=w;scratch.height=h;cache.motionScratch=scratch}const target=scratch.getContext('2d');target.globalAlpha=1;target.fillStyle='white';target.fillRect(0,0,w,h);if(cache.has(schedule.background))target.drawImage(cache.get(schedule.background),0,0);
  const size=cache.motionSize||[w,h];schedule.layers.forEach((layer,index)=>{
    const transform=fileMotionObjectTransform(schedule,layer.target,time),hit=schedule.hits.find(hit=>hit.target===layer.target),opacity=fileMotionTargetOpacity(schedule,layer.target,time);let image=cache.get('layer:'+index);
    if(transform.colors.length)image=fileMotionColorImage(cache,index,image,transform.colors,schedule.colors);
    target.save();target.globalAlpha=Math.max(0,Math.min(1,opacity));if(hit){const cx=(hit.box[0]+hit.box[2]/2)*size[0],cy=(hit.box[1]+hit.box[3]/2)*size[1],base=hit.rotation||0;target.translate(cx+transform.offset[0]*size[0],cy+transform.offset[1]*size[1]);target.rotate((base+transform.angle)*Math.PI/180);target.scale(transform.scale[0],transform.scale[1]);target.rotate(-base*Math.PI/180);target.translate(-cx,-cy)}else{target.translate(transform.offset[0]*size[0],transform.offset[1]*size[1])}target.drawImage(image,0,0);target.restore();
  });
  if(time<schedule.transition.duration&&schedule.transition.kind!=='cut'){cache.set('current-scene',scratch);fileMotionFrame(ctx,cache,{initial:schedule.previous,segments:[{start:0,end:schedule.transition.duration,from:schedule.previous,to:'current-scene',kind:schedule.transition.kind,direction:schedule.transition.direction}]},time);return}
  ctx.globalAlpha=1;ctx.clearRect(0,0,w,h);ctx.drawImage(scratch,0,0);
}
function fileMotionObjectTransform(schedule,target,time){const result={offset:[0,0],angle:0,scale:[1,1],colors:[]};for(const e of schedule.segments){if(e.target!==target||time<e.start)continue;const p=Math.max(0,Math.min(1,(time-e.start)/(e.end-e.start)));if(e.kind==='path')result.offset=fileMotionPoint(e.path,p);else if(e.kind==='rotate')result.angle=e.from[0]+(e.to[0]-e.from[0])*p;else if(e.kind==='scale')result.scale=e.from.map((v,i)=>v+(e.to[i]-v)*p);else if(e.kind==='color'||e.kind==='gradient')result.colors.push({effect:e,progress:p})}return result}
function fileMotionPageIDs(slide){return [...(slide?.layers||[]).flatMap(l=>[l.black,l.white]),...(slide?.probe||[]),slide?.background||0,...(slide?.colors||[]).flatMap(c=>[c.from_black,c.from_white,c.to_black,c.to_white,...(c.samples||[]).flatMap(s=>[s.black,s.white])])]}
function fileMotionNeedsFinal(slide){return slide?.layers?.length&&slide.effects.some(e=>['rotate','scale','color','gradient'].includes(e.kind))}
function fileMotionPreviousIDs(slide){return fileMotionNeedsFinal(slide)?fileMotionPageIDs(slide):[]}

function fileMotionValidateLayers(slide,frames){
  if(!slide.hits)slide.hits=[];
  const validID=id=>Number.isInteger(id)&&id>0&&id<=frames;
  for(const e of slide.effects){if(e.kind==='path'&&(!Array.isArray(e.path)||e.path.length<2||e.path.length>512||e.path.some(p=>!Number.isFinite(p.x)||!Number.isFinite(p.y)||Math.abs(p.x)>16||Math.abs(p.y)>16)))throw Error('Invalid motion path');if(e.event!==undefined&&(typeof e.event!=='string'||e.event.length>256))throw Error('Invalid interactive target')}
  for(const e of slide.effects){if(['rotate','scale','color','gradient'].includes(e.kind)){const length=e.kind==='gradient'&&e.property==='stops'?e.from?.length:{rotate:1,scale:2,color:3,gradient:1}[e.kind];if(!Array.isArray(e.from)||!Array.isArray(e.to)||e.from.length!==length||e.to.length!==length||[...e.from,...e.to].some(v=>!Number.isFinite(v))||(e.kind==='rotate'&&[...e.from,...e.to].some(v=>Math.abs(v)>3600))||(e.kind==='scale'&&[...e.from,...e.to].some(v=>v<.01||v>8))||(e.kind==='color'&&[...e.from,...e.to].some(v=>v<0||v>255)))throw Error('Invalid transform parameters')}}
  const covered=new Set();
  for(const color of slide.colors||[]){
    if(!Number.isInteger(color.effect)||!['color','gradient'].includes(slide.effects[color.effect]?.kind)||covered.has(color.effect)||[color.from_black,color.from_white,color.to_black,color.to_white].some(id=>!validID(id)))throw Error('Invalid paint layer');covered.add(color.effect);
    const joint=color.geometry_effect!==undefined;
    if(joint){const g=slide.effects[color.geometry_effect],c=slide.effects[color.effect];if(!Number.isInteger(color.geometry_effect)||g?.kind!=='gradient'||c.kind!=='color'||c.property!=='fill'||g.target!==c.target||covered.has(color.geometry_effect))throw Error('Invalid joint paint');covered.add(color.geometry_effect)}
    if(!color.samples){if(joint)throw Error('Missing joint samples');continue}
    const samples=color.samples;if(!Array.isArray(samples)||samples.length<2||samples.length>(joint?45:9)||samples.some(s=>!Number.isFinite(s.at)||s.at<0||s.at>1||!validID(s.black)||!validID(s.white)))throw Error('Invalid color samples');
    const rows=[];for(const sample of samples){const q=joint?(sample.geometry_at??0):0;if(!Number.isFinite(q)||q<0||q>1)throw Error('Invalid geometry sample');if(!rows.length||rows.at(-1).q!==q){if(rows.length&&q<=rows.at(-1).q)throw Error('Unordered joint samples');rows.push({q,values:[]})}rows.at(-1).values.push(sample.at)}
    const first=rows[0].values;if(rows[0].q!==0||(joint&&(rows.length<2||rows.length>9||rows.at(-1).q!==1))||first.length<2||first.length>9||first[0]!==0||first.at(-1)!==1||first.some((v,i)=>i&&v<=first[i-1])||rows.some(r=>r.values.length!==first.length||r.values.some((v,i)=>v!==first[i])))throw Error('Incomplete paint grid');
  }
  if((slide.colors||[]).length>12||slide.effects.some((e,i)=>['color','gradient'].includes(e.kind)&&!covered.has(i)))throw Error('Missing color layers');
  for(const e of slide.effects){if(e.kind!=='gradient')continue;if(!['angle','focus-x','focus-y','focus-width','focus-height','stops'].includes(e.property)||[...e.from,...e.to].some(v=>e.property==='angle'?Math.abs(v)>7200:v<0||v>1)||(e.property==='stops'&&(e.from.length<2||e.from.length>16||[e.from,e.to].some(values=>values.some((v,i)=>i&&v<values[i-1])))))throw Error('Invalid gradient geometry')}
  if(!slide.layers?.length){if(slide.effects.some(e=>['path','rotate','scale','color','gradient'].includes(e.kind)||e.trigger==='with'||e.event))throw Error('Missing animation layers');return}
  if(slide.layers.length>25||!validID(slide.background)||!Array.isArray(slide.probe)||slide.probe.length!==2||slide.probe.some(id=>!validID(id))||slide.layers.some(l=>!validID(l.black)||!validID(l.white)||typeof l.hidden!=='boolean'||typeof l.target!=='string')||!Array.isArray(slide.hits)||slide.hits.length>1000||slide.hits.some(h=>typeof h.target!=='string'||!Array.isArray(h.box)||h.box.length!==4||h.box.some(n=>!Number.isFinite(n)||Math.abs(n)>16)||(h.rotation!==undefined&&(!Number.isFinite(h.rotation)||Math.abs(h.rotation)>360))))throw Error('Invalid layer manifest');
}
function fileMotionDecodeLayers(cache,slide){
  const black=cache.get(slide.probe[0]),white=cache.get(slide.probe[1]);if(!black||!white)throw Error('Missing matte probes');const bx=black.getContext('2d'),wx=white.getContext('2d');for(const [x,y] of [[1,1],[Math.max(1,Math.floor(black.motionSize[0]/2)),Math.max(1,Math.floor(black.motionSize[1]/2))]]){const b=bx.getImageData(x,y,1,1).data,w=wx.getImageData(x,y,1,1).data;if([0,1,2].some(c=>b[c]>12||w[c]<243))throw Error(mailImageText('Офисный сервер не сохранил фоны слоёв. Воспроизведение остановлено.','Office server did not preserve layer mattes. Playback stopped.'))}
  cache.motionSize=black.motionSize;
  const decode=(blackID,whiteID,key)=>{const b=cache.get(blackID),w=cache.get(whiteID),width=b.width,height=b.height,bd=b.getContext('2d').getImageData(0,0,width,height).data,wd=w.getContext('2d').getImageData(0,0,width,height).data,canvas=document.createElement('canvas');canvas.width=width;canvas.height=height;const ctx=canvas.getContext('2d'),out=ctx.createImageData(width,height);for(let i=0;i<bd.length;i+=4){const alpha=Math.max(0,Math.min(255,255-Math.max(wd[i]-bd[i],wd[i+1]-bd[i+1],wd[i+2]-bd[i+2])));out.data[i+3]=alpha;if(alpha>1)for(let c=0;c<3;c++)out.data[i+c]=Math.min(255,bd[i+c]*255/alpha)}ctx.putImageData(out,0,0);ctx.clearRect(Math.ceil(b.motionSize[0]),0,width,height);ctx.clearRect(0,Math.ceil(b.motionSize[1]),width,height);if(key.startsWith('color:')){cache.set(key,ctx.getImageData(0,0,width,height).data)}else{cache.set(key,canvas)}cache.delete(blackID);cache.delete(whiteID)};
  slide.layers.forEach((layer,index)=>decode(layer.black,layer.white,'layer:'+index));for(const color of slide.colors||[]){if(color.samples){color.samples.forEach((s,i)=>decode(s.black,s.white,'color:'+color.effect+':sample:'+i))}else{decode(color.from_black,color.from_white,'color:'+color.effect+':from');decode(color.to_black,color.to_white,'color:'+color.effect+':to')}}cache.delete(slide.probe[0]);cache.delete(slide.probe[1]);
}

function fileMotionTargetOpacity(schedule,target,time){let opacity=schedule.layers?.find(l=>l.target===target)?.hidden?0:1;for(const e of schedule.segments){if(e.target!==target||!['cut','fade'].includes(e.kind)||time<e.start)continue;const p=Math.min(1,(time-e.start)/(e.end-e.start));opacity=e.kind==='cut'?(e.exit?0:1):e.exit?1-p:p}return opacity}

function fileMotionColorImage(cache,index,base,active,manifest){
  let data=cache.get('base-pixels:'+index);if(!data){data=base.getContext('2d').getImageData(0,0,base.width,base.height).data;cache.set('base-pixels:'+index,data)}
  const running=new Map(active.map(a=>[a.effect.effectIndex,a]));
  const phase=index=>{const a=running.get(index);return a?(a.effect.discrete?(a.progress<.5?0:1):a.progress):0};
  const bracket=(values,p)=>{let left=0;while(left<values.length-2&&p>values[left+1])left++;return [left,Math.max(0,Math.min(1,(p-values[left])/(values[left+1]-values[left])))]};
  const changes=manifest.filter(c=>running.has(c.effect)||running.has(c.geometry_effect)).map(color=>{
    const samples=color.samples||[{at:0,key:'from'},{at:1,key:'to'}],joint=color.geometry_effect!==undefined,p=phase(color.effect),q=joint?phase(color.geometry_effect):0;
    let columns=samples.length;if(joint){columns=samples.findIndex(s=>(s.geometry_at??0)>0)}
    const [x,u]=bracket(samples.slice(0,columns).map(s=>s.at),p);let row=0,v=0;
    if(joint)[row,v]=bracket(samples.filter((_,i)=>i%columns===0).map(s=>s.geometry_at??0),q);
    const key=i=>'color:'+color.effect+':'+(color.samples?'sample:'+i:samples[i].key),weights=joint?[[row*columns+x,(1-u)*(1-v)],[row*columns+x+1,u*(1-v)],[(row+1)*columns+x,(1-u)*v],[(row+1)*columns+x+1,u*v]]:[[x,1-u],[x+1,u]];
    return {from:cache.get(key(0)),terms:weights.map(([i,weight])=>({data:cache.get(key(i)),weight}))};
  });
  let canvas=cache.motionColorScratch;if(!canvas){canvas=document.createElement('canvas');canvas.width=base.width;canvas.height=base.height;cache.motionColorScratch=canvas}const ctx=canvas.getContext('2d'),out=ctx.createImageData(base.width,base.height);
  for(let i=0;i<data.length;i+=4){let alpha=data[i+3];const premul=[data[i]*alpha/255,data[i+1]*alpha/255,data[i+2]*alpha/255];for(const c of changes){const initial=c.from[i+3];alpha-=initial;for(let k=0;k<3;k++)premul[k]-=c.from[i+k]*initial/255;for(const term of c.terms){const a=term.data[i+3]*term.weight;alpha+=a;for(let k=0;k<3;k++)premul[k]+=term.data[i+k]*a/255}}alpha=Math.max(0,Math.min(255,alpha));out.data[i+3]=alpha;if(alpha>1)for(let k=0;k<3;k++)out.data[i+k]=Math.max(0,Math.min(255,premul[k]*255/alpha))}
  ctx.putImageData(out,0,0);return canvas;
}
