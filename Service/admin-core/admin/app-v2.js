(function() {
// ============================================
// 市舶司万能管理后台 - 前端引擎
// 配置驱动、Widget组合、动态渲染
// ============================================

var API_BASE = '/api/v1';
var accessToken = localStorage.getItem('admin_token') || '';
var currentUser = null;
var currentPage = '';
var modulesConfig = null;
var servicesData = null;

// ============================================
// 工具函数
// ============================================
function showToast(msg, type = 'success') {
  const t = document.getElementById('toast');
  t.textContent = msg;
  t.className = `toast ${type} show`;
  clearTimeout(t._timeout);
  t._timeout = setTimeout(() => t.classList.remove('show'), 3000);
}

async function apiFetch(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;
  
  try {
    const res = await fetch(API_BASE + path, { ...options, headers });
    
    let data;
    const text = await res.text();
    try {
      data = JSON.parse(text);
    } catch (e) {
      data = { code: res.status, msg: '非JSON响应', data: null };
    }
    
    if (data.code === 401) { 
      showToast('登录已过期，请重新登录', 'error'); 
      setTimeout(() => location.reload(), 1500); 
    }
    
    // 检测服务不可用状态
    if (res.status === 503 || (data.msg && data.msg.includes('熔断器'))) {
      return { code: 503, msg: '服务暂时不可用', data: null, _serviceOffline: true };
    }
    
    return data;
  } catch (networkError) {
    // 网络错误 - 服务可能离线
    console.warn('网络请求失败:', path, networkError.message);
    return { 
      code: -1, 
      msg: '网络错误: ' + networkError.message, 
      data: null, 
      _networkError: true,
      _serviceOffline: true 
    };
  }
}

function formatTime(t) { if (!t) return '-'; return new Date(t).toLocaleString('zh-CN'); }
function formatBytes(bytes) {
  if (!bytes) return '0 B';
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024*1024) return (bytes/1024).toFixed(1) + ' KB';
  if (bytes < 1024*1024*1024) return (bytes/1024/1024).toFixed(1) + ' MB';
  return (bytes/1024/1024/1024).toFixed(2) + ' GB';
}

// ============================================
// 登录
// ============================================
function showLogin() {
  // 使用内嵌的登录页（在 index.html 中定义）
  document.querySelectorAll('.sidebar, .main, .toast, .modal-overlay').forEach(el => el.style.display = 'none');
  var loginPage = document.getElementById('loginPage');
  if (loginPage) {
    loginPage.style.display = 'flex';
  }
  // 如果内嵌脚本未绑定（如 DOMContentLoaded 已触发），确保按钮可用
  var btn = document.getElementById('loginBtn');
  if (btn && !btn._bound) {
    btn._bound = true;
    btn.addEventListener('click', doLogin);
  }
  var pass = document.getElementById('loginPass');
  if (pass && !pass._bound) {
    pass._bound = true;
    pass.addEventListener('keydown', function(e) {
      if (e.key === 'Enter' || e.keyCode === 13) doLogin();
    });
  }
}

async function doLogin() {
  var username = document.getElementById('loginUser').value.trim();
  var password = document.getElementById('loginPass').value.trim();
  var errEl = document.getElementById('loginError');
  var btn = document.getElementById('loginBtn');

  if (!username || !password) {
    errEl.textContent = '请输入用户名和密码';
    return;
  }

  btn.disabled = true;
  btn.textContent = '登录中...';
  errEl.textContent = '';

  try {
    var res = await fetch(API_BASE + '/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: username, password: password })
    });

    if (!res.ok) {
      errEl.textContent = '服务器错误: ' + res.status;
      btn.disabled = false;
      btn.textContent = '登 录';
      return;
    }

    var text = await res.text();
    var data;
    try {
      data = JSON.parse(text);
    } catch(e) {
      errEl.textContent = '响应解析失败';
      btn.disabled = false;
      btn.textContent = '登 录';
      return;
    }

    if (data.code === 0 && data.data && data.data.access_token) {
      accessToken = data.data.access_token;
      localStorage.setItem('admin_token', accessToken);
      if (data.data.refresh_token) {
        localStorage.setItem('admin_refresh', data.data.refresh_token);
      }
      currentUser = data.data.user;
      location.reload();
    } else {
      errEl.textContent = data.msg || '登录失败';
      btn.disabled = false;
      btn.textContent = '登 录';
    }
  } catch(e) {
    errEl.textContent = '网络错误: ' + e.message;
    btn.disabled = false;
    btn.textContent = '登 录';
  }
}

// ============================================
// 动态加载模块配置
// ============================================
async function loadModulesConfig() {
  try {
    const data = await apiFetch('/admin/modules');
    if (data.code === 0) {
      modulesConfig = data.data;
      renderSidebar();
    } else {
      showToast(data.msg || '加载配置失败', 'error');
    }
  } catch(e) {
    console.error('加载模块配置失败:', e);
    showToast('加载配置失败', 'error');
  }
}

// ============================================
// 动态渲染侧边栏
// ============================================
function renderSidebar() {
  const nav = document.getElementById('sidebarNav');
  if (!nav || !modulesConfig) return;

  let html = '';
  // 按 order 排序模块
  const sortedModules = Object.entries(modulesConfig)
    .sort((a, b) => (a[1].order || 0) - (b[1].order || 0));

  sortedModules.forEach(([moduleKey, module]) => {
    // 模块分组标题
    html += `<div class="sidebar-group-title">${module.icon || '📁'} ${module.name}</div>`;
    
    // 页面导航项
    if (module.pages) {
      module.pages.forEach(page => {
        const icon = page.icon || getPageIcon(page.type);
        const activeClass = currentPage === page.id ? ' active' : '';
        html += `<div class="nav-item${activeClass}" data-page="${page.id}">
          <span class="icon">${icon}</span> ${page.title}
        </div>`;
      });
    }
  });

  nav.innerHTML = html;
  
  // 绑定点击事件
  nav.querySelectorAll('.nav-item').forEach(item => {
    item.addEventListener('click', () => {
      const pageId = item.dataset.page;
      switchPage(pageId);
      if (window.innerWidth <= 950) {
        document.getElementById('sidebar').classList.remove('open');
      }
    });
  });
}

function getPageIcon(type) {
  const icons = {
    'stats': '📊',
    'table': '📋',
    'logs': '📝',
    'monitor': '🖥️',
    'overview': '📰',
    'custom': '⚙️'
  };
  return icons[type] || '📄';
}

// ============================================
// 页面切换与渲染
// ============================================
async function switchPage(pageId) {
  currentPage = pageId;
  
  // 更新导航高亮
  document.querySelectorAll('.nav-item').forEach(n => {
    n.classList.toggle('active', n.dataset.page === pageId);
  });

  // 查找页面配置
  const pageConfig = findPageConfig(pageId);
  if (!pageConfig) {
    showToast('页面未找到: ' + pageId, 'error');
    return;
  }

  // 更新标题
  document.getElementById('pageTitle').textContent = pageConfig.title || pageId;

  // 渲染页面内容
  const content = document.getElementById('contentArea');
  content.innerHTML = `<div class="loading">加载中...</div>`;

  try {
    const html = await renderPage(pageConfig);
    if (!html || html.trim() === '') {
      // 空数据 - 显示暂无内容
      content.innerHTML = `
        <div class="card">
          <div class="card-title">${pageConfig.title}</div>
          <div style="text-align:center;padding:40px;color:var(--text-secondary);">
            <div style="font-size:48px;margin-bottom:16px;">📭</div>
            <div>此页面暂无数据</div>
            <div style="margin-top:8px;font-size:13px;">请稍后刷新或联系管理员</div>
          </div>
          <div style="text-align:center;margin-top:16px;">
            <button class="btn btn-primary" onclick="switchPage('${pageId}')">🔄 刷新页面</button>
          </div>
        </div>`;
    } else {
      content.innerHTML = html;
    }
    
    // 绑定事件
    bindPageEvents(pageConfig);
  } catch(e) {
    if (e.isServiceOffline) {
      // 服务离线 - 显示服务状态卡片
      content.innerHTML = renderServiceOfflineCard(pageConfig, e);
    } else {
      // 其他错误
      content.innerHTML = `
        <div class="card">
          <div class="card-title">${pageConfig.title}</div>
          <div class="error-msg" style="padding:16px;">
            <div style="font-size:48px;margin-bottom:16px;text-align:center;">⚠️</div>
            <div style="text-align:center;margin-bottom:16px;">页面加载失败</div>
            <div style="background:var(--bg);padding:12px;border-radius:6px;font-size:13px;color:var(--text-secondary);margin-bottom:16px;">
              ${e.message}
            </div>
            <div style="text-align:center;">
              <button class="btn btn-primary" onclick="switchPage('${pageId}')">🔄 重试</button>
            </div>
          </div>
        </div>`;
    }
    console.error('页面渲染失败:', e);
  }
}

// 渲染服务离线状态卡片
function renderServiceOfflineCard(pageConfig, error) {
  // 尝试获取相关服务信息
  let serviceName = '';
  let serviceStatus = 'unknown';
  
  // 从 URL 中提取服务名
  if (error.url && error.url.includes('/proxy/')) {
    const parts = error.url.split('/proxy/');
    if (parts.length > 1) {
      serviceName = parts[1].split('/')[0];
    }
  }
  
  // 获取服务状态
  if (servicesData && servicesData[serviceName]) {
    serviceStatus = servicesData[serviceName].status || 'offline';
  }
  
  const statusIcons = {
    'online': '🟢',
    'offline': '🔴',
    'unhealthy': '🟡',
    'unknown': '⚪'
  };
  
  const statusTexts = {
    'online': '运行中',
    'offline': '离线',
    'unhealthy': '异常',
    'unknown': '未知'
  };
  
  return `
    <div class="card" style="border-left:4px solid var(--danger);">
      <div class="card-title">
        <span style="color:var(--danger);">⚠️ 服务不可用</span>
      </div>
      
      <div style="padding:24px 0;">
        <div style="text-align:center;margin-bottom:24px;">
          <div style="font-size:64px;margin-bottom:12px;">${statusIcons[serviceStatus] || '🔴'}</div>
          <div style="font-size:18px;font-weight:600;margin-bottom:8px;">
            ${serviceName ? '服务: ' + serviceName : '相关服务'} ${statusTexts[serviceStatus] || '离线'}
          </div>
          <div style="color:var(--text-secondary);font-size:14px;">
            ${error.message || '无法连接到目标服务'}
          </div>
        </div>
        
        <div style="background:var(--bg);padding:16px;border-radius:8px;margin-bottom:24px;">
          <div style="font-weight:600;margin-bottom:12px;">可能的原因：</div>
          <div style="font-size:13px;color:var(--text-secondary);line-height:1.8;">
            • 目标服务未启动或已停止<br>
            • 服务正在重启中<br>
            • 网络连接问题<br>
            • 服务负载过高，熔断器触发<br>
            • 接口地址或路径配置错误
          </div>
        </div>
        
        <div style="display:flex;gap:12px;justify-content:center;flex-wrap:wrap;">
          <button class="btn btn-primary" onclick="switchPage('${pageConfig.id}')">🔄 重试加载</button>
          <button class="btn btn-warning" onclick="refreshServices()">🔍 检查服务状态</button>
          <button class="btn btn-sm" onclick="showToast('请确保相关服务已启动', 'warning')">📖 查看文档</button>
        </div>
      </div>
      
      <div style="border-top:1px solid var(--border);padding-top:16px;margin-top:16px;">
        <div style="font-size:12px;color:var(--text-secondary);text-align:center;">
          页面: ${pageConfig.title} | URL: ${error.url || '未知'}
        </div>
      </div>
    </div>`;
}

function findPageConfig(pageId) {
  if (!modulesConfig) return null;
  for (const [key, module] of Object.entries(modulesConfig)) {
    if (module.pages) {
      const page = module.pages.find(p => p.id === pageId);
      if (page) return page;
    }
  }
  return null;
}

// ============================================
// 页面模板渲染引擎
// ============================================
async function renderPage(pageConfig) {
  const type = pageConfig.type;
  
  switch (type) {
    case 'stats':
      return renderStatsPage(pageConfig);
    case 'table':
      return renderTablePage(pageConfig);
    case 'logs':
      return renderLogsPage(pageConfig);
    case 'monitor':
      return renderMonitorPage(pageConfig);
    case 'overview':
      return renderOverviewPage(pageConfig);
    case 'custom':
      return renderCustomPage(pageConfig);
    case 'portal':
      return renderPortalPage(pageConfig);
    default:
      return `<div class="card"><div class="card-title">${pageConfig.title}</div><p>页面类型: ${type}</p></div>`;
  }
}

// ============ stats 类型：统计卡片页 ============
async function renderStatsPage(pageConfig) {
  let cardsHTML = '';
  let systemInfoHTML = '';

  // 加载数据源
  if (pageConfig.data_source && pageConfig.data_source.url) {
    const data = await fetchData(pageConfig.data_source);
    const cards = transformStatsData(data, pageConfig);
    
    if (cards && cards.length > 0) {
      cardsHTML = `<div class="stats-row">${cards.map(c => renderStatCard(c)).join('')}</div>`;
    }
    
    // 系统信息部分
    if (data) {
      systemInfoHTML = renderSystemInfo(data);
    }
  }

  // 如果有 widgets，渲染 widgets
  let widgetsHTML = '';
  if (pageConfig.widgets && pageConfig.widgets.length > 0) {
    for (const widget of pageConfig.widgets) {
      widgetsHTML += await renderWidget(widget, pageConfig);
    }
  }

  return `
    ${cardsHTML}
    ${widgetsHTML}
    ${systemInfoHTML}
  `;
}

function renderStatCard(card) {
  const color = card.color || 'blue';
  const icon = card.icon || '📊';
  return `
    <div class="stat-card">
      <div class="stat-icon ${color}">${icon}</div>
      <div>
        <div class="stat-value">${card.value ?? '-'}</div>
        <div class="stat-label">${card.label}</div>
      </div>
    </div>
  `;
}

function renderSystemInfo(data) {
  // 尝试提取系统状态信息
  const info = [];
  
  // 服务器信息
  if (data.server) {
    info.push(['运行时间', data.server.uptime || '-']);
    info.push(['Go版本', data.server.go_version || '-']);
    info.push(['协程数', data.server.goroutines || '-']);
  }
  
  // 内存信息
  if (data.memory) {
    info.push(['内存分配', formatBytes(data.memory.alloc)]);
    info.push(['内存使用', formatBytes(data.memory.used)]);
  }
  
  // 数据库信息
  if (data.database) {
    info.push(['数据库大小', formatBytes(data.database.size)]);
    info.push(['表数量', data.database.tables || '-']);
  }

  // 从 admin-core dashboard 获取的数据
  if (data.version) info.push(['版本', data.version]);
  if (data.cpu_usage) info.push(['CPU', data.cpu_usage]);
  if (data.memory_usage) info.push(['内存', data.memory_usage]);
  if (data.disk_usage) info.push(['磁盘', data.disk_usage]);
  if (data.goroutines) info.push(['协程数', data.goroutines]);
  if (data.db_connections) info.push(['DB连接', data.db_connections]);
  if (data.uptime) info.push(['运行时间', data.uptime]);

  if (info.length === 0) return '';

  return `
    <div class="card">
      <div class="card-title">系统状态</div>
      <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;font-size:14px;">
        ${info.map(([k, v]) => `<div>${k}: <strong>${v}</strong></div>`).join('')}
      </div>
    </div>
  `;
}

