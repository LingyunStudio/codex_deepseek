import { App } from "../bindings/codeseek/cmd/codeseek-gui/index.js";

const $=(s,p)=>(p||document).querySelector(s);
const $$=(s,p)=>[...(p||document).querySelectorAll(s)];

let cfg=null, port='38440', running=false, activeModel='codeseek', providerName='';
let darkMode=false;

// ── Init ──
async function init(){
  try{
    cfg = await App.LoadConfig();
    port = cfg.addr ? cfg.addr.split(':').pop() : '38440';
    activeModel = cfg.active_model || 'codeseek';
    providerName = (cfg.provider_names||[])[0] || 'deepseek';
    populateModelSelector();
    await updateSidebar();
  }catch(e){
    cfg=null;
    $('#dot').className='dot';
    $('#stLabel').textContent='未加载: '+esc(e.message||'');
  }
}

function populateModelSelector(){
  const sel=$('#topModel');
  if(!sel) return;
  const names=cfg?.model_names||[];
  sel.innerHTML=names.map(n=>`<option value="${esc(n)}" ${n===activeModel?'selected':''}>${esc(n)}</option>`).join('');
}

// ── Nav ──
$$('nav a').forEach(a=>a.addEventListener('click',e=>{
  e.preventDefault();
  const v=a.dataset.view;
  $$('nav a').forEach(x=>x.classList.remove('active'));
  a.classList.add('active');
  $$('.view').forEach(x=>x.classList.remove('active'));
  const el=$('#view-'+v);
  if(el)el.classList.add('active');
  load(v);
}));

function show(v){ const a=$(`nav a[data-view="${v}"]`); if(a)a.click(); }

// ── Theme ──
window.toggleTheme=function(){
  darkMode=!darkMode;
  document.body.className=darkMode?'dark':'light';
  $('#themeBtn').textContent=darkMode?'☀️':'🌙';
  localStorage.setItem('codeseek-theme',darkMode?'dark':'light');
};

(function(){
  darkMode=localStorage.getItem('codeseek-theme')==='dark';
  document.body.className=darkMode?'dark':'light';
  const btn=$('#themeBtn'); if(btn) btn.textContent=darkMode?'☀️':'🌙';
})();

// ── Model ──
window.changeModel=function(){
  const sel=$('#topModel');
  if(!sel) return;
  activeModel=sel.value;
  App.SetActiveModel(activeModel);
  dash();
};

// ── Toast ──
function toast(msg,ty){
  const el=document.createElement('div');
  el.className='toast '+(ty||'ok'); el.textContent=msg;
  $('#toasts').appendChild(el);
  setTimeout(()=>el.remove(),3000);
}

// ── Dashboard ──
function dash(){
  if(!cfg) return;
  const p=cfg.providers||[],m=cfg.models||[],r=cfg.routes||[];
  $('#view-dashboard').innerHTML=`<h2>概览</h2><div class="cards">
    <div class="card"><h3>当前模型</h3><div class="val" style="font-size:18px">${esc(activeModel)}</div></div>
    <div class="card"><h3>运行模式</h3><div class="val">${modeLabel(cfg.mode)}</div></div>
    <div class="card"><h3>监听地址</h3><div class="val" style="font-size:14px">${esc(cfg.addr||'-')}</div></div>
    <div class="card"><h3>服务状态</h3><div class="val" style="font-size:18px;color:${running?'var(--green)':'var(--text3)'}">${running?'运行中':'未启动'}</div></div>
    <div class="card"><h3>服务商</h3><div class="val">${p.length}</div></div>
    <div class="card"><h3>模型</h3><div class="val">${m.length}</div></div>
    <div class="card"><h3>路由</h3><div class="val">${r.length}</div></div>
  </div>
  <div style="margin-top:20px;display:flex;gap:8px;">
	    <button class="btn a" onclick="toggleServer()">${running?"停止服务":"启动服务"}</button>
    <button class="btn" onclick="restoreCodex()">恢复 Codex 配置</button>
  </div>`;
}

// ── Providers ──
function providers(){
  if(!cfg) return;
  const list=cfg.providers||[];
  $('#view-providers').innerHTML=`<h2>服务商</h2><div class="tw"><table>
    <thead><tr><th>名称</th><th>Base URL</th><th>协议</th><th>模型数</th></tr></thead>
    <tbody>${list.length?list.map(p=>`<tr>
      <td><strong>${esc(p.key)}</strong></td>
      <td style="font-size:12px;color:var(--text2)">${esc(p.base_url||'')}</td>
      <td><span class="badge ${esc(p.protocol||'default')}">${protoLabel(p.protocol)}</span></td>
      <td>${(p.models||[]).length}</td>
    </tr>`).join(''):'<tr><td colspan="4" style="color:var(--text3);text-align:center;padding:24px;">暂无</td></tr>'}</tbody>
  </table></div>`;
}

