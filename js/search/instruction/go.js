// ============================================================
//  搜索引擎管理中心（全局单例）
// ============================================================
window.SearchEngineManager = (function () {
  var SEARCH_ENGINE_BASE = 'http://localhost:8081';

  var DEFAULT_ENGINES = {
    "local":      { name: "市舶司", url: "__local__" },
    "localGO":      { name: "市舶司GO", url: "http://localhost:8081/?q=" },
    "bilibili":   { name: "哔哩哔哩", url: "https://search.bilibili.com/all?keyword=" },
    "douyin":     { name: "抖音",     url: "https://www.douyin.com/root/search/" },
    "baidu":      { name: "百度",     url: "https://www.baidu.com/s?wd=" },
    "sogou":      { name: "搜狗",     url: "https://www.sogou.com/web?query=" },
    "360":        { name: "360",      url: "https://www.so.com/s?ie=UTF-8&q=" },
    "360ai":      { name: "360AI",    url: "https://www.so.com/s?ie=UTF-8&q=" },
    "yande":      { name: "Yandex",   url: "https://yandex.com/search/?text=" },
    "magi":       { name: "magi",     url: "https://magi.com/search?q=" },
    "duckduckgo": { name: "DuckDuckGo",url: "https://duckduckgo.com/?q=" },
    "github":     { name: "GitHub",   url: "https://github.com/search?q=" },
    "bing":       { name: "Bing",     url: "https://www.bing.com/search?q=" },
    "tx":         { name: "腾讯下载", url: "https://pc.qq.com/search.html#!keyword=" },
    "google":     { name: "Google",   url: "https://www.google.com/search?q=" },
    "163music":   { name: "网易云音乐",url:"https://music.163.com/#/search/m/?s=" },
    "qqmusic":    { name: "QQ音乐",  url: "https://y.qq.com/n/ryqq/search?w=" },
  };

  var STORAGE_KEY_ENGINES = 'custom_engines';
  var STORAGE_KEY_SELECTED = 'selected_engine';

  var engines = {};
  var customIds = [];

  function saveCustomEngines() {
    var custom = {};
    customIds.forEach(function (id) { custom[id] = engines[id]; });
    try { localStorage.setItem(STORAGE_KEY_ENGINES, JSON.stringify(custom)); } catch(e) {}
  }

  function loadCustomEngines() {
    try {
      var raw = localStorage.getItem(STORAGE_KEY_ENGINES);
      if (raw) { return JSON.parse(raw); }
    } catch(e) {}
    return {};
  }

  function saveSelected(id) {
    try { localStorage.setItem(STORAGE_KEY_SELECTED, id); } catch(e) {}
  }

  function loadSelected() {
    try { return localStorage.getItem(STORAGE_KEY_SELECTED); } catch(e) { return null; }
  }

  function init() {
    for (var k in DEFAULT_ENGINES) { engines[k] = DEFAULT_ENGINES[k]; }
    var custom = loadCustomEngines();
    for (var k in custom) {
      engines[k] = custom[k];
      customIds.push(k);
    }
    syncSelectDOM();
    var lastId = loadSelected();
    if (lastId && engines[lastId]) {
      var sel = document.getElementById('searchEngine');
      if (sel) sel.value = lastId;
    }
    updatePlaceholder();
    var sel = document.getElementById('searchEngine');
    if (sel) {
      sel.addEventListener('change', function () {
        saveSelected(this.value);
        updatePlaceholder();
      });
    }
  }

  function syncSelectDOM() {
    var sel = document.getElementById('searchEngine');
    if (!sel) return;
    var cur = sel.value;
    sel.innerHTML = '';
    for (var id in engines) {
      var opt = document.createElement('option');
      opt.value = id;
      opt.textContent = engines[id].name;
      sel.appendChild(opt);
    }
    if (engines[cur]) { sel.value = cur; }
  }

  function updatePlaceholder() {
    var sel = document.getElementById('searchEngine');
    var input = document.getElementById('searchQuery');
    if (sel && input && engines[sel.value]) {
      input.placeholder = engines[sel.value].name + ' 搜索';
    }
  }

  function getEngines() { return engines; }

  function getUrl(query) {
    var sel = document.getElementById('searchEngine');
    var engineId = sel ? sel.value : 'bing';
    if (!engines[engineId]) engineId = 'bing';
    return engines[engineId].url + encodeURIComponent(query);
  }

  function addEngine(id, name, url) {
    if (!id || !name || !url) return false;
    if (DEFAULT_ENGINES[id]) return false;
    engines[id] = { name: name, url: url };
    customIds.push(id);
    saveCustomEngines();
    syncSelectDOM();
    return true;
  }

  function removeEngine(id) {
    var idx = customIds.indexOf(id);
    if (idx === -1) return false;
    delete engines[id];
    customIds.splice(idx, 1);
    saveCustomEngines();
    syncSelectDOM();
    return true;
  }

  function getCustomList() {
    var list = [];
    customIds.forEach(function (id) {
      if (engines[id]) {
        list.push({ id: id, name: engines[id].name, url: engines[id].url });
      }
    });
    return list;
  }

  return {
    init: init,
    getEngines: getEngines,
    getUrl: getUrl,
    addEngine: addEngine,
    removeEngine: removeEngine,
    updatePlaceholder: updatePlaceholder,
    saveSelected: saveSelected,
    syncSelectDOM: syncSelectDOM,
    getCustomList: getCustomList
  };
})();


