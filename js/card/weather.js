/* ========================================
   天气图标映射表
   ======================================== */
var weatherIconMap = {
    '100': 'weather-sunny.svg', '101': 'weather-cloudy.svg', '102': 'weather-cloudy.svg',
    '103': 'weather-lightning-rainy.svg', '104': 'weather-lightning.svg',
    '200': 'weather-windy.svg',
    '301': 'weather-rainy.svg', '302': 'weather-pouring.svg', '303': 'weather-lightning-rainy.svg',
    '304': 'weather-lightning.svg',
    '307': 'weather-rainy.svg', '308': 'weather-pouring.svg', '309': 'weather-partly-rainy.svg',
    '310': 'weather-pouring.svg', '311': 'weather-pouring.svg', '312': 'weather-pouring.svg',
    '401': 'weather-snowy.svg', '402': 'weather-snowy.svg', '403': 'weather-snowy.svg',
    '404': 'weather-snowy-heavy.svg', '405': 'weather-snowy-heavy.svg',
    '406': 'weather-snowy-rainy.svg', '407': 'weather-snowy.svg', '408': 'weather-night.svg',
    '409': 'weather-snowy-rainy.svg', '410': 'weather-fog.svg',
    '456': 'weather-night-partly-cloudy.svg', '457': 'weather-night-partly-cloudy.svg',
    '499': 'weather-cloudy.svg', '500': 'weather-fog.svg',
    '502': 'weather-dust.svg', '503': 'weather-dust.svg', '507': 'weather-tornado.svg',
    '999': 'weather-cloudy.svg'
};

/* ========================================
   天气文本到图标代码的映射表（备用方案）
   ======================================== */
var weatherTextToIconMap = {
    '晴': '100', '晴朗': '100', '晴天': '100',
    '多云': '101', '少云': '101', '晴间多云': '101', '局部多云': '101',
    '阴': '102', '阴天': '102',
    '雷阵雨': '103', '雷暴': '103', '雷雨': '103',
    '雷阵雨冰雹': '104', '雷暴冰雹': '104',
    '阵风': '200',
    '雨': '301', '小雨': '301', '小到中雨': '301',
    '冻雨': '302', '大雨': '308', '中雨': '307',
    '毛毛雨': '309', '细雨': '309',
    '暴雨': '310', '大暴雨': '311', '特大暴雨': '312',
    '雪': '401', '小雪': '402', '小到中雪': '402',
    '中雪': '403', '大雪': '404', '大到暴雪': '404',
    '暴雪': '405',
    '雨夹雪': '406', '雪夹雨': '406',
    '阵雪': '407',
    '夜间阵雪': '408',
    '雾': '410', '大雾': '410',
    '沙尘': '502', '扬沙': '502', '浮尘': '503', '沙尘暴': '507',
    '龙卷风': '507'
};

function getWeatherIconCodeFromText(weatherText) {
    if (!weatherText) return null;
    var text = weatherText.trim();
    for (var key in weatherTextToIconMap) {
        if (text.indexOf(key) !== -1) {
            return weatherTextToIconMap[key];
        }
    }
    return null;
}

function getWeatherIconPath(iconCode, weatherText) {
    var iconFile;
    if (weatherIconMap[iconCode]) {
        iconFile = weatherIconMap[iconCode];
    } else {
        var fallbackCode = getWeatherIconCodeFromText(weatherText);
        if (fallbackCode && weatherIconMap[fallbackCode]) {
            iconFile = weatherIconMap[fallbackCode];
        } else {
            iconFile = 'weather-sunny.svg';
        }
    }
    return './img/weather/img/' + iconFile;
}

/* ---- 天气emoji ---- */
function getWeatherEmoji(iconCode) {
    var m = {
        '100': '☀️','101': '⛅','102': '☁️','103': '⛈️','104': '⛈️','200': '💨',
        '301': '🌧️','302': '🌧️','303': '⛈️','304': '⛈️','307': '🌧️','308': '🌧️',
        '309': '🌦️','310': '🌧️','311': '🌧️','312': '🌧️',
        '401': '🌨️','402': '🌨️','403': '🌨️','404': '❄️','405': '❄️','406': '🌨️',
        '407': '🌨️','408': '🌙','409': '🌨️','410': '🌫️',
        '500': '🌫️','502': '🌪️','503': '🌪️','507': '🌪️'
    };
    return m[iconCode] || '🌤️';
}