// ============ table 类型：CRUD 表格页 ============
async function renderTablePage(pageConfig) {
  let tableHTML = '';
  let actionBarHTML = '';
  let paginationHTML = '';

  // 操作按钮
  if (pageConfig.actions && pageConfig.actions.length > 0) {
    actionBarHTML = `<div class="action-bar">`;
    pageConfig.actions.forEach((action, idx) => {
      const btnClass = action.type === 'delete' ? 'btn-danger' : 
                       action.type === 'create' ? 'btn-success' : 'btn-primary';
      const confirmAttr = action.confirm ? `data-confirm="${action.label}"` : '';
      actionBarHTML += `<button class="btn ${btnClass}" data-action="${idx}" ${confirmAttr}>
        ${action.type === 'fetch' ? '🔄 ' : action.type === 'create' ? '➕ ' : '⚡ '}${action.label}
      </button>`;
    });
    actionBarHTML += `</div>`;
  }

  // 表格数据
  if (pageConfig.data_source && pageConfig.data_source.url) {
    const data = await fetchData(pageConfig.data_source);
    const items = extractTableData(data);
    
    if (items && items.length > 0) {
      const columns = pageConfig.columns || Object.keys(items[0]).map(k => ({ key: k, label: k }));
      
      tableHTML = `
        <div class="table-wrap">
          <table>
            <thead>
              <tr>${columns.map(c => `<th style="width:${c.width || 'auto'}">${c.label}</th>`).join('')}
              <th>操作</th>
              </tr>
            </thead>
            <tbody>
              ${items.map((row, rowIdx) => `
                <tr data-row-idx="${rowIdx}">
                  ${columns.map(col => `<td>${renderCell(row[col.key], col)}</td>`).join('')}
                  <td>${renderRowActions(pageConfig, row, rowIdx)}</td>
                </tr>
              `).join('')}
            </tbody>
          </table>
        </div>
      `;

      // 保存行数据供后续操作使用
      pageConfig._rowData = items;
      
      // 分页
      if (data.total > 0 || data.list) {
        paginationHTML = renderPagination(data);
      }
    } else {
      tableHTML = `<div class="card"><p style="text-align:center;color:var(--text-secondary);">暂无数据</p></div>`;
    }
  }

  return `
    <div class="card">
      <div class="card-title">${pageConfig.title}</div>
      ${actionBarHTML}
      ${tableHTML}
      ${paginationHTML}
    </div>
  `;
}

function renderCell(value, column) {
  if (value === null || value === undefined) return '-';
  
  const render = column.render || 'text';
  switch (render) {
    case 'tag':
      const tagColor = column.tag_map ? (column.tag_map[value] || 'default') : 'default';
      return `<span class="tag tag-${tagColor}">${value}</span>`;
    case 'boolean':
      const activeClass = value ? 'active' : 'disabled';
      const label = value ? '启用' : '禁用';
      return `<span class="tag tag-${activeClass}">${label}</span>`;
    case 'datetime':
      return formatTime(value);
    case 'number':
      return value.toLocaleString();
    case 'truncate':
      return `<span title="${value}">${String(value).length > 30 ? String(value).substring(0, 30) + '...' : value}</span>`;
    default:
      return String(value);
  }
}

function renderRowActions(pageConfig, row, rowIdx) {
  if (!pageConfig.actions) return '';

  const rowActions = pageConfig.actions.filter(a => ['edit', 'delete', 'toggle_status', 'update'].includes(a.type));
  if (rowActions.length === 0) return '';

  return rowActions.map((action) => {
    const actionIdx = pageConfig.actions.indexOf(action);
    const btnClass = action.type === 'delete' ? 'btn-danger' : 'btn-primary';
    return `<button class="btn btn-sm ${btnClass}" data-row-action="${actionIdx}" data-row-idx="${rowIdx}">${action.label}</button>`;
  }).join(' ');
}

function extractTableData(data) {
  if (!data) return [];
  if (Array.isArray(data)) return data;
  if (data.list) return data.list;
  if (data.data) {
    // data 可能是数组或包含 items/results/list 字段
    if (Array.isArray(data.data)) return data.data;
    if (data.data.items) return data.data.items;
    if (data.data.results) return data.data.results;
    if (data.data.list) return data.data.list;
    return data.data; // 返回 data 对象本身（可能是单条记录）
  }
  if (data.services) return data.services;
  if (data.items) return data.items;
  if (data.results) return data.results;
  if (data.result) return Array.isArray(data.result) ? data.result : [data.result];
  return [];
}

function renderPagination(data) {
  const total = data.total || data.count || 0;
  const pageSize = data.per_page || data.page_size || 20;
  const totalPages = Math.ceil(total / pageSize);
  
  if (totalPages <= 1) return '';
  
  return `<div class="pagination">
    <button>上一页</button>
    <span>1 / ${totalPages}</span>
    <button>下一页</button>
  </div>`;
}

// ============ logs 类型：日志查看页 ============
async function renderLogsPage(pageConfig) {
  if (!pageConfig.data_source || !pageConfig.data_source.url) {
    return `<div class="card"><p>未配置日志数据源</p></div>`;
  }

  const data = await fetchData(pageConfig.data_source);
  const items = extractTableData(data);
  const fields = pageConfig.fields || [];
  const logLevels = pageConfig.log_levels || ['DEBUG', 'INFO', 'WARNING', 'ERROR', 'FATAL'];

  let html = `
    <div class="card">
      <div class="card-title">${pageConfig.title}</div>
      <div class="search-bar">
        <select class="form-select" id="logLevelFilter">
          <option value="">全部级别</option>
          ${logLevels.map(l => `<option value="${l}">${l}</option>`).join('')}
        </select>
        <button class="btn btn-primary" onclick="switchPage('${pageConfig.id}')">🔄 刷新</button>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>${fields.map(f => `<th>${fieldLabel(f)}</th>`).join('')}</tr>
          </thead>
          <tbody>
            ${items.length > 0 ? items.slice(0, 100).map(row => `
              <tr class="log-${row.level ? row.level.toLowerCase() : ''}">
                ${fields.map(f => `<td>${row[f] ?? '-'}</td>`).join('')}
              </tr>
            `).join('') : '<tr><td colspan="' + fields.length + '" style="text-align:center;">暂无日志</td></tr>'}
          </tbody>
        </table>
      </div>
      <div class="pagination">
        <span>共 ${items.length} 条日志</span>
      </div>
    </div>
  `;

  return html;
}

function fieldLabel(field) {
  const labels = {
    'id': 'ID', 'level': '级别', 'module': '模块', 
    'message': '消息', 'log_time': '时间', 'created_at': '创建时间',
    'username': '用户', 'action': '操作', 'resource': '资源',
    'ip': 'IP', 'status': '状态', 'detail': '详情',
    'bot_name': '机器人', 'name': '名称', 'type': '类型',
    'schedule': '调度策略', 'last_run': '最后运行', 'success_rate': '成功率',
    'description': '描述', 'config_key': '配置项', 'config_value': '配置值',
    'auto_managed': '自动管理',
  };
  return labels[field] || field;
}

// ============ monitor 类型：服务监控页 ============
async function renderMonitorPage(pageConfig) {
  const data = await apiFetch('/admin/services');
  let cardsHTML = '';

  if (data.code === 0 && data.data.services) {
    // 填充全局 servicesData 变量，供其他页面使用
    servicesData = {};
    data.data.services.forEach(svc => {
      servicesData[svc.name] = {
        status: svc.status || 'unknown',
        config: svc.config,
        last_check: svc.last_check,
        registered_at: svc.registered_at
      };
    });

    cardsHTML = data.data.services.map(svc => {
      const status = svc.status || 'unknown';
      const statusConfig = {
        'online': { color: 'var(--success)', text: '运行中', dot: '●' },
        'offline': { color: 'var(--danger)', text: '离线', dot: '○' },
        'unhealthy': { color: 'var(--warning)', text: '异常', dot: '●' },
        'unknown': { color: 'var(--text-secondary)', text: '未知', dot: '○' }
      };
      const sc = statusConfig[status];
      const webURL = svc.config?.web_url || svc.config?.base_url || '';
      const openBtn = webURL ? `<button class="btn btn-sm btn-success" onclick="openService('${webURL}')">🚪 打开</button>` : '';
      
      return `
        <div class="service-card">
          <div class="service-card-header">
            <div class="service-icon">${svc.config?.icon || '📦'}</div>
            <div class="service-name">${svc.config?.name || svc.name}</div>
          </div>
          <div class="service-status" style="color:${sc.color};">
            ${sc.dot} ${sc.text}
          </div>
          <div class="service-info">
            <div>端口: ${svc.config?.port || '-'}</div>
            <div>最后检查: ${svc.last_check || '-'}</div>
            ${svc.config?.description ? '<div>' + svc.config.description + '</div>' : ''}
          </div>
          <div class="service-actions">
            <button class="btn btn-sm btn-primary" onclick="checkService('${svc.name}')">🔍 详情</button>
            ${openBtn}
            ${svc.config?.start_command ? `<button class="btn btn-sm btn-success" onclick="startService('${svc.name}')" ${status === 'online' || status === 'starting' ? 'disabled' : ''}>▶ 启动</button>` : ''}
            ${svc.config?.start_command ? `<button class="btn btn-sm btn-danger" onclick="stopService('${svc.name}')" ${status === 'offline' || status === 'unknown' ? 'disabled' : ''}>■ 停止</button>` : ''}
          </div>
        </div>
      `;
    }).join('');
  }

  return `
    <div class="card">
      <div class="card-title">服务状态监控</div>
      <div style="margin-bottom:16px;display:flex;gap:12px;align-items:center;">
        <button class="btn btn-primary" onclick="refreshServices()">🔄 刷新状态</button>
        <span style="color:var(--text-secondary);font-size:13px;">
          ${data.data?.count || 0} 个服务 | 
          在线: ${Object.values(servicesData || {}).filter(s => s.status === 'online').length} | 
          离线: ${Object.values(servicesData || {}).filter(s => s.status === 'offline').length}
        </span>
      </div>
      <div class="service-grid">
        ${cardsHTML || '<p style="text-align:center;color:var(--text-secondary);">暂未注册服务</p>'}
      </div>
      
      <div style="margin-top:24px;padding:16px;background:var(--bg);border-radius:8px;">
        <div style="font-weight:600;margin-bottom:12px;">💡 管理后台功能说明</div>
        <div style="font-size:13px;color:var(--text-secondary);line-height:1.8;">
          • <strong>万能管理</strong>：通过配置文件定义的模块和页面，无需编码即可扩展<br>
          • <strong>服务代理</strong>：统一代理到各服务的 API，支持熔断保护<br>
          • <strong>健康检查</strong>：每15秒自动检查所有服务的运行状态<br>
          • <strong>安全认证</strong>：JWT + RBAC + IP白名单 + 限流 多层保护<br>
          • <strong>动态配置</strong>：支持服务自注册、心跳检测、自动下线
        </div>
      </div>
    </div>
  `;
}

// ============ portal 类型：服务门户页 ============
async function renderPortalPage(pageConfig) {
  const data = await apiFetch('/admin/services');
  let cardsHTML = '';

  if (data.code === 0 && data.data.services) {
    cardsHTML = data.data.services.map(svc => {
      const status = svc.status || 'unknown';
      const isOnline = status === 'online';
      const webURL = svc.config?.web_url || svc.config?.base_url || '';
      const icon = svc.config?.icon || '📦';
      const name = svc.config?.name || svc.name;
      const desc = svc.config?.description || '';
      const opacity = isOnline ? '1' : '0.5';
      
      return `
        <div class="portal-card" style="opacity:${opacity};" onclick="${webURL ? 'openService(\'' + webURL + '\')' : ''}">
          <div class="portal-icon">${icon}</div>
          <div class="portal-name">${name}</div>
          <div class="portal-desc">${desc || '暂无描述'}</div>
          <div class="portal-status ${isOnline ? 'online' : 'offline'}">
            ${isOnline ? '● 可访问' : '○ 离线'}
          </div>
          ${webURL ? '<div class="portal-hint">点击打开 →</div>' : ''}
        </div>
      `;
    }).join('');
  }

  return `
    <div class="card">
      <div class="card-title">${pageConfig.title || '服务门户'}</div>
      ${pageConfig.description ? '<p style="color:var(--text-secondary);margin-bottom:16px;">' + pageConfig.description + '</p>' : ''}
      <div class="portal-grid">
        ${cardsHTML || '<p style="text-align:center;color:var(--text-secondary);">暂无服务</p>'}
      </div>
    </div>
  `;
}

function openService(url) {
  if (!url) {
    showToast('服务未配置访问地址', 'error');
    return;
  }
  showToast('正在打开: ' + url);
  const win = window.open(url, '_blank');
  if (!win) {
    // 弹窗被拦截，在新标签页打开
    window.open(url, '_blank', 'noopener');
    if (!win) {
      // 如果仍然失败，直接在当前页面跳转
      setTimeout(() => { window.location.href = url; }, 300);
    }
  }
}