// ============================================================
//  搜索历史管理（全局单例）
// ============================================================
window.SearchHistoryManager = (function () {
  var STORAGE_KEY = 'search_history';
  var MAX_RECORDS = 20;

  function load() {
    try {
      var raw = localStorage.getItem(STORAGE_KEY);
      return raw ? JSON.parse(raw) : [];
    } catch(e) { return []; }
  }

  function save(list) {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(list)); } catch(e) {}
  }

  function addRecord(keyword, engineId) {
    var list = load();
    list = list.filter(function (r) { return r.keyword !== keyword; });
    list.unshift({ keyword: keyword, engineId: engineId, time: Date.now() });
    if (list.length > MAX_RECORDS) list = list.slice(0, MAX_RECORDS);
    save(list);
    renderHistoryUI();
  }

  function getAll() { return load(); }

  function clearAll() {
    save([]);
    renderHistoryUI();
  }

  function deleteOne(keyword) {
    var list = load().filter(function (r) { return r.keyword !== keyword; });
    save(list);
    renderHistoryUI();
  }

  function renderHistoryUI() {
    var container = document.getElementById('searchHistoryList');
    if (!container) return;
    var list = getAll();
    if (list.length === 0) {
      container.innerHTML = '<p class="history-empty-msg">暂无搜索记录</p>';
      return;
    }
    var html = '';
    list.forEach(function (r) {
      html += '<div class="history-item">';
      html += '  <a href="javascript:void(0)" onclick="SearchHistoryManager.research(\'' + r.keyword.replace(/'/g, "\\'") + '\')">' + escapeHtml(r.keyword) + '</a>';
      html += '  <span class="engine-tag">' + r.engineId + '</span>';
      html += '  <button class="btn-del-hist" onclick="SearchHistoryManager.deleteOne(\'' + r.keyword.replace(/'/g, "\\'") + '\')" title="删除">&times;</button>';
      html += '</div>';
    });
    container.innerHTML = html;
  }

  function escapeHtml(s) {
    var d = document.createElement('div');
    d.textContent = s;
    return d.innerHTML;
  }

  function research(keyword) {
    var input = document.getElementById('searchQuery');
    if (input) input.value = keyword;
    if (window.redirectToSearch) window.redirectToSearch();
  }

  return {
    addRecord: addRecord,
    getAll: getAll,
    clearAll: clearAll,
    deleteOne: deleteOne,
    research: research,
    renderHistoryUI: renderHistoryUI
  };
})();


