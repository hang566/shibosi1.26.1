/* ---- 子标签页切换（与 primoUI 的 switchTab 互不干扰）---- */
function switchSubTab(tabId, btnEl) {
    // 在面板内查找 tab-pane 和 tab-browser（限定作用域）
    var wrapper = btnEl.closest('.panel-tabs-wrapper');
    if (!wrapper) return;
    wrapper.querySelectorAll('.tab-pane').forEach(function (p) {
        p.classList.remove('active');
        // 清除可能由旧逻辑设置的 inline display，避免覆盖 CSS 的 active 状态
        if (p.style.display) p.style.display = '';
    });
    wrapper.querySelectorAll('.tab-browser').forEach(function (t) { t.classList.remove('active'); });
    var target = document.getElementById(tabId);
    if (target) target.classList.add('active');
    btnEl.classList.add('active');
}

/* ---- 指令快捷输入 ---- */
function insertCmd(cmdText) {
    var input = document.getElementById('searchQuery');
    if (!input) return;
    input.value = cmdText;
    input.focus();
}

/* ---- 加载并显示可用 /go 跳转目标 ---- */
function loadCmdTargets() {
    fetch('./js/search/instruction/commands.json')
        .then(function (r) { return r.json(); })
        .then(function (commands) {
            var container = document.getElementById('cmdTargetList');
            if (!container) return;
            var keys = Object.keys(commands);
            if (keys.length === 0) {
                container.innerHTML = '<p style="color:#999;font-size:12px;">暂无跳转目标</p>';
                return;
            }
            var html = '<div style="display:flex;flex-wrap:wrap;gap:6px;">';
            keys.forEach(function (k) {
                html += '<span class="cmd-chip" onclick="insertCmd(\'/go ' + k + '\')">' + k + '</span>';
            });
            html += '</div>';
            container.innerHTML = html;
        })
        .catch(function () {
            var c = document.getElementById('cmdTargetList');
            if (c) c.innerHTML = '<p style="color:#999;font-size:12px;">加载失败</p>';
        });
}

/* ---- 自定义引擎 ---- */
var _editingEngineId = null; // 正在编辑的引擎 id，null 表示新增模式

function addCustomEngine() {
    var id = document.getElementById('customEngineId').value.trim();
    var name = document.getElementById('customEngineName').value.trim();
    var urlInput = document.getElementById('customEngineUrl').value.trim();
    var url = urlInput.replace('%s', '');

    var msgEl = document.getElementById('engineMsg');
    if (!id || !name || !urlInput) {
        msgEl.textContent = '⚠ 请填写完整信息';
        msgEl.style.color = '#e74c3c';
        return;
    }
    if (!window.SearchEngineManager) {
        msgEl.textContent = '⚠ 引擎管理器未加载';
        msgEl.style.color = '#e74c3c';
        return;
    }

    var result;
    if (_editingEngineId) {
        // 编辑模式：使用当前 id 更新（不允许改标识）
        result = SearchEngineManager.updateEngine(_editingEngineId, name, url);
    } else {
        // 新增模式
        result = SearchEngineManager.addEngine(id, name, url);
    }

    if (result.ok) {
        msgEl.textContent = '✓ ' + result.msg;
        msgEl.style.color = '#27ae60';
        cancelEditEngine();
        refreshCustomList();
    } else {
        msgEl.textContent = '✗ ' + result.msg;
        msgEl.style.color = '#e74c3c';
    }

    setTimeout(function () { msgEl.textContent = ''; }, 3000);
}

function editCustomEngine(id) {
    var list = SearchEngineManager.getCustomList();
    var item = list.find(function (e) { return e.id === id; });
    if (!item) return;

    _editingEngineId = id;
    document.getElementById('customEngineId').value = item.id;
    document.getElementById('customEngineId').disabled = true; // 标识不可修改
    document.getElementById('customEngineName').value = item.name;
    document.getElementById('customEngineUrl').value = item.url;

    var addBtn = document.querySelector('#sub-tab2 .btn-add');
    addBtn.textContent = '保存修改';
    addBtn.className = 'btn-add btn-edit-mode';

    // 确保取消按钮存在
    var actionBar = addBtn.parentElement;
    if (!document.getElementById('cancelEditBtn')) {
        var cancelBtn = document.createElement('button');
        cancelBtn.id = 'cancelEditBtn';
        cancelBtn.type = 'button';
        cancelBtn.className = 'btn-add btn-cancel';
        cancelBtn.textContent = '取消编辑';
        cancelBtn.onclick = cancelEditEngine;
        actionBar.insertBefore(cancelBtn, addBtn.nextSibling);
    }
}

function cancelEditEngine() {
    _editingEngineId = null;
    document.getElementById('customEngineId').value = '';
    document.getElementById('customEngineId').disabled = false;
    document.getElementById('customEngineName').value = '';
    document.getElementById('customEngineUrl').value = '';

    var addBtn = document.querySelector('#sub-tab2 .btn-add');
    if (addBtn) {
        addBtn.textContent = '添加引擎';
        addBtn.className = 'btn-add';
    }

    var cancelBtn = document.getElementById('cancelEditBtn');
    if (cancelBtn) cancelBtn.remove();
}

function removeCustomEngine(id) {
    if (!confirm('确定删除该引擎吗？')) return;
    SearchEngineManager.removeEngine(id);
    refreshCustomList();
}

function refreshCustomList() {
    var list = SearchEngineManager.getCustomList();
    var container = document.getElementById('customEngineList');
    if (!container) return;
    if (list.length === 0) {
        container.innerHTML = '<p class="history-empty-msg">暂无自定义引擎</p>';
        return;
    }
    var html = '<table class="engine-list-table"><thead><tr><th>名称</th><th>标识</th><th style="text-align:center;">操作</th></tr></thead><tbody>';
    list.forEach(function (item) {
        html += '<tr>';
        html += '<td>' + escapeHtml(item.name) + '</td>';
        html += '<td>' + escapeHtml(item.id) + '</td>';
        html += '<td style="text-align:center;white-space:nowrap;">';
        html += '<button class="btn-edit" onclick="editCustomEngine(\'' + item.id.replace(/'/g, "\\'") + '\')" style="margin-right:6px;">编辑</button>';
        html += '<button class="btn-del" onclick="removeCustomEngine(\'' + item.id.replace(/'/g, "\\'") + '\')">删除</button>';
        html += '</td>';
        html += '</tr>';
    });
    html += '</tbody></table>';
    container.innerHTML = html;
}

function escapeHtml(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
}

// 页面加载后渲染（每个调用独立 try-catch，防止一个失败阻塞其他）
function safeExec(fn) {
    try { fn(); } catch (e) { console.error(e); }
}

document.addEventListener('DOMContentLoaded', function () {
    setTimeout(function () {
        safeExec(function () { if (window.SearchEngineManager) refreshCustomList(); });
        safeExec(function () { if (window.SearchHistoryManager) SearchHistoryManager.renderHistoryUI(); });
        safeExec(loadCmdTargets);
    }, 100);
});

// 支持 bfcache 后退导航（用户从搜索结果页返回时重新渲染）
window.addEventListener('pageshow', function () {
    safeExec(function () { if (window.SearchHistoryManager) SearchHistoryManager.renderHistoryUI(); });
    safeExec(function () { if (window.SearchEngineManager) refreshCustomList(); });
});