async function startService(name) {
  // 弹出实时日志窗口
  showModal('启动服务 - ' + name, `
    <div id="svcLog" style="background:#1e1e1e;color:#0f0;font-family:monospace;font-size:12px;padding:12px;border-radius:6px;height:300px;overflow-y:auto;white-space:pre-wrap;"></div>
    <div id="svcStatus" style="margin-top:12px;text-align:center;font-size:14px;color:var(--text-secondary);">⏳ 正在启动...</div>
    <button class="btn btn-primary" style="margin-top:12px;width:100%;" onclick="closeModal()">关闭</button>
  `);
  const logEl = document.getElementById('svcLog');
  const statusEl = document.getElementById('svcStatus');
  logEl.textContent = '[' + new Date().toLocaleTimeString('zh-CN') + '] 正在发送启动命令...\n';

  const data = await apiFetch(`/admin/services/${name}/start`, { method: 'POST' });
  if (data.code !== 0) {
    logEl.textContent += '[错误] ' + (data.msg || '启动失败') + '\n';
    statusEl.innerHTML = '❌ 启动失败: ' + (data.msg || '');
    statusEl.style.color = 'var(--danger)';
    return;
  }

  logEl.textContent += '[' + new Date().toLocaleTimeString('zh-CN') + '] ✅ 启动命令已执行 (端口: ' + (data.data?.port || '?') + ')\n';
  logEl.textContent += '[信息] 工作目录: ' + (data.data?.work_dir || '.') + '\n';
  logEl.textContent += '[信息] 正在等待服务就绪...\n';
  statusEl.innerHTML = '⏳ 服务启动中，等待端口监听...';

  // 轮询服务状态，直到 online 或超时
  let attempts = 0;
  const maxAttempts = 10;
  const poll = async () => {
    attempts++;
    try {
      const resp = await apiFetch('/admin/services/' + name);
      if (resp.code === 0) {
        const status = resp.data?.config?.status || resp.data?.Config?.status || 'unknown';
        const now = new Date().toLocaleTimeString('zh-CN');
        if (status === 'online') {
          logEl.textContent += '[' + now + '] 🎉 服务已上线！端口监听正常\n';
          statusEl.innerHTML = '✅ 服务已成功启动！';
          statusEl.style.color = 'var(--success)';
          setTimeout(() => { closeModal(); switchPage('service-monitor'); }, 1500);
          return;
        }
        if (status === 'starting') {
          logEl.textContent += '[' + now + '] ⏳ 仍在启动中... (' + attempts + '/' + maxAttempts + ')\n';
        } else {
          logEl.textContent += '[' + now + '] 当前状态: ' + status + '\n';
        }
      }
    } catch(e) {
      logEl.textContent += '[' + new Date().toLocaleTimeString('zh-CN') + '] 轮询失败: ' + e.message + '\n';
    }
    logEl.scrollTop = logEl.scrollHeight;
    if (attempts < maxAttempts) {
      setTimeout(poll, 2000);
    } else {
      logEl.textContent += '[超时] 已轮询 ' + maxAttempts + ' 次，服务可能仍在启动中\n';
      statusEl.innerHTML = '⚠️ 服务可能还在启动中，请稍后查看状态';
      statusEl.style.color = 'var(--warning)';
      setTimeout(() => switchPage('service-monitor'), 2000);
    }
  };
  setTimeout(poll, 1500);
}

async function stopService(name) {
  if (!confirm('确定要停止服务 ' + name + ' 吗？')) return;

  showModal('停止服务 - ' + name, `
    <div id="svcLog" style="background:#1e1e1e;color:#ff0;font-family:monospace;font-size:12px;padding:12px;border-radius:6px;height:200px;overflow-y:auto;white-space:pre-wrap;"></div>
    <div id="svcStatus" style="margin-top:12px;text-align:center;font-size:14px;color:var(--text-secondary);">⏳ 正在停止...</div>
    <button class="btn btn-primary" style="margin-top:12px;width:100%;" onclick="closeModal()">关闭</button>
  `);
  const logEl = document.getElementById('svcLog');
  const statusEl = document.getElementById('svcStatus');
  logEl.textContent = '[' + new Date().toLocaleTimeString('zh-CN') + '] 正在发送停止命令...\n';

  const data = await apiFetch(`/admin/services/${name}/stop`, { method: 'POST' });
  if (data.code === 0) {
    logEl.textContent += '[' + new Date().toLocaleTimeString('zh-CN') + '] ✅ 停止命令已执行\n';
    statusEl.innerHTML = '✅ 服务已停止';
    statusEl.style.color = 'var(--success)';
    setTimeout(() => { closeModal(); switchPage('service-monitor'); }, 1200);
  } else {
    logEl.textContent += '[错误] ' + (data.msg || '停止失败') + '\n';
    statusEl.innerHTML = '❌ 停止失败';
    statusEl.style.color = 'var(--danger)';
  }
}

async function checkService(name) {
  const data = await apiFetch('/admin/services/' + name);
  if (data.code === 0) {
    const svc = data.data || {};
    // 兼容 Go JSON 小写字段（config vs Config）
    const cfg = svc.config || svc.Config || {};
    const registeredAt = svc.registered_at || svc.RegisteredAt;
    const lastHeartbeat = svc.last_heartbeat || svc.LastHeartbeat;
    const expiresAt = svc.expires_at || svc.ExpiresAt;
    const statusMap = { 'online': '<span style="color:var(--success);">● 运行中</span>', 'offline': '<span style="color:var(--danger);">○ 离线</span>', 'starting': '<span style="color:var(--warning);">◎ 启动中</span>', 'unknown': '<span style="color:var(--text-secondary);">○ 未知</span>', 'unhealthy': '<span style="color:var(--warning);">● 异常</span>' };
    const tags = (cfg.tags && cfg.tags.length) ? cfg.tags.map(t => '<span class="tag tag-primary" style="margin-right:6px;">' + t + '</span>').join('') : '-';
    const caps = (cfg.capabilities && cfg.capabilities.length) ? cfg.capabilities.join(', ') : '-';
    const fmtTime = t => t ? new Date(t).toLocaleString('zh-CN') : '-';
    showModal('服务详情 - ' + (cfg.name || name), `
      <div class="form-group"><label class="form-label">服务名</label><div style="font-weight:600;">${name}</div></div>
      <div class="form-group"><label class="form-label">图标 / 显示名</label><div>${cfg.icon || '📦'} ${cfg.name || '-'}</div></div>
      <div class="form-group"><label class="form-label">状态</label><div>${statusMap[cfg.status] || cfg.status || 'unknown'}</div></div>
      <div class="form-group"><label class="form-label">描述</label><div>${cfg.description || '-'}</div></div>
      <div class="form-group"><label class="form-label">端口</label><div>${cfg.port || '-'}</div></div>
      <div class="form-group"><label class="form-label">版本</label><div>${cfg.version || '-'}</div></div>
      <div class="form-group"><label class="form-label">标签</label><div>${tags}</div></div>
      <div class="form-group"><label class="form-label">能力</label><div>${caps}</div></div>
      <div class="form-group"><label class="form-label">基础URL</label><div>${cfg.base_url ? '<a href="' + cfg.base_url + '" target="_blank" style="color:var(--link-color);">' + cfg.base_url + '</a>' : '-'}</div></div>
      <div class="form-group"><label class="form-label">Web访问</label><div>${cfg.web_url ? '<a href="' + cfg.web_url + '" target="_blank" style="color:var(--link-color);">' + cfg.web_url + '</a>' : '-'}</div></div>
      <div class="form-group"><label class="form-label">健康检查路径</label><div>${cfg.health_path || '-'}</div></div>
      <div class="form-group"><label class="form-label">工作目录</label><div style="font-family:monospace;">${cfg.work_dir || '.'}</div></div>
      <div class="form-group"><label class="form-label">启动命令</label><div style="font-family:monospace;background:var(--bg-soft);padding:8px;border-radius:6px;word-break:break-all;">${cfg.start_command || '<span style="color:var(--text-secondary);">（不支持）</span>'}</div></div>
      <div class="form-group"><label class="form-label">停止策略</label><div style="font-family:monospace;">${cfg.stop_command ? cfg.stop_command : '进程管理（kill）+ 端口回收'}</div></div>
      <div class="form-group"><label class="form-label">最后检查</label><div>${cfg.last_check || '-'}</div></div>
      <div class="form-group"><label class="form-label">注册时间</label><div>${fmtTime(registeredAt)}</div></div>
      <div class="form-group"><label class="form-label">最后心跳</label><div>${fmtTime(lastHeartbeat)}</div></div>
      <div class="form-group"><label class="form-label">到期时间</label><div>${fmtTime(expiresAt)}</div></div>
      <div style="margin-top:20px;display:flex;gap:10px;justify-content:flex-end;">
        ${cfg.web_url ? '<a class="btn btn-sm btn-success" href="' + cfg.web_url + '" target="_blank">🚪 打开页面</a>' : ''}
        ${cfg.base_url ? '<a class="btn btn-sm btn-primary" href="' + cfg.base_url + '" target="_blank">🌐 访问接口</a>' : ''}
      </div>
    `);
  } else {
    showToast(data.msg || '获取失败', 'error');
  }
}

async function refreshServices() {
  await apiFetch('/admin/services/refresh', { method: 'POST' });
  showToast('服务状态已刷新');
  switchPage('service-monitor');
}

// ============ overview 类型：概览页 ============
async function renderOverviewPage(pageConfig) {
  let widgetsHTML = '';
  
  // 渲染所有 widgets
  if (pageConfig.widgets && pageConfig.widgets.length > 0) {
    for (const widget of pageConfig.widgets) {
      widgetsHTML += await renderWidget(widget, pageConfig);
    }
  }
  
  // 如果有 data_source，也渲染统计卡片
  let cardsHTML = '';
  if (pageConfig.data_source && pageConfig.data_source.url) {
    try {
      const data = await fetchData(pageConfig.data_source);
      const cards = transformStatsData(data, pageConfig);
      if (cards && cards.length > 0) {
        cardsHTML = `<div class="stats-row">${cards.map(c => renderStatCard(c)).join('')}</div>`;
      }
    } catch(e) {
      console.error(e);
    }
  }

  return `${cardsHTML}${widgetsHTML}`;
}

// ============ custom 类型：自定义页 ============
async function renderCustomPage(pageConfig) {
  // 6 大运维模块 - 按 pageConfig.id 分发
  switch (pageConfig.id) {
    case 'firewall':    return renderFirewallPage();
    case 'ssh-blocks':  return renderSSHBlocksPage();
    case 'file-manager':return renderFileManagerPage();
    case 'crontab':     return renderCrontabPage();
    case 'terminal':    return renderTerminalPage();
    case 'log-viewer':  return renderLogViewerPage();
    case 'bot-overview': return renderBotOverviewPage();
    case 'bot-manage':   return renderBotManagePage();
    case 'bot-config':   return renderBotConfigPage();
    case 'bot-logs':     return renderBotLogsPage();
  }

  // 如果有 renderer 字段，尝试使用自定义渲染函数
  if (pageConfig.extra && pageConfig.extra.renderer && typeof window[pageConfig.extra.renderer] === 'function') {
    return await window[pageConfig.extra.renderer]();
  }

  // 默认：渲染 widgets
  let html = '';
  if (pageConfig.widgets && pageConfig.widgets.length > 0) {
    for (const widget of pageConfig.widgets) {
      html += await renderWidget(widget, pageConfig);
    }
  }
  return html || `<div class="card"><div class="card-title">${pageConfig.title}</div><p>暂无内容</p></div>`;
}

// ============================================
// Widget 渲染器
// ============================================
async function renderWidget(widget, pageConfig) {
  const type = widget.type;
  
  switch (type) {
    case 'stat-cards':
      return renderStatCardsWidget(widget);
    case 'action-group':
      return renderActionGroupWidget(widget);
    case 'data-table':
      return renderDataTableWidget(widget);
    case 'bar-chart':
    case 'line-chart':
      return renderChartWidget(widget);
    case 'service-cards':
      return renderServiceCardsWidget(widget);
    case 'log-viewer':
      return renderLogViewerWidget(widget);
    case 'system-info':
      return renderSystemInfoWidget(widget);
    case 'iframe':
      return `<iframe src="${widget.config?.url || widget.data_source}" style="width:100%;height:500px;border:none;border-radius:8px;"></iframe>`;
    default:
      return '';
  }
}

async function renderStatCardsWidget(widget) {
  if (!widget.data_source) return '';
  const data = await fetchData({ url: widget.data_source });
  const cards = transformStatsData(data, null, widget.config);
  
  if (!cards || cards.length === 0) return '';
  
  return `<div class="stats-row">${cards.map(c => renderStatCard(c)).join('')}</div>`;
}

async function renderActionGroupWidget(widget) {
  const actions = widget.config?.actions || [];
  if (actions.length === 0) return '';
  
  let html = `<div class="card"><div class="card-title">${widget.title || '操作'}</div>`;
  html += `<div class="action-bar">`;
  actions.forEach((action, idx) => {
    const btnClass = action.type === 'danger' ? 'btn-danger' : 
                     action.type === 'create' ? 'btn-success' : 'btn-primary';
    html += `<button class="btn ${btnClass}" data-widget-action="${idx}" data-action-url="${action.url}" 
      data-action-method="${action.method || 'GET'}" 
      data-action-body='${JSON.stringify(action.body || {})}'
      data-action-confirm="${action.confirm ? 'true' : 'false'}"
      data-action-danger="${action.danger ? 'true' : 'false'}">
      ${action.label}
    </button>`;
  });
  html += `</div></div>`;
  return html;
}

async function renderDataTableWidget(widget) {
  if (!widget.data_source) return '';
  const data = await fetchData({ url: widget.data_source });
  const items = extractTableData(data);
  
  if (!items || items.length === 0) {
    return `<div class="card"><div class="card-title">${widget.title}</div><p>暂无数据</p></div>`;
  }
  
  const columns = Object.keys(items[0]).map(k => ({ key: k, label: fieldLabel(k) }));
  
  return `
    <div class="card">
      <div class="card-title">${widget.title || '数据'}</div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>${columns.map(c => `<th>${c.label}</th>`).join('')}</tr>
          </thead>
          <tbody>
            ${items.map(row => `
              <tr>${columns.map(col => `<td>${row[col.key] ?? '-'}</td>`).join('')}</tr>
            `).join('')}
          </tbody>
        </table>
      </div>
    </div>
  `;
}

async function renderChartWidget(widget) {
  if (!widget.data_source) return '';
  const data = await fetchData({ url: widget.data_source });
  
  // 尝试渲染简单的柱状图
  let barsHTML = '';
  if (data && typeof data === 'object') {
    // 分析常见数据格式
    if (data.analysis && data.analysis.categories) {
      const cats = data.analysis.categories;
      const colors = ['#1890ff', '#52c41a', '#faad14', '#ff4d4f', '#722ed1', '#13c2c2'];
      const maxValue = Math.max(...Object.values(cats), 1);
      
      barsHTML = Object.entries(cats).map(([name, value], idx) => {
        const percent = (value / maxValue * 100).toFixed(1);
        const color = colors[idx % colors.length];
        return `
          <div class="chart-bar-row">
            <div class="chart-bar-label">${name}</div>
            <div class="chart-bar-wrap">
              <div class="chart-bar" style="width:${percent}%;background:${color};"></div>
            </div>
            <div class="chart-bar-value">${value}</div>
          </div>
        `;
      }).join('');
    }
  }
  
  return `
    <div class="card">
      <div class="card-title">${widget.title || '分析结果'}</div>
      <div class="chart-container">
        ${barsHTML || '<p style="text-align:center;color:var(--text-secondary);">暂无图表数据</p>'}
      </div>
    </div>
  `;
}

async function renderServiceCardsWidget(widget) {
  return renderMonitorPage({ id: 'service-monitor' });
}

async function renderLogViewerWidget(widget) {
  return renderLogsPage({ data_source: { url: widget.data_source }, fields: ['level', 'module', 'message', 'time'] });
}

async function renderSystemInfoWidget(widget) {
  if (!widget.data_source) return '';
  const data = await fetchData({ url: widget.data_source });
  return renderSystemInfo(data);
}

// ============================================
// 数据获取与转换
// ============================================