/* ---- 获取AQI等级描述 ---- */
function getAqiInfo(aqi) {
    if (aqi === undefined || aqi === null) return null;
    if (aqi <= 50) return { label: '优', color: '#00e400' };
    if (aqi <= 100) return { label: '良', color: '#ffff00' };
    if (aqi <= 150) return { label: '轻度污染', color: '#ff7e00' };
    if (aqi <= 200) return { label: '中度污染', color: '#ff0000' };
    if (aqi <= 300) return { label: '重度污染', color: '#99004c' };
    return { label: '严重污染', color: '#7e0023' };
}

/* ---- 格式化时间 ---- */
function formatTime(dateStr) {
    if (!dateStr) return '';
    var parts = dateStr.split(' ');
    if (parts.length >= 2) {
        var t = parts[1].split(':');
        return t[0] + ':' + t[1];
    }
    return dateStr;
}

/* ========================================
   天气数据 & 状态
   ======================================== */
var weatherData = null;
var loadedTabs = {};

/* ---- 渲染标签页框架 ---- */
function renderTabs(data) {
    var container = document.getElementById('weatherContent');
    if (!container) return;

    weatherData = data;
    var hasAlerts = data.alerts && data.alerts.length > 0;
    var hasAqi = data.aqi !== undefined && data.aqi !== null;
    var hasForecast = data.forecast && data.forecast.length > 0;

    var html =
        '<div class="weather-tabs">' +
            '<div class="weather-tab-bar">' +
                '<button class="weather-tab-btn active" data-tab="now" onclick="switchWeatherTab(\'now\')">🌤 实况</button>' +
                (hasForecast ? '<button class="weather-tab-btn" data-tab="fc" onclick="switchWeatherTab(\'fc\')">📅 预报</button>' : '') +
                (hasAqi ? '<button class="weather-tab-btn" data-tab="air" onclick="switchWeatherTab(\'air\')">🌬 空气</button>' : '') +
                '<button class="weather-tab-btn" data-tab="tips" onclick="switchWeatherTab(\'tips\')">📋 生活' +
                    (hasAlerts ? '<span class="weather-alert-dot"></span>' : '') +
                '</button>' +
            '</div>' +
            '<div class="weather-tab-content" id="weatherTabNow"></div>' +
            (hasForecast ? '<div class="weather-tab-content weather-tab-hide" id="weatherTabFc"></div>' : '') +
            (hasAqi ? '<div class="weather-tab-content weather-tab-hide" id="weatherTabAir"></div>' : '') +
            '<div class="weather-tab-content weather-tab-hide" id="weatherTabTips"></div>' +
        '</div>';

    container.innerHTML = html;

    // 默认加载第1个标签页
    loadedTabs = {};
    renderNowTab(data);
}

/* ---- 标签页切换 ---- */
function switchWeatherTab(tabId) {
    var container = document.getElementById('weatherContent');
    if (!container || !weatherData) return;

    // 切换按钮高亮
    container.querySelectorAll('.weather-tab-btn').forEach(function(btn) {
        btn.classList.toggle('active', btn.dataset.tab === tabId);
    });

    // 切换内容区显示
    container.querySelectorAll('.weather-tab-content').forEach(function(el) {
        if (el.id === 'weatherTab' + tabId.charAt(0).toUpperCase() + tabId.slice(1)) {
            el.classList.remove('weather-tab-hide');
        } else {
            el.classList.add('weather-tab-hide');
        }
    });

    // 懒加载
    if (!loadedTabs[tabId]) {
        loadedTabs[tabId] = true;
        if (tabId === 'fc') renderFcTab(weatherData);
        else if (tabId === 'air') renderAirTab(weatherData);
        else if (tabId === 'tips') renderTipsTab(weatherData);
    }
}

/* ========================================
   Tab 1: 实况天气
   ======================================== */
