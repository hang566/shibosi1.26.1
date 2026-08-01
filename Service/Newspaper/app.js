const API_BASE = 'http://localhost:8082/api';
const newsCache = new Map();
const state = {
    currentPage: 1,
    perPage: 20,
    totalNews: 0,
    categories: [],
    searchKeyword: '',
    currentFilter: {
        category: 'all',
        dateFrom: '',
        dateTo: '',
    },
    favoritesPage: 1,
    favoritesPerPage: 20,
    favoritesTotal: 0,
    searchEngineAvailable: false,
    showFavoritesPanel: false,
};

function init() {
    updateCurrentDate();
    loadCategories();
    loadNews();
    loadStats();
    loadFavoriteCount();
    checkSearchEngine();
    setupEventListeners();
}

function updateCurrentDate() {
    const now = new Date();
    const options = { year: 'numeric', month: 'long', day: 'numeric', weekday: 'long' };
    document.getElementById('currentDate').textContent = now.toLocaleDateString('zh-CN', options);
    document.getElementById('exportDateFrom').value = formatDate(now);
    const tomorrow = new Date(now);
    tomorrow.setDate(tomorrow.getDate() + 1);
    document.getElementById('exportDateTo').value = formatDate(tomorrow);
    document.getElementById('exportTitle').value = `昨日晚报 - ${formatDate(now, 'zh-CN')}`;
}

function formatDate(date, locale = 'en') {
    const y = date.getFullYear();
    const m = String(date.getMonth() + 1).padStart(2, '0');
    const d = String(date.getDate()).padStart(2, '0');
    if (locale === 'zh-CN') {
        return `${y}年${m}月${d}日`;
    }
    return `${y}-${m}-${d}`;
}

function setupEventListeners() {
    const searchInput = document.getElementById('searchInput');
    if (searchInput) {
        searchInput.addEventListener('keypress', (e) => {
            if (e.key === 'Enter') {
                state.searchKeyword = e.target.value.trim();
                state.currentPage = 1;
                loadNews();
            }
        });
    }

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && state.showFavoritesPanel) {
            toggleFavoritesPanel();
        }
    });
}

async function apiRequest(endpoint, options = {}) {
    const url = `${API_BASE}${endpoint}`;
    const defaultOptions = {
        headers: {
            'Content-Type': 'application/json',
        },
    };
    try {
        const response = await fetch(url, { ...defaultOptions, ...options });
        if (!response.ok) {
            const text = await response.text();
            throw new Error(`${response.status}: ${text || '请求失败'}`);
        }
        if (options.responseType === 'blob') {
            return response.blob();
        }
        const text = await response.text();
        if (!text) return null;
        return JSON.parse(text);
    } catch (error) {
        console.error('API请求失败:', endpoint, error);
        throw error;
    }
}

async function loadCategories() {
    try {
        const categories = await apiRequest('/categories');
        state.categories = categories;
        const select = document.getElementById('categoryFilter');
        if (select) {
            select.innerHTML = '<option value="all">全部分类</option>';
            categories.forEach((cat) => {
                const option = document.createElement('option');
                option.value = cat;
                option.textContent = cat;
                select.appendChild(option);
            });
        }

        const exportSelect = document.getElementById('exportCategory');
        if (exportSelect) {
            exportSelect.innerHTML = '<option value="">全部分类</option>';
            categories.forEach((cat) => {
                const option = document.createElement('option');
                option.value = cat;
                option.textContent = cat;
                exportSelect.appendChild(option);
            });
        }
    } catch (error) {
        console.error('加载分类失败:', error);
    }
}