// 自定义错误类型 - 区分服务离线和空数据
class ServiceOfflineError extends Error {
  constructor(message, url) {
    super(message);
    this.name = 'ServiceOfflineError';
    this.url = url;
    this.isServiceOffline = true;
  }
}

async function fetchData(dataSource) {
  if (!dataSource || !dataSource.url) return null;
  
  let url = dataSource.url;
  let method = dataSource.method || 'GET';
  let body = dataSource.body;
  
  const options = { method };
  if (body) {
    options.headers = { 'Content-Type': 'application/json' };
    options.body = JSON.stringify(body);
  }
  
  try {
    const response = await apiFetch(url, options);
    
    // 检查服务是否离线
    if (response._serviceOffline || response.code === 503) {
      throw new ServiceOfflineError(
        response.msg || '服务暂时不可用', 
        url
      );
    }
    
    // 返回数据
    return response.code === 0 ? response.data : null;
  } catch(e) {
    if (e instanceof ServiceOfflineError) {
      throw e; // 重新抛出服务离线错误
    }
    console.warn('数据获取失败:', url, e);
    throw new ServiceOfflineError(e.message || '请求失败', url);
  }
}

function transformStatsData(data, pageConfig, widgetConfig) {
  if (!data) return [];
  
  const transform = widgetConfig?.transform || pageConfig?.data_source?.transform;
  
  // 算法机器人总览转换
  if (transform === 'bot_overview' || data.total !== undefined && data.running !== undefined) {
    return [
      { key: 'total', label: '机器人总数', value: data.total || 0, icon: '🤖', color: 'blue' },
      { key: 'running', label: '运行中', value: data.running || 0, icon: '✅', color: 'green' },
      { key: 'stopped', label: '已停止', value: data.stopped || 0, icon: '⏹️', color: 'orange' },
      { key: 'error', label: '异常', value: data.error || 0, icon: '⚠️', color: 'red' },
      { key: 'crawler', label: '爬虫', value: data.crawler || 0, icon: '🕷️', color: 'purple' },
      { key: 'analyzer', label: '分析器', value: data.analyzer || 0, icon: '🔬', color: 'cyan' },
    ];
  }
  
  // 新闻概览转换
  if (transform === 'news_overview' || data.news) {
    const news = data.news || {};
    return [
      { key: 'total', label: '新闻总数', value: news.total_count || news.total || 0, icon: '📰', color: 'blue' },
      { key: 'today', label: '今日新增', value: news.today_count || 0, icon: '🆕', color: 'green' },
      { key: 'sources', label: '新闻源', value: news.source_count || 0, icon: '📡', color: 'orange' },
      { key: 'analyzed', label: '已分析', value: data.analysis?.analyzed_count || 0, icon: '✅', color: 'purple' },
      { key: 'memory', label: '内存使用', value: formatBytes(data.memory?.alloc), icon: '💾', color: 'red' },
      { key: 'uptime', label: '运行时间', value: data.server?.uptime || '-', icon: '⏱️', color: 'cyan' },
    ];
  }
  
  // Dashboard 数据转换
  if (data.total_users !== undefined && data.server_time !== undefined) {
    return [
      { key: 'total_users', label: '总用户数', value: data.total_users, icon: '👥', color: 'blue' },
      { key: 'today_new', label: '今日新增', value: data.today_new, icon: '🆕', color: 'green' },
      { key: 'logins', label: '总登录次数', value: data.total_logins, icon: '🔑', color: 'orange' },
      { key: 'regs', label: '总注册数', value: data.total_regs, icon: '📝', color: 'red' },
    ];
  }

  // 用户中心统计转换 (格式1: /native/user/stats - 详细版)
  if (data.active_users !== undefined && data.online_users !== undefined) {
    return [
      { key: 'active_users', label: '活跃用户', value: data.active_users, icon: '👥', color: 'blue' },
      { key: 'online_users', label: '在线用户', value: data.online_users, icon: '🟢', color: 'green' },
      { key: 'today_new', label: '今日新增', value: data.today_new, icon: '🆕', color: 'orange' },
      { key: 'total_users', label: '总用户数', value: data.total_users, icon: '📊', color: 'purple' },
      { key: 'total_logins', label: '总登录次数', value: data.total_logins, icon: '🔑', color: 'cyan' },
      { key: 'total_regs', label: '总注册数', value: data.total_regs, icon: '📝', color: 'red' },
    ];
  }

  // 用户中心统计转换 (格式2: /admin/users/stats - 简洁版)
  if (data.active !== undefined && data.total !== undefined && !data.server_time) {
    return [
      { key: 'active', label: '活跃用户', value: data.active, icon: '👥', color: 'blue' },
      { key: 'disabled', label: '已禁用', value: data.disabled || 0, icon: '🚫', color: 'red' },
      { key: 'today_new', label: '今日新增', value: data.today_new || 0, icon: '🆕', color: 'green' },
      { key: 'total', label: '总用户数', value: data.total, icon: '📊', color: 'purple' },
    ];
  }

  // 搜索引擎统计转换
  if (data.engine_status !== undefined && data.index_count !== undefined) {
    return [
      { key: 'engine_status', label: '引擎状态', value: data.engine_status === 'online' ? '在线' : '离线', icon: '🟢', color: data.engine_status === 'online' ? 'green' : 'red' },
      { key: 'index_count', label: '索引数量', value: data.index_count, icon: '📚', color: 'blue' },
      { key: 'query_count', label: '查询次数', value: data.query_count, icon: '🔍', color: 'orange' },
    ];
  }
  
  // 通用转换 - 自动检测统计字段
  const cards = [];
  Object.entries(data).forEach(([key, value]) => {
    if (typeof value === 'number' || typeof value === 'string') {
      cards.push({
        key,
        label: key,
        value,
        icon: '📊',
        color: 'blue'
      });
    }
  });
  
  return cards;
}

// ============================================
// 事件绑定
// ============================================
function bindPageEvents(pageConfig) {
  // 6 大运维模块 - 自定义事件绑定
  const customBinders = {
    'firewall': bindFirewallEvents,
    'ssh-blocks': bindSSHBlocksEvents,
    'file-manager': bindFileManagerEvents,
    'crontab': bindCrontabEvents,
    'terminal': bindTerminalEvents,
    'log-viewer': bindLogViewerEvents,
  };
  if (customBinders[pageConfig.id]) {
    customBinders[pageConfig.id]();
    return;
  }

  // 绑定操作按钮
  document.querySelectorAll('[data-widget-action]').forEach(btn => {
    btn.addEventListener('click', async () => {
      const url = btn.dataset.actionUrl;
      const method = btn.dataset.actionMethod;
      const body = btn.dataset.actionBody ? JSON.parse(btn.dataset.actionBody) : null;
      const needConfirm = btn.dataset.actionConfirm === 'true';
      
      if (needConfirm) {
        if (!confirm('确定执行此操作？')) return;
      }
      
      try {
        const options = { method };
        if (body && method !== 'GET') {
          options.headers = { 'Content-Type': 'application/json' };
          options.body = JSON.stringify(body);
        }
        const data = await apiFetch(url, options);
        if (data.code === 0) {
          showToast('操作成功');
          // 刷新当前页面
          switchPage(pageConfig.id);
        } else {
          showToast(data.msg || '操作失败', 'error');
        }
      } catch(e) {
        showToast('请求失败: ' + e.message, 'error');
      }
    });
  });
  
  // 绑定表格行操作
  document.querySelectorAll('[data-row-action]').forEach(btn => {
    btn.addEventListener('click', async (e) => {
      const actionIdx = parseInt(btn.dataset.rowAction);
      const rowIdx = parseInt(btn.dataset.rowIdx);
      const action = pageConfig.actions?.[actionIdx];
      if (!action) return;

      // 从保存的行数据中获取原始数据
      const rowData = pageConfig._rowData?.[rowIdx] || {};

      // 如果有 fields，显示编辑表单
      if (action.fields && action.fields.length > 0) {
        showEditModal(action, pageConfig.id, rowData);
        return;
      }

      if (action.confirm && !confirm(`确定执行 "${action.label}"？`)) return;

      try {
        let url = action.url || '';
        // 替换 :id 及其他 :field 占位符
        Object.keys(rowData).forEach(k => {
          url = url.replace(':' + k, encodeURIComponent(rowData[k]));
        });
        const options = { method: action.method || 'GET' };
        if (action.body) {
          options.headers = { 'Content-Type': 'application/json' };
          options.body = JSON.stringify(action.body);
        }
        const data = await apiFetch(url, options);
        if (data.code === 0) {
          showToast('操作成功');
          switchPage(pageConfig.id);
        } else {
          showToast(data.msg || '操作失败', 'error');
        }
      } catch(e) {
        showToast('请求失败: ' + e.message, 'error');
      }
    });
  });
  
  // 绑定 action-bar 按钮
  document.querySelectorAll('.action-bar .btn').forEach((btn, idx) => {
    if (btn.dataset.widgetAction || btn.dataset.rowAction) return; // 已绑定

    // 查找对应的 action：优先用 data-action 属性，否则按顺序
    const actionIdx = btn.dataset.action !== undefined ? parseInt(btn.dataset.action) : idx;
    const action = pageConfig.actions?.[actionIdx];
    if (!action) return;

    btn.addEventListener('click', async () => {
      if (action.confirm && !confirm(`确定执行 "${action.label}"？`)) return;
      
      if (action.fields && action.fields.length > 0) {
        // 显示表单弹窗
        showActionModal(action, pageConfig.id);
        return;
      }
      
      try {
        const options = { method: action.method || 'GET' };
        if (action.body && action.method !== 'GET') {
          options.headers = { 'Content-Type': 'application/json' };
          options.body = JSON.stringify(action.body);
        }
        const data = await apiFetch(action.url, options);
        if (data.code === 0) {
          showToast('操作成功');
          switchPage(pageConfig.id);
        } else {
          showToast(data.msg || '操作失败', 'error');
        }
      } catch(e) {
        showToast('请求失败: ' + e.message, 'error');
      }
    });
  });
}

function showActionModal(action, pageId) {
  const fieldsHtml = action.fields.map(f => {
    if (f.options) {
      return `<div class="form-group">
        <label class="form-label">${f.label}</label>
        <select class="form-select" id="field_${f.key}">
          ${f.options.map(o => `<option value="${o}">${o}</option>`).join('')}
        </select>
      </div>`;
    }
    return `<div class="form-group">
      <label class="form-label">${f.label}</label>
      <input class="form-input" id="field_${f.key}" placeholder="请输入${f.label}">
    </div>`;
  }).join('');
  
  showModal(action.label, `
    <div id="actionForm">${fieldsHtml}</div>
    <button class="btn btn-primary" style="margin-top:16px;width:100%;" onclick="submitAction('${action.url}', '${action.method || 'POST'}', '${pageId}')">执行</button>
  `);
  
  // 存储 action 信息
  window._currentAction = action;
}

async function submitAction(url, method, pageId) {
  const action = window._currentAction;
  if (!action) return;
  
  const body = {};
  action.fields.forEach(f => {
    const el = document.getElementById('field_' + f.key);
    if (el) body[f.key] = el.value;
  });
  
  try {
    const options = { method };
    options.headers = { 'Content-Type': 'application/json' };
    options.body = JSON.stringify(body);
    
    const data = await apiFetch(url, options);
    if (data.code === 0) {
      showToast('操作成功');
      closeModal();
      switchPage(pageId);
    } else {
      showToast(data.msg || '操作失败', 'error');
    }
  } catch(e) {
    showToast('请求失败: ' + e.message, 'error');
  }
}

// 显示编辑表单弹窗（预填行数据）
function showEditModal(action, pageId, rowData) {
  const fieldsHtml = action.fields.map(f => {
    const val = rowData[f.key] || '';
    if (f.options) {
      return `<div class="form-group">
        <label class="form-label">${f.label}</label>
        <select class="form-select" id="field_${f.key}">
          ${f.options.map(o => `<option value="${o}" ${val === o ? 'selected' : ''}>${o}</option>`).join('')}
        </select>
      </div>`;
    }
    return `<div class="form-group">
      <label class="form-label">${f.label}</label>
      <input class="form-input" id="field_${f.key}" value="${String(val).replace(/"/g, '&quot;')}" placeholder="请输入${f.label}">
    </div>`;
  }).join('');

  let url = action.url || '';
  Object.keys(rowData).forEach(k => {
    url = url.replace(':' + k, encodeURIComponent(rowData[k]));
  });

  showModal(action.label, `
    <div id="actionForm">${fieldsHtml}</div>
    <button class="btn btn-primary" style="margin-top:16px;width:100%;" onclick="submitEditAction('${url}', '${action.method || 'PUT'}', '${pageId}')">保存</button>
  `);

  window._currentAction = action;
}

async function submitEditAction(url, method, pageId) {
  const action = window._currentAction;
  if (!action) return;

  const body = {};
  action.fields.forEach(f => {
    const el = document.getElementById('field_' + f.key);
    if (el) body[f.key] = el.value;
  });

  try {
    const options = { method };
    options.headers = { 'Content-Type': 'application/json' };
    options.body = JSON.stringify(body);

    const data = await apiFetch(url, options);
    if (data.code === 0) {
      showToast('保存成功');
      closeModal();
      switchPage(pageId);
    } else {
      showToast(data.msg || '保存失败', 'error');
    }
  } catch(e) {
    showToast('请求失败: ' + e.message, 'error');
  }
}
function showModal(title, html) {
  const overlay = document.getElementById('modalOverlay');
  const content = document.getElementById('modalContent');
  content.innerHTML = `
    <div class="modal-header">${title}<button class="modal-close" onclick="closeModal()">✕</button></div>
    <div>${html}</div>
  `;
  overlay.classList.add('show');
}

function closeModal() {
  document.getElementById('modalOverlay').classList.remove('show');
}

// ============================================
// 初始化
// ============================================
function initApp() {
  // 恢复主题
  if (localStorage.getItem('admin_theme') === 'dark') {
    document.documentElement.setAttribute('data-theme', 'dark');
  }

  // 检查登录状态
  if (!accessToken) {
    showLogin();
    return;
  }

  // 隐藏登录页，显示主内容
  var loginPage = document.getElementById('loginPage');
  if (loginPage) loginPage.style.display = 'none';
  document.querySelectorAll('.sidebar, .main, .toast, .modal-overlay').forEach(el => el.style.display = '');

  // 绑定事件
  document.getElementById('menuToggle').addEventListener('click', function() {
    document.getElementById('sidebar').classList.toggle('open');
  });

  document.getElementById('themeToggle').addEventListener('click', function() {
    var html = document.documentElement;
    var theme = html.getAttribute('data-theme') === 'dark' ? '' : 'dark';
    html.setAttribute('data-theme', theme);
    localStorage.setItem('admin_theme', theme);
  });

  document.getElementById('logoutBtn').addEventListener('click', async function() {
    await apiFetch('/auth/logout', { method: 'POST' });
    localStorage.removeItem('admin_token');
    localStorage.removeItem('admin_refresh');
    accessToken = '';
    showLogin();
  });

  document.getElementById('modalOverlay').addEventListener('click', function(e) {
    if (e.target.id === 'modalOverlay') closeModal();
  });

  // 加载模块配置并渲染首页
  loadModulesConfig().then(function() {
    if (modulesConfig) {
      var firstModule = Object.values(modulesConfig)[0];
      if (firstModule && firstModule.pages && firstModule.pages.length > 0) {
        switchPage(firstModule.pages[0].id);
      }
    }
  });
}