function renderNowTab(data) {
    var el = document.getElementById('weatherTabNow');
    if (!el) return;

    var iconPath = getWeatherIconPath(data.weather_icon, data.weather);
    var location = data.city || '';
    if (data.province && data.province !== data.city) location = data.province + '·' + data.city;
    
    var iconCode = data.weather_icon || getWeatherIconCodeFromText(data.weather);
    var weatherEmoji = getWeatherEmoji(iconCode);

    var windFull = data.wind_direction || '';
    if (data.wind_power) windFull += ' ' + data.wind_power;
    if (!windFull) windFull = '暂无数据';

    var humidityStr = data.humidity !== undefined ? data.humidity + '%' : '--';
    var feelsLike = data.feels_like !== undefined ? data.feels_like + '°C' : '--';
    var uvInfo = data.uv !== undefined ? data.uv + '级' : '--';
    var visInfo = data.visibility !== undefined ? data.visibility + 'km' : '--';
    var pressInfo = data.pressure !== undefined ? data.pressure + 'hPa' : '--';

    // 预警提示条
    var alertBanner = '';
    if (data.alerts && data.alerts.length > 0) {
        var topAlert = data.alerts[0];
        var levelColor = topAlert.level === '红色' ? '#e74c3c' : topAlert.level === '橙色' ? '#ff7e00' : topAlert.level === '黄色' ? '#f1c40f' : '#3498db';
        alertBanner = '<div class="weather-alert-banner" onclick="switchWeatherTab(\'tips\')" style="border-left-color:' + levelColor + '">' +
            '<span class="alert-badge-sm" style="background:' + levelColor + '">' + topAlert.level + '</span>' +
            '<span class="alert-text-sm">' + topAlert.title + '</span>' +
            '<span class="alert-arrow">›</span>' +
        '</div>';
    }

    el.innerHTML =
        '<div class="weather-now-top">' +
            '<div class="weather-left">' +
                '<img class="weather-icon lazy-img" data-src="' + iconPath + '" alt="' + data.weather + '" src="data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 24 24\'%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'10\' fill=\'%23eee\'/%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'6\' fill=\'%23ddd\'/%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'2\' fill=\'%23ccc\'/%3E%3C/svg%3E">' +
                '<div class="weather-temp">' +
                    '<span class="temp-value">' + data.temperature + '</span>' +
                    '<span class="temp-unit">°C</span>' +
                '</div>' +
            '</div>' +
            '<div class="weather-right">' +
                '<div class="weather-city">' + location + '</div>' +
                '<div class="weather-desc">' + weatherEmoji + ' ' + data.weather + '</div>' +
            '</div>' +
        '</div>' +
        alertBanner +
        '<div class="weather-ext-grid">' +
            '<div class="weather-ext-item"><span class="ext-label">体感</span><span class="ext-val">' + feelsLike + '</span></div>' +
            '<div class="weather-ext-item"><span class="ext-label">湿度</span><span class="ext-val">' + humidityStr + '</span></div>' +
            '<div class="weather-ext-item"><span class="ext-label">紫外线</span><span class="ext-val">' + uvInfo + '</span></div>' +
            '<div class="weather-ext-item"><span class="ext-label">能见度</span><span class="ext-val">' + visInfo + '</span></div>' +
            '<div class="weather-ext-item"><span class="ext-label">风力</span><span class="ext-val">' + windFull + '</span></div>' +
            '<div class="weather-ext-item"><span class="ext-label">气压</span><span class="ext-val">' + pressInfo + '</span></div>' +
        '</div>' +
        '<div class="weather-report-time">更新时间 ' + (formatTime(data.report_time) || data.report_time || '') + '</div>';

    loadedTabs.now = true;
    
    if (window.LazyLoad) {
        setTimeout(function() {
            LazyLoad.observeElements();
        }, 100);
    }
}

/* ========================================
   Tab 2: 天气预报（懒加载）
   ======================================== */
function renderFcTab(data) {
    var el = document.getElementById('weatherTabFc');
    if (!el || !data.forecast) return;

    var html = '<div class="fc-list">';
    var today = new Date().toISOString().split('T')[0];

    data.forecast.forEach(function(day) {
        var isToday = day.date === today;
        var weatherDesc = day.weather_day || '--';
        
        var iconCode = day.weather_icon_day || day.weather_icon || getWeatherIconCodeFromText(weatherDesc);
        var iconDay = getWeatherIconPath(iconCode, weatherDesc);
        var emojiDay = getWeatherEmoji(iconCode);

        var popHtml = day.pop !== undefined ? '<span class="fc-pop ' + (day.pop > 50 ? 'fc-pop-high' : '') + '">💧 ' + day.pop + '%</span>' : '';

        html +=
            '<div class="fc-day' + (isToday ? ' fc-today' : '') + '">' +
                '<div class="fc-date">' +
                    '<span class="fc-week">' + (isToday ? '今天' : (day.week || '')) + '</span>' +
                    '<span class="fc-day-date">' + day.date.slice(5) + '</span>' +
                '</div>' +
                '<div class="fc-icon-temp">' +
                    '<img class="fc-icon lazy-img" data-src="' + iconDay + '" alt="' + weatherDesc + '" src="data:image/svg+xml,%3Csvg xmlns=\'http://www.w3.org/2000/svg\' viewBox=\'0 0 24 24\'%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'10\' fill=\'%23eee\'/%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'6\' fill=\'%23ddd\'/%3E%3Ccircle cx=\'12\' cy=\'12\' r=\'2\' fill=\'%23ccc\'/%3E%3C/svg%3E">' +
                    '<div class="fc-temps">' +
                        '<span class="fc-temp-high">' + (day.temp_max !== undefined ? day.temp_max + '°' : '--') + '</span>' +
                        '<span class="fc-temp-low">' + (day.temp_min !== undefined ? day.temp_min + '°' : '--') + '</span>' +
                    '</div>' +
                '</div>' +
                '<div class="fc-weather">' + emojiDay + ' ' + weatherDesc + '</div>' +
                '<div class="fc-extras">' +
                    popHtml +
                    (day.uv_index !== undefined ? '<span class="fc-uv">☀ ' + day.uv_index + '</span>' : '') +
                    (day.wind_scale_day ? '<span class="fc-wind">💨 ' + day.wind_scale_day + '</span>' : '') +
                '</div>' +
            '</div>';
    });

    html += '</div>';
    el.innerHTML = html;
    
    if (window.LazyLoad) {
        setTimeout(function() {
            LazyLoad.observeElements();
        }, 100);
    }
}

