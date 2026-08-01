var SEARCH_ENGINE_BASE = 'http://localhost:8081';
var seCurrentPage = 1;
var seCurrentQuery = '';
var seCustomEnginesCache = null;

async function redirectToSearch() {
    var engines = {
        "bilibili": "https://search.bilibili.com/all?keyword=",
        "douyin": "https://www.douyin.com/root/search/",
        "baidu": "https://www.baidu.com/s?wd=",
        "sogou": "https://www.sogou.com/web?query=",
        "360": "https://www.so.com/s?ie=UTF-8&q=",
        "360ai": "https://www.so.com/s?ie=UTF-8&q=",
        "yande": "https://yandex.com/search/?text=",
        "magi": "https://magi.com/search?q=",
        "duckduckgo": "https://duckduckgo.com/?q=",
        "github": "https://github.com/search?q=",
        "bing": "https://www.bing.com/search?q=",
        "tx": "https://pc.qq.com/search.html#!keyword=",
        "google": "https://www.google.com/search?q=",
        "163music": "https://music.163.com/#/search/m/?s=",
        "qqmusic": "https://y.qq.com/n/ryqq/search?w=",
    };

    var engine = document.getElementById("searchEngine").value;
    var query = document.getElementById("searchQuery").value.trim();

    if (engine === 'local') {
        if (!query) {
            openFullSearchEngine();
            return;
        }
        await searchLocalEngine(query, 1);
        return;
    }

    if (!query) {
        var url = engines[engine];
        var homeUrl = url.split(/[?#]/)[0];
        const newTab = window.open();
        newTab.location.href = homeUrl;
        return;
    }

    var url = engines[engine] + encodeURIComponent(query);
    const newTab = window.open();
    newTab.location.href = url;
}

async function searchLocalEngine(query, page) {
    seCurrentQuery = query;
    seCurrentPage = page || 1;

    var panel = document.getElementById('searchEnginePanel');
    panel.style.display = 'block';
    document.getElementById('sePanelMeta').textContent = '搜索中...';
    document.getElementById('seKeywordFeedback').innerHTML = '';
    document.getElementById('seCustomEngines').innerHTML = '';
    document.getElementById('seResultsContainer').innerHTML = '<div class="se-loading"><div class="spinner"></div><br>正在搜索...</div>';
    document.getElementById('sePagination').innerHTML = '';

    if (!seCustomEnginesCache) {
        try {
            var ceRes = await fetch(SEARCH_ENGINE_BASE + '/api/custom-engines');
            if (ceRes.ok) seCustomEnginesCache = await ceRes.json();
        } catch (e) { seCustomEnginesCache = []; }
    }
    renderCustomEngineChips();

    try {
        var res = await fetch(SEARCH_ENGINE_BASE + '/api/search?q=' + encodeURIComponent(query) + '&page=' + seCurrentPage + '&limit=10');
        if (!res.ok) throw new Error('服务不可用 (' + res.status + ')');
        var data = await res.json();
        renderSearchResults(data);
    } catch (err) {
        document.getElementById('seResultsContainer').innerHTML =
            '<div class="se-empty"><div class="se-empty-icon">⚠️</div>搜索服务不可用，请确保 SearchEngine 服务已启动 (端口 8081)<br><small style="opacity:0.7">' + err.message + '</small></div>';
        document.getElementById('sePanelMeta').textContent = '错误';
    }
}

function renderSearchResults(data) {
    var container = document.getElementById('seResultsContainer');
    var meta = document.getElementById('sePanelMeta');
    var kfDiv = document.getElementById('seKeywordFeedback');

    meta.textContent = '共 ' + (data.total || 0) + ' 条 · 第 ' + data.page + ' 页';

    if (data.keyword_feedback) {
        var kf = data.keyword_feedback;
        kfDiv.innerHTML = '<div class="se-kf-card">' +
            '<div class="se-kf-title">' + escapeHtml(kf.title) + '</div>' +
            '<div class="se-kf-content">' + escapeHtml(kf.content) + '</div>' +
            (kf.link_url ? '<a class="se-kf-link" href="' + escapeHtml(kf.link_url) + '" target="_blank" rel="noopener">' + escapeHtml(kf.link_text || '了解更多') + ' →</a>' : '') +
            '</div>';
    } else {
        kfDiv.innerHTML = '';
    }

    if (!data.results || data.results.length === 0) {
        container.innerHTML = '<div class="se-empty"><div class="se-empty-icon">🔍</div>没有找到相关结果<br><small style="opacity:0.6">试试其他关键词或提交网站收录</small></div>';
        document.getElementById('sePagination').innerHTML = '';
        return;
    }

    var html = '';
    data.results.forEach(function(r, idx) {
        var displayUrl = r.URL;
        try {
            var u = new URL(r.URL);
            displayUrl = u.hostname + u.pathname;
        } catch(e) {}

        var snippet = escapeHtml(r.Snippet || '');
        if (seCurrentQuery) {
            var re = new RegExp('(' + seCurrentQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
            snippet = snippet.replace(re, '<mark>$1</mark>');
        }

        var sources = r.Sources && r.Sources.length > 0 ? r.Sources : (r.Source ? [r.Source] : []);
        var resultCount = r.ResultCount || sources.length;
        var isMultiSource = resultCount > 1;
        var sourcesHtml = '';
        if (sources.length > 0) {
            sourcesHtml = '<div class="se-result-sources">' + sources.map(function(s) {
                return '<span class="se-result-source">' + escapeHtml(s) + '</span>';
            }).join('') + '</div>';
        }
        var hotBadge = isMultiSource ? '<span class="se-hot-badge" title="' + resultCount + '个搜索引擎都收录了此结果">🔥 ' + resultCount + '引擎</span>' : '';

        html += '<div class="se-result-card' + (isMultiSource ? ' se-multi-source' : '') + '">' +
            '<div class="se-result-title-row">' +
                '<a class="se-result-title" href="' + escapeHtml(r.URL) + '" target="_blank" rel="noopener">' + escapeHtml(r.Title) + '</a>' +
                hotBadge +
                '<span class="se-result-drag" draggable="true" title="拖拽到搜罗器">⠿</span>' +
            '</div>' +
            sourcesHtml +
            '<div class="se-result-url">' + escapeHtml(displayUrl) + '</div>' +
            '<div class="se-result-snippet">' + snippet + '</div>' +
        '</div>';
    });

    container.innerHTML = html;

    container.querySelectorAll('.se-result-drag').forEach(function(el) {
        el.addEventListener('dragstart', function(e) {
            var card = el.closest('.se-result-card');
            var title = card.querySelector('.se-result-title').textContent;
            var url = card.querySelector('.se-result-title').href;
            var snippet = card.querySelector('.se-result-snippet').textContent;
            e.dataTransfer.setData('text/plain', title + '\n' + url + '\n' + snippet);
            e.dataTransfer.setData('application/x-search-result', JSON.stringify({title: title, url: url, snippet: snippet}));
            el.classList.add('dragging');
            try {
                localStorage.setItem('scraper_pending_result', JSON.stringify({title: title, url: url, snippet: snippet, ts: Date.now()}));
            } catch(e) {}
        });
        el.addEventListener('dragend', function() {
            el.classList.remove('dragging');
        });
    });

    renderPagination(data);
}

function renderPagination(data) {
    var pg = document.getElementById('sePagination');
    var totalPages = Math.ceil((data.total || 0) / (data.per_page || 10));
    if (totalPages <= 1) { pg.innerHTML = ''; return; }

    var html = '';
    html += '<button ' + (data.page <= 1 ? 'disabled' : '') + ' onclick="searchLocalEngine(\'' + escapeJs(seCurrentQuery) + '\',' + (data.page - 1) + ')">上一页</button>';
    html += '<span class="se-page-info">' + data.page + ' / ' + totalPages + '</span>';
    html += '<button ' + (data.page >= totalPages ? 'disabled' : '') + ' onclick="searchLocalEngine(\'' + escapeJs(seCurrentQuery) + '\',' + (data.page + 1) + ')">下一页</button>';
    pg.innerHTML = html;
}

function renderCustomEngineChips() {
    var div = document.getElementById('seCustomEngines');
    if (!seCustomEnginesCache || seCustomEnginesCache.length === 0) { div.innerHTML = ''; return; }

    var html = '<div class="se-ce-bar"><span class="se-ce-label">外部引擎：</span>';
    seCustomEnginesCache.forEach(function(ce) {
        if (!ce.enabled) return;
        var url = (ce.url_template || '').replace('%s', encodeURIComponent(seCurrentQuery || ''));
        html += '<a class="se-ce-chip" href="' + escapeHtml(url) + '" target="_blank" rel="noopener">' + escapeHtml(ce.name || '引擎') + '</a>';
    });
    html += '</div>';
    div.innerHTML = html;
}

function closeSearchEngine() {
    document.getElementById('searchEnginePanel').style.display = 'none';
    seCustomEnginesCache = null;
}

function openFullSearchEngine() {
    var url = SEARCH_ENGINE_BASE + '/';
    if (seCurrentQuery) {
        url += '?q=' + encodeURIComponent(seCurrentQuery);
    }
    window.open(url, '_blank');
}

function escapeHtml(str) {
    var div = document.createElement('div');
    div.textContent = str || '';
    return div.innerHTML;
}

function escapeJs(str) {
    return (str || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/"/g, '\\"');
}

document.addEventListener('keydown', function (event) {
    if (event.key === 'Enter' && document.activeElement.id === 'searchQuery') {
        redirectToSearch();
    }
});


const btn = document.getElementById('btn');
const input = document.getElementById('searchQuery');
const searchCardUntop = document.querySelector('.search-card-untop');

if (btn && input && searchCardUntop) {
    btn.addEventListener('click', () => {
        input.value = '/';
        input.focus();
        checkSearchCard();
    });

    input.addEventListener('input', function () {
        checkSearchCard();
    });
}

function checkSearchCard() {
    if (!searchCardUntop) return;
    const val = input.value.trim();
    if (val.includes('/')) {
        searchCardUntop.style.display = 'block';
    } else {
        searchCardUntop.style.display = 'none';
    }
}

function toggleFullscreen() {
    const btn = document.getElementById('fullBtn');
    if (!btn) return;

    const docEl = document.documentElement;
    const requestFullscreen = docEl.requestFullscreen ||
        docEl.webkitRequestFullscreen ||
        docEl.mozRequestFullScreen ||
        docEl.msRequestFullscreen;
    const exitFullscreen = document.exitFullscreen ||
        document.webkitExitFullscreen ||
        document.mozCancelFullScreen ||
        document.msExitFullscreen;
    const fullscreenElement = document.fullscreenElement ||
        document.webkitFullscreenElement ||
        document.mozFullScreenElement ||
        document.msFullscreenElement;

    try {
        if (!fullscreenElement) {
            if (requestFullscreen) {
                requestFullscreen.call(docEl);
            }
        } else {
            if (exitFullscreen) {
                exitFullscreen.call(document);
            }
        }
    } catch (e) {
        console.error('全屏操作失败:', e);
    }
}

async function refreshBg() {
    const saveBg = localStorage.getItem('chooseBg') || 'bing';
    if (typeof setBg === 'function') {
        if (saveBg.startsWith('360-') || saveBg === 'hd' || saveBg === 'sina' || saveBg === 'bing') {
            document.body.style.background = '';
            setTimeout(() => {
                setBg(saveBg);
            }, 50);
        } else {
            setBg(saveBg);
        }
    } else {
        const bgUrl = 'https://api.paugram.com/bing/?t=' + Date.now();
        document.body.style.background = `url("${bgUrl}") center center / cover fixed`;
    }
}

const bgOverlay = document.getElementById('bgOverlay');
const quickSettingsBox = document.querySelector('.QuickSettingsBox');

if (bgOverlay && quickSettingsBox) {
    let leaveTimeout;
    
    quickSettingsBox.addEventListener('mouseenter', () => {
        clearTimeout(leaveTimeout);
        bgOverlay.classList.add('active');
        quickSettingsBox.classList.add('active');
    });

    quickSettingsBox.addEventListener('mouseleave', () => {
        leaveTimeout = setTimeout(() => {
            bgOverlay.classList.remove('active');
            quickSettingsBox.classList.remove('active');
        }, 500);
    });
}

const toggleBtn = document.getElementById('toggleSearch');
const searchBox = document.querySelector('.SearchBox');

if (toggleBtn && searchBox) {
    toggleBtn.addEventListener('click', () => {
        searchBox.classList.toggle('hidden');
        if (searchBox.classList.contains('hidden')) {
            toggleBtn.textContent = '显示搜索框';
        } else {
            toggleBtn.textContent = '隐藏搜索框';
        }
    });
}

const toggleCardsBtn = document.getElementById('toggleCards');
const toggleableCards = document.querySelectorAll('.toggleable-card');
let cardsHidden = localStorage.getItem('cardsHidden') === 'true' || false;

const cardSwitchMap = {
    'switchWeather': 'weatherCard',
    'switchApp': 'appCard',
    'switchLights': 'lightsCard',
    'switchZhihu': 'zhihuCard'
};

if (cardsHidden) {
    toggleableCards.forEach(function (card) {
        card.classList.add('card-collapsed');
    });
    if (toggleCardsBtn) {
        toggleCardsBtn.textContent = '显示所有卡片';
    }
}

if (toggleCardsBtn) {
    toggleCardsBtn.addEventListener('click', () => {
        cardsHidden = !cardsHidden;
        localStorage.setItem('cardsHidden', cardsHidden.toString());

        toggleableCards.forEach(function (card) {
            if (cardsHidden) {
                card.classList.add('card-collapsed');
            } else {
                card.classList.remove('card-collapsed');
            }
        });
        toggleCardsBtn.textContent = cardsHidden ? '显示所有卡片' : '隐藏所有卡片';
    });
}

function scrollToCard(cardId, event) {
    if (event && typeof event.stopPropagation === 'function') {
        event.stopPropagation();
    }

    const target = document.getElementById(cardId);
    if (!target) return;

    const isCollapsed = target.classList.contains('card-collapsed');
    const shouldExpand = isCollapsed || cardsHidden;

    if (shouldExpand) {
        toggleableCards.forEach(function (card) {
            card.classList.remove('card-collapsed');
            card.classList.remove('card-highlight');
        });
        cardsHidden = false;
        localStorage.setItem('cardsHidden', 'false');
        if (toggleCardsBtn) {
            toggleCardsBtn.textContent = '隐藏所有卡片';
        }

        const switchId = Object.keys(cardSwitchMap).find(key => cardSwitchMap[key] === cardId);
        if (switchId) {
            const switchEl = document.getElementById(switchId);
            if (switchEl) {
                switchEl.checked = true;
                localStorage.setItem('cardVisible_' + cardId, 'true');
            }
        }
    }

    const scrollDelay = shouldExpand ? 450 : 100;
    setTimeout(() => {
        target.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }, scrollDelay);

    setTimeout(() => {
        target.classList.remove('card-highlight');
        setTimeout(() => {
            target.classList.add('card-highlight');
            setTimeout(function () {
                target.classList.remove('card-highlight');
            }, 1800);
        }, 50);
    }, shouldExpand ? 500 : 50);
}

function initCardSwitches() {
    Object.keys(cardSwitchMap).forEach(function(switchId) {
        const switchEl = document.getElementById(switchId);
        const cardId = cardSwitchMap[switchId];
        const cardEl = document.getElementById(cardId);

        if (!switchEl || !cardEl) return;

        const isVisible = localStorage.getItem('cardVisible_' + cardId);
        if (isVisible === 'false') {
            switchEl.checked = false;
            cardEl.classList.add('card-collapsed');
        } else {
            switchEl.checked = true;
            cardEl.classList.remove('card-collapsed');
            localStorage.setItem('cardVisible_' + cardId, 'true');
        }

        switchEl.addEventListener('change', function(e) {
            e.stopPropagation();
            const checked = this.checked;

            if (checked) {
                cardEl.classList.remove('card-collapsed');
                localStorage.setItem('cardVisible_' + cardId, 'true');
                loadCardData(cardId);
            } else {
                cardEl.classList.add('card-collapsed');
                localStorage.setItem('cardVisible_' + cardId, 'false');
            }
        });
    });
}

function loadCardData(cardId) {
    switch (cardId) {
        case 'weatherCard':
            if (typeof triggerLoadWeather === 'function') {
                triggerLoadWeather();
            } else {
                if (typeof LazyLoad !== 'undefined') {
                    LazyLoad.loadScript('js/weather.js');
                }
            }
            break;
        case 'zhihuCard':
            if (typeof LazyLoad !== 'undefined') {
                LazyLoad.loadScript('js/api/Portal/ZhihuHotSearch/HotSearch.js');
            }
            break;
        case 'lightsCard':
            break;
        case 'appCard':
            break;
        default:
            break;
    }
}

function toggleSearchLogo() {
    const logoTitle = document.querySelector('.searchInputBox h1');
    const switchLogo = document.getElementById('switchLogo');
    if (logoTitle && switchLogo) {
        logoTitle.style.display = switchLogo.checked ? 'block' : 'none';
        localStorage.setItem('searchLogoVisible', switchLogo.checked);
    }
}

function updateLogoType() {
    const logoTextRow = document.getElementById('logoTextRow');
    const logoImageRow = document.getElementById('logoImageRow');
    const textRadio = document.querySelector('input[name="logoType"][value="text"]');
    const logoTitle = document.querySelector('.searchInputBox h1');

    if (logoTextRow && logoImageRow && logoTitle) {
        if (textRadio.checked) {
            logoTextRow.style.display = 'flex';
            logoImageRow.style.display = 'none';
            const savedLogoText = localStorage.getItem('logoText') || '市舶司';
            const savedLogoSize = localStorage.getItem('logoSize') || 68;
            logoTitle.textContent = savedLogoText;
            logoTitle.style.fontSize = savedLogoSize + 'px';
        } else {
            logoTextRow.style.display = 'none';
            logoImageRow.style.display = 'flex';
            const savedLogoImage = localStorage.getItem('logoImage');
            if (savedLogoImage) {
                logoTitle.innerHTML = `<img src="${savedLogoImage}" style="width: 100%; max-height: 80px; object-fit: contain;">`;
            }
        }
        localStorage.setItem('logoType', textRadio.checked ? 'text' : 'image');
    }
}

function updateLogoText() {
    const logoText = document.getElementById('logoText');
    const logoSize = document.getElementById('logoSize');
    const logoTitle = document.querySelector('.searchInputBox h1');

    if (logoText && logoTitle) {
        const size = logoSize ? logoSize.value : 68;
        const text = logoText.value || '市舶司';
        logoTitle.textContent = text;
        logoTitle.style.fontSize = size + 'px';
        localStorage.setItem('logoText', text);
        localStorage.setItem('logoSize', size);
    }
}

function updateLogoSize() {
    const logoSize = document.getElementById('logoSize');
    const logoSizeValue = document.getElementById('logoSizeValue');
    const logoTitle = document.querySelector('.searchInputBox h1');

    if (logoSize && logoTitle) {
        const textRadio = document.querySelector('input[name="logoType"][value="text"]');
        if (textRadio && textRadio.checked) {
            const logoText = document.getElementById('logoText');
            const text = logoText ? (logoText.value || '市舶司') : '市舶司';
            logoTitle.textContent = text;
            logoTitle.style.fontSize = logoSize.value + 'px';
        }
        if (logoSizeValue) {
            logoSizeValue.textContent = logoSize.value + 'px';
        }
        localStorage.setItem('logoSize', logoSize.value);
    }
}

function uploadLogoImage() {
    const logoImageInput = document.getElementById('logoImageInput');
    const logoTitle = document.querySelector('.searchInputBox h1');

    if (logoImageInput && logoTitle && logoImageInput.files[0]) {
        const file = logoImageInput.files[0];
        const reader = new FileReader();
        reader.onload = function (e) {
            logoTitle.innerHTML = `<img src="${e.target.result}" style="width: 100%; max-height: 80px; object-fit: contain;">`;
            localStorage.setItem('logoImage', e.target.result);
        };
        reader.readAsDataURL(file);
    }
}

function updateSearchOpacity() {
    const searchOpacity = document.getElementById('searchOpacity');
    const opacityValue = document.getElementById('opacityValue');
    const searchBox = document.querySelector('.SearchBox');

    if (searchOpacity && searchBox) {
        const opacity = searchOpacity.value / 100;
        searchBox.style.background = `rgba(255, 255, 255, ${opacity})`;
        if (opacityValue) {
            opacityValue.textContent = searchOpacity.value + '%';
        }
        localStorage.setItem('searchOpacity', searchOpacity.value);
    }
}

function updateSearchWidth() {
    const searchWidth = document.getElementById('searchWidth');
    const widthValue = document.getElementById('widthValue');
    const searchBox = document.querySelector('.SearchBox');

    if (searchWidth && searchBox) {
        searchBox.style.maxWidth = searchWidth.value + 'px';
        if (widthValue) {
            widthValue.textContent = searchWidth.value + 'px';
        }
        localStorage.setItem('searchWidth', searchWidth.value);
    }
}

function updateSearchRadius() {
    const searchRadius = document.getElementById('searchRadius');
    const radiusValue = document.getElementById('radiusValue');
    const searchBox = document.querySelector('.SearchBox');
    const searchInput = document.getElementById('searchQuery');

    if (searchRadius && searchBox) {
        searchBox.style.borderRadius = searchRadius.value + 'px';
        if (searchInput) {
            searchInput.style.borderRadius = searchRadius.value + 'px';
        }
        if (radiusValue) {
            radiusValue.textContent = searchRadius.value + 'px';
        }
        localStorage.setItem('searchRadius', searchRadius.value);
    }
}

function updateSearchPlaceholder() {
    const searchPlaceholder = document.getElementById('searchPlaceholder');
    const searchInput = document.getElementById('searchQuery');

    if (searchPlaceholder && searchInput) {
        searchInput.placeholder = searchPlaceholder.value || '输入搜索内容';
        localStorage.setItem('searchPlaceholder', searchPlaceholder.value);
    }
}

function toggleAutoSlash() {
    const switchAutoSlash = document.getElementById('switchAutoSlash');
    const autoSlashBtn = document.getElementById('btn');

    if (autoSlashBtn) {
        autoSlashBtn.style.display = switchAutoSlash.checked ? 'flex' : 'none';
        localStorage.setItem('autoSlashVisible', switchAutoSlash.checked);
    }
}

function resetSearchSettings() {
    if (confirm('确定要恢复搜索框的默认设置吗？')) {
        localStorage.removeItem('searchLogoVisible');
        localStorage.removeItem('logoType');
        localStorage.removeItem('logoText');
        localStorage.removeItem('logoSize');
        localStorage.removeItem('logoImage');
        localStorage.removeItem('searchOpacity');
        localStorage.removeItem('searchWidth');
        localStorage.removeItem('searchRadius');
        localStorage.removeItem('searchPlaceholder');
        localStorage.removeItem('autoSlashVisible');

        const switchLogo = document.getElementById('switchLogo');
        const textRadio = document.querySelector('input[name="logoType"][value="text"]');
        const imageRadio = document.querySelector('input[name="logoType"][value="image"]');
        const logoText = document.getElementById('logoText');
        const logoSize = document.getElementById('logoSize');
        const logoSizeValue = document.getElementById('logoSizeValue');
        const searchOpacity = document.getElementById('searchOpacity');
        const opacityValue = document.getElementById('opacityValue');
        const searchWidth = document.getElementById('searchWidth');
        const widthValue = document.getElementById('widthValue');
        const searchRadius = document.getElementById('searchRadius');
        const radiusValue = document.getElementById('radiusValue');
        const searchPlaceholder = document.getElementById('searchPlaceholder');
        const switchAutoSlash = document.getElementById('switchAutoSlash');
        const logoTitle = document.querySelector('.searchInputBox h1');
        const searchBox = document.querySelector('.SearchBox');
        const searchInput = document.getElementById('searchQuery');
        const autoSlashBtn = document.getElementById('btn');
        const logoTextRow = document.getElementById('logoTextRow');
        const logoImageRow = document.getElementById('logoImageRow');

        if (switchLogo) switchLogo.checked = true;
        if (textRadio) textRadio.checked = true;
        if (imageRadio) imageRadio.checked = false;
        if (logoText) logoText.value = '市舶司';
        if (logoSize) logoSize.value = 68;
        if (logoSizeValue) logoSizeValue.textContent = '68px';
        if (searchOpacity) searchOpacity.value = 25;
        if (opacityValue) opacityValue.textContent = '25%';
        if (searchWidth) searchWidth.value = 400;
        if (widthValue) widthValue.textContent = '400px';
        if (searchRadius) searchRadius.value = 25;
        if (radiusValue) radiusValue.textContent = '25px';
        if (searchPlaceholder) searchPlaceholder.value = '输入搜索内容';
        if (switchAutoSlash) switchAutoSlash.checked = true;

        if (logoTitle) {
            logoTitle.textContent = '市舶司';
            logoTitle.style.fontSize = '68px';
            logoTitle.style.display = 'block';
        }
        if (searchBox) {
            searchBox.style.background = 'rgba(255, 255, 255, 0.25)';
            searchBox.style.maxWidth = '400px';
            searchBox.style.borderRadius = '25px';
        }
        if (searchInput) {
            searchInput.style.borderRadius = '25px';
            searchInput.placeholder = '输入搜索内容';
        }
        if (autoSlashBtn) {
            autoSlashBtn.style.display = 'flex';
        }
        if (logoTextRow) logoTextRow.style.display = 'flex';
        if (logoImageRow) logoImageRow.style.display = 'none';
    }
}

document.addEventListener('DOMContentLoaded', initCardSwitches);

function initSearchSettings() {
    const savedLogoVisible = localStorage.getItem('searchLogoVisible');
    const savedLogoType = localStorage.getItem('logoType');
    const savedLogoText = localStorage.getItem('logoText');
    const savedLogoSize = localStorage.getItem('logoSize');
    const savedSearchOpacity = localStorage.getItem('searchOpacity');
    const savedSearchWidth = localStorage.getItem('searchWidth');
    const savedSearchRadius = localStorage.getItem('searchRadius');
    const savedSearchPlaceholder = localStorage.getItem('searchPlaceholder');
    const savedAutoSlashVisible = localStorage.getItem('autoSlashVisible');

    const switchLogo = document.getElementById('switchLogo');
    const textRadio = document.querySelector('input[name="logoType"][value="text"]');
    const imageRadio = document.querySelector('input[name="logoType"][value="image"]');
    const logoText = document.getElementById('logoText');
    const logoSize = document.getElementById('logoSize');
    const logoSizeValue = document.getElementById('logoSizeValue');
    const searchOpacity = document.getElementById('searchOpacity');
    const opacityValue = document.getElementById('opacityValue');
    const searchWidth = document.getElementById('searchWidth');
    const widthValue = document.getElementById('widthValue');
    const searchRadius = document.getElementById('searchRadius');
    const radiusValue = document.getElementById('radiusValue');
    const searchPlaceholder = document.getElementById('searchPlaceholder');
    const switchAutoSlash = document.getElementById('switchAutoSlash');
    const logoTitle = document.querySelector('.searchInputBox h1');
    const searchBox = document.querySelector('.SearchBox');
    const searchInput = document.getElementById('searchQuery');
    const autoSlashBtn = document.getElementById('btn');
    const logoTextRow = document.getElementById('logoTextRow');
    const logoImageRow = document.getElementById('logoImageRow');

    if (savedLogoVisible === 'false') {
        if (switchLogo) switchLogo.checked = false;
        if (logoTitle) logoTitle.style.display = 'none';
    }

    if (savedLogoType === 'image') {
        if (imageRadio) imageRadio.checked = true;
        if (textRadio) textRadio.checked = false;
        if (logoTextRow) logoTextRow.style.display = 'none';
        if (logoImageRow) logoImageRow.style.display = 'flex';
        const savedLogoImage = localStorage.getItem('logoImage');
        if (savedLogoImage && logoTitle) {
            logoTitle.innerHTML = `<img src="${savedLogoImage}" style="width: 100%; max-height: 80px; object-fit: contain;">`;
        }
    }

    if (savedLogoText) {
        if (logoText) logoText.value = savedLogoText;
        if (logoTitle && savedLogoType !== 'image') {
            logoTitle.textContent = savedLogoText;
        }
    }

    if (savedLogoSize) {
        if (logoSize) logoSize.value = savedLogoSize;
        if (logoSizeValue) logoSizeValue.textContent = savedLogoSize + 'px';
        if (logoTitle && savedLogoType !== 'image') {
            logoTitle.style.fontSize = savedLogoSize + 'px';
        }
    }

    if (savedSearchOpacity) {
        if (searchOpacity) searchOpacity.value = savedSearchOpacity;
        if (opacityValue) opacityValue.textContent = savedSearchOpacity + '%';
        if (searchBox) {
            searchBox.style.background = `rgba(255, 255, 255, ${savedSearchOpacity / 100})`;
        }
    }

    if (savedSearchWidth) {
        if (searchWidth) searchWidth.value = savedSearchWidth;
        if (widthValue) widthValue.textContent = savedSearchWidth + 'px';
        if (searchBox) {
            searchBox.style.maxWidth = savedSearchWidth + 'px';
        }
    }

    if (savedSearchRadius) {
        if (searchRadius) searchRadius.value = savedSearchRadius;
        if (radiusValue) radiusValue.textContent = savedSearchRadius + 'px';
        if (searchBox) {
            searchBox.style.borderRadius = savedSearchRadius + 'px';
        }
        if (searchInput) {
            searchInput.style.borderRadius = savedSearchRadius + 'px';
        }
    }

    if (savedSearchPlaceholder) {
        if (searchPlaceholder) searchPlaceholder.value = savedSearchPlaceholder;
        if (searchInput) searchInput.placeholder = savedSearchPlaceholder;
    }

    if (savedAutoSlashVisible === 'false') {
        if (switchAutoSlash) switchAutoSlash.checked = false;
        if (autoSlashBtn) autoSlashBtn.style.display = 'none';
    }
}

document.addEventListener('DOMContentLoaded', initSearchSettings);

window.SEARCH_ENGINE_BASE = SEARCH_ENGINE_BASE;
window.searchLocalEngine = searchLocalEngine;
window.closeSearchEngine = closeSearchEngine;
window.openFullSearchEngine = openFullSearchEngine;