// ============================================
// 将所有内联事件处理函数绑定到 window，确保 onclick 可访问
// ============================================
window.checkService = checkService;
window.startService = startService;
window.stopService = stopService;
window.openService = openService;
window.refreshServices = refreshServices;
window.switchPage = switchPage;
window.closeModal = closeModal;
window.showToast = showToast;
window.submitAction = submitAction;
window.submitEditAction = submitEditAction;

// 全局错误处理 - 捕获未处理的异常并显示给用户
if (!window.__ADMIN_APP_LOADED__) {
  window.__ADMIN_APP_LOADED__ = true;
  
  window.addEventListener('error', function(e) {
    console.error('[全局错误]', e.error || e.message);
    showToast('操作出错: ' + (e.error?.message || e.message || '未知错误'), 'error');
  });

  // 启动应用
  initApp();
}

// ============================================
// ===== 6 大运维模块渲染器（对标宝塔） =====
// ============================================

// V2 API 调用辅助（直接使用 /api/v2 绝对路径）
async function apiV2(path, options = {}) {
  const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
  if (accessToken) headers['Authorization'] = `Bearer ${accessToken}`;
  
  try {
    const res = await fetch('/api/v2' + path, { ...options, headers });
    
    let data;
    const text = await res.text();
    try {
      data = JSON.parse(text);
    } catch (e) {
      data = { code: res.status, msg: '非JSON响应', data: null };
    }
    
    if (data.code === 401) { 
      showToast('登录已过期，请重新登录', 'error'); 
      setTimeout(() => location.reload(), 1500); 
    }
    
    return data;
  } catch (e) {
    return { code: 500, msg: '网络错误: ' + e.message, data: null };
  }
}