/* ========================================
   Tab 3: 空气质量（懒加载）
   ======================================== */
function renderAirTab(data) {
    var el = document.getElementById('weatherTabAir');
    if (!el) return;

    var aqiInfo = getAqiInfo(data.aqi);
    var aqiColor = aqiInfo ? aqiInfo.color : '#999';
    var aqiLabel = aqiInfo ? aqiInfo.label : '--';

    var pollutants = data.air_pollutants || {};

    el.innerHTML =
        '<div class="air-top">' +
            '<div class="air-aqi-display">' +
                '<div class="air-aqi-circle" style="border-color:' + aqiColor + ';color:' + aqiColor + '">' +
                    '<span class="air-aqi-num">' + (data.aqi !== undefined ? data.aqi : '--') + '</span>' +
                    '<span class="air-aqi-label">' + aqiLabel + '</span>' +
                '</div>' +
            '</div>' +
            '<div class="air-aqi-info">' +
                '<div class="air-info-row"><span>首要污染物</span><span class="air-info-val">' + (data.aqi_primary || '无') + '</span></div>' +
                '<div class="air-info-row"><span>空气质量</span><span class="air-info-val">' + (data.aqi_category || aqiLabel) + '</span></div>' +
                '<div class="air-info-row"><span>级别</span><span class="air-info-val">' + (data.aqi_level !== undefined ? data.aqi_level + '级' : '--') + '</span></div>' +
            '</div>' +
        '</div>' +
        '<div class="air-pollutants">' +
            '<div class="air-poll-item"><span>PM2.5</span><span class="poll-val">' + (pollutants.pm25 !== undefined ? pollutants.pm25 + ' μg' : '--') + '</span></div>' +
            '<div class="air-poll-item"><span>PM10</span><span class="poll-val">' + (pollutants.pm10 !== undefined ? pollutants.pm10 + ' μg' : '--') + '</span></div>' +
            '<div class="air-poll-item"><span>O₃</span><span class="poll-val">' + (pollutants.o3 !== undefined ? pollutants.o3 + ' μg' : '--') + '</span></div>' +
            '<div class="air-poll-item"><span>NO₂</span><span class="poll-val">' + (pollutants.no2 !== undefined ? pollutants.no2 + ' μg' : '--') + '</span></div>' +
            '<div class="air-poll-item"><span>SO₂</span><span class="poll-val">' + (pollutants.so2 !== undefined ? pollutants.so2 + ' μg' : '--') + '</span></div>' +
            '<div class="air-poll-item"><span>CO</span><span class="poll-val">' + (pollutants.co !== undefined ? pollutants.co + ' mg' : '--') + '</span></div>' +
        '</div>';
}

/* ========================================
   Tab 3: 生活提示 + 预警（懒加载）
   ======================================== */