// ============================================================
//  /go 和 /Email 指令系统
// ============================================================
var CmdMapping = {
  '抖音': ['douyin', 'dy'],
  '哔哩哔哩': ['bilibili', 'b站', 'blbl'],
  '百度': ['baidu', 'bd'],
  '知乎': ['zhihu', 'zh'],
  'GitHub': ['github', 'git'],
  '微博': ['weibo', 'wb'],
  '小红书': ['xiaohongshu', 'xhs'],
  '淘宝': ['taobao', 'tb'],
  '京东': ['jd'],
  '美团': ['meituan', 'mt'],
  '饿了么': ['eleme', 'elm'],
  '腾讯': ['tencent', 'tx'],
  '阿里': ['ali'],
  '华为': ['huawei', 'hw'],
  '小米': ['xiaomi', 'xm'],
  '苹果': ['apple', 'pg'],
  '微软': ['microsoft'],
  '谷歌': ['google', 'gg'],
  '必应': ['bing', 'biying'],
  'CSDN': ['csdn'],
  '掘金': ['juejin'],
  'QQ': ['qq'],
  '微信': ['weixin', 'wx'],
  '飞书': ['feishu'],
  '钉钉': ['dingtalk'],
  '快手': ['kuaishou'],
  '拼多多': ['pinduoduo'],
  '唯品会': ['vip'],
  '苏宁': ['suning'],
  '得物': ['dewu'],
  'B站': ['bilibili', 'b站'],
  '百度文库': ['baiduwenku'],
  '百度贴吧': ['baidutieba'],
  '百度网盘': ['baidupan'],
  '百度地图': ['baidumap'],
  '百度翻译': ['baidufanyi'],
  'QQ邮箱': ['qqmail'],
  'QQ音乐': ['qqmusic'],
  '微信公众平台': ['weixinmp'],
  '微信开放平台': ['weixinopen'],
  '微博热搜': ['weibohot'],
  '今日头条': ['toutiao'],
  '西瓜视频': ['ixigua'],
  '飞书文档': ['feishudoc'],
  '钉钉文档': ['dingtalkdoc'],
  '皮皮虾': ['pipix'],
  'vivo': ['vivo'],
  'OPPO': ['oppo'],
  '一加': ['oneplus'],
  'realme': ['realme'],
  'iQOO': ['iqoo'],
  'Office': ['office'],
  'Azure': ['azure'],
  'Gitee': ['gitee'],
  '思否': ['segmentfault'],
  '开源中国': ['oschina'],
  '菜鸟教程': ['runoob'],
  'W3school': ['w3school'],
  'MDN': ['mdn'],
  'LeetCode': ['leetcode'],
  '牛客网': ['nowcoder'],
  '拉勾网': ['lagou'],
  '智联招聘': ['zhaopin'],
  '前程无忧': ['51job'],
  'BOSS直聘': ['zhipin'],
  '猎聘': ['liepin'],
  '脉脉': ['maimai'],
  '实习僧': ['shixiseng'],
  '站酷': ['zcool'],
  '花瓣网': ['huaban'],
  '千库网': ['588ku'],
  '包图网': ['ibaotu'],
  '摄图网': ['699pic'],
  '图虫创意': ['tuchong'],
  '视觉中国': ['vcg'],
  'Pinterest': ['pinterest'],
  'Canva': ['canva'],
  '稿定设计': ['gaoding'],
  '创客贴': ['chuangkit'],
  'Axure': ['axure'],
  'Figma': ['figma'],
  'Dribbble': ['dribbble'],
  'Behance': ['behance'],
  '央视网': ['cctv'],
  '人民网': ['people'],
  '新华网': ['xinhua'],
  '光明网': ['gmw'],
  '央广网': ['cnr'],
  '中国新闻网': ['chinanews'],
  '凤凰网': ['ifeng'],
  '新浪': ['sina'],
  '搜狐': ['sohu'],
  '网易新闻': ['163news'],
  '澎湃新闻': ['thepaper'],
  '界面新闻': ['jiemian'],
  '36氪': ['36kr'],
  '虎嗅网': ['huxiu'],
  '钛媒体': ['tmtpost'],
  '爱范儿': ['ifanr'],
  '机器之心': ['jiqizhixin'],
  '观察者网': ['guancha'],
  '财新网': ['caixin'],
  '第一财经': ['yicai'],
  '东方财富网': ['eastmoney'],
  '雪球': ['xueqiu'],
  '同花顺': ['10jqka'],
  '上海证券交易所': ['sse'],
  '深圳证券交易所': ['szse'],
  '中国证券网': ['cnstock'],
  '央行': ['pbc'],
  '银保监会': ['nfra'],
  '证监会': ['csrc'],
  '国家统计局': ['stats'],
  '中国政府网': ['gov'],
  '学习强国': ['xuexi'],
  '中国知网': ['cnki'],
  '万方数据': ['wanfang'],
  '维普网': ['cqvip'],
  '超星学习通': ['chaoxing'],
  '学堂在线': ['xuetangx'],
  '中国大学MOOC': ['icourse163'],
  '网易云课堂': ['mooc163'],
  '慕课网': ['imooc'],
  '极客时间': ['geekbang'],
  '得到': ['dedao'],
  '喜马拉雅': ['ximalaya'],
  '荔枝播客': ['lizhi'],
  '蜻蜓FM': ['qingting'],
  'AcFun': ['acfun'],
  '腾讯动漫': ['qqcomic'],
  '快看漫画': ['kuaikan'],
  '动漫之家': ['dmzj'],
  '起点中文网': ['qidian'],
  '晋江文学城': ['jjwxc'],
  '番茄小说': ['fanqie'],
  '七猫免费小说': ['qimao'],
  '纵横中文网': ['zongheng'],
  '17K小说网': ['17k'],
  '飞卢小说网': ['faloo'],
  '微信读书': ['weread'],
  '掌阅': ['ireader'],
  '网易云音乐': ['music163'],
  '酷狗音乐': ['kugou'],
  '酷我音乐': ['kuwo'],
  '咪咕音乐': ['migumusic'],
  '网易游戏': ['game163'],
  '腾讯游戏': ['qqgame'],
  'Steam中国': ['steamchina'],
  'Steam': ['steam'],
  'Epic': ['epic'],
  'TapTap': ['taptap'],
  '游民星空': ['gamersky'],
  '3DM': ['3dmgame'],
  '游侠网': ['ali213'],
  '英雄联盟': ['lol'],
  '王者荣耀': ['wzry'],
  '和平精英': ['hpjy'],
  '原神': ['genshin'],
  '米游社': ['miyoushe'],
  '网易我的世界': ['mc163'],
  '完美世界': ['wanmei'],
  '西山居': ['xishanju'],
  '莉莉丝': ['lilith'],
  '米哈游': ['mihoyo'],
  '58同城': ['58'],
  '赶集网': ['ganji'],
  '安居客': ['anjuke'],
  '贝壳找房': ['ke'],
  '链家': ['lianjia'],
  '汽车之家': ['autohome'],
  '易车网': ['yiche'],
  '马蜂窝': ['mafengwo'],
  '携程': ['ctrip'],
  '去哪儿': ['qunar'],
  '飞猪': ['fliggy'],
  '同程': ['lv'],
  '途牛': ['tuniu'],
  '大众点评': ['dianping'],
  '美图秀秀': ['xiuxiu'],
  '醒图': ['xingtu'],
  '剪映': ['capcut'],
  '必剪': ['bcut'],
  '火绒': ['huorong'],
  '迅雷': ['xunlei'],
  '蓝奏云': ['lanzou'],
  '阿里云盘': ['aliyundrive'],
  '夸克网盘': ['quarkpan'],
  '文心一言': ['yiyan'],
  '通义千问': ['tongyi'],
  '豆包': ['doubao'],
  '讯飞星火': ['xinghuo'],
  '智谱清言': ['zhipu'],
  'Kimi': ['kimi'],
  'ChatGPT': ['chatgpt'],
  'Midjourney': ['midjourney'],
  'Stable Diffusion': ['stablediffusion'],
  '讯飞听见': ['ting'],
  '讯飞输入法': ['ime'],
  '石墨文档': ['shimo'],
  '金山文档': ['kdocs'],
  'WPS': ['wps'],
  '印象笔记': ['yinxiang'],
  '有道云笔记': ['youdao'],
  '幕布': ['mubu'],
  'XMind': ['xmind'],
  'ProcessOn': ['processon'],
  '语雀': ['yuque'],
  'Notion': ['notion'],
  '135编辑器': ['135editor'],
  '秀米': ['xiumi'],
  '壹伴': ['yiban'],
  '新榜': ['newrank'],
  '清博大数据': ['gsdata'],
  '微信指数': ['wechatindex'],
  '百度指数': ['baiduindex'],
  '巨量算数': ['trendinsight'],
  '蝉妈妈': ['chanmama'],
  '飞瓜数据': ['feigua'],
  '抖查查': ['douchacha'],
  '站长工具': ['chinaz'],
  '爱站网': ['aizhan'],
  '5118': ['5118'],
  'ICP备案查询': ['beian'],
  'WHOIS查询': ['whois'],
  'IP查询': ['ip138'],
  'JSON格式化': ['jsoncn'],
  '代码格式化': ['codeformat'],
  '正则测试': ['regex101'],
  '二维码生成': ['cliim'],
  '短链接': ['dwz'],
  'SM.MS图床': ['smms'],
  'Imgur': ['imgur'],
  '阿里云': ['aliyun'],
  '腾讯云': ['tencentcloud'],
  '华为云': ['huaweicloud'],
  '百度智能云': ['baiducloud'],
  '金山云': ['ksyun'],
  'UCloud': ['ucloud'],
  '青云': ['qingcloud'],
  '七牛云': ['qiniu'],
  '又拍云': ['upyun'],
  '网易云信': ['yunxin'],
  '京东云': ['jdcloud'],
  'GitLab': ['gitlab'],
  'SVN': ['svn'],
  'Jenkins': ['jenkins'],
  'Docker': ['docker'],
  'Kubernetes': ['kubernetes'],
  'Vue': ['vue'],
  'React': ['react'],
  'Angular': ['angular'],
  'Element Plus': ['elementplus'],
  'Ant Design': ['antdesign'],
  'uni-app': ['uniapp'],
  'DCloud': ['dcloud'],
  'Electron': ['electron'],
  'Vite': ['vite'],
  'Webpack': ['webpack'],
  'Node.js': ['nodejs'],
  'Python': ['python'],
  'Java': ['java'],
  'Go语言': ['golang'],
  'Rust': ['rust'],
  'PHP': ['php'],
  'MySQL': ['mysql'],
  'Redis': ['redis'],
  'MongoDB': ['mongodb'],
  'PostgreSQL': ['postgresql'],
  'Elasticsearch': ['elasticsearch'],
  'Nginx': ['nginx'],
  'Apache': ['apache'],
  'Tomcat': ['tomcat'],
  '宝塔面板': ['bt'],
  'OneinStack': ['oneinstack'],
  '极客学院': ['jikexueyuan'],
  '传智播客': ['itheima'],
  '尚硅谷': ['atguigu'],
  '达内': ['tedu'],
  '开课吧': ['kaikeba'],
  '51CTO': ['51cto'],
  'Stack Overflow': ['stackoverflow'],
  'Stack Overflow中文': ['stackoverflowcn'],
  'V2EX': ['v2ex'],
  'Hostloc': ['hostloc'],
  'Chiphell': ['chiphell'],
  '中关村在线': ['zol'],
  '太平洋电脑网': ['pconline'],
  'IT之家': ['ithome'],
  '什么值得买': ['smzdm'],
  '慢慢买': ['manmanbuy'],
  '返利网': ['fanli'],
  '蔚来': ['nio'],
  '理想': ['lixiang'],
  '小鹏': ['xiaopeng'],
  '特斯拉': ['tesla'],
  '比亚迪': ['byd'],
  '宝马': ['bmw'],
  '奔驰': ['benz'],
  '奥迪': ['audi'],
  '雷克萨斯': ['lexus'],
  '凯迪拉克': ['cadillac'],
  '沃尔沃': ['volvo'],
  '林肯': ['lincoln'],
  '保时捷': ['porsche'],
  '路虎': ['landrover'],
  '捷豹': ['jaguar'],
  '玛莎拉蒂': ['maserati'],
  '法拉利': ['ferrari'],
  '兰博基尼': ['lamborghini'],
  '宾利': ['bentley'],
  '劳斯莱斯': ['rollsroyce'],
  '顺丰': ['sf'],
  '中通': ['zto'],
  '圆通': ['yto'],
  '申通': ['sto'],
  '韵达': ['yunda'],
  '极兔': ['jtexpress'],
  'EMS': ['ems'],
  '快递100': ['kuaidi100'],
  '12306': ['12306'],
  '航旅纵横': ['umetrip'],
  '飞常准': ['variflight'],
  '国航': ['airchina'],
  '南航': ['csair'],
  '东航': ['ceair'],
  '海航': ['hnair'],
  '哈啰': ['hellobike'],
  '平安银行': ['pingan'],
  '招商银行': ['cmbchina'],
  '工商银行': ['icbc'],
  '建设银行': ['ccb'],
  '农业银行': ['abchina'],
  '中国银行': ['boc'],
  '交通银行': ['bankcomm'],
  '兴业银行': ['cib'],
  '浦发银行': ['spdb'],
  '中信银行': ['citic'],
  '光大银行': ['cebbank'],
  '民生银行': ['cmbc'],
  '华夏银行': ['hxbank'],
  '浙商银行': ['czbank'],
  '平安保险': ['pinganinsurance'],
  '中国人寿': ['chinalife'],
  '太平洋保险': ['cpic'],
  '丁香医生': ['dxy'],
  '春雨医生': ['chunyuyisheng'],
  '好大夫': ['haodf'],
  '微医': ['guahao'],
  '京东健康': ['jdhealth'],
  '阿里健康': ['alihealth'],
  '39健康网': ['39'],
  '宝宝树': ['babytree'],
  '妈妈网': ['ma'],
  '乐高': ['lego'],
  '迪士尼': ['disney'],
  '任天堂': ['nintendo'],
  '索尼': ['sony'],
  'Xbox': ['xbox'],
  'PlayStation': ['playstation'],
  '完美日记': ['perfectdiary'],
  '花西子': ['huaxizi'],
  '毛戈平': ['maogeping'],
  '珀莱雅': ['proya'],
  '自然堂': ['chcedo'],
  '百雀羚': ['pechoin'],
  '丝芙兰': ['sephora'],
  '屈臣氏': ['watsons'],
  '欧莱雅': ['loreal'],
  '雅诗兰黛': ['esteelauder'],
  '兰蔻': ['lancome'],
  '香奈儿': ['chanel'],
  '迪奥': ['dior'],
  '古驰': ['gucci'],
  'LV': ['lv'],
  '普拉达': ['prada'],
  '爱马仕': ['hermes'],
  '优衣库': ['uniqlo'],
  'ZARA': ['zara'],
  'H&M': ['hm'],
  'GAP': ['gap'],
  '无印良品': ['muji'],
  '名创优品': ['miniso'],
  '泡泡玛特': ['popmart'],
  '三丽鸥': ['sanrio'],
  'LINE FRIENDS': ['linefriends'],
  '洽洽': ['qiaqia'],
  '三只松鼠': ['3songshu'],
  '良品铺子': ['517lppz'],
  '百草味': ['baicaowei'],
  '卫龙': ['weilong'],
  '康师傅': ['masterkong'],
  '伊利': ['yili'],
  '蒙牛': ['mengniu'],
  '光明乳业': ['brightdairy'],
  '三元': ['sanyuan'],
  '农夫山泉': ['nongfushanquan'],
  '娃哈哈': ['wahaha'],
  '可口可乐': ['coca'],
  '百事可乐': ['pepsi'],
  '星巴克': ['starbucks'],
  '瑞幸': ['luckin'],
  '喜茶': ['heytea'],
  '奈雪的茶': ['naixue'],
  '蜜雪冰城': ['mixue'],
  '书亦烧仙草': ['shuyi'],
  '古茗': ['guming'],
  '茶百道': ['chabaidao'],
  '沪上阿姨': ['hushangayi'],
  '肯德基': ['kfc'],
  '麦当劳': ['mcdonalds'],
  '汉堡王': ['bk'],
  '必胜客': ['pizzahut'],
  '海底捞': ['haidilao'],
  '呷哺呷哺': ['xiabuxian'],
  '巴奴': ['banu'],
  '西贝': ['xibei'],
  '太二': ['tai2'],
  '老乡鸡': ['laxiangji'],
  '永和大王': ['yonghe'],
  '真功夫': ['zkungfu'],
  '德克士': ['dicos'],
  '下厨房': ['xiachufang'],
  '美食杰': ['meishij'],
  '豆果美食': ['douguo'],
  '美食天下': ['meishichina'],
  '土巴兔': ['to8to'],
  '齐家网': ['jia'],
  '住小帮': ['zhuxiaobang'],
  '一兜糖': ['yidoutang'],
  '好好住': ['haohaozhu'],
  '设计本': ['shejiben'],
  '尚品宅配': ['sphome'],
  '索菲亚': ['suofeiya'],
  '欧派': ['oppein'],
  '顾家': ['kukahome'],
  '慕思': ['mousse'],
  '红星美凯龙': ['redstar'],
  '居然之家': ['juran'],
  '宜家': ['ikea'],
  '曲美': ['qumei'],
  '全友': ['quanyou'],
  '林氏家居': ['linsy'],
  '网易严选': ['you163'],
  '小米有品': ['youpin'],
  '京东京造': ['jingzao'],
  '得力': ['deli'],
  '晨光': ['mgpen'],
  '齐心': ['comix'],
  '罗技': ['logitech'],
  '雷蛇': ['razer'],
  '赛睿': ['steelseries'],
  '海盗船': ['corsair'],
  '机械师': ['machenike'],
  '联想': ['lenovo'],
  '戴尔': ['dell'],
  '惠普': ['hp'],
  '华硕': ['asus'],
  '微星': ['msi'],
  '宏碁': ['acer'],
  '机械革命': ['mechrevo'],
  '七彩虹': ['colorful'],
  '影驰': ['galaxy'],
  'ROG': ['rog'],
  '明基': ['benq'],
  'AOC': ['aoc'],
  'HKC': ['hkc'],
  '海信': ['hisense'],
  'TCL': ['tcl'],
  '创维': ['skyworth'],
  '美的': ['midea'],
  '格力': ['gree'],
  '海尔': ['haier'],
  '方太': ['fotile'],
  '老板': ['robam'],
  '苏泊尔': ['supor'],
  '九阳': ['joyoung'],
  '小熊电器': ['bear'],
  '北鼎': ['buydeem'],
  '科沃斯': ['ecovacs'],
  '石头科技': ['roborock'],
  '追觅': ['dreame'],
  '添可': ['tineco'],
  '广汽埃安': ['gaia'],
  '哪吒': ['nezha'],
  '零跑': ['leapmotor'],
  '极氪': ['zeekr'],
  '问界': ['aito'],
  '智己': ['immotor'],
  '飞凡': ['risingauto'],
  '上汽大通': ['saicmaxus'],
  '吉利': ['geely'],
  '长城': ['gwm'],
  '长安': ['changan'],
  '广汽传祺': ['gac'],
  '东风': ['dfmc'],
  '一汽大众': ['fawvw']
};