// ── Models ──
function modelsV(){
  if(!cfg) return;
  const list=cfg.models||[];
  $('#view-models').innerHTML=`<h2>模型</h2><div class="tw"><table>
    <thead><tr><th>标识</th><th>显示名</th><th>上下文窗口</th><th>最大输出</th></tr></thead>
    <tbody>${list.length?list.map(m=>`<tr>
      <td><strong>${esc(m.slug)}</strong></td>
      <td>${esc(m.display_name||'')}</td>
      <td>${fmtNum(m.context_window||0)}</td>
      <td>${fmtNum(m.max_output_tokens||0)}</td>
    </tr>`).join(''):'<tr><td colspan="4" style="color:var(--text3);text-align:center;padding:24px;">暂无</td></tr>'}</tbody>
  </table></div>`;
}

// ── Routes ──
function routesV(){
  if(!cfg) return;
  const list=cfg.routes||[];
  $('#view-routes').innerHTML=`<h2>路由</h2><div class="tw"><table>
    <thead><tr><th>别名</th><th>服务商</th><th>上游模型</th></tr></thead>
    <tbody>${list.length?list.map(r=>`<tr>
      <td><strong>${esc(r.alias)}</strong></td>
      <td>${esc(r.provider)}</td>
      <td>${esc(r.model)}</td>
    </tr>`).join(''):'<tr><td colspan="3" style="color:var(--text3);text-align:center;padding:24px;">暂无</td></tr>'}</tbody>
  </table></div>`;
}

// ── Settings ──
async function settings(){
  if(!cfg) return;
  const c=cfg.cache||{},w=cfg.web_search||{};
  let apiKey='';
  try{ apiKey=await App.GetAPIKey(providerName); }catch(e){}

  $('#view-settings').innerHTML=`<h2>设置</h2>
    <div class="sc"><h3>服务器</h3>
      <div class="fr">
        <div class="f"><label>监听端口</label><div class="pr"><span>127.0.0.1:</span><input id="sPort" value="${esc(port)}"></div></div>
        <div class="f"><label>运行模式</label><select id="sMode">${['Transform','CaptureAnthropic','CaptureResponse'].map(v=>`<option value="${v}" ${cfg.mode===v?'selected':''}>${modeLabel(v)}</option>`).join('')}</select></div>
      </div>
      <div class="fr" style="margin-top:12px;">
        <div class="f"><label>日志级别</label><select id="sLog">${['debug','info','warn','error'].map(v=>`<option value="${v}" ${cfg.log_level===v?'selected':''}>${v.toUpperCase()}</option>`).join('')}</select></div>
      </div>
    </div>
    <div class="sc"><h3>API Key</h3>
      <div class="f"><label>${esc(providerName)} API Key</label>
        <div class="pw-row">
          <input id="sAPIKey" type="password" value="${esc(apiKey)}" placeholder="在此处输入 API Key，自动同步到配置文件">
          <button onclick="toggleAPIKey()" id="sAPIKeyBtn">显示</button>
        </div>
      </div>
      <button class="btn a" onclick="saveAPIKey()">保存 API Key</button>
    </div>
    <div class="sc"><h3>缓存</h3>
      <div class="fr">
        <div class="f"><label>模式</label><select id="sCache">${['off','explicit','automatic','hybrid'].map(v=>`<option value="${v}" ${c.mode===v?'selected':''}>${cacheLabel(v)}</option>`).join('')}</select></div>
        <div class="f"><label>有效期</label><select id="sTTL">${['5m','1h'].map(v=>`<option value="${v}" ${c.ttl===v?'selected':''}>${v==='5m'?'5 分钟':'1 小时'}</option>`).join('')}</select></div>
      </div>
    </div>
    <div class="sc"><h3>网页搜索</h3>
      <div class="fr">
        <div class="f"><label>模式</label><select id="sWS">${['auto','enabled','disabled','injected'].map(v=>`<option value="${v}" ${w.support===v?'selected':''}>${wsLabel(v)}</option>`).join('')}</select></div>
        <div class="f"><label>Tavily API Key</label><input id="sTavily" type="password" value="${esc(w.tavily_key||'')}"></div>
      </div>
    </div>
    <button class="btn a" onclick="saveSettings()">保存设置</button>`;
}