async function loadNews() {
    const main = document.getElementById('newspaperMain');
    if (!main) return;

    main.innerHTML = `
        <div class="loading">
            <div class="spinner"></div>
            <p>加载新闻中...</p>
        </div>
    `;

    try {
        const params = new URLSearchParams();
        params.set('page', state.currentPage);
        params.set('per_page', state.perPage);

        if (state.searchKeyword) {
            params.set('keyword', state.searchKeyword);
        }
        if (state.currentFilter.category && state.currentFilter.category !== 'all') {
            params.set('category', state.currentFilter.category);
        }
        if (state.currentFilter.dateFrom) {
            params.set('date_from', state.currentFilter.dateFrom);
        }
        if (state.currentFilter.dateTo) {
            params.set('date_to', state.currentFilter.dateTo);
        }

        const data = await apiRequest(`/news?${params}`);
        state.totalNews = data.total;
        renderNews(data.news);
        renderPagination(data.total);
    } catch (error) {
        main.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M19 3H5c-1.1 0-2 .9-2 2v14c0 1.1.9 2 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2zm0 16H5V5h14v14zM7 10h2v7H7zm4-3h2v10h-2zm4 6h2v4h-2z"/>
                </svg>
                <p>加载新闻失败，请检查后端服务是否启动</p>
                <button class="btn-primary" onclick="loadNews()" style="margin-top:12px;">重试</button>
            </div>
        `;
    }
}

function renderNews(newsList) {
    const main = document.getElementById('newspaperMain');
    if (!main) return;

    newsCache.clear();

    if (!newsList || newsList.length === 0) {
        main.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M20 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2zm0 4l-8 5-8-5V6l8 5 8-5v2z"/>
                </svg>
                <p>${state.searchKeyword ? '没有找到相关新闻' : '暂无新闻数据，请先抓取新闻'}</p>
                ${state.searchKeyword ? '' : '<button class="btn-primary" onclick="fetchNews()" style="margin-top:12px;">立即抓取</button>'}
            </div>
        `;
        return;
    }

    const groupedNews = {};
    const categoryOrder = ['时政', '财经', '科技', '体育', '社会', '国际', '军事', '教育', '健康', '娱乐', '汽车', '房产', '综合'];

    newsList.forEach((news) => {
        const cat = news.category || '综合';
        if (!groupedNews[cat]) {
            groupedNews[cat] = [];
        }
        groupedNews[cat].push(news);
        newsCache.set(news.id, news);
    });

    let html = '';
    const orderedCats = [];

    categoryOrder.forEach((cat) => {
        if (groupedNews[cat]) {
            orderedCats.push(cat);
        }
    });

    Object.keys(groupedNews).forEach((cat) => {
        if (!orderedCats.includes(cat)) {
            orderedCats.push(cat);
        }
    });

    orderedCats.forEach((cat) => {
        const list = groupedNews[cat];
        if (!list || list.length === 0) return;

        html += `
            <section class="news-section">
                <div class="news-section-header">
                    <h2>${cat}</h2>
                    <span class="news-section-count">${list.length} 条</span>
                </div>
                <div class="news-grid">
        `;

        list.forEach((news, index) => {
            const isFeatured = index === 0 && list.length > 2;
            const summary = escapeHtml(news.summary || '暂无摘要');
            const title = escapeHtml(news.title);
            const time = formatRelativeTime(news.published_at);
            const analysis = getAnalysisBadge(news);

            const kicker = `<span class="news-kicker">${escapeHtml(news.category || '综合')}</span>`;

            let summaryHtml;
            if (isFeatured && summary.length > 0) {
                const firstChar = summary.charAt(0);
                const restChars = summary.slice(1);
                summaryHtml = `<p class="news-summary featured-summary"><span class="drop-cap">${firstChar}</span>${escapeHtml(restChars)}</p>`;
            } else {
                summaryHtml = `<p class="news-summary">${summary}</p>`;
            }

            html += `
                <article class="news-card ${isFeatured ? 'featured' : ''}" onclick="openNewsDetail(${news.id})">
                    <div class="news-card-body">
                        <div class="news-analysis ${news.has_private_ad ? 'has-warning' : ''}">
                            ${analysis}
                            ${news.is_permanent ? '<span class="badge permanent-badge">永久保存</span>' : ''}
                        </div>
                        ${kicker}
                        <h3 class="news-title">
                            ${news.url ? `<a href="${escapeHtml(news.url)}" target="_blank" onclick="event.stopPropagation()">${title}</a>` : title}
                        </h3>
                        ${summaryHtml}
                        <div class="news-meta">
                            <span class="news-source">${escapeHtml(news.source || '未知来源')}</span>
                            <span class="news-time">🕐 ${time}</span>
                            <span class="news-stance">${getStanceLabel(news)}</span>
                            <button class="btn-sm favorite-btn" data-news-id="${news.id}" onclick="toggleFavorite(${news.id}, this); event.stopPropagation()" title="收藏">
                                ♡ 收藏
                            </button>
                            <button class="btn-sm permanent-btn ${news.is_permanent ? 'active' : ''}" onclick="togglePermanent(${news.id}, ${!news.is_permanent}); event.stopPropagation()">
                                ${news.is_permanent ? '★' : '☆'} ${news.is_permanent ? '已永久' : '永久保存'}
                            </button>
                        </div>
                    </div>
                </article>
            `;
        });

        html += `
                </div>
            </section>
        `;
    });

    main.innerHTML = html;
    updateAllFavoriteButtons();
}

function renderPagination(total) {
    const pagination = document.getElementById('pagination');
    if (!pagination) return;

    const totalPages = Math.ceil(total / state.perPage);
    if (totalPages <= 1) {
        pagination.innerHTML = '';
        return;
    }

    let html = `<button ${state.currentPage <= 1 ? 'disabled' : ''} onclick="goToPage(${state.currentPage - 1})">上一页</button>`;

    let startPage = Math.max(1, state.currentPage - 2);
    let endPage = Math.min(totalPages, startPage + 4);
    startPage = Math.max(1, endPage - 4);

    for (let i = startPage; i <= endPage; i++) {
        html += `<button class="${i === state.currentPage ? 'active' : ''}" onclick="goToPage(${i})">${i}</button>`;
    }

    html += `<button ${state.currentPage >= totalPages ? 'disabled' : ''} onclick="goToPage(${state.currentPage + 1})">下一页</button>`;

    pagination.innerHTML = html;
}

function goToPage(page) {
    state.currentPage = page;
    loadNews();
    window.scrollTo({ top: 0, behavior: 'smooth' });
}