// ============ 1. 防火墙管理 ============
async function renderFirewallPage() {
  const data = await apiV2('/firewall/rules');
  const rules = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">防火墙规则
        <button class="btn btn-primary btn-sm" style="float:right;" onclick="showFirewallModal()">+ 添加规则</button>
      </div>
      <table>
        <thead><tr><th>端口</th><th>协议</th><th>策略</th><th>来源</th><th>状态</th><th>操作</th></tr></thead>
        <tbody>
          ${rules.length === 0 ? '<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">暂无规则</td></tr>' : rules.map(r => `
            <tr>
              <td>${r.port}</td>
              <td>${r.protocol}</td>
              <td><span class="tag ${r.action === 'ACCEPT' ? 'tag-success' : 'tag-danger'}">${r.action}</span></td>
              <td>${r.source || '0.0.0.0/0'}</td>
              <td><span class="tag ${r.enabled ? 'tag-success' : 'tag-default'}">${r.enabled ? '生效' : '禁用'}</span></td>
              <td>
                <button class="btn btn-sm" onclick="toggleFirewallRule(${r.id})">${r.enabled ? '禁用' : '启用'}</button>
                <button class="btn btn-sm btn-danger" onclick="deleteFirewallRule(${r.id})">删除</button>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

function bindFirewallEvents() {}

function showFirewallModal() {
  showModal('添加防火墙规则', `
    <div class="form-group"><label class="form-label">端口</label><input class="form-input" id="fw_port" placeholder="8080"></div>
    <div class="form-group"><label class="form-label">协议</label><select class="form-select" id="fw_protocol"><option value="tcp">TCP</option><option value="udp">UDP</option></select></div>
    <div class="form-group"><label class="form-label">策略</label><select class="form-select" id="fw_action"><option value="ACCEPT">放行</option><option value="DROP">禁止</option></select></div>
    <div class="form-group"><label class="form-label">来源 IP（可选）</label><input class="form-input" id="fw_source" placeholder="0.0.0.0/0"></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitFirewallRule()">添加</button>
  `);
}

async function submitFirewallRule() {
  const body = {
    port: parseInt(document.getElementById('fw_port').value),
    protocol: document.getElementById('fw_protocol').value,
    action: document.getElementById('fw_action').value,
    source: document.getElementById('fw_source').value,
  };
  if (!body.port) { showToast('请输入端口', 'error'); return; }
  const data = await apiV2('/firewall/rules', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('规则已添加'); closeModal(); switchPage('firewall'); }
  else { showToast(data.msg || '添加失败', 'error'); }
}

async function toggleFirewallRule(id) {
  const data = await apiV2('/firewall/rules/' + id + '/toggle', { method: 'PUT' });
  if (data.code === 0) { showToast('已切换'); switchPage('firewall'); }
  else { showToast(data.msg || '操作失败', 'error'); }
}

async function deleteFirewallRule(id) {
  if (!confirm('确定删除此规则？')) return;
  const data = await apiV2('/firewall/rules/' + id, { method: 'DELETE' });
  if (data.code === 0) { showToast('已删除'); switchPage('firewall'); }
  else { showToast(data.msg || '删除失败', 'error'); }
}

// ============ 2. SSH 封禁列表 ============
async function renderSSHBlocksPage() {
  const data = await apiV2('/firewall/blocked');
  const list = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">SSH 爆破封禁列表</div>
      <table>
        <thead><tr><th>IP 地址</th><th>过期时间</th><th>剩余（秒）</th><th>操作</th></tr></thead>
        <tbody>
          ${list.length === 0 ? '<tr><td colspan="4" style="text-align:center;color:var(--text-secondary);">暂无封禁 IP</td></tr>' : list.map(b => `
            <tr>
              <td>${b.ip}</td>
              <td>${b.expires}</td>
              <td>${b.remaining}</td>
              <td><button class="btn btn-sm btn-danger" onclick="unblockIP('${b.ip}')">解封</button></td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

function bindSSHBlocksEvents() {}

async function unblockIP(ip) {
  if (!confirm('确定解封 ' + ip + '？')) return;
  const data = await apiV2('/firewall/blocked/' + encodeURIComponent(ip), { method: 'DELETE' });
  if (data.code === 0) { showToast('已解封'); switchPage('ssh-blocks'); }
  else { showToast(data.msg || '操作失败', 'error'); }
}

// ============ 3. 文件管理器 ============
var _fileCurrentPath = '/';

async function renderFileManagerPage() {
  const data = await apiV2('/files/tree?path=' + encodeURIComponent(_fileCurrentPath) + '&depth=1');
  const items = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">文件管理器
        <span style="margin-left:12px;font-size:13px;color:var(--text-secondary);">${_fileCurrentPath}</span>
        <button class="btn btn-sm" style="float:right;margin-left:4px;" onclick="fileGoUp()">↑ 上级</button>
        <button class="btn btn-primary btn-sm" style="float:right;" onclick="showMkdirModal()">+ 新建文件夹</button>
      </div>
      <table>
        <thead><tr><th>名称</th><th>大小</th><th>修改时间</th><th>操作</th></tr></thead>
        <tbody>
          ${items.length === 0 ? '<tr><td colspan="4" style="text-align:center;color:var(--text-secondary);">空目录</td></tr>' : items.map(f => `
            <tr>
              <td>${f.is_dir ? '📁' : '📄'} <a href="javascript:void(0)" onclick="${f.is_dir ? `fileOpenDir('${f.path}')` : `fileOpenFile('${f.path}')`}" style="color:var(--primary);cursor:pointer;">${f.name}</a></td>
              <td>${f.is_dir ? '-' : formatBytes(f.size)}</td>
              <td>${formatTime(f.mod_time)}</td>
              <td>
                ${!f.is_dir ? `<button class="btn btn-sm" onclick="fileOpenFile('${f.path}')">编辑</button>` : ''}
                <button class="btn btn-sm btn-danger" onclick="fileDelete('${f.path}')">删除</button>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

function bindFileManagerEvents() {}

function fileGoUp() {
  const parts = _fileCurrentPath.split('/').filter(Boolean);
  parts.pop();
  _fileCurrentPath = '/' + parts.join('/');
  if (_fileCurrentPath === '/') _fileCurrentPath = '/';
  switchPage('file-manager');
}

function fileOpenDir(path) {
  _fileCurrentPath = path;
  switchPage('file-manager');
}

async function fileOpenFile(path) {
  const data = await apiV2('/files/read?path=' + encodeURIComponent(path));
  if (data.code !== 0) { showToast(data.msg || '读取失败', 'error'); return; }
  const content = data.data || '';
  showModal('编辑文件: ' + path, `
    <textarea class="form-input" id="fileEditor" style="width:100%;height:400px;font-family:monospace;font-size:13px;">${content.replace(/</g,'&lt;').replace(/>/g,'&gt;')}</textarea>
    <button class="btn btn-primary" style="margin-top:8px;width:100%;" onclick="fileSave('${path}')">保存</button>
  `);
}

async function fileSave(path) {
  const content = document.getElementById('fileEditor').value;
  const data = await apiV2('/files/write', { method: 'POST', body: JSON.stringify({ path, content }) });
  if (data.code === 0) { showToast('已保存'); closeModal(); }
  else { showToast(data.msg || '保存失败', 'error'); }
}

async function fileDelete(path) {
  if (!confirm('确定删除 ' + path + '？')) return;
  const data = await apiV2('/files?path=' + encodeURIComponent(path), { method: 'DELETE' });
  if (data.code === 0) { showToast('已删除'); switchPage('file-manager'); }
  else { showToast(data.msg || '删除失败', 'error'); }
}

function showMkdirModal() {
  showModal('新建文件夹', `
    <div class="form-group"><label class="form-label">文件夹名称</label><input class="form-input" id="mkdir_name" placeholder="new_folder"></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitMkdir()">创建</button>
  `);
}

async function submitMkdir() {
  const name = document.getElementById('mkdir_name').value;
  if (!name) { showToast('请输入名称', 'error'); return; }
  const path = _fileCurrentPath.replace(/\/$/, '') + '/' + name;
  const data = await apiV2('/files/mkdir', { method: 'POST', body: JSON.stringify({ path }) });
  if (data.code === 0) { showToast('已创建'); closeModal(); switchPage('file-manager'); }
  else { showToast(data.msg || '创建失败', 'error'); }
}

// ============ 5. 计划任务 ============
async function renderCrontabPage() {
  const data = await apiV2('/crontabs');
  const list = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">计划任务
        <button class="btn btn-primary btn-sm" style="float:right;" onclick="showCrontabModal()">+ 新建任务</button>
      </div>
      <table>
        <thead><tr><th>名称</th><th>类型</th><th>Cron 表达式</th><th>上次状态</th><th>操作</th></tr></thead>
        <tbody>
          ${list.length === 0 ? '<tr><td colspan="5" style="text-align:center;color:var(--text-secondary);">暂无任务</td></tr>' : list.map(t => `
            <tr>
              <td>${t.name} ${t.enabled ? '' : '<span class="tag tag-default">禁用</span>'}</td>
              <td>${t.type}</td>
              <td style="font-family:monospace;">${t.expression}</td>
              <td><span class="tag ${t.last_status === 'success' ? 'tag-success' : t.last_status === 'failed' ? 'tag-danger' : 'tag-default'}">${t.last_status || '—'}</span></td>
              <td>
                <button class="btn btn-sm" onclick="triggerCrontab(${t.id})">执行</button>
                <button class="btn btn-sm" onclick="showCrontabLogs(${t.id})">日志</button>
                <button class="btn btn-sm btn-danger" onclick="deleteCrontab(${t.id})">删除</button>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

function bindCrontabEvents() {}

function showCrontabModal() {
  showModal('新建计划任务', `
    <div class="form-group"><label class="form-label">任务名称</label><input class="form-input" id="ct_name" placeholder="每日备份"></div>
    <div class="form-group"><label class="form-label">类型</label><select class="form-select" id="ct_type"><option value="shell">Shell命令</option><option value="backup-db">数据库备份</option><option value="backup-site">网站备份</option></select></div>
    <div class="form-group"><label class="form-label">Cron 表达式</label><input class="form-input" id="ct_expr" placeholder="0 3 * * * *" oninput="translateCron(this.value)"></div>
    <div id="ct_translated" style="font-size:12px;color:var(--text-secondary);margin-bottom:8px;"></div>
    <div class="form-group"><label class="form-label">命令/脚本</label><textarea class="form-input" id="ct_cmd" rows="3" placeholder="echo hello"></textarea></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitCrontab()">创建</button>
  `);
}

async function translateCron(expr) {
  if (!expr) return;
  const data = await apiV2('/crontabs/translate?expr=' + encodeURIComponent(expr));
  const el = document.getElementById('ct_translated');
  if (el && data.code === 0 && data.data) el.textContent = '中文: ' + (data.data.chinese || '—');
}

async function submitCrontab() {
  const body = {
    name: document.getElementById('ct_name').value,
    type: document.getElementById('ct_type').value,
    expression: document.getElementById('ct_expr').value,
    command: document.getElementById('ct_cmd').value,
    enabled: true,
  };
  if (!body.name || !body.expression) { showToast('请填写必填项', 'error'); return; }
  const data = await apiV2('/crontabs', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('已创建'); closeModal(); switchPage('crontab'); }
  else { showToast(data.msg || '创建失败', 'error'); }
}

async function triggerCrontab(id) {
  const data = await apiV2('/crontabs/' + id + '/trigger', { method: 'POST' });
  if (data.code === 0) { showToast('已触发执行'); switchPage('crontab'); }
  else { showToast(data.msg || '触发失败', 'error'); }
}

async function showCrontabLogs(id) {
  const data = await apiV2('/crontabs/' + id + '/logs');
  const logs = data.code === 0 ? (data.data || []) : [];
  showModal('执行日志', `
    <table><thead><tr><th>状态</th><th>耗时</th><th>开始时间</th><th>输出</th></tr></thead>
    <tbody>${logs.map(l => `<tr>
      <td><span class="tag ${l.status === 'success' ? 'tag-success' : 'tag-danger'}">${l.status}</span></td>
      <td>${(l.duration / 1000).toFixed(2)}s</td>
      <td>${formatTime(l.started_at)}</td>
      <td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;">${(l.output || '').slice(0, 80)}</td>
    </tr>`).join('')}</tbody></table>
  `);
}

async function deleteCrontab(id) {
  if (!confirm('确定删除此任务？')) return;
  const data = await apiV2('/crontabs/' + id, { method: 'DELETE' });
  if (data.code === 0) { showToast('已删除'); switchPage('crontab'); }
  else { showToast(data.msg || '删除失败', 'error'); }
}

// ============ 6. Web 终端 ============
var _termWS = null;
var _termSimMode = false;

async function renderTerminalPage() {
  const data = await apiV2('/terminal/sessions');
  const sessions = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">Web 终端
        <div style="float:right;display:flex;gap:8px;">
          <button class="btn btn-primary btn-sm" onclick="openTerminal()">+ 打开终端</button>
          <button class="btn btn-sm" onclick="toggleSimMode()">${_termSimMode ? '切换到远程模式' : '切换到模拟模式'}</button>
          <button class="btn btn-sm" onclick="clearTerminal()">清屏</button>
        </div>
      </div>
      <div style="background:#1a1a2e;color:#0f0;font-family:monospace;font-size:13px;padding:12px;border-radius:6px;height:400px;overflow-y:auto;white-space:pre-wrap;" id="termOutput">
<div style="color:#f7b731;">╔══════════════════════════════════════════╗</div>
<div style="color:#f7b731;">║     市舶司管理后台 - Web 终端              ║</div>
<div style="color:#f7b731;">╠══════════════════════════════════════════╣</div>
<div>系统: ${navigator.platform || 'Unknown'}</div>
<div>时间: ${new Date().toLocaleString('zh-CN')}</div>
<div style="color:#26de81;">就绪。输入 help 查看可用命令。</div>
<div style="color:#555;">提示: Windows 环境下可使用模拟模式体验终端功能</div>
      </div>
      <div style="display:flex;gap:4px;margin-top:8px;">
        <span style="color:var(--text-secondary);line-height:36px;">$</span>
        <input class="form-input" id="termInput" placeholder="输入命令回车执行 (help 查看帮助)" style="font-family:monospace;" onkeydown="if(event.key==='Enter')sendTermCmd(this.value,this)">
      </div>
    </div>
    <div class="card">
      <div class="card-title">活动会话</div>
      <table><thead><tr><th>ID</th><th>类型</th><th>用户</th><th>连接时间</th><th>操作</th></tr></thead>
      <tbody>${sessions.length === 0 ? '<tr><td colspan="5" style="text-align:center;color:var(--text-secondary);">暂无活动会话</td></tr>' : sessions.map(s => `<tr>
        <td>${s.id}</td><td>${s.type}</td><td>${s.user}</td><td>${formatTime(s.started)}</td>
        <td><button class="btn btn-sm btn-danger" onclick="closeTerminalSession('${s.id}')">关闭</button></td>
      </tr>`).join('')}</tbody></table>
    </div>`;
}

function bindTerminalEvents() {}

function toggleSimMode() {
  _termSimMode = !_termSimMode;
  const out = document.getElementById('termOutput');
  if (out) {
    out.innerHTML += '<div style="color:' + (_termSimMode ? '#f7b731' : '#26de81') + ';">[' + new Date().toLocaleTimeString() + '] ' + (_termSimMode ? '已切换到模拟模式' : '已切换到远程模式') + '</div>';
    out.scrollTop = out.scrollHeight;
  }
  showToast(_termSimMode ? '模拟模式已启用' : '远程模式已启用');
}

function openTerminal() {
  if (_termWS) { try { _termWS.close(); } catch(e) {} }
  const out = document.getElementById('termOutput');

  // 如果已在模拟模式，直接启用模拟
  if (_termSimMode) {
    out.innerHTML += '<div style="color:#26de81;">[模拟模式] 终端已就绪\n</div>';
    out.scrollTop = out.scrollHeight;
    return;
  }

  try {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const token = localStorage.getItem('admin_token');
    _termWS = new WebSocket(`${proto}//${location.host}/api/v2/terminal/ws?token=${encodeURIComponent(token)}`);
    _termWS.onopen = () => { out.innerHTML += '<div style="color:#26de81;">✅ 远程终端已连接</div>'; out.scrollTop = out.scrollHeight; };
    _termWS.onmessage = (ev) => {
      try {
        const msg = JSON.parse(ev.data);
        if (msg.type === 'output') out.innerHTML += msg.data;
        else if (msg.type === 'error') {
          out.innerHTML += '<div style="color:#fc5c65;">❌ ' + msg.data + '</div>';
          out.innerHTML += '<div style="color:#f7b731;">提示: 远程终端连接失败，自动切换到模拟模式</div>';
          _termSimMode = true;
        }
      } catch(e) { out.innerHTML += ev.data; }
      out.scrollTop = out.scrollHeight;
    };
    _termWS.onclose = () => {
      out.innerHTML += '<div style="color:#fd9644;">🔌 远程连接已关闭，切换到模拟模式</div>';
      _termSimMode = true;
      out.scrollTop = out.scrollHeight;
    };
    _termWS.onerror = () => {
      out.innerHTML += '<div style="color:#fc5c65;">❌ 远程连接错误，切换到模拟模式</div>';
      _termSimMode = true;
      out.scrollTop = out.scrollHeight;
    };

    // 超时后自动切换到模拟模式
    setTimeout(() => {
      if (_termWS && _termWS.readyState !== 1) {
        out.innerHTML += '<div style="color:#f7b731;">⚠️ 连接超时，切换到模拟模式</div>';
        _termSimMode = true;
        out.scrollTop = out.scrollHeight;
      }
    }, 3000);
  } catch(e) {
    out.innerHTML += '<div style="color:#fc5c65;">❌ 无法建立 WebSocket 连接</div>';
    out.innerHTML += '<div style="color:#f7b731;">提示: 自动切换到模拟模式</div>';
    _termSimMode = true;
    out.scrollTop = out.scrollHeight;
  }
}

function clearTerminal() {
  const out = document.getElementById('termOutput');
  if (out) {
    out.innerHTML = '<div style="color:#26de81;">终端已清空。输入 help 查看可用命令。</div>';
  }
}

// 模拟命令处理
function handleSimCommand(cmd) {
  const out = document.getElementById('termOutput');
  const parts = cmd.trim().split(/\s+/);
  const command = parts[0].toLowerCase();
  const args = parts.slice(1);
  const now = new Date().toLocaleString('zh-CN');

  switch (command) {
    case 'help':
      out.innerHTML += '<div style="color:#26de81;">可用命令:</div>' +
        '<div>  help        - 显示帮助</div>' +
        '<div>  status     - 系统状态</div>' +
        '<div>  info       - 系统信息</div>' +
        '<div>  date       - 当前时间</div>' +
        '<div>  echo [msg] - 回显消息</div>' +
        '<div>  bot [id]   - 查看机器人状态</div>' +
        '<div>  bots       - 列出所有机器人</div>' +
        '<div>  bot-start [id] - 启动机器人</div>' +
        '<div>  bot-stop [id]  - 停止机器人</div>' +
        '<div>  logs [n]   - 查看最近日志</div>' +
        '<div>  net        - 网络状态</div>' +
        '<div>  cpu        - CPU 信息</div>' +
        '<div>  mem        - 内存信息</div>' +
        '<div>  clear      - 清屏</div>' +
        '<div>  exit       - 退出模拟模式</div>';
      break;
    case 'status':
      out.innerHTML += `<div style="color:#4ecdc4;">[系统状态]</div>` +
        `<div>CPU: ${(20 + Math.random() * 30).toFixed(1)}% (模拟)</div>` +
        `<div>内存: ${(40 + Math.random() * 20).toFixed(1)}% (模拟)</div>` +
        `<div>磁盘: 65.2% (模拟)</div>` +
        `<div>运行时间: ${Math.floor(Math.random() * 720)}小时 (模拟)</div>`;
      break;
    case 'info':
      out.innerHTML += `<div style="color:#4ecdc4;">[系统信息]</div>` +
        `<div>操作系统: Windows ${navigator.platform}</div>` +
        `<div>浏览器: ${navigator.userAgent.split(')')[0].split('(')[1] || 'Unknown'}</div>` +
        `<div>窗口大小: ${window.innerWidth}x${window.innerHeight}</div>` +
        `<div>时区: ${Intl.DateTimeFormat().resolvedOptions().timeZone}</div>`;
      break;
    case 'date':
      out.innerHTML += `<div>${now}</div>`;
      break;
    case 'echo':
      out.innerHTML += `<div>${args.join(' ')}</div>`;
      break;
    case 'bot':
      if (args.length === 0) {
        out.innerHTML += '<div style="color:#fc5c65;">用法: bot [id] - 查看机器人状态</div>';
      } else {
        const botId = args[0];
        out.innerHTML += `<div style="color:#4ecdc4;">[机器人 #${botId}]</div>`;
        out.innerHTML += `<div>状态: ${Math.random() > 0.3 ? '运行中' : '已停止'}</div>`;
        out.innerHTML += `<div>运行次数: ${Math.floor(Math.random() * 1000)}</div>`;
        out.innerHTML += `<div>成功率: ${(85 + Math.random() * 14).toFixed(1)}%</div>`;
        out.innerHTML += `<div>CPU 占用: ${(10 + Math.random() * 40).toFixed(1)}%</div>`;
      }
      break;
    case 'bots':
      out.innerHTML += '<div style="color:#4ecdc4;">[机器人列表]</div>' +
        '<div>1. 🕷️ 新闻爬虫-通用     [已停止]</div>' +
        '<div>2. 📊 舆情分析器       [已停止]</div>' +
        '<div>3. ⏰ 智能调度器       [已停止]</div>' +
        '<div>4. 🔔 异常通知器       [已停止]</div>' +
        '<div>5. 🛡️ 网络安全监控     [已停止]</div>' +
        '<div>6. 🧠 AI智能助手       [已停止]</div>' +
        '<div style="color:#f7b731;">使用 bot-start [id] 启动机器人</div>';
      break;
    case 'bot-start':
    case 'botstart':
      if (args.length === 0) {
        out.innerHTML += '<div style="color:#fc5c65;">用法: bot-start [id]</div>';
      } else {
        out.innerHTML += `<div style="color:#26de81;">✅ 机器人 #${args[0]} 已启动（模拟）</div>`;
      }
      break;
    case 'bot-stop':
    case 'botstop':
      if (args.length === 0) {
        out.innerHTML += '<div style="color:#fc5c65;">用法: bot-stop [id]</div>';
      } else {
        out.innerHTML += `<div style="color:#fd9644;">⏹️ 机器人 #${args[0]} 已停止（模拟）</div>`;
      }
      break;
    case 'logs':
      out.innerHTML += '<div style="color:#4ecdc4;">[最近日志]</div>' +
        '<div>[INFO] 系统启动完成</div>' +
        '<div>[INFO] 连接到数据库</div>' +
        '<div>[WARNING] 检测到可疑登录尝试</div>' +
        '<div>[INFO] 防火墙规则已加载</div>' +
        '<div>[INFO] 6 个机器人已就绪</div>' +
        '<div>[INFO] 系统监控已启动</div>';
      break;
    case 'net':
      out.innerHTML += '<div style="color:#4ecdc4;">[网络状态]</div>' +
        '<div>接口 | 状态 | 速度</div>' +
        '<div>eth0 | 正常 | 1000Mbps (模拟)</div>' +
        '<div>流量: ↓ 12.3MB/s ↑ 5.1MB/s (模拟)</div>';
      break;
    case 'cpu':
      out.innerHTML += '<div style="color:#4ecdc4;">[CPU 信息]</div>' +
        '<div>核心数: 8 (模拟)</div>' +
        `<div>使用率: ${(20 + Math.random() * 30).toFixed(1)}% (模拟)</div>` +
        '<div>温度: 45°C (模拟)</div>';
      break;
    case 'mem':
      out.innerHTML += '<div style="color:#4ecdc4;">[内存信息]</div>' +
        '<div>总内存: 16GB (模拟)</div>' +
        `<div>已使用: ${(40 + Math.random() * 20).toFixed(1)}% (模拟)</div>` +
        '<div>缓存: 3.2GB (模拟)</div>';
      break;
    case 'clear':
    case 'cls':
      clearTerminal();
      return;
    case 'exit':
      _termSimMode = false;
      out.innerHTML += '<div style="color:#fd9644;">已退出模拟模式</div>';
      break;
    default:
      out.innerHTML += `<div style="color:#fc5c65;">命令未找到: ${command}</div><div style="color:#555;">输入 help 查看所有可用命令</div>`;
  }
  out.scrollTop = out.scrollHeight;
}

function sendTermCmd(cmd, input) {
  if (!cmd.trim()) return;
  const out = document.getElementById('termOutput');
  out.innerHTML += '<div style="color:#888;">$ ' + cmd + '</div>';

  // 模拟模式
  if (_termSimMode) {
    handleSimCommand(cmd);
  }
  // 远程模式
  else if (_termWS && _termWS.readyState === 1) {
    _termWS.send(JSON.stringify({ type: 'input', data: cmd + '\n' }));
  } else {
    out.innerHTML += '<div style="color:#fc5c65;">❌ 终端未连接，已自动切换到模拟模式</div>';
    _termSimMode = true;
    handleSimCommand(cmd);
  }

  input.value = '';
  out.scrollTop = out.scrollHeight;
}

async function closeTerminalSession(id) {
  const data = await apiV2('/terminal/sessions/' + id, { method: 'DELETE' });
  if (data.code === 0) { showToast('已关闭'); switchPage('terminal'); }
  else { showToast(data.msg || '操作失败', 'error'); }
}

// ============ 7. 实时日志 ============
var _logWS = null;

async function renderLogViewerPage() {
  const data = await apiV2('/logs');
  const files = data.code === 0 ? (data.data || []) : [];
  return `
    <div class="card">
      <div class="card-title">实时日志追踪</div>
      <div class="search-bar">
        <select class="form-select" id="logFile" style="width:300px;">
          ${files.length === 0 ? '<option value="">暂无日志文件</option>' : files.map(f => `<option value="${f.path}">${f.name} (${formatBytes(f.size)})</option>`).join('')}
        </select>
        <input class="form-input" id="logFilter" placeholder="关键词过滤" style="width:200px;">
        <button class="btn btn-primary btn-sm" onclick="startLogTail()">开始追踪</button>
        <button class="btn btn-sm" onclick="stopLogTail()">停止</button>
      </div>
      <div id="logViewer" style="background:var(--bg);border:1px solid var(--border);border-radius:6px;padding:12px;height:400px;overflow-y:auto;font-family:monospace;font-size:12px;"></div>
    </div>`;
}

function bindLogViewerEvents() {}

function startLogTail() {
  const file = document.getElementById('logFile').value;
  if (!file) { showToast('请选择日志文件', 'error'); return; }
  stopLogTail();
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const token = localStorage.getItem('admin_token');
  const wsUrl = `${proto}//${location.host}/api/v2/logs/tail?file=${encodeURIComponent(file)}&token=${encodeURIComponent(token)}`;
  _logWS = new WebSocket(wsUrl);
  const viewer = document.getElementById('logViewer');
  const filter = document.getElementById('logFilter');
  _logWS.onmessage = (ev) => {
    const line = ev.data;
    if (filter.value && !line.includes(filter.value)) return;
    const div = document.createElement('div');
    div.textContent = line;
    div.className = line.includes('error') || line.includes('ERROR') ? 'log-error' : '';
    viewer.appendChild(div);
    viewer.scrollTop = viewer.scrollHeight;
    while (viewer.children.length > 500) viewer.removeChild(viewer.firstChild);
  };
  _logWS.onclose = () => {
    const div = document.createElement('div');
    div.textContent = '— 日志追踪已停止 —';
    div.style.color = 'var(--text-secondary)';
    viewer.appendChild(div);
  };
  showToast('开始追踪');
}

function stopLogTail() {
  if (_logWS) { try { _logWS.close(); } catch(e) {} _logWS = null; }
}

// ============================================
// 算法机器人模块 - 前端渲染
// ============================================

// 类型映射
const BOT_TYPE_MAP = {
  crawler: { name: '爬虫', icon: '🕷️', color: '#ff6b6b' },
  analyzer: { name: '分析器', icon: '📊', color: '#4ecdc4' },
  scheduler: { name: '调度器', icon: '⏰', color: '#45b7d1' },
  notifier: { name: '通知器', icon: '🔔', color: '#f7b731' },
  security: { name: '安全', icon: '🛡️', color: '#a55eea' },
  ai_agent: { name: 'AI代理', icon: '🧠', color: '#26de81' }
};

const BOT_STATUS_MAP = {
  stopped: { label: '已停止', class: 'tag-default' },
  running: { label: '运行中', class: 'tag-success' },
  error: { label: '错误', class: 'tag-danger' },
  pending: { label: '待启动', class: 'tag-warning' }
};

// 机器人总览页面
async function renderBotOverviewPage() {
  const statsData = await apiV2('/bots/stats');
  const stats = statsData.code === 0 ? statsData.data : {};
  const listData = await apiV2('/bots');
  const bots = listData.code === 0 ? (listData.data || []) : [];

  const logStats = stats.logs || {};
  const byType = stats.by_type || [];

  return `
    <div style="display:grid;grid-template-columns:repeat(4,1fr);gap:16px;margin-bottom:16px;">
      <div class="card" style="text-align:center;padding:20px;">
        <div style="font-size:28px;font-weight:bold;color:var(--primary);">${stats.total || bots.length}</div>
        <div style="color:var(--text-secondary);">机器人总数</div>
      </div>
      <div class="card" style="text-align:center;padding:20px;">
        <div style="font-size:28px;font-weight:bold;color:#26de81;">${stats.running || 0}</div>
        <div style="color:var(--text-secondary);">运行中</div>
      </div>
      <div class="card" style="text-align:center;padding:20px;">
        <div style="font-size:28px;font-weight:bold;color:#fd9644;">${stats.stopped || 0}</div>
        <div style="color:var(--text-secondary);">已停止</div>
      </div>
      <div class="card" style="text-align:center;padding:20px;">
        <div style="font-size:28px;font-weight:bold;color:#fc5c65;">${stats.error || 0}</div>
        <div style="color:var(--text-secondary);">错误</div>
      </div>
    </div>

    <div class="card" style="margin-bottom:16px;">
      <div class="card-title">快速操作
        <button class="btn btn-sm btn-primary" onclick="startAllBots()" style="float:right;">▶ 启动全部</button>
        <button class="btn btn-sm btn-danger" onclick="stopAllBots()" style="float:right;margin-right:8px;">■ 停止全部</button>
        <button class="btn btn-sm" onclick="autoConfigBot('full')" style="float:right;margin-right:8px;">⚙ 自动配置</button>
      </div>
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(180px,1fr));gap:12px;margin-top:12px;">
        ${byType.map(t => `
          <div style="padding:12px;border-radius:8px;background:linear-gradient(135deg,var(--card-bg),var(--bg));text-align:center;">
            <div style="font-size:24px;">${(BOT_TYPE_MAP[t.type] || {}).icon || '🤖'}</div>
            <div style="font-weight:bold;margin:4px 0;">${(BOT_TYPE_MAP[t.type] || {}).name || t.type}</div>
            <div style="color:var(--text-secondary);font-size:14px;">${t.count} 个</div>
          </div>
        `).join('')}
      </div>
    </div>

    <div class="card" style="margin-bottom:16px;">
      <div class="card-title">最近日志
        <button class="btn btn-sm" style="float:right;" onclick="switchPage('bot-logs')">查看全部 →</button>
      </div>
      <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin-top:8px;">
        <div style="text-align:center;padding:16px;background:var(--bg);border-radius:8px;">
          <div style="font-size:24px;font-weight:bold;">${logStats.total || 0}</div>
          <div style="color:var(--text-secondary);">日志总数</div>
        </div>
        <div style="text-align:center;padding:16px;background:var(--bg);border-radius:8px;">
          <div style="font-size:24px;font-weight:bold;color:var(--primary);">${logStats.today || 0}</div>
          <div style="color:var(--text-secondary);">今日新增</div>
        </div>
        <div style="text-align:center;padding:16px;background:var(--bg);border-radius:8px;">
          <div style="font-size:24px;font-weight:bold;color:#fc5c65;">${logStats.errors || 0}</div>
          <div style="color:var(--text-secondary);">错误日志</div>
        </div>
      </div>
    </div>

    <div class="card">
      <div class="card-title">机器人状态
        <button class="btn btn-sm" style="float:right;" onclick="switchPage('bot-manage')">管理 →</button>
      </div>
      <table>
        <thead><tr><th>名称</th><th>类型</th><th>状态</th><th>运行次数</th><th>成功率</th><th>操作</th></tr></thead>
        <tbody>
          ${bots.length === 0 ? '<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">暂无机器人</td></tr>' : bots.slice(0, 5).map(b => `
            <tr>
              <td>${b.icon || '🤖'} ${b.display_name || b.name}</td>
              <td><span class="tag">${(BOT_TYPE_MAP[b.type] || {}).name || b.type}</span></td>
              <td><span class="tag ${(BOT_STATUS_MAP[b.status] || {}).class || 'tag-default'}">${(BOT_STATUS_MAP[b.status] || {}).label || b.status}</span></td>
              <td>${b.run_count || 0}</td>
              <td style="color:${b.success_rate >= 90 ? '#26de81' : b.success_rate >= 70 ? '#f7b731' : '#fc5c65'};">${(b.success_rate || 0).toFixed(1)}%</td>
              <td>
                ${b.status === 'running' ? `<button class="btn btn-sm btn-danger" onclick="stopBot(${b.id})">停止</button>` : `<button class="btn btn-sm btn-success" onclick="startBot(${b.id})">启动</button>`}
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

// 机器人管理页面
async function renderBotManagePage() {
  const data = await apiV2('/bots');
  const bots = data.code === 0 ? (data.data || []) : [];

  return `
    <div class="card">
      <div class="card-title">机器人列表
        <button class="btn btn-primary btn-sm" style="float:right;" onclick="showBotModal()">+ 新建机器人</button>
        <button class="btn btn-sm" style="float:right;margin-right:8px;" onclick="startAllBots()">▶ 全部启动</button>
        <button class="btn btn-sm btn-danger" style="float:right;margin-right:8px;" onclick="stopAllBots()">■ 全部停止</button>
      </div>
      <table>
        <thead><tr><th>ID</th><th>图标</th><th>名称</th><th>类型</th><th>状态</th><th>优先级</th><th>并发</th><th>成功率</th><th>CPU</th><th>操作</th></tr></thead>
        <tbody>
          ${bots.length === 0 ? '<tr><td colspan="10" style="text-align:center;color:var(--text-secondary);">暂无机器人，请点击右上角创建</td></tr>' : bots.map(b => `
            <tr>
              <td>${b.id}</td>
              <td style="font-size:20px;">${b.icon || '🤖'}</td>
              <td>
                <div style="font-weight:bold;">${b.display_name || b.name}</div>
                <div style="font-size:12px;color:var(--text-secondary);">${b.description || ''}</div>
              </td>
              <td><span class="tag" style="background:${(BOT_TYPE_MAP[b.type] || {}).color || '#ccc'};color:#fff;">${(BOT_TYPE_MAP[b.type] || {}).name || b.type}</span></td>
              <td><span class="tag ${(BOT_STATUS_MAP[b.status] || {}).class || 'tag-default'}">${(BOT_STATUS_MAP[b.status] || {}).label || b.status}</span></td>
              <td>${b.priority}</td>
              <td>${b.concurrency}</td>
              <td style="color:${b.success_rate >= 90 ? '#26de81' : b.success_rate >= 70 ? '#f7b731' : '#fc5c65'};">${(b.success_rate || 0).toFixed(1)}%</td>
              <td>${b.cpu_usage ? b.cpu_usage.toFixed(1) + '%' : '-'}</td>
              <td>
                ${b.status === 'running' ? `<button class="btn btn-sm btn-danger" onclick="stopBot(${b.id})">■</button>` : `<button class="btn btn-sm btn-success" onclick="startBot(${b.id})">▶</button>`}
                <button class="btn btn-sm" onclick="triggerBot(${b.id})">⚡</button>
                <button class="btn btn-sm" onclick="showBotEditModal(${b.id})">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="deleteBot(${b.id})">🗑️</button>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>`;
}

// 机器人配置页面
async function renderBotConfigPage() {
  const cfgData = await apiV2('/bots/configs/list');
  const configs = cfgData.code === 0 ? (cfgData.data || []) : [];
  const types = [
    { key: 'crawler', label: '爬虫' },
    { key: 'analyzer', label: '分析器' },
    { key: 'scheduler', label: '调度器' },
    { key: 'notifier', label: '通知器' },
    { key: 'security', label: '安全机器人' },
    { key: 'ai_agent', label: 'AI代理' }
  ];

  return `
    <div class="card" style="margin-bottom:16px;">
      <div class="card-title">自动配置优化</div>
      <div style="display:flex;gap:8px;margin-bottom:12px;">
        <button class="btn btn-sm" onclick="autoConfigBot('full')">全模块优化</button>
        <button class="btn btn-sm" onclick="autoConfigBot('crawler')">爬虫优化</button>
        <button class="btn btn-sm" onclick="autoConfigBot('analyzer')">分析器优化</button>
        <button class="btn btn-sm" onclick="autoConfigBot('schedule')">调度优化</button>
      </div>
      <div id="autoConfigResult" style="color:var(--text-secondary);"></div>
    </div>

    <div class="card" style="margin-bottom:16px;">
      <div class="card-title">全局配置
        <button class="btn btn-sm btn-primary" style="float:right;" onclick="showConfigModal()">+ 新增配置</button>
      </div>
      <table>
        <thead><tr><th>Key</th><th>值</th><th>类型</th><th>分类</th><th>描述</th><th>操作</th></tr></thead>
        <tbody>
          ${configs.length === 0 ? '<tr><td colspan="6" style="text-align:center;color:var(--text-secondary);">暂无配置</td></tr>' : configs.map(c => `
            <tr>
              <td><code>${c.key}</code></td>
              <td><code>${c.value}</code></td>
              <td><span class="tag">${c.type}</span></td>
              <td>${c.category}</td>
              <td>${c.description}</td>
              <td>
                <button class="btn btn-sm" onclick="showConfigEditModal(${c.id})">编辑</button>
                <button class="btn btn-sm btn-danger" onclick="deleteBotConfig(${c.id})">删除</button>
              </td>
            </tr>`).join('')}
        </tbody>
      </table>
    </div>

    <div class="card">
      <div class="card-title">机器人类型说明</div>
      <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:12px;margin-top:8px;">
        ${types.map(t => {
          const info = BOT_TYPE_MAP[t.key];
          return `<div style="padding:12px;border-radius:8px;border:1px solid var(--border);">
            <div style="font-size:24px;">${info.icon}</div>
            <div style="font-weight:bold;color:${info.color};">${t.label}</div>
            <div style="font-size:12px;color:var(--text-secondary);margin-top:4px;">${t.key}</div>
          </div>`;
        }).join('')}
      </div>
    </div>`;
}

// 机器人日志页面
async function renderBotLogsPage() {
  const listData = await apiV2('/bots');
  const bots = listData.code === 0 ? (listData.data || []) : [];
  const logsData = await apiV2('/bots/logs/list?limit=100');
  const logs = logsData.code === 0 ? (logsData.data || []) : [];

  const levelColors = { DEBUG: '#95a5a6', INFO: '#3498db', WARNING: '#f39c12', ERROR: '#e74c3c', FATAL: '#8e44ad' };

  return `
    <div style="display:grid;grid-template-columns:1fr;3fr;gap:16px;">
      <div class="card" style="max-height:600px;overflow-y:auto;">
        <div class="card-title">机器人筛选</div>
        <div style="padding:8px;">
          <div style="padding:8px;cursor:pointer;border-radius:4px;${!window._currentBotFilter ? 'background:var(--primary);color:#fff;' : ''}" onclick="filterBotLogs(0)">
            📋 全部日志
          </div>
          ${bots.map(b => `
            <div style="padding:8px;cursor:pointer;border-radius:4px;margin:4px 0;${window._currentBotFilter === b.id ? 'background:var(--primary);color:#fff;' : 'background:var(--bg);'}" onclick="filterBotLogs(${b.id})">
              ${b.icon || '🤖'} ${b.display_name || b.name}
              <div style="font-size:11px;opacity:0.7;">${(BOT_TYPE_MAP[b.type] || {}).name || b.type}</div>
            </div>
          `).join('')}
        </div>
      </div>

      <div class="card">
        <div class="card-title">
          运行日志 (${logs.length})
          <button class="btn btn-sm" style="float:right;" onclick="cleanBotLogs()">清理旧日志</button>
          <button class="btn btn-sm btn-danger" style="float:right;margin-right:8px;" onclick="clearBotLogsDisplay()">清屏</button>
        </div>
        <div style="max-height:550px;overflow-y:auto;background:#1a1a2e;border-radius:8px;padding:12px;font-family:monospace;font-size:12px;">
          ${logs.length === 0 ? '<div style="color:#666;text-align:center;padding:40px;">暂无日志记录</div>' : logs.map(l => `
            <div style="padding:4px 8px;border-left:3px solid ${levelColors[l.level] || '#666'};margin-bottom:4px;background:rgba(255,255,255,0.03);">
              <div style="display:flex;gap:8px;">
                <span style="color:${levelColors[l.level] || '#666'};font-weight:bold;">[${l.level}]</span>
                <span style="color:#888;">${l.bot_name || 'Unknown'}</span>
                <span style="color:#555;margin-left:auto;">${formatTime(l.created_at)}</span>
              </div>
              <div style="color:#ddd;margin-left:20px;">${l.message}</div>
              ${l.detail ? `<div style="color:#999;margin-left:20px;font-size:11px;">${l.detail}</div>` : ''}
              <div style="color:#666;margin-left:20px;font-size:11px;">耗时: ${l.duration}ms | 状态: ${l.status}</div>
            </div>`).join('')}
        </div>
      </div>
    </div>`;
}

// ============ 机器人操作函数 ============

async function startBot(id) {
  const data = await apiV2('/bots/' + id + '/start', { method: 'POST' });
  if (data.code === 0) { showToast('机器人已启动'); switchPage('bot-overview'); }
  else { showToast(data.msg || '启动失败', 'error'); }
}

async function stopBot(id) {
  const data = await apiV2('/bots/' + id + '/stop', { method: 'POST' });
  if (data.code === 0) { showToast('机器人已停止'); switchPage('bot-overview'); }
  else { showToast(data.msg || '停止失败', 'error'); }
}

async function triggerBot(id) {
  const data = await apiV2('/bots/' + id + '/trigger', { method: 'POST' });
  if (data.code === 0) { showToast('已触发执行'); switchPage('bot-overview'); }
  else { showToast(data.msg || '触发失败', 'error'); }
}

async function startAllBots() {
  const data = await apiV2('/bots/start-all', { method: 'POST' });
  if (data.code === 0) { showToast('已启动全部机器人'); switchPage('bot-overview'); }
  else { showToast(data.msg || '操作失败', 'error'); }
}

async function stopAllBots() {
  const data = await apiV2('/bots/stop-all', { method: 'POST' });
  if (data.code === 0) { showToast('已停止全部机器人'); switchPage('bot-overview'); }
  else { showToast(data.msg || '操作失败', 'error'); }
}

async function autoConfigBot(mode) {
  const data = await apiV2('/bots/auto-config', { method: 'POST', body: JSON.stringify({ mode }) });
  if (data.code === 0) {
    const result = data.data || {};
    const container = document.getElementById('autoConfigResult');
    if (container) {
      const optimizations = result.optimizations || [];
      container.innerHTML = `<div style="color:#26de81;margin-bottom:8px;">✅ 优化完成 (${result.optimized_count || 0} 项)</div>` +
        (optimizations.length > 0 ? '<ul style="margin-left:20px;">' + optimizations.map(o => `<li>${o}</li>`).join('') + '</ul>' : '<div>无需优化</div>');
    }
    showToast('自动配置完成');
  } else {
    showToast(data.msg || '配置失败', 'error');
  }
}

// 新建机器人弹窗
function showBotModal() {
  showModal('新建机器人', `
    <div class="form-group"><label class="form-label">名称</label><input class="form-input" id="bot_name" placeholder="my-bot"></div>
    <div class="form-group"><label class="form-label">显示名称</label><input class="form-input" id="bot_display_name" placeholder="我的机器人"></div>
    <div class="form-group"><label class="form-label">类型</label>
      <select class="form-select" id="bot_type">
        <option value="crawler">🕷️ 爬虫</option>
        <option value="analyzer">📊 分析器</option>
        <option value="scheduler">⏰ 调度器</option>
        <option value="notifier">🔔 通知器</option>
        <option value="security">🛡️ 安全机器人</option>
        <option value="ai_agent">🧠 AI代理</option>
      </select>
    </div>
    <div class="form-group"><label class="form-label">图标 (Emoji)</label><input class="form-input" id="bot_icon" placeholder="🤖" value="🤖"></div>
    <div class="form-group"><label class="form-label">描述</label><textarea class="form-input" id="bot_desc" placeholder="机器人功能描述" style="min-height:60px;"></textarea></div>
    <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:8px;">
      <div class="form-group"><label class="form-label">优先级</label><input class="form-input" id="bot_priority" type="number" value="5" min="1" max="10"></div>
      <div class="form-group"><label class="form-label">并发数</label><input class="form-input" id="bot_concurrency" type="number" value="1" min="1"></div>
      <div class="form-group"><label class="form-label">超时(秒)</label><input class="form-input" id="bot_timeout" type="number" value="30" min="1"></div>
    </div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitBot()">创建机器人</button>
  `);
}

async function submitBot() {
  const body = {
    name: document.getElementById('bot_name').value,
    display_name: document.getElementById('bot_display_name').value,
    type: document.getElementById('bot_type').value,
    icon: document.getElementById('bot_icon').value || '🤖',
    description: document.getElementById('bot_desc').value,
    priority: parseInt(document.getElementById('bot_priority').value) || 5,
    concurrency: parseInt(document.getElementById('bot_concurrency').value) || 1,
    timeout: parseInt(document.getElementById('bot_timeout').value) || 30,
    enabled: true
  };
  if (!body.name) { showToast('请输入名称', 'error'); return; }
  const data = await apiV2('/bots', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('机器人已创建'); closeModal(); switchPage('bot-manage'); }
  else { showToast(data.msg || '创建失败', 'error'); }
}

// 编辑机器人弹窗
async function showBotEditModal(id) {
  const data = await apiV2('/bots/' + id);
  if (data.code !== 0) { showToast(data.msg || '加载失败', 'error'); return; }
  const b = data.data;
  showModal('编辑机器人: ' + (b.display_name || b.name), `
    <div class="form-group"><label class="form-label">名称</label><input class="form-input" id="bot_name" value="${b.name}"></div>
    <div class="form-group"><label class="form-label">显示名称</label><input class="form-input" id="bot_display_name" value="${b.display_name || ''}"></div>
    <div class="form-group"><label class="form-label">类型</label>
      <select class="form-select" id="bot_type">
        <option value="crawler" ${b.type === 'crawler' ? 'selected' : ''}>🕷️ 爬虫</option>
        <option value="analyzer" ${b.type === 'analyzer' ? 'selected' : ''}>📊 分析器</option>
        <option value="scheduler" ${b.type === 'scheduler' ? 'selected' : ''}>⏰ 调度器</option>
        <option value="notifier" ${b.type === 'notifier' ? 'selected' : ''}>🔔 通知器</option>
        <option value="security" ${b.type === 'security' ? 'selected' : ''}>🛡️ 安全机器人</option>
        <option value="ai_agent" ${b.type === 'ai_agent' ? 'selected' : ''}>🧠 AI代理</option>
      </select>
    </div>
    <div class="form-group"><label class="form-label">图标</label><input class="form-input" id="bot_icon" value="${b.icon || '🤖'}"></div>
    <div class="form-group"><label class="form-label">描述</label><textarea class="form-input" id="bot_desc" style="min-height:60px;">${b.description || ''}</textarea></div>
    <div style="display:grid;grid-template-columns:repeat(3,1fr);gap:8px;">
      <div class="form-group"><label class="form-label">优先级</label><input class="form-input" id="bot_priority" type="number" value="${b.priority}"></div>
      <div class="form-group"><label class="form-label">并发数</label><input class="form-input" id="bot_concurrency" type="number" value="${b.concurrency}"></div>
      <div class="form-group"><label class="form-label">超时(秒)</label><input class="form-input" id="bot_timeout" type="number" value="${b.timeout}"></div>
    </div>
    <div class="form-group"><label class="form-label">配置 (JSON)</label><textarea class="form-input" id="bot_config" style="min-height:80px;font-family:monospace;">${b.config || ''}</textarea></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitBotEdit(${id})">保存修改</button>
  `);
}

async function submitBotEdit(id) {
  const body = {
    name: document.getElementById('bot_name').value,
    display_name: document.getElementById('bot_display_name').value,
    type: document.getElementById('bot_type').value,
    icon: document.getElementById('bot_icon').value,
    description: document.getElementById('bot_desc').value,
    priority: parseInt(document.getElementById('bot_priority').value),
    concurrency: parseInt(document.getElementById('bot_concurrency').value),
    timeout: parseInt(document.getElementById('bot_timeout').value),
    config: document.getElementById('bot_config').value,
    enabled: true
  };
  const data = await apiV2('/bots/' + id, { method: 'PUT', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('保存成功'); closeModal(); switchPage('bot-manage'); }
  else { showToast(data.msg || '保存失败', 'error'); }
}

async function deleteBot(id) {
  if (!confirm('确定删除此机器人？')) return;
  const data = await apiV2('/bots/' + id, { method: 'DELETE' });
  if (data.code === 0) { showToast('已删除'); switchPage('bot-manage'); }
  else { showToast(data.msg || '删除失败', 'error'); }
}

// 配置操作
function showConfigModal() {
  showModal('新增配置项', `
    <div class="form-group"><label class="form-label">Key</label><input class="form-input" id="cfg_key" placeholder="bot.global.timeout"></div>
    <div class="form-group"><label class="form-label">值</label><input class="form-input" id="cfg_value" placeholder="60"></div>
    <div class="form-group"><label class="form-label">类型</label>
      <select class="form-select" id="cfg_type">
        <option value="string">字符串</option>
        <option value="number">数字</option>
        <option value="boolean">布尔</option>
        <option value="json">JSON</option>
      </select>
    </div>
    <div class="form-group"><label class="form-label">分类</label><input class="form-input" id="cfg_category" placeholder="global"></div>
    <div class="form-group"><label class="form-label">描述</label><input class="form-input" id="cfg_desc" placeholder="配置项说明"></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitBotConfig()">保存</button>
  `);
}

async function submitBotConfig() {
  const body = {
    key: document.getElementById('cfg_key').value,
    value: document.getElementById('cfg_value').value,
    type: document.getElementById('cfg_type').value,
    category: document.getElementById('cfg_category').value,
    description: document.getElementById('cfg_desc').value
  };
  if (!body.key) { showToast('请输入Key', 'error'); return; }
  const data = await apiV2('/bots/configs/save', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('已保存'); closeModal(); switchPage('bot-config'); }
  else { showToast(data.msg || '保存失败', 'error'); }
}

async function showConfigEditModal(id) {
  const data = await apiV2('/bots/configs/list');
  if (data.code !== 0) return;
  const cfg = (data.data || []).find(c => c.id === id);
  if (!cfg) return;
  showModal('编辑配置: ' + cfg.key, `
    <div class="form-group"><label class="form-label">Key</label><input class="form-input" id="cfg_key" value="${cfg.key}"></div>
    <div class="form-group"><label class="form-label">值</label><input class="form-input" id="cfg_value" value="${cfg.value}"></div>
    <div class="form-group"><label class="form-label">类型</label>
      <select class="form-select" id="cfg_type">
        <option value="string" ${cfg.type === 'string' ? 'selected' : ''}>字符串</option>
        <option value="number" ${cfg.type === 'number' ? 'selected' : ''}>数字</option>
        <option value="boolean" ${cfg.type === 'boolean' ? 'selected' : ''}>布尔</option>
        <option value="json" ${cfg.type === 'json' ? 'selected' : ''}>JSON</option>
      </select>
    </div>
    <div class="form-group"><label class="form-label">分类</label><input class="form-input" id="cfg_category" value="${cfg.category}"></div>
    <div class="form-group"><label class="form-label">描述</label><input class="form-input" id="cfg_desc" value="${cfg.description || ''}"></div>
    <button class="btn btn-primary" style="width:100%;" onclick="submitBotConfigEdit(${id})">保存</button>
  `);
}

async function submitBotConfigEdit(id) {
  const body = {
    id: id,
    key: document.getElementById('cfg_key').value,
    value: document.getElementById('cfg_value').value,
    type: document.getElementById('cfg_type').value,
    category: document.getElementById('cfg_category').value,
    description: document.getElementById('cfg_desc').value
  };
  const data = await apiV2('/bots/configs/save', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('已保存'); closeModal(); switchPage('bot-config'); }
  else { showToast(data.msg || '保存失败', 'error'); }
}

async function deleteBotConfig(id) {
  if (!confirm('确定删除此配置？')) return;
  const data = await apiV2('/bots/configs/' + id, { method: 'DELETE' });
  if (data.code === 0) { showToast('已删除'); switchPage('bot-config'); }
  else { showToast(data.msg || '删除失败', 'error'); }
}

// 日志操作
function filterBotLogs(botId) {
  window._currentBotFilter = botId;
  switchPage('bot-logs');
}

function clearBotLogsDisplay() {
  showToast('日志已清空显示');
  switchPage('bot-logs');
}

async function cleanBotLogs() {
  const days = prompt('清理多少天前的日志？', '30');
  if (!days) return;
  const body = { days: parseInt(days) };
  if (window._currentBotFilter) body.bot_id = window._currentBotFilter;
  const data = await apiV2('/bots/logs/clean', { method: 'POST', body: JSON.stringify(body) });
  if (data.code === 0) { showToast('日志已清理'); switchPage('bot-logs'); }
  else { showToast(data.msg || '清理失败', 'error'); }
}

// ============================================
// V2 运维模块函数绑定到 window
// ============================================
window.renderFirewallPage = renderFirewallPage;
window.renderSSHBlocksPage = renderSSHBlocksPage;
window.renderFileManagerPage = renderFileManagerPage;
window.renderCrontabPage = renderCrontabPage;
window.renderTerminalPage = renderTerminalPage;
window.renderLogViewerPage = renderLogViewerPage;
window.renderBotOverviewPage = renderBotOverviewPage;
window.renderBotManagePage = renderBotManagePage;
window.renderBotConfigPage = renderBotConfigPage;
window.renderBotLogsPage = renderBotLogsPage;
window.apiV2 = apiV2;
window.showFirewallModal = showFirewallModal;
window.submitFirewallRule = submitFirewallRule;
window.toggleFirewallRule = toggleFirewallRule;
window.deleteFirewallRule = deleteFirewallRule;
window.unblockIP = unblockIP;
window.fileGoUp = fileGoUp;
window.fileOpenDir = fileOpenDir;
window.fileOpenFile = fileOpenFile;
window.fileSave = fileSave;
window.fileDelete = fileDelete;
window.showMkdirModal = showMkdirModal;
window.submitMkdir = submitMkdir;
window.showCrontabModal = showCrontabModal;
window.translateCron = translateCron;
window.submitCrontab = submitCrontab;
window.triggerCrontab = triggerCrontab;
window.showCrontabLogs = showCrontabLogs;
window.deleteCrontab = deleteCrontab;
window.openTerminal = openTerminal;
window.clearTerminal = clearTerminal;
window.sendTermCmd = sendTermCmd;
window.closeTerminalSession = closeTerminalSession;
window.startLogTail = startLogTail;
window.stopLogTail = stopLogTail;
window.startBot = startBot;
window.stopBot = stopBot;
window.triggerBot = triggerBot;
window.startAllBots = startAllBots;
window.stopAllBots = stopAllBots;
window.autoConfigBot = autoConfigBot;
window.showBotModal = showBotModal;
window.submitBot = submitBot;
window.showBotEditModal = showBotEditModal;
window.submitBotEdit = submitBotEdit;
window.deleteBot = deleteBot;
window.showConfigModal = showConfigModal;
window.submitBotConfig = submitBotConfig;
window.showConfigEditModal = showConfigEditModal;
window.submitBotConfigEdit = submitBotConfigEdit;
window.deleteBotConfig = deleteBotConfig;
window.filterBotLogs = filterBotLogs;
window.clearBotLogsDisplay = clearBotLogsDisplay;
window.cleanBotLogs = cleanBotLogs;
})();
