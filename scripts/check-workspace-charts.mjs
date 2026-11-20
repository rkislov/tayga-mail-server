import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
const source=fs.readFileSync('frontend/src/app.js','utf8');const functions=source.slice(source.indexOf('function workspaceChartPath('),source.indexOf('function renderMonitorHistory('));
const context=vm.createContext({lang:'ru',Intl,Date,Number,Math,escapeHtml:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),mailImageText:ru=>ru});vm.runInContext(functions,context);
// A smooth curve must not invent extrema between recorded values.
for(const points of [[[0,0],[1,10],[2,0],[3,30]],[[0,4],[2,4],[4,4]],[[0,9],[1,4],[3,3],[7,0]]]){
 const d=context.workspaceChartPath(points);assert.ok(!/NaN|Infinity/.test(d));const curves=[...d.matchAll(/C([-\d.]+),([-\d.]+) ([-\d.]+),([-\d.]+) ([-\d.]+),([-\d.]+)/g)];assert.equal(curves.length,points.length-1);
 curves.forEach((curve,i)=>{const c=curve.slice(1).map(Number),start=points[i],end=points[i+1];assert.equal(c[4],end[0]);assert.equal(c[5],end[1]);for(let k=0;k<=100;k++){const t=k/100,u=1-t,y=u*u*u*start[1]+3*u*u*t*c[1]+3*u*t*t*c[3]+t*t*t*end[1];assert.ok(y>=Math.min(start[1],end[1])-.001&&y<=Math.max(start[1],end[1])+.001)}});
}
const host={id:'test',innerHTML:''};context.renderTimeChart(host,'test',[{label:'Очередь',value:4}],'Письма',[{time:0,boot:'a',values:[1]},{time:60000,boot:'a',values:[2]},{time:300000,boot:'a',values:[3]},{time:360000,boot:'a',values:[4]}],{from:0,to:360000});assert.equal((host.innerHTML.match(/class="workspace-chart-area"/g)||[]).length,2,'missing observations must split the curve');assert.match(host.innerHTML,/aria-label="Очередь"/);assert.ok(!/NaN|Infinity/.test(host.innerHTML));
context.renderTimeChart(host,'test',[{label:'Очередь',value:4}],'Письма',[]);assert.match(host.innerHTML,/История пока не накоплена/);
console.log('Charts: curves preserve observed values, cannot invent peaks, split missing intervals, label actual series and handle empty history — OK');