function openNewsDetail(id) {
    const main = document.getElementById('newspaperMain');
    if (!main) return;

    main.innerHTML = `
        <div class="loading">
            <div class="spinner"></div>
            <p>加载详情中...</p>
        </div>
    `;

    apiRequest(`/news/${id}`)
        .then((news) => {
            newsCache.set(news.id, news);
            const title = escapeHtml(news.title);
            const summary = escapeHtml(news.summary || '');
            const content = escapeHtml(news.content || summary || '暂无内容');
            const time = news.published_at || news.fetched_at || '';

            main.innerHTML = `
                <div class="news-single">
                    <button class="btn-secondary" onclick="loadNews()" style="margin-bottom:16px;">← 返回列表</button>
                    <div style="display:flex;gap:8px;margin-bottom:16px;">
                        <button class="btn-sm favorite-btn" data-news-id="${news.id}" onclick="toggleFavorite(${news.id}, this)" title="收藏">
                            ♡ 收藏
                        </button>
                        <button class="btn-sm permanent-btn ${news.is_permanent ? 'active' : ''}" onclick="togglePermanent(${news.id}, ${!news.is_permanent})">
                            ${news.is_permanent ? '★' : '☆'} ${news.is_permanent ? '已永久' : '永久保存'}
                        </button>
                    </div>
                    <h2>${news.url ? `<a href="${escapeHtml(news.url)}" target="_blank" style="color:inherit;text-decoration:none;">${title}</a>` : title}</h2>
                    <div class="news-meta" style="border:none;padding:0;margin-bottom:16px;">
                        <span class="news-source">${escapeHtml(news.source || '未知来源')}</span>
                        <span class="news-time">🕐 ${time}</span>
                        <span class="news-source" style="background:var(--gs-block);padding:2px 8px;border-radius:4px;">${escapeHtml(news.category || '综合')}</span>
                    </div>
                    <div class="full-content">${content}</div>
                </div>
            `;
        })
        .catch((error) => {
            main.innerHTML = `
                <div class="empty-state">
                    <p>加载详情失败</p>
                    <button class="btn-secondary" onclick="loadNews()" style="margin-top:12px;">返回列表</button>
                </div>
            `;
        });
}

async function fetchNews(sourceId = null) {
    const btn = document.getElementById('fetchBtn');
    const btn2 = document.getElementById('fetchBtn2');

    if (btn) btn.disabled = true;
    if (btn2) btn2.disabled = true;
    showToast('正在抓取新闻...', 'info');

    try {
        const url = sourceId ? `/fetch?source_id=${sourceId}` : '/fetch';
        const data = await apiRequest(url, { method: 'POST' });

        if (data.success) {
            if (data.saved === 0) {
                showToast(data.message || '没有获取到新新闻', 'warning');
            } else {
                showToast(data.message || '抓取成功！', 'success');
            }
            state.currentPage = 1;
            loadNews();
            loadStats();
        } else {
            showToast(data.message || '抓取失败', 'error');
        }
    } catch (error) {
        let errorMsg = '抓取请求失败';
        if (error.message && error.message.includes('404')) {
            errorMsg = '抓取接口不存在，请检查服务是否正常运行';
        } else if (error.message && error.message.includes('500')) {
            errorMsg = '服务器内部错误，请查看日志';
        } else if (error.message) {
            errorMsg = error.message;
        }
        showToast(errorMsg, 'error');
        console.error('抓取失败详情:', error);
    } finally {
        if (btn) btn.disabled = false;
        if (btn2) btn2.disabled = false;
    }
}

async function loadStats() {
    try {
        const [stats, analysisStats] = await Promise.all([
            apiRequest('/stats').catch(() => ({})),
            apiRequest('/analysis-stats').catch(() => ({})),
        ]);

        document.getElementById('totalNews').textContent = stats.total_count || 0;
        document.getElementById('totalSources').textContent = stats.source_count || 0;
        document.getElementById('lastUpdate').textContent = stats.latest_fetch ? `最近更新: ${stats.latest_fetch}` : '未更新';

        const permEl = document.getElementById('permanentCount');
        if (permEl) permEl.textContent = stats.permanent_count || 0;

        populateSidebarStats(analysisStats);
    } catch (error) {
        console.error('加载统计失败:', error);
    }
}

function populateSidebarStats(stats) {
    const sidebar = document.getElementById('sidebarStats');
    if (!sidebar) return;

    const analyzedCount = stats.analyzed_count || 0;
    const leftCount = stats.left_count || 0;
    const rightCount = stats.right_count || 0;
    const neutralCount = stats.neutral_count || 0;
    const adCount = stats.private_ad_count || 0;

    const total = leftCount + rightCount + neutralCount || 1;
    const leftPct = ((leftCount / total) * 100).toFixed(0);
    const rightPct = ((rightCount / total) * 100).toFixed(0);
    const neutralPct = ((neutralCount / total) * 100).toFixed(0);

    sidebar.innerHTML = `
        <div class="sidebar-stat-item">
            <div class="sidebar-stat-header">
                <span class="sidebar-stat-label">已分析</span>
                <span class="sidebar-stat-value">${analyzedCount}</span>
            </div>
            <div class="sidebar-stat-bar">
                <div class="bar-left" style="width:${leftPct}%" title="左/保守 ${leftCount}"></div>
                <div class="bar-neutral" style="width:${neutralPct}%" title="中立 ${neutralCount}"></div>
                <div class="bar-right" style="width:${rightPct}%" title="右/自由 ${rightCount}"></div>
            </div>
            <div class="sidebar-stat-legend">
                <span class="legend-item"><i class="dot-left"></i>左 ${leftCount}</span>
                <span class="legend-item"><i class="dot-neutral"></i>中立 ${neutralCount}</span>
                <span class="legend-item"><i class="dot-right"></i>右 ${rightCount}</span>
            </div>
        </div>
        <div class="sidebar-stat-item">
            <div class="sidebar-stat-row">
                <span class="sidebar-stat-icon">⚠️</span>
                <span class="sidebar-stat-label">夹带私活</span>
                <span class="sidebar-stat-value">${adCount}</span>
            </div>
        </div>
        <div class="sidebar-stat-item">
            <div class="sidebar-stat-row">
                <span class="sidebar-stat-icon">📊</span>
                <span class="sidebar-stat-label">分析完成率</span>
                <span class="sidebar-stat-value">${stats.analyzed_count > 0 ? Math.round((stats.analyzed_count / (stats.analyzed_count + 1)) * 100) : 0}%</span>
            </div>
        </div>
        <div class="sidebar-stat-actions">
            <button class="sidebar-btn" onclick="window.location.href='/admin'">📊 详细分析 →</button>
        </div>
    `;
}