function fuzzySearch(keyword, target) {
  var kw = keyword.toLowerCase().trim();
  var tg = target.toLowerCase().trim();
  if (tg.indexOf(kw) !== -1) return true;
  var aliases = CmdMapping[target];
  if (aliases) {
    for (var i = 0; i < aliases.length; i++) {
      if (aliases[i].toLowerCase().indexOf(kw) !== -1) return true;
    }
  }
  for (var name in CmdMapping) {
    var maps = CmdMapping[name];
    if (name === target) continue;
    if (maps && maps.indexOf(target) !== -1) {
      if (name.toLowerCase().indexOf(kw) !== -1) return true;
      for (var j = 0; j < maps.length; j++) {
        if (maps[j].toLowerCase().indexOf(kw) !== -1) return true;
      }
    }
  }
  return false;
}

function matchScore(text, kw) {
  var t = String(text == null ? '' : text).toLowerCase();
  var k = String(kw == null ? '' : kw).toLowerCase();
  if (!k) return 0;
  var idx = t.indexOf(k);
  if (idx === 0) return 100;
  if (idx !== -1) return 80;
  var score = 0, ti = 0, lastIdx = -1, gapSum = 0;
  for (var i = 0; i < k.length; i++) {
    var ch = k.charAt(i);
    var found = t.indexOf(ch, ti);
    if (found === -1) return 0;
    if (lastIdx >= 0) gapSum += (found - lastIdx - 1);
    lastIdx = found;
    ti = found + 1;
    score += 10;
  }
  score -= gapSum;
  return score;
}

