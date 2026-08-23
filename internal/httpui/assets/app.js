const $ = id => document.getElementById(id);
let current = null;

function activeUser(){ return $('user').value.trim(); }
function key(prefix){ return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`; }
function notice(message,error=false){ const box=$('notice');box.textContent=message;box.className=error?'error':'';box.style.display='block';setTimeout(()=>box.style.display='none',3200); }
async function api(path,options={}){
  const headers=Object.assign({'X-User-ID':activeUser()},options.headers||{});
  const response=await fetch(path,Object.assign({},options,{headers}));
  const text=await response.text();let body={};try{body=text?JSON.parse(text):{}}catch{body={raw:text}}
  if(!response.ok)throw new Error(body.error?.message||`请求失败 (${response.status})`);
  return body;
}
function mutationOptions(version,body){ const options={method:'POST',headers:{'X-Expected-Version':String(version),'Idempotency-Key':key('request')}};if(body!==undefined){options.headers['Content-Type']='application/json';options.body=JSON.stringify(body)}return options; }
function rows(value,columns){return value.split('\n').map(line=>line.trim()).filter(Boolean).map(line=>{const parts=line.split(',').map(v=>v.trim());while(parts.length<columns)parts.push('');return parts})}

async function loadSessions(selectCurrent=true){
  const data=await api('/api/sessions');const selected=selectCurrent&&current?current.id:$('session').value;
  $('session').innerHTML='<option value="">请选择</option>'+data.sessions.map(s=>`<option value="${s.id}">${escapeHTML(s.name)} · ${s.status}</option>`).join('');
  if(selected){$('session').value=selected;await loadCurrent(selected)}else{current=null;renderAll()}
}
async function loadCurrent(id){ if(!id){current=null;renderAll();return}current=await api(`/api/sessions/${id}`);$('statusLine').textContent=`${current.name} · ${current.status} · 版本 ${current.version}`;renderAll(); }
function escapeHTML(value){return String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}

function renderAll(){ renderHost();renderTasks();renderVerification();renderReveal(); }
function renderHost(){
  const summary=$('hostSummary'),actions=$('hostActions');
  if(!current){summary.innerHTML='<p class="meta">未选择会话</p>';actions.innerHTML='';return}
  summary.innerHTML=`<p><span class="badge">${current.status}</span> ${escapeHTML(current.productCategory)}</p><p>评审员：${current.reviewerUserIDs.map(escapeHTML).join('、')}　样品：${current.samples?.length||0}　计划哈希：${current.planHash?current.planHash.slice(0,16):'未冻结'}</p>`;
  const buttons=[];if(current.status==='draft'){buttons.push(actionButton('冻结盲码','freeze'));}if(current.status==='frozen'){buttons.push(actionButton('启动采集','start'));}if(current.status==='collecting'){buttons.push(actionButton('关闭采集','close'));}actions.innerHTML=buttons.join('');
  $('sampleForm').querySelector('button').disabled=current.status!=='draft';
}
function actionButton(label,action){return `<button type="button" onclick="sessionAction('${action}')">${label}</button>`}
async function sessionAction(action){try{current=await api(`/api/sessions/${current.id}/${action}`,mutationOptions(current.version));notice('操作已完成');await loadSessions()}catch(e){notice(e.message,true)}}

async function renderTasks(){
  const root=$('tasks');if(!current||!['collecting','verifying'].includes(current.status)){root.innerHTML='<p class="meta">当前没有可提交的评审任务</p>';return}
  try{const data=await api(`/api/sessions/${current.id}/reviewer-tasks`);root.innerHTML=data.tasks.map(task=>`<form class="task" onsubmit="submitScore(event,'${task.assignmentID}')"><h3>第 ${task.sequence} 个盲样 · 盲码 ${escapeHTML(task.blindCode)}</h3><div class="score-grid">${task.scales.map(scale=>`<label>${escapeHTML(scale.name)}<input name="score-${scale.key}" type="number" min="${scale.min}" max="${scale.max}" step="0.01" value="${task.scores?.[scale.key]??''}" required></label>`).join('')}</div><label>备注<textarea name="comment">${escapeHTML(task.comment||'')}</textarea></label><button ${task.submitted?'disabled':''}>${task.submitted?'已提交':'提交评分'}</button></form>`).join('')||'<p class="meta">没有任务</p>'}catch(e){root.innerHTML=`<p class="meta">${escapeHTML(e.message)}</p>`}
}
async function submitScore(event,assignmentID){event.preventDefault();const form=event.currentTarget;const scores={};for(const input of form.querySelectorAll('[name^="score-"]'))scores[input.name.slice(6)]=Number(input.value);try{current=await api(`/api/sessions/${current.id}/evaluations`,mutationOptions(current.version,{assignmentID,scores,comment:form.comment.value,startedAt:new Date(Date.now()-20000).toISOString()}));notice('评分已提交');await loadSessions()}catch(e){notice(e.message,true)}}

function renderVerification(){
  const progress=$('progress'),root=$('findings');if(!current){progress.innerHTML='';root.innerHTML='';return}
  progress.innerHTML=`<p>有效评分 ${current.evaluations?.filter(e=>e.validityStatus==='valid').length||0}，核验发现 ${current.findings?.length||0}</p>`;
  root.innerHTML=(current.findings||[]).map(f=>`<article class="finding"><h3><span class="badge ${f.status==='open'?'open':''}">${f.status}</span> ${escapeHTML(f.ruleCode)} · ${escapeHTML(f.severity)}</h3><p class="meta">${escapeHTML(JSON.stringify(f.evidence))}</p>${f.status==='open'?`<div class="actions"><input class="finding-check" type="checkbox" data-id="${f.id}" aria-label="选择 ${f.id}"><select class="finding-resolution" data-id="${f.id}"><option value="accept">接受记录</option><option value="void">作废记录</option><option value="rework">退回重评</option></select><button onclick="resolveFinding('${f.id}','accept')">接受记录</button><button onclick="resolveFinding('${f.id}','void')">作废记录</button><button onclick="resolveFinding('${f.id}','rework')">退回重评</button></div>`:`<p>裁定：${escapeHTML(f.resolution)} · ${escapeHTML(f.resolvedBy)}</p>`}</article>`).join('')||'<p class="meta">没有核验发现</p>';
  $('reverify').disabled=current.status!=='verifying';
}
async function resolveFinding(id,resolution){try{current=await api(`/api/sessions/${current.id}/findings/${id}/resolve`,mutationOptions(current.version,{resolution}));notice('裁定已记录');await loadSessions()}catch(e){notice(e.message,true)}}
async function resolveBatch(){const resolutions=[...document.querySelectorAll('.finding-check:checked')].map(box=>({findingID:box.dataset.id,resolution:document.querySelector(`.finding-resolution[data-id="${box.dataset.id}"]`).value}));if(!resolutions.length)return notice('请选择核验发现',true);try{const result=await api(`/api/sessions/${current.id}/findings/batch-resolve`,mutationOptions(current.version,{resolutions}));current=result;notice(`已裁定 ${result.summary.resolved} 条，待处理 ${result.summary.open} 条`);await loadSessions()}catch(e){notice(e.message,true)}}

function renderReveal(){
  const root=$('conclusions');if(!current){root.innerHTML='';return}
  $('approve').disabled=current.status!=='verifying';$('seal').disabled=current.status!=='revealed';
  const approvals=(current.revealApprovals||[]).map(a=>`${escapeHTML(a.role)}：${escapeHTML(a.userID)}`).join('、')||'尚未批准';
  const conclusions=(current.conclusions||[]).map(c=>`<article class="finding"><h3>${escapeHTML(c.dimension)}</h3><p>优选样品：${(c.winnerIDs||[c.winnerID]).map(escapeHTML).join('、')}</p><div class="mapping">${Object.entries(c.sampleStats||{}).map(([id,s])=>`<div>${escapeHTML(id)} · 均值 ${s.mean.toFixed(2)} · n=${s.count} · ${s.min.toFixed(2)}–${s.max.toFixed(2)}</div>`).join('')}</div></article>`).join('');
  const mapping=current.mapping?`<h3>盲码映射</h3><div class="mapping">${Object.entries(current.mapping).flatMap(([reviewer,codes])=>Object.entries(codes).map(([code,name])=>`<div>${escapeHTML(reviewer)} · ${escapeHTML(code)} → ${escapeHTML(name)}</div>`)).join('')}</div>`:'';
  root.innerHTML=`<p>批准记录：${approvals}</p>${conclusions}${mapping}`;
}

async function loadArchives(){try{const data=await api('/api/archives');$('archives').innerHTML=data.archives.map(r=>`<article class="receipt"><h3>${escapeHTML(r.id)}</h3><p>${escapeHTML(r.sessionID)} · ${new Date(r.sealedAt).toLocaleString()}</p><button type="button" onclick="checkArchive('${r.id}')">校验凭据</button><div id="archive-${r.id}"></div></article>`).join('')||'<p class="meta">暂无归档</p>'}catch(e){notice(e.message,true)}}
async function checkArchive(id){try{const report=await api(`/api/archives/${id}/validate`);const root=$(`archive-${id}`);root.innerHTML=report.valid?`<p>一致性校验通过</p><a href="/api/archives/${id}/export" target="_blank">凭据 JSON</a>　<a href="/api/archives/${id}/package" target="_blank">封存包 JSON</a>`:`<p class="error">${report.issues.map(i=>escapeHTML(i.message)).join('；')}</p>`}catch(e){notice(e.message,true)}}

document.querySelectorAll('.tabs button').forEach(button=>button.onclick=()=>{document.querySelectorAll('.tabs button,.view').forEach(el=>el.classList.remove('active'));button.classList.add('active');$(button.dataset.tab).classList.add('active');if(button.dataset.tab==='reviewer')renderTasks();if(button.dataset.tab==='archive')loadArchives()});
function sessionPayload(f){return{name:f.name.value,productCategory:f.category.value,hostUserID:f.host.value,reviewerUserIDs:f.reviewers.value.split(',').map(v=>v.trim()).filter(Boolean),scheduledAt:new Date(f.scheduled.value).toISOString(),seed:f.seed.value,scales:rows(f.scales.value,5).map(v=>({key:v[0],name:v[1],min:Number(v[2]),max:Number(v[3]),order:Number(v[4])}))}}
$('sessionForm').onsubmit=async event=>{event.preventDefault();const f=event.currentTarget;try{const created=await api('/api/sessions',{method:'POST',headers:{'Content-Type':'application/json','Idempotency-Key':key('create')},body:JSON.stringify(sessionPayload(f))});current=created;notice('会话已创建');await loadSessions()}catch(e){notice(e.message,true)}};
$('updateSession').onclick=async()=>{if(!current)return notice('请先选择会话',true);const f=$('sessionForm');try{current=await api(`/api/sessions/${current.id}`,{...mutationOptions(current.version,sessionPayload(f)),method:'PATCH'});notice('会话配置已更新');await loadSessions()}catch(e){notice(e.message,true)}};
$('sampleForm').onsubmit=async event=>{event.preventDefault();if(!current)return notice('请先选择会话',true);const f=event.currentTarget;const samples=rows(f.samples.value,5).map(v=>({id:v[0],internalCode:v[1],displayName:v[2],batchRef:v[3],notes:v[4]}));try{current=await api(`/api/sessions/${current.id}/samples/batch`,mutationOptions(current.version,{samples}));notice(`已登记 ${samples.length} 个样品`);await loadSessions()}catch(e){notice(e.message,true)}};
$('approve').onclick=async()=>{if(!current)return;try{current=await api(`/api/sessions/${current.id}/reveal/approve`,mutationOptions(current.version,{role:$('approvalRole').value}));notice('批准已记录');await loadSessions()}catch(e){notice(e.message,true)}};
$('seal').onclick=async()=>{if(!current)return;try{const result=await api(`/api/sessions/${current.id}/seal`,mutationOptions(current.version));current=result.session;notice(`已封存：${result.receipt.id}`);await loadSessions()}catch(e){notice(e.message,true)}};
$('reverify').onclick=async()=>{if(!current)return;try{current=await api(`/api/sessions/${current.id}/reverify`,mutationOptions(current.version));notice('重新核验完成');await loadSessions()}catch(e){notice(e.message,true)}};
$('batchResolve').onclick=resolveBatch;
$('refresh').onclick=()=>loadSessions();$('session').onchange=event=>loadCurrent(event.target.value);$('user').onchange=()=>{if(current)loadCurrent(current.id)};$('loadArchives').onclick=loadArchives;
const local=new Date(Date.now()-new Date().getTimezoneOffset()*60000).toISOString().slice(0,16);$('sessionForm').scheduled.value=local;loadSessions(false).catch(e=>notice(e.message,true));