function searchNews() {
    const input = document.getElementById('searchInput');
    state.searchKeyword = input ? input.value.trim() : '';
    state.currentPage = 1;
    loadNews();
}

function applyFilters() {
    state.currentFilter.category = document.getElementById('categoryFilter')?.value || 'all';
    state.currentFilter.dateFrom = document.getElementById('dateFrom')?.value || '';
    state.currentFilter.dateTo = document.getElementById('dateTo')?.value || '';
    state.currentPage = 1;
    loadNews();
}

function resetFilters() {
    state.searchKeyword = '';
    state.currentFilter = { category: 'all', dateFrom: '', dateTo: '' };
    const searchInput = document.getElementById('searchInput');
    if (searchInput) searchInput.value = '';
    const categoryFilter = document.getElementById('categoryFilter');
    if (categoryFilter) categoryFilter.value = 'all';
    const dateFrom = document.getElementById('dateFrom');
    if (dateFrom) dateFrom.value = '';
    const dateTo = document.getElementById('dateTo');
    if (dateTo) dateTo.value = '';
    state.currentPage = 1;
    loadNews();
}

function openSettings() {
    loadSources();
    loadStatsDetail();
    showModal('settingsModal');
}

function openExport() {
    showModal('exportModal');
}

function showModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.add('active');
    }
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) {
        modal.classList.remove('active');
    }
}

function switchSettingsTab(tabName, element) {
    const tabs = element.parentElement;
    tabs.querySelectorAll('.tab-browser').forEach((t) => t.classList.remove('active'));
    element.classList.add('active');

    document.querySelectorAll('#settingsModal .tab-pane').forEach((p) => p.classList.remove('active'));
    document.getElementById(`${tabName}-tab`)?.classList.add('active');
}

async function loadSources() {
    const listEl = document.getElementById('sourcesList');
    if (!listEl) return;

    listEl.innerHTML = '<p class="loading">加载中...</p>';

    try {
        const sources = await apiRequest('/sources');
        if (!sources || sources.length === 0) {
            listEl.innerHTML = '<p style="text-align:center;color:var(--gs-hint);">暂无新闻源</p>';
            return;
        }

        listEl.innerHTML = sources.map((src) => `
            <div class="source-item">
                <div class="source-info">
                    <p class="source-name">${escapeHtml(src.name)}</p>
                    <p class="source-url">${escapeHtml(src.url)}</p>
                </div>
                <div class="source-actions">
                    <button class="btn-toggle ${src.enabled ? '' : 'disabled'}" onclick="toggleSource(${src.id}, ${src.enabled ? 0 : 1})">
                        ${src.enabled ? '启用' : '禁用'}
                    </button>
                    <button class="btn-delete" onclick="deleteSource(${src.id})">删除</button>
                </div>
            </div>
        `).join('');
    } catch (error) {
        listEl.innerHTML = '<p style="text-align:center;color:var(--gs-danger);">加载失败</p>';
    }
}

async function addSource() {
    const name = document.getElementById('srcName')?.value.trim();
    const url = document.getElementById('srcUrl')?.value.trim();
    const type = document.getElementById('srcType')?.value || 'rss';
    const category = document.getElementById('srcCategory')?.value || '综合';

    if (!name || !url) {
        showToast('请填写源名称和URL', 'error');
        return;
    }

    try {
        await apiRequest('/sources', {
            method: 'POST',
            body: JSON.stringify({ name, url, type, category }),
        });
        showToast('添加成功！', 'success');
        document.getElementById('srcName').value = '';
        document.getElementById('srcUrl').value = '';
        loadSources();
    } catch (error) {
        showToast('添加失败', 'error');
    }
}

async function toggleSource(id, enabled) {
    try {
        await apiRequest('/sources', {
            method: 'PUT',
            body: JSON.stringify({ id, enabled: enabled === 1 }),
        });
        showToast('更新成功', 'success');
        loadSources();
    } catch (error) {
        showToast('更新失败', 'error');
    }
}

async function deleteSource(id) {
    if (!confirm('确定删除该新闻源吗？')) return;

    try {
        await apiRequest('/sources', {
            method: 'DELETE',
            body: JSON.stringify({ id }),
        });
        showToast('删除成功', 'success');
        loadSources();
    } catch (error) {
        showToast('删除失败', 'error');
    }
}

async function loadStatsDetail() {
    const grid = document.getElementById('statsGrid');
    if (!grid) return;

    grid.innerHTML = '<p class="loading">加载中...</p>';

    try {
        const stats = await apiRequest('/stats');
        grid.innerHTML = `
            <div class="stat-card">
                <div class="stat-value">${stats.total_count || 0}</div>
                <div class="stat-label">新闻总数</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">${stats.category_count || 0}</div>
                <div class="stat-label">分类数</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">${stats.source_count || 0}</div>
                <div class="stat-label">来源数</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">${stats.today_count || 0}</div>
                <div class="stat-label">今日新增</div>
            </div>
        `;
    } catch (error) {
        grid.innerHTML = '<p style="color:var(--gs-danger);">加载失败</p>';
    }
}