(async function initCommandSystem() {
  var searchInput = document.getElementById('searchQuery');
  var autoSlashBtn = document.getElementById('btn');
  var panelCard = document.querySelector('.search-card-untop');
  var cmdSuggest = document.getElementById('cmdSuggest');
  var suggestList = document.getElementById('suggestList');
  var commands = {};
  var keywordList = [];
  var commonTargets = ['抖音', '哔哩哔哩', '百度', '知乎', 'GitHub', '微博', '小红书', '淘宝', '京东', '美团'];
  var suggestItems = [];
  var selectedIndex = -1;

  SearchEngineManager.init();
  SearchHistoryManager.renderHistoryUI();

  function hideSuggest() {
    if (cmdSuggest) {
      cmdSuggest.style.display = 'none';
      suggestItems = [];
      selectedIndex = -1;
    }
  }

  function showSuggest() {
    if (!cmdSuggest || !searchInput) return;
    
    var rect = searchInput.getBoundingClientRect();
    var parentRect = document.getElementById('mainSearchBox').getBoundingClientRect();
    
    cmdSuggest.style.left = parentRect.left + 'px';
    cmdSuggest.style.top = rect.bottom + 'px';
    cmdSuggest.style.width = parentRect.width + 'px';
    cmdSuggest.style.display = 'block';
  }

  // 滚动跟随：多事件源确保下拉框位置同步
  var cmdRafPending = false;
  function onCmdScroll() {
    if (!cmdSuggest || cmdSuggest.style.display === 'none') return;
    if (cmdRafPending) return;
    cmdRafPending = true;
    requestAnimationFrame(function () {
      cmdRafPending = false;
      if (!cmdSuggest || cmdSuggest.style.display === 'none' || !searchInput) return;
      var rect = searchInput.getBoundingClientRect();
      if (rect.bottom < 0 || rect.top > window.innerHeight) {
        hideSuggest();
        return;
      }
      var parentRect = document.getElementById('mainSearchBox').getBoundingClientRect();
      var topPos = rect.bottom;
      var maxTop = window.innerHeight - cmdSuggest.offsetHeight - 8;
      if (topPos > maxTop) {
        topPos = rect.top - cmdSuggest.offsetHeight;
        if (topPos < 8) topPos = 8;
      }
      cmdSuggest.style.left = parentRect.left + 'px';
      cmdSuggest.style.top = topPos + 'px';
      cmdSuggest.style.width = parentRect.width + 'px';
    });
  }
  window.addEventListener('scroll', onCmdScroll, { passive: true });
  window.addEventListener('wheel', onCmdScroll, { passive: true });
  window.addEventListener('touchmove', onCmdScroll, { passive: true });
  document.addEventListener('scroll', onCmdScroll, { passive: true });
  window.addEventListener('resize', onCmdScroll);

  function renderSuggest(items, highlightKey) {
    if (!suggestList) return;
    suggestItems = items;
    selectedIndex = -1;
    
    if (items.length === 0) {
      suggestList.innerHTML = '<div class="suggest-item" style="padding:8px 12px;font-size:13px;color:var(--gs-hint);">无匹配结果</div>';
      return;
    }
    
    var html = '';
    items.slice(0, 15).forEach(function(item, idx) {
      var displayText = item;
      if (highlightKey) {
        var regex = new RegExp('(' + highlightKey + ')', 'gi');
        displayText = item.replace(regex, '<mark style="background:var(--gs-accent-bg);color:var(--gs-accent);border-radius:2px;padding:0 2px;">$1</mark>');
      }
      html += '<div class="suggest-item" data-index="' + idx + '" data-value="' + item.replace(/"/g, '&quot;') + '" style="padding:8px 12px;font-size:13px;color:var(--gs-text);cursor:pointer;border-left:3px solid transparent;transition:background 0.15s;">';
      html += '  <span style="margin-right:8px;color:var(--gs-accent);font-size:11px;">↗</span>' + displayText;
      html += '</div>';
    });
    
    suggestList.innerHTML = html;
    
    var itemsEl = suggestList.querySelectorAll('.suggest-item');
    itemsEl.forEach(function(el) {
      el.addEventListener('click', function() {
        var val = this.getAttribute('data-value');
        searchInput.value = '/go ' + val;
        hideSuggest();
        searchInput.focus();
      });
      el.addEventListener('mouseenter', function() {
        selectedIndex = parseInt(this.getAttribute('data-index'));
        updateSelection();
      });
    });
  }

  function updateSelection() {
    var itemsEl = suggestList.querySelectorAll('.suggest-item');
    itemsEl.forEach(function(el, idx) {
      if (idx === selectedIndex) {
        el.style.background = 'var(--gs-accent-bg)';
        el.style.borderLeftColor = 'var(--gs-accent)';
        el.scrollIntoView({ block: 'nearest' });
      } else {
        el.style.background = 'transparent';
        el.style.borderLeftColor = 'transparent';
      }
    });
  }

  function selectNext() {
    if (suggestItems.length === 0) return;
    selectedIndex = (selectedIndex + 1) % suggestItems.length;
    updateSelection();
  }

  function selectPrev() {
    if (suggestItems.length === 0) return;
    selectedIndex = selectedIndex <= 0 ? suggestItems.length - 1 : selectedIndex - 1;
    updateSelection();
  }

  function acceptSelection() {
    if (selectedIndex >= 0 && selectedIndex < suggestItems.length) {
      var currentVal = searchInput.value;
      var prefix = currentVal.toLowerCase().startsWith('/s') ? '/s ' : '/go ';
      searchInput.value = prefix + suggestItems[selectedIndex];
      hideSuggest();
      searchInput.focus();
    }
  }

  function updatePanelVisibility() {
    var val = searchInput.value;
    var lowerVal = val.toLowerCase().trim();
    
    if (lowerVal.startsWith('/go') || lowerVal.startsWith('/s')) {
      if (panelCard) {
        panelCard.classList.remove('panel-visible');
        panelCard.style.display = 'none';
      }
      
      var prefix = lowerVal.startsWith('/s') ? '/s' : '/go';
      var key = lowerVal.replace(new RegExp('^' + prefix + '\\s*', 'i'), '').trim();
      var matched;
      
      if (prefix === '/s') {
        var allEngines = SearchEngineManager.getEngines();
        var engineKeys = Object.keys(allEngines);
        if (key === '') {
          matched = engineKeys.slice(0, 30);
        } else {
          matched = [];
          engineKeys.forEach(function(k) {
            var s1 = matchScore(k, key);
            var s2 = matchScore(allEngines[k].name, key);
            if (Math.max(s1, s2) > 0) matched.push(k);
          });
        }
        var displayItems = matched.map(function(k) { return allEngines[k].name || k; });
        if (displayItems.length > 0 || key !== '') {
          renderSuggest(displayItems, key);
          showSuggest();
        } else {
          hideSuggest();
        }
      } else {
        if (key === '') {
          matched = commonTargets.filter(function(k) { return commands[k]; });
        } else {
          matched = keywordList.filter(function(k) { return fuzzySearch(key, k); });
        }
        if (matched.length > 0 || key !== '') {
          renderSuggest(matched, key);
          showSuggest();
        } else {
          hideSuggest();
        }
      }
      return;
    }
    
    hideSuggest();
    
    if (!panelCard) return;
    if (val.length > 0 && val[0] === '/') {
      if (!panelCard.classList.contains('panel-visible')) {
        panelCard.classList.add('panel-visible');
        switchSubTab('sub-tab1', document.querySelector('.panel-tabs-wrapper .tab-browser'));
      }
      panelCard.style.display = 'block';
      filterCmdTargets(val);
    } else {
      panelCard.classList.remove('panel-visible');
      panelCard.style.display = 'none';
    }
  }

  function filterCmdTargets(inputVal) {
    var container = document.getElementById('cmdTargetList');
    if (!container) return;
    var lowerVal = inputVal.toLowerCase().trim();

    if (lowerVal === '/') {
      container.innerHTML = '<p style="color:#999;font-size:12px;padding:6px 0;">输入 <b>/go</b> 后继续键入以筛选跳转目标</p>';
      return;
    }

    if (lowerVal === '/email' || lowerVal.startsWith('/email ')) {
      var emailHint = inputVal.replace(/^\/email\s*/i, '').trim();
      if (!emailHint) {
        container.innerHTML = '<p style="color:#999;font-size:12px;padding:6px 0;">输入收件人邮箱后回车打开邮箱客户端，如 <b>/email someone@example.com</b></p>';
      } else {
        container.innerHTML = '<p style="color:#999;font-size:12px;padding:6px 0;">回车将通过系统邮箱发送给 <b>' + emailHint + '</b></p>';
      }
      return;
    }

    if (lowerVal === '/s' || lowerVal.startsWith('/s ')) {
      var sKey = lowerVal.replace(/^\/s\s*/i, '').trim();
      var allEngines = SearchEngineManager.getEngines();
      var engineKeys = Object.keys(allEngines);
      var matchedKeys;
      if (sKey === '') {
        matchedKeys = engineKeys;
      } else {
        matchedKeys = engineKeys.filter(function (k) {
          var s1 = matchScore(k, sKey);
          var s2 = matchScore(allEngines[k].name, sKey);
          return Math.max(s1, s2) > 0;
        });
      }
      if (matchedKeys.length === 0) {
        container.innerHTML = '<p style="color:var(--gs-hint);font-size:12px;padding:6px 0;">无匹配引擎</p>';
        return;
      }
      var sHtml = '<p style="font-size:11px;color:var(--gs-hint);opacity:0.7;margin:0 0 8px;">点击填入搜索框，回车切换引擎</p>';
      sHtml += '<div style="display:flex;flex-wrap:wrap;gap:8px;">';
      matchedKeys.slice(0, 30).forEach(function (k) {
        var displayName = allEngines[k].name;
        var displayKey = k;
        if (sKey) {
          try {
            var regex = new RegExp('(' + sKey.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + ')', 'gi');
            displayName = displayName.replace(regex, '<mark style="background:var(--gs-accent-bg);color:var(--gs-accent);border-radius:2px;padding:0 2px;">$1</mark>');
          } catch (e) {}
        }
        sHtml += '<span class="cmd-chip" onclick="insertCmd(\'/s ' + k + '\')"><b>' + displayName + '</b></span>';
      });
      sHtml += '</div>';
      container.innerHTML = sHtml;
      return;
    }

    var key = lowerVal.replace(/^\/go\s*/i, '').trim();

    if (key === '') {
      container.innerHTML = '<p style="font-size:11px;color:var(--text);opacity:0.4;margin:0 0 8px;">输入关键词筛选跳转目标，如 /go 抖音</p>';
      container.innerHTML += '<div style="display:flex;flex-wrap:wrap;gap:8px;">';
      commonTargets.forEach(function(k) {
        if (commands[k]) {
          container.innerHTML += '<span class="cmd-chip" onclick="insertCmd(\'/go ' + k + '\')">' + k + '</span>';
        }
      });
      container.innerHTML += '</div>';
      return;
    }

    var matched = keywordList.filter(function(k) {
      return fuzzySearch(key, k);
    });
    renderTargetChips(matched, key);
  }

  function renderTargetChips(list, highlightKey) {
    var container = document.getElementById('cmdTargetList');
    if (!container) return;
    
    if (list.length === 0) {
      container.innerHTML = '<p class="history-empty-msg">无匹配结果</p>';
      return;
    }
    
    var html = '<div style="display:flex;flex-wrap:wrap;gap:8px;">';
    list.slice(0, 30).forEach(function(k) {
      var displayText = k;
      if (highlightKey) {
        var regex = new RegExp('(' + highlightKey + ')', 'gi');
        displayText = k.replace(regex, '<mark style="background:var(--gs-accent-bg);color:var(--gs-accent);border-radius:2px;padding:0 2px;">$1</mark>');
      }
      html += '<span class="cmd-chip" onclick="insertCmd(\'/go ' + k.replace(/'/g, "\\'") + '\')">' + displayText + '</span>';
    });
    html += '</div>';
    if (list.length > 30) {
      html += '<p style="font-size:11px;color:#999;margin-top:8px;">仅显示前30条，继续输入以缩小范围</p>';
    }
    
    container.innerHTML = html;
  }

  if (autoSlashBtn) {
    autoSlashBtn.addEventListener('click', function () {
      searchInput.value = '/';
      searchInput.focus();
      updatePanelVisibility();
    });
  }

  searchInput.addEventListener('input', updatePanelVisibility);

  searchInput.addEventListener('keydown', function(e) {
    if (!cmdSuggest || cmdSuggest.style.display === 'none') {
      return;
    }
    
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectNext();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectPrev();
    } else if (e.key === 'Tab') {
      e.preventDefault();
      if (selectedIndex < 0 && suggestItems.length > 0) {
        selectedIndex = 0;
        updateSelection();
      } else {
        acceptSelection();
      }
    } else if (e.key === 'Enter') {
      if (selectedIndex >= 0) {
        e.preventDefault();
        acceptSelection();
      }
    } else if (e.key === 'Escape') {
      hideSuggest();
    }
  });

  document.addEventListener('click', function(e) {
    if (cmdSuggest && !cmdSuggest.contains(e.target) && e.target !== searchInput) {
      hideSuggest();
    }
  });

  window.redirectToSearch = function () {
    var val = (searchInput.value || '').trim();

    if (val.toLowerCase().startsWith('/go')) {
      var key = val.replace(/^\/go\s*/i, '').trim();
      if (key === '') { alert('请输入跳转目标，如 /go 抖音'); return; }
      if (commands[key]) { 
        window.open(commands[key], '_blank'); 
        searchInput.value = ''; 
        updatePanelVisibility();
        return; 
      }
      var fuzzyMatches = keywordList.filter(function(k) { return fuzzySearch(key, k); });
      if (fuzzyMatches.length > 0) {
        var target = fuzzyMatches[0];
        if (fuzzyMatches.length === 1 || confirm('找到多个匹配结果，是否跳转到第一个：' + target + '？')) {
          window.open(commands[target], '_blank');
          searchInput.value = '';
          updatePanelVisibility();
          return;
        }
      }
      alert('未找到指令：' + key);
      return;
    }

    if (val.toLowerCase() === '/email' || val.toLowerCase().startsWith('/email ')) {
      var emailInput = val.replace(/^\/email\s*/i, '').trim();
      if (emailInput) {
        var addrs = emailInput.split(',').map(function (s) { return s.trim(); }).filter(Boolean);
        var emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
        var invalid = addrs.filter(function (a) { return !emailRegex.test(a); });
        if (invalid.length > 0) {
          alert('邮箱地址格式不正确：' + invalid.join(', '));
          return;
        }
        emailInput = addrs.join(',');
      }
      window.location.href = 'mailto:' + emailInput;
      searchInput.value = '';
      updatePanelVisibility();
      return;
    }

    if (val.toLowerCase() === '/clear') {
      SearchHistoryManager.clearAll();
      searchInput.value = '';
      updatePanelVisibility();
      return;
    }

    if (val.toLowerCase().startsWith('/s')) {
      var sKey = val.replace(/^\/s\s*/i, '').trim();
      if (sKey === '') {
        var sEngines = SearchEngineManager.getEngines();
        var sKeys = Object.keys(sEngines);
        if (sKeys.length > 0) {
          var firstEngine = sKeys[0];
          var firstEngineName = sEngines[firstEngine].name;
          var sel = document.getElementById('searchEngine');
          if (sel) {
            sel.value = firstEngine;
            sel.dispatchEvent(new Event('change'));
          }
          SearchEngineManager.saveSelected(firstEngine);
          SearchEngineManager.updatePlaceholder();
          alert('已切换搜索引擎：' + firstEngineName);
          searchInput.value = '';
          updatePanelVisibility();
          return;
        }
        return;
      }
      var allEngines = SearchEngineManager.getEngines();
      var engineKeys = Object.keys(allEngines);
      var matchedEngine = null;
      var scoreThreshold = 0;
      engineKeys.forEach(function (k) {
        var sk = matchScore(k, sKey);
        var sn = matchScore(allEngines[k].name, sKey);
        var s = Math.max(sk, sn);
        if (s > scoreThreshold) {
          scoreThreshold = s;
          matchedEngine = k;
        }
      });
      if (matchedEngine) {
        var engineSel = document.getElementById('searchEngine');
        if (engineSel) {
          engineSel.value = matchedEngine;
          engineSel.dispatchEvent(new Event('change'));
        }
        SearchEngineManager.saveSelected(matchedEngine);
        SearchEngineManager.updatePlaceholder();
        alert('已切换搜索引擎：' + allEngines[matchedEngine].name);
        searchInput.value = '';
        updatePanelVisibility();
      } else {
        alert('未找到匹配的搜索引擎：' + sKey);
      }
      return;
    }

    if (val.trim() === '') { return; }

    var engineSelect = document.getElementById('searchEngine');
    var engineId = engineSelect ? engineSelect.value : 'bing';

    if (engineId === 'local') {
      SearchHistoryManager.addRecord(val, engineId);
      searchInput.value = '';
      updatePanelVisibility();
      if (typeof window.searchLocalEngine === 'function') {
        window.searchLocalEngine(val, 1);
      } else {
        alert('本地搜索引擎模块未加载，请刷新页面');
      }
      return;
    }

    SearchHistoryManager.addRecord(val, engineId);

    var url = SearchEngineManager.getUrl(val);
    window.open(url, '_blank');
    searchInput.value = '';
    updatePanelVisibility();
  };

  window.insertCmd = function (cmd) {
    searchInput.value = cmd;
    searchInput.focus();
    updatePanelVisibility();
  };

  // switchSubTab 使用 utils.js 中的全局实现（基于 active 类切换，避免 inline display 与 CSS 冲突）

  try {
    var resp = await fetch('js/search/instruction/commands.json?' + Date.now());
    if (resp.ok) {
      var data = await resp.json();
      if (typeof data === 'object') {
        for (var k in data) {
          commands[k] = data[k];
        }
        keywordList = Object.keys(commands);
      }
    }
  } catch (e) {
    console.error('指令加载失败:', e);
  }
})();