function renderTipsTab(data) {
    var el = document.getElementById('weatherTabTips');
    if (!el) return;

    var tips = [];

    // 根据天气数据生成生活提示
    if (data.temperature !== undefined) {
        var t = data.temperature;
        if (t >= 35) tips.push({ icon: '🥵', text: '气温极高，注意防暑降温，避免长时间户外活动' });
        else if (t >= 30) tips.push({ icon: '🥵', text: '天气较热，注意补充水分，做好防晒' });
        else if (t >= 25) tips.push({ icon: '🌤', text: '气温舒适，适合户外活动' });
        else if (t >= 20) tips.push({ icon: '🌿', text: '天气宜人，适宜出行' });
        else if (t >= 15) tips.push({ icon: '🍂', text: '气温适中，建议带一件外套' });
        else if (t >= 10) tips.push({ icon: '🧥', text: '气温偏凉，注意添衣保暖' });
        else if (t >= 5) tips.push({ icon: '🧣', text: '天气较冷，建议穿厚外套或毛衣' });
        else if (t >= 0) tips.push({ icon: '🧤', text: '气温寒冷，需穿羽绒服等保暖衣物' });
        else tips.push({ icon: '❄️', text: '气温零下，注意防寒保暖，小心路面结冰' });
    }

    if (data.humidity !== undefined) {
        if (data.humidity >= 85) tips.push({ icon: '💧', text: '湿度过高，体感闷热，注意除湿通风' });
        else if (data.humidity <= 30) tips.push({ icon: '🏜️', text: '空气干燥，多喝水并使用保湿护肤品' });
    }

    if (data.uv !== undefined) {
        if (data.uv >= 7) tips.push({ icon: '🧴', text: '紫外线很强，外出务必涂抹防晒霜' });
        else if (data.uv >= 5) tips.push({ icon: '🕶', text: '紫外线较强，建议做好防晒措施' });
        else if (data.uv <= 2) tips.push({ icon: '☁️', text: '紫外线较弱，无需特别防护' });
    }

    if (data.visibility !== undefined) {
        if (data.visibility < 5) tips.push({ icon: '🌫️', text: '能见度较低，驾驶请注意安全' });
    }

    if (data.aqi !== undefined) {
        if (data.aqi > 150) tips.push({ icon: '😷', text: '空气质量较差，建议佩戴口罩出行' });
        else if (data.aqi > 100) tips.push({ icon: '😷', text: '空气质量一般，敏感人群注意防护' });
        else if (data.aqi <= 50) tips.push({ icon: '🌳', text: '空气清新，适合开窗通风' });
    }

    // 默认提示
    if (tips.length === 0) {
        tips.push({ icon: '👍', text: '天气条件良好，祝您生活愉快！' });
    }

    // 预警信息
    var alertsHtml = '';
    if (data.alerts && data.alerts.length > 0) {
        alertsHtml = '<div class="weather-alerts">';
        data.alerts.forEach(function(alert) {
            var levelClass = alert.level === '红色' ? 'alert-red' : alert.level === '橙色' ? 'alert-orange' : alert.level === '黄色' ? 'alert-yellow' : alert.level === '蓝色' ? 'alert-blue' : '';
            var guidanceHtml = '';
            if (alert.guidance && alert.guidance.length > 0) {
                guidanceHtml = '<ul class="alert-guidance">';
                alert.guidance.forEach(function(g) {
                    guidanceHtml += '<li>' + g + '</li>';
                });
                guidanceHtml += '</ul>';
            }
            alertsHtml +=
                '<div class="alert-item ' + levelClass + '">' +
                    '<div class="alert-header">' +
                        '<span class="alert-badge">' + alert.level + '预警</span>' +
                        '<span class="alert-title">' + alert.title + '</span>' +
                    '</div>' +
                    '<div class="alert-body">' +
                        '<p class="alert-text">' + alert.text + '</p>' +
                        guidanceHtml +
                    '</div>' +
                '</div>';
        });
        alertsHtml += '</div>';
    }

    var tipsHtml = '<div class="life-tips">';
    tips.forEach(function(tip) {
        tipsHtml += '<div class="life-tip-item"><span class="life-tip-icon">' + tip.icon + '</span><span class="life-tip-text">' + tip.text + '</span></div>';
    });
    tipsHtml += '</div>';

    el.innerHTML = alertsHtml + tipsHtml;
}


/* ---- 加载天气数据 ---- */
async function loadWeather() {
    try {
        var response = await fetch('https://uapis.cn/api/v1/misc/weather?extended=true&forecast=true');
        if (!response.ok) throw new Error('HTTP ' + response.status);
        var data = await response.json();
        renderTabs(data);
    } catch (err) {
        console.error('天气加载失败:', err);
        var container = document.getElementById('weatherContent');
        if (container) {
            container.innerHTML = '<p class="weather-error">天气加载失败，请稍后重试</p>';
        }
    }
}

/* ---- 页面加载后获取天气 ---- */
function initWeather() {
    // 检查天气卡片是否开启（从 localStorage 获取状态）
    const isWeatherEnabled = localStorage.getItem('cardVisible_weatherCard');
    if (isWeatherEnabled !== 'false') {
        loadWeather();
    }
}

// 页面加载完成时初始化
document.addEventListener('DOMContentLoaded', initWeather);

// 如果脚本加载时页面已经完成，立即初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initWeather);
} else {
    initWeather();
}

/* ---- 手动触发加载天气 ---- */
function triggerLoadWeather() {
    loadWeather();
}