async function cleanOldNews(days) {
    if (!confirm(`确定清理 ${days} 天前的新闻吗？`)) return;

    try {
        const data = await apiRequest(`/clean?days=${days}`, { method: 'POST' });
        showToast(data.message || '清理成功', 'success');
        loadStats();
        loadStatsDetail();
        loadNews();
    } catch (error) {
        showToast('清理失败', 'error');
    }
}

async function doExport() {
    const format = document.querySelector('input[name="exportFormat"]:checked')?.value || 'md';
    const title = document.getElementById('exportTitle')?.value || '';
    const dateFrom = document.getElementById('exportDateFrom')?.value || '';
    const dateTo = document.getElementById('exportDateTo')?.value || '';
    const category = document.getElementById('exportCategory')?.value || '';
    const maxItems = parseInt(document.getElementById('exportMaxItems')?.value || '200');

    showToast('正在生成导出文件...', 'info');

    try {
        const response = await fetch(`${API_BASE}/export`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                format,
                title,
                date_from: dateFrom,
                date_to: dateTo,
                category,
                max_items: maxItems,
            }),
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const contentDisposition = response.headers.get('Content-Disposition');
        let filename = `newspaper_${Date.now()}.${format}`;
        if (contentDisposition) {
            const match = contentDisposition.match(/filename=(.+)/);
            if (match) {
                filename = match[1];
            }
        }

        const blob = await response.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);

        showToast('导出成功！', 'success');
        closeModal('exportModal');
    } catch (error) {
        showToast('导出失败: ' + error.message, 'error');
    }
}

function toggleNavbar() {
    const nav = document.getElementById('navbar-nav');
    if (nav) {
        nav.classList.toggle('active');
    }
}

function showToast(message, type = 'info') {
    const toast = document.getElementById('toast');
    if (!toast) return;

    toast.textContent = message;
    toast.className = `toast ${type} show`;

    setTimeout(() => {
        toast.classList.remove('show');
    }, 3000);
}

async function togglePermanent(newsId, isPermanent) {
    try {
        const data = await apiRequest('/news/permanent', {
            method: 'POST',
            body: JSON.stringify({ id: newsId, is_permanent: isPermanent })
        });
        showToast(data.message, 'success');
        loadNews();
    } catch (e) {
        showToast('操作失败', 'error');
    }
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text || '';
    return div.innerHTML;
}

function formatRelativeTime(dateStr) {
    if (!dateStr) return '';

    const date = new Date(dateStr);
    const now = new Date();
    const diffMs = now - date;
    const diffSec = Math.floor(diffMs / 1000);
    const diffMin = Math.floor(diffSec / 60);
    const diffHour = Math.floor(diffMin / 60);
    const diffDay = Math.floor(diffHour / 24);

    if (diffSec < 60) return '刚刚';
    if (diffMin < 60) return `${diffMin}分钟前`;
    if (diffHour < 24) return `${diffHour}小时前`;
    if (diffDay < 7) return `${diffDay}天前`;

    return dateStr.split(' ')[0];
}

document.addEventListener('DOMContentLoaded', init);

document.addEventListener('click', (e) => {
    const nav = document.getElementById('navbar-nav');
    const toggle = document.getElementById('navbar-toggle');
    if (nav && nav.classList.contains('active') && !nav.contains(e.target) && !toggle.contains(e.target)) {
        nav.classList.remove('active');
    }
});

document.querySelectorAll('.modal').forEach((modal) => {
    modal.addEventListener('click', (e) => {
        if (e.target === modal) {
            modal.classList.remove('active');
        }
    });
});

function getAnalysisBadge(news) {
    if (!news.political_stance && !news.has_private_ad && !news.reliability) {
        return '';
    }

    let badges = '';

    const stanceColors = {
        'left': { bg: '#c62828', label: '← 倾向保守' },
        'center-left': { bg: '#ef5350', label: '↖ 中左' },
        'neutral': { bg: '#757575', label: '○ 中立' },
        'center-right': { bg: '#64b5f6', label: '↗ 中右' },
        'right': { bg: '#1565c0', label: '→ 倾向自由' },
    };

    if (news.political_stance && stanceColors[news.political_stance]) {
        const c = stanceColors[news.political_stance];
        badges += `<span class="analysis-badge" style="background:${c.bg}">${c.label}</span>`;
    }

    if (news.reliability === 'reliable') {
        badges += `<span class="analysis-badge" style="background:#2e7d32" title="交叉验证置信度高">✓ 可靠</span>`;
    } else if (news.reliability === 'mixed') {
        badges += `<span class="analysis-badge" style="background:#f57c00" title="交叉验证结果不一致">⚑ 混合</span>`;
    } else if (news.reliability === 'unreliable') {
        badges += `<span class="analysis-badge warning" style="background:#c62828" title="交叉验证置信度低">✗ 不可靠</span>`;
    }

    if (news.confidence_score && news.confidence_score > 0) {
        const confPct = Math.round(news.confidence_score * 100);
        const confColor = news.confidence_score >= 0.7 ? '#2e7d32' : news.confidence_score >= 0.4 ? '#f57c00' : '#c62828';
        badges += `<span class="analysis-badge" style="background:${confColor}" title="交叉验证置信度">置信 ${confPct}%</span>`;
    }

    if (news.topic_category && news.topic_category !== '综合') {
        badges += `<span class="analysis-badge" style="background:#4527a0" title="检测话题">${escapeHtml(news.topic_category)}</span>`;
    }

    if (news.has_private_ad) {
        badges += `<span class="analysis-badge warning" title="疑似夹带私活/广告">⚠️ 私活</span>`;
    } else if (news.ad_score > 0.3) {
        badges += `<span class="analysis-badge warning-light" title="可能包含推广内容">🔍 低可信度</span>`;
    }

    return badges;
}

