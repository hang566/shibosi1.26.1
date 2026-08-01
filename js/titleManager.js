        (function() {
            const STORAGE_KEY = 'visitCount';
            let count = parseInt(localStorage.getItem(STORAGE_KEY), 10) || 0;
            
            // 首次访问显示 a，再次访问显示 b
            document.title = count === 0 ? '欢迎！这里是市舶司' : '欢迎回来👏，这里是市舶司';
            
            // 访问次数 +1
            localStorage.setItem(STORAGE_KEY, String(count + 1));
        })();