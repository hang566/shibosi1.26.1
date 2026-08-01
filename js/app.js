const STORAGE_KEY = 'apps_data_v3';

const DEFAULT_APPS = [
    { name: '云枢', url: 'https://yunshu-web.pages.dev/', icon: 'https://yunshu-web.pages.dev/favicon.ico', desc: '云枢工作台' },
    { name: '玄影楼阁', url: 'https://xuanyinglouge.pages.dev/', icon: 'https://xuanyinglouge.pages.dev/favicon.ico', desc: '玄影楼阁' },
    { name: '镜序', url: 'https://jingxv.pages.dev/', icon: 'https://jingxv.pages.dev/favicon.ico', desc: '镜序系统' },
    { name: '典学阁', url: 'https://dianxuege.pages.dev/', icon: 'https://dianxuege.pages.dev/favicon.ico', desc: '典学阁' }
];

let apps = [];
let selectedIndex = -1;

function loadApps() {
    try {
        const stored = localStorage.getItem(STORAGE_KEY);
        if (stored) {
            apps = JSON.parse(stored);
        }
        if (!Array.isArray(apps) || apps.length === 0) {
            apps = JSON.parse(JSON.stringify(DEFAULT_APPS));
            saveApps();
        }
    } catch (e) {
        apps = JSON.parse(JSON.stringify(DEFAULT_APPS));
        saveApps();
    }
}

function saveApps() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(apps));
}

function renderApps() {
    const container = document.getElementById('appList');
    if (!container) return;

    if (!apps || apps.length === 0) {
        container.innerHTML = '<p class="app-empty">暂无应用</p>';
        return;
    }

    container.innerHTML = apps.map((app, index) => `
        <div class="app-card">
            <a href="${app.url}" target="_blank" class="app-link">
                <div class="app-icon-wrap">
                    <img src="${app.icon}" alt="${app.name}" class="app-icon" onerror="this.src='img/app/earth.svg'">
                </div>
                <div class="app-name">${app.name}</div>
                <div class="app-desc">${app.desc || app.url}</div>
            </a>
            <div class="app-card-actions">
                <button onclick="event.stopPropagation(); editAppDirect(${index})">编辑</button>
                <button onclick="event.stopPropagation(); deleteApp(${index})">删除</button>
            </div>
        </div>
    `).join('');
}

function addApp() {
    const name = document.getElementById('addName').value.trim();
    const url = document.getElementById('addUrl').value.trim();
    const icon = document.getElementById('addIcon').value.trim();
    const desc = document.getElementById('addDesc').value.trim();

    if (!name || !url) {
        showToast('请填写名称和URL！', 2000, 'error');
        return;
    }

    apps.push({ name, url, icon: icon || 'img/app/earth.svg', desc });
    saveApps();
    renderApps();

    document.getElementById('addName').value = '';
    document.getElementById('addUrl').value = '';
    document.getElementById('addIcon').value = '';
    document.getElementById('addDesc').value = '';
}

function openEditModal(index) {
    selectedIndex = index;
    const app = apps[index];
    document.getElementById('editName').value = app.name;
    document.getElementById('editUrl').value = app.url;
    document.getElementById('editIcon').value = app.icon;
    document.getElementById('editDesc').value = app.desc || '';
    openModal('editAppModal');
}

function editAppDirect(index) {
    openEditModal(index);
}

function editApp() {
    if (selectedIndex < 0) return;

    const name = document.getElementById('editName').value.trim();
    const url = document.getElementById('editUrl').value.trim();
    const icon = document.getElementById('editIcon').value.trim();
    const desc = document.getElementById('editDesc').value.trim();

    if (!name || !url) {
        showToast('请填写名称和URL！', 2000, 'error');
        return;
    }

    apps[selectedIndex] = { name, url, icon: icon || 'img/app/earth.svg', desc };
    saveApps();
    renderApps();
    selectedIndex = -1;
}

async function deleteApp(index) {
    const confirmed = await showPopconfirm('确定要删除此应用吗？');
    if (confirmed) {
        apps.splice(index, 1);
        saveApps();
        renderApps();
        selectedIndex = -1;
        showToast('删除成功！', 2000, 'success');
    }
}

function initApps() {
    loadApps();
    renderApps();
}

document.addEventListener('DOMContentLoaded', initApps);