function getStanceLabel(news) {
    if (!news.political_stance || news.political_stance === 'neutral') {
        return '';
    }

    const labels = {
        'left': '立场偏左',
        'center-left': '立场中左',
        'right': '立场偏右',
        'center-right': '立场中右',
    };

    return `<span class="news-stance-tag">${labels[news.political_stance] || ''}</span>`;
}

async function reanalyzeNews() {
    showToast('正在重新分析新闻...', 'info');
    try {
        const data = await apiRequest('/reanalyze', { method: 'POST' });
        showToast(`分析完成，处理了 ${data.count} 条新闻`, 'success');
        loadNews();
        loadStatsDetail();
    } catch (error) {
        showToast('分析失败', 'error');
    }
}

async function loadAnalysisStats() {
    try {
        const stats = await apiRequest('/analysis-stats');
        return stats;
    } catch (error) {
        console.error('加载分析统计失败:', error);
        return null;
    }
}

window.reanalyzeNews = reanalyzeNews;
window.loadAnalysisStats = loadAnalysisStats;

async function loadFavoriteCount() {
    try {
        const data = await apiRequest('/favorites/count');
        const count = data.count || 0;
        const badge = document.getElementById('favoriteCount');
        if (badge) {
            badge.textContent = count;
            badge.style.display = count > 0 ? 'inline' : 'none';
        }
        const sidebarFavCount = document.getElementById('sidebarFavCount');
        if (sidebarFavCount) {
            sidebarFavCount.textContent = count;
        }
        const favCountBadge = document.getElementById('favCountBadge');
        if (favCountBadge) {
            favCountBadge.textContent = count;
        }
    } catch (error) {
        console.error('加载收藏数失败:', error);
    }
}

async function toggleFavorite(idOrUrl, buttonEl) {
    try {
        let news;
        if (typeof idOrUrl === 'object') {
            news = idOrUrl;
        } else {
            news = newsCache.get(idOrUrl) || newsCache.get(String(idOrUrl));
        }
        
        if (!news) {
            showToast('新闻数据不可用，请刷新页面', 'error');
            return;
        }
        
        const isFavData = await apiRequest(`/favorites/check?url=${encodeURIComponent(news.url)}`);
        if (isFavData.is_favorite) {
            await apiRequest('/favorites/', {
                method: 'DELETE',
                body: JSON.stringify({ url: news.url })
            });
            showToast('已取消收藏', 'info');
            if (buttonEl) setFavoriteButtonState(buttonEl, false);
        } else {
            const payload = {
                news_id: news.id || 0,
                title: news.title,
                url: news.url,
                summary: news.summary || '',
                source: news.source || '',
                category: news.category || '',
                stance: news.political_stance || news.stance || 'neutral',
                stance_score: news.political_score || news.stance_score || 0,
            };
            await apiRequest('/favorites', {
                method: 'POST',
                body: JSON.stringify(payload)
            });
            showToast('已添加到收藏', 'success');
            if (buttonEl) setFavoriteButtonState(buttonEl, true);

            // 如果用户已登录：同步一份到用户中心云端收藏（source=newspaper），静默失败
            try {
                const token = localStorage.getItem('shibosi_token');
                if (token) {
                    const summary = payload.summary || '';
                    const src = payload.source ? ('\n来源：' + payload.source) : '';
                    await fetch('/api/user/favorites', {
                        method: 'POST',
                        headers: {
                            'Content-Type': 'application/json',
                            'Authorization': 'Bearer ' + token
                        },
                        body: JSON.stringify({
                            type: 'article',
                            source: 'newspaper',
                            title: payload.title,
                            url: payload.url,
                            content: summary + src
                        })
                    });
                }
            } catch (e) {
                // 同步云端失败，忽略
            }
        }
        loadFavoriteCount();
        if (state.showFavoritesPanel) {
            loadFavorites();
        }
    } catch (error) {
        showToast('操作失败', 'error');
    }
}

async function checkIsFavorite(url) {
    try {
        const data = await apiRequest(`/favorites/check?url=${encodeURIComponent(url)}`);
        return data;
    } catch (error) {
        return { is_favorite: false, id: 0 };
    }
}

function setFavoriteButtonState(button, isFav) {
    if (!button) return;
    if (isFav) {
        button.classList.add('active');
        button.innerHTML = '♥ 已收藏';
    } else {
        button.classList.remove('active');
        button.innerHTML = '♡ 收藏';
    }
}

async function updateAllFavoriteButtons() {
    const buttons = document.querySelectorAll('.favorite-btn');
    for (const button of buttons) {
        const newsId = button.dataset.newsId;
        if (!newsId) continue;
        const news = newsCache.get(newsId) || newsCache.get(String(newsId));
        if (!news || !news.url) continue;
        try {
            const data = await checkIsFavorite(news.url);
            setFavoriteButtonState(button, data.is_favorite);
        } catch (e) {
        }
    }
}

