function copyCode(elementId) {
      const codeBlock = document.getElementById(elementId);
      const code = codeBlock.querySelector('code').textContent;
      navigator.clipboard.writeText(code).then(function() {
        showToast('代码已复制！', 1500, 'success');
      });
    }

    function getRandomStr(len = 10){
      const pool = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ';
      let res = '';
      for(let i=0;i<len;i++){
        let r = Math.floor(Math.random() * pool.length);
        res += pool[r];
      }
      return res;
    }

    function getFullCode(){
      const stamp = Date.now();
      const rand10 = getRandomStr(10);
      return stamp + rand10;
    }

    function createCode(){
      const code = getFullCode();
      const box = document.getElementById('genResult');
      box.innerHTML += code + '\n';
    }

    function parseCode(){
      const code = document.getElementById('codeInput').value.trim();
      const infoBox = document.getElementById('parseInfo');
      if(!code){
        infoBox.innerText = '请输入需要解析的编码！';
        return;
      }
      let stampStr = '';
      let randStr = '';
      if(code.length > 10){
        randStr = code.slice(-10);
        stampStr = code.slice(0, code.length - 10);
      }else{
        infoBox.innerText = '编码长度不足，非标准格式！\n标准：毫秒时间戳+10位随机字符';
        return;
      }
      const timestamp = Number(stampStr);
      if(isNaN(timestamp)){
        infoBox.innerText = '前部无法识别为时间戳数字，编码格式错误';
        return;
      }
      const timeObj = new Date(timestamp);
      const localTime = timeObj.toLocaleString('zh-CN',{
        year:'numeric',month:'2-digit',day:'2-digit',
        hour:'2-digit',minute:'2-digit',second:'2-digit',
        hour12:false
      });
      let numCount = 0, lowerCount = 0, upperCount = 0;
      for(let c of randStr){
        if(/[0-9]/.test(c)) numCount++;
        else if(/[a-z]/.test(c)) lowerCount++;
        else if(/[A-Z]/.test(c)) upperCount++;
      }
      const text =
`【完整原始编号】：${code}

1. 时间戳分段信息
· 毫秒时间戳数值：${timestamp}
· 对应生成时间：${localTime}
· 含义：编码创建精确毫秒时间，用于排序、溯源

2. 随机校验串（固定10位）
· 随机字符段：${randStr}
· 数字数量：${numCount} 个
· 小写字母：${lowerCount} 个
· 大写字母：${upperCount} 个
· 字符池：0-9、a-z、A-Z 62种随机组合
· 作用：避免同毫秒编码重复，简易校验标识

3. 整体规则说明
格式 = 毫秒时间戳(不定长数字) + 10位混合随机字符
时序由时间戳保证，高随机串消除重复，适配订单/批次/设备唯一编码`;
      infoBox.innerText = text;
    }

    document.addEventListener('DOMContentLoaded', function() {
      initCarousel('demo-carousel');
      initScrollspy('.navbar-nav', 'div[id]');
    });