window.toggleAPIKey=function(){
  const inp=$('#sAPIKey'), btn=$('#sAPIKeyBtn');
  if(!inp) return;
  const show=inp.type==='password';
  inp.type=show?'text':'password';
  if(btn) btn.textContent=show?'隐藏':'显示';
};

window.saveAPIKey=async function(){
  const key=$('#sAPIKey')?.value||'';
  try{
    await App.SetAPIKey(providerName, key);
    toast('API Key 已保存到配置文件');
  }catch(e){ toast('保存失败: '+e.message,'err'); }
};

window.saveSettings=async function(){
  const p=$('#sPort')?.value||'38440';
  await App.SetPort(p);
  port=p;
  await updateSidebar();
  toast('设置已保存，重启服务后生效');
};

// ── Codex backup/restore ──
window.restoreCodex=async function(){
  if(!confirm('确定恢复 Codex 原始配置？')) return;
  try{ await App.RestoreCodexConfig(); toast('Codex 配置已恢复'); }
  catch(e){ toast('恢复失败: '+e.message,'err'); }
};

// ── Server ──
window.toggleServer=async function(){
  try{
    if(running){ await App.StopServer(); running=false; }
    else{ await App.StartServer(); running=true; }
    await updateSidebar();
    dash(); // refresh dashboard button
    toast(running?'服务已启动':'服务已停止');
  }catch(e){ toast(e.message,'err'); }
};

// ── Logs ──
async function logs(){
  const el=$('#view-logs');
  el.innerHTML='<h2>运行日志</h2><div class="log-wrap" id="logContent">加载中...</div>';
  await refreshLogs();
}

async function refreshLogs(){
  try{
    const lines=await App.GetLogs();
    const el=$('#logContent');
    if(el && lines){
      el.textContent=lines.join('\n');
      el.scrollTop=el.scrollHeight;
    }
  }catch(e){}
}

// ── Sidebar ──
async function updateSidebar(){
  try{ running=await App.IsRunning(); }catch(e){ running=false; }
  $('#dot').className='dot '+(running?'on':'');
  $('#stLabel').textContent=running?'运行中':(cfg?'已加载':'就绪');
  $('#stAddr').textContent=running?('127.0.0.1:'+port):'';
  const btn=$('#srvBtn');
  if(btn){
    btn.textContent=running?'停止':'启动';
    btn.className='srv-btn'+(running?' running':'');
  }
}

// ── Labels ──
function modeLabel(v){ return {Transform:'协议转换',CaptureAnthropic:'Anthropic 代理',CaptureResponse:'OpenAI 代理','协议转换':'Transform','Anthropic 代理':'CaptureAnthropic','OpenAI 代理':'CaptureResponse'}[v]||v; }
function protoLabel(v){ return {anthropic:'Anthropic','openai-chat':'OpenAI Chat','openai-response':'OpenAI 响应','google-genai':'Google AI'}[v]||v; }
function cacheLabel(v){ return {off:'关闭',explicit:'手动',automatic:'自动',hybrid:'混合'}[v]||v; }
function wsLabel(v){ return {auto:'自动',enabled:'启用',disabled:'禁用',injected:'注入'}[v]||v; }

function esc(s){return s?String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;'):'';}
function fmtNum(n){if(n>=1e6)return (n/1e6).toFixed(1)+'M';if(n>=1e3)return (n/1e3).toFixed(1)+'K';return String(n);}

// ── View load ──
function load(v){
  switch(v){
    case 'dashboard': dash(); break;
    case 'providers': providers(); break;
    case 'models':    modelsV(); break;
    case 'routes':    routesV(); break;
    case 'settings':  settings(); break;
    case 'logs':      logs(); break;
  }
}

// ── Bootstrap ──
(async()=>{
  // Create view containers
  ['dashboard','providers','models','routes','settings','logs'].forEach(v=>{
    const el=document.createElement('div'); el.id='view-'+v; el.className='view';
    if(v==='dashboard')el.classList.add('active');
    $('#viewContainer').appendChild(el);
  });
  await init();
  if(cfg) show('dashboard');

  // Poll
  setInterval(async ()=>{
    try{ running=await App.IsRunning(); }catch(e){}
    await updateSidebar();
    if($('#view-logs')?.classList?.contains('active')) refreshLogs();
  }, 2000);
})();