async function loadFavorites() {
    const container = document.getElementById('favoritesList');
    if (!container) return;

    const countBadge = document.getElementById('favCountBadge');

    try {
        const params = new URLSearchParams();
        params.set('page', state.favoritesPage);
        params.set('per_page', state.favoritesPerPage);

        const data = await apiRequest(`/favorites?${params}`);
        state.favoritesTotal = data.total;

        if (countBadge) countBadge.textContent = data.total;

        if (!data.favorites || data.favorites.length === 0) {
            container.innerHTML = `
                <div class="favorites-empty">
                    <svg viewBox="0 0 24 24" fill="currentColor" width="64" height="64">
                        <path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/>
                    </svg>
                    <p>还没有收藏的新闻</p>
                    <p>点击新闻旁边的 ♡ 按钮来收藏</p>
                </div>
            `;
            return;
        }

        const stanceColors = {
            'left': { bg: '#c62828', label: '← 左' },
            'center-left': { bg: '#ef5350', label: '↖ 中左' },
            'neutral': { bg: '#757575', label: '○ 中立' },
            'center-right': { bg: '#64b5f6', label: '↗ 中右' },
            'right': { bg: '#1565c0', label: '→ 右' },
        };

        container.innerHTML = data.favorites.map(fav => {
            const stance = stanceColors[fav.stance] || stanceColors['neutral'];
            const favId = fav.id;
            return `
                <div class="favorite-item" data-url="${escapeHtml(fav.url)}">
                    <div class="favorite-header">
                        <a href="${escapeHtml(fav.url)}" target="_blank" class="favorite-title">${escapeHtml(fav.title)}</a>
                        <button class="favorite-remove" onclick="removeFavoriteItem(${favId})" title="取消收藏">×</button>
                    </div>
                    <div class="favorite-meta">
                        <span class="favorite-source">${escapeHtml(fav.source || '未知')}</span>
                        <span class="favorite-cat">${escapeHtml(fav.category || '综合')}</span>
                        <span class="analysis-badge" style="background:${stance.bg}">${stance.label}</span>
                        <span class="favorite-time">${escapeHtml(fav.favorited_at)}</span>
                    </div>
                    ${fav.summary ? `<p class="favorite-summary">${escapeHtml(fav.summary)}</p>` : ''}
                </div>
            `;
        }).join('');

        const totalPages = Math.ceil(state.favoritesTotal / state.favoritesPerPage);
        if (totalPages > 1) {
            let paginationHtml = '';
            for (let i = 1; i <= totalPages; i++) {
                paginationHtml += `<button class="${i === state.favoritesPage ? 'active' : ''}" onclick="goToFavoritesPage(${i})">${i}</button>`;
            }
            container.innerHTML += `<div class="pagination" style="margin-top:16px;grid-column:1/-1;">${paginationHtml}</div>`;
        }
    } catch (error) {
        container.innerHTML = '<p style="color:var(--gs-danger);padding:20px;">加载收藏失败</p>';
    }
}

function goToFavoritesPage(page) {
    state.favoritesPage = page;
    loadFavorites();
}

async function removeFavoriteItem(id) {
    if (!confirm('确定取消收藏这条新闻吗？')) return;
    try {
        await apiRequest(`/favorites/${id}`, {
            method: 'DELETE',
            body: JSON.stringify({ id })
        });
        showToast('已取消收藏', 'info');
        loadFavorites();
        loadFavoriteCount();
    } catch (error) {
        showToast('操作失败', 'error');
    }
}

async function clearAllFavorites() {
    if (!confirm('确定清空所有收藏吗？此操作不可恢复！')) return;
    try {
        await apiRequest('/favorites/clear', { method: 'POST' });
        showToast('已清空收藏', 'success');
        state.favoritesPage = 1;
        loadFavorites();
        loadFavoriteCount();
    } catch (error) {
        showToast('操作失败', 'error');
    }
}

function toggleFavoritesPanel() {
    state.showFavoritesPanel = !state.showFavoritesPanel;
    const panel = document.getElementById('favoritesPanel');
    const overlay = document.getElementById('favoritesOverlay');
    
    if (state.showFavoritesPanel) {
        panel.classList.add('active');
        overlay.classList.add('active');
        document.body.style.overflow = 'hidden';
        loadFavorites();
    } else {
        panel.classList.remove('active');
        overlay.classList.remove('active');
        document.body.style.overflow = '';
    }
}

async function checkSearchEngine() {
    try {
        const data = await apiRequest('/search-engine/health');
        state.searchEngineAvailable = data.available;
        const indicator = document.getElementById('searchEngineStatus');
        if (indicator) {
            indicator.className = 'se-status ' + (data.available ? 'online' : 'offline');
            indicator.title = data.message || (data.available ? 'SearchEngine 服务可用' : 'SearchEngine 服务不可用');
        }
    } catch (error) {
        state.searchEngineAvailable = false;
        const indicator = document.getElementById('searchEngineStatus');
        if (indicator) {
            indicator.className = 'se-status offline';
            indicator.title = 'SearchEngine 服务不可用';
        }
    }
}

async function searchEngineSearch(query, limit = 10) {
    const main = document.getElementById('newspaperMain');
    if (!main) return;

    main.innerHTML = `
        <div class="loading">
            <div class="spinner"></div>
            <p>正在从全网搜索中获取结果...</p>
            <p style="color:var(--np-color-muted);font-size:13px;">查询: ${escapeHtml(query)}</p>
        </div>
    `;

    try {
        const data = await apiRequest(`/search-engine/search?q=${encodeURIComponent(query)}&limit=${limit}`);
        
        if (!data.results || data.results.length === 0) {
            main.innerHTML = `
                <div class="empty-state">
                    <svg viewBox="0 0 24 24" fill="currentColor">
                        <path d="M15.5 14h-.79l-.28-.27C15.41 12.59 16 11.11 16 9.5 16 5.91 13.09 3 9.5 3S3 5.91 3 9.5 5.91 16 9.5 16c1.61 0 3.09-.59 4.23-1.57l.27.28v.79l5 4.99L20.49 19l-4.99-5zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
                    </svg>
                    <p>全网搜索没有找到相关结果</p>
                    <button class="btn-secondary" onclick="loadNews()" style="margin-top:12px;">返回新闻列表</button>
                </div>
            `;
            return;
        }

        const keywordFeedback = data.keyword_feedback;
        let feedbackHtml = '';
        if (keywordFeedback && keywordFeedback.message) {
            feedbackHtml = `
                <div class="search-feedback ${keywordFeedback.type || 'info'}">
                    <span>${escapeHtml(keywordFeedback.message)}</span>
                    ${keywordFeedback.resources ? `<div class="feedback-resources">${keywordFeedback.resources.map(r => `<a href="${escapeHtml(r.url)}" target="_blank">${escapeHtml(r.title)}</a>`).join(' | ')}</div>` : ''}
                </div>
            `;
        }

        const resultsHtml = data.results.map((result, index) => {
            const isAd = result.ad || result.is_ad;
            const sources = result.Sources || [result.Source];
            const sourcesHtml = sources.map(s => `<span class="engine-source">${escapeHtml(s)}</span>`).join('');
            
            return `
                <article class="search-engine-result ${isAd ? 'ad-result' : ''}">
                    <div class="result-header">
                        <h3 class="result-title">
                            <a href="${escapeHtml(result.URL)}" target="_blank" rel="noopener noreferrer">${escapeHtml(result.Title)}</a>
                        </h3>
                        ${isAd ? '<span class="ad-label">广告</span>' : ''}
                    </div>
                    <div class="result-meta">
                        <span class="result-url">${escapeHtml(result.URL)}</span>
                        <span class="result-sources">${sourcesHtml}</span>
                    </div>
                    <p class="result-snippet">${escapeHtml(result.Snippet)}</p>
                    <div class="result-actions">
                        <button class="btn-sm" onclick="saveSearchResultToFavorites('${escapeHtml(result.Title)}', '${escapeHtml(result.URL)}', '${escapeHtml(result.Snippet)}', '${escapeHtml(sources.join(', '))}')">
                            ♡ 收藏
                        </button>
                        <a href="${escapeHtml(result.URL)}" target="_blank" class="btn-sm">访问 →</a>
                    </div>
                </article>
            `;
        }).join('');

        main.innerHTML = `
            <div class="search-engine-results">
                <div class="results-header">
                    <h2>🔍 全网搜索结果</h2>
                    <div class="results-meta">
                        <span>查询: "${escapeHtml(query)}"</span>
                        <span>共 ${data.total || data.results.length} 条结果</span>
                        <button class="btn-secondary btn-sm" onclick="loadNews()">← 返回新闻列表</button>
                    </div>
                </div>
                ${feedbackHtml}
                <div class="results-list">
                    ${resultsHtml}
                </div>
                ${data.total > limit ? `<div class="results-footer"><p>显示 ${data.results.length} 条结果（共 ${data.total} 条）</p></div>` : ''}
            </div>
        `;
    } catch (error) {
        let errorMsg = '全网搜索失败';
        if (error.message && error.message.includes('SearchEngine 服务不可用')) {
            errorMsg = 'SearchEngine 服务不可用，请确保 http://localhost:8081 正在运行';
        }
        main.innerHTML = `
            <div class="empty-state">
                <svg viewBox="0 0 24 24" fill="currentColor">
                    <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-6h2v6zm0-8h-2V7h2v2z"/>
                </svg>
                <p>${errorMsg}</p>
                <p style="color:var(--np-color-muted);font-size:13px;">启动命令: cd Service/SearchEngine && searchengine.exe</p>
                <button class="btn-secondary" onclick="loadNews()" style="margin-top:12px;">返回新闻列表</button>
            </div>
        `;
    }
}

async function saveSearchResultToFavorites(title, url, snippet, sources) {
    try {
        await apiRequest('/favorites', {
            method: 'POST',
            body: JSON.stringify({
                news_id: 0,
                title: title,
                url: url,
                summary: snippet,
                source: sources,
                category: '全网搜索',
                stance: 'neutral',
                stance_score: 0,
            })
        });
        showToast('已收藏', 'success');
        loadFavoriteCount();
    } catch (error) {
        showToast('收藏失败', 'error');
    }
}

async function fallbackSearchEngine() {
    if (!state.searchKeyword) {
        showToast('请先输入搜索关键词', 'warning');
        return;
    }
    if (!state.searchEngineAvailable) {
        showToast('SearchEngine 服务不可用', 'error');
        return;
    }
    searchEngineSearch(state.searchKeyword, 20);
}

window.toggleFavorite = toggleFavorite;
window.togglePermanent = togglePermanent;
window.removeFavoriteItem = removeFavoriteItem;
window.goToFavoritesPage = goToFavoritesPage;
window.clearAllFavorites = clearAllFavorites;
window.toggleFavoritesPanel = toggleFavoritesPanel;
window.searchEngineSearch = searchEngineSearch;
window.fallbackSearchEngine = fallbackSearchEngine;
window.saveSearchResultToFavorites = saveSearchResultToFavorites;