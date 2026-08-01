window.addEventListener('load', function () {
    const saveBg = localStorage.getItem('chooseBg') || 'bing';
    setBg(saveBg);

    const bgSwitch = document.getElementById('bgSwitch');
    if (bgSwitch) {
        bgSwitch.addEventListener('change', function () {
            const body = document.body;
            if (this.checked) {
                body.style.background = '';
                body.style.backgroundColor = '';
            } else {
                const saveBg = localStorage.getItem('chooseBg') || 'bing';
                setBg(saveBg);
            }
        });
    }
});

async function setBg(type) {
    const body = document.body;
    const infoDiv = document.getElementById('wallpaperInfo');

    localStorage.setItem('chooseBg', type);

    if (infoDiv) {
        infoDiv.innerHTML = '';
    }

    if (type === 'bing') {
        try {
            const response = await fetch('https://uapis.cn/api/v1/image/bing-daily-history?page_size=1');
            const data = await response.json();

            if (data && data.success && data.data && data.data.length > 0) {
                const wallpaper = data.data[0];
                let imageUrl = wallpaper.image_url || 'https://api.paugram.com/bing/';

                body.style.background = `url(${imageUrl}) center/cover fixed`;

                if (infoDiv) {
                    let infoHtml = '<div>必应每日壁纸</div>';
                    if (wallpaper.title) {
                        infoHtml += `<div>${wallpaper.title}</div>`;
                    }
                    if (wallpaper.subtitle) {
                        infoHtml += `<div style="opacity: 0.8;">${wallpaper.subtitle}</div>`;
                    }
                    if (wallpaper.copyright) {
                        infoHtml += `<div style="opacity: 0.6; font-size: 10px; margin-top: 4px;">© ${wallpaper.copyright}</div>`;
                    }
                    infoDiv.innerHTML = infoHtml;
                }
            } else {
                body.style.background = 'url(https://api.paugram.com/bing/) center/cover fixed';
                if (infoDiv) {
                    infoDiv.innerHTML = '必应每日壁纸';
                }
            }
        } catch (error) {
            console.error('获取必应壁纸失败，使用备用方案:', error);
            body.style.background = 'url(https://api.paugram.com/bing/) center/cover fixed';
            if (infoDiv) {
                infoDiv.innerHTML = '必应每日壁纸';
            }
        }
    } else if (type === 'sina') {
        body.style.background = 'url(https://api.paugram.com/wallpaper/?source=sina&category=us) center/cover fixed';
        if (infoDiv) {
            infoDiv.innerHTML = '随机动漫壁纸';
        }
    } else if (type === 'none') {
        body.style.background = 'var(--border) center/cover fixed';
        if (infoDiv) {
            infoDiv.innerHTML = '主题颜色背景';
        }
    } else if (type === 'hd') {
        body.style.background = 'url(https://wp.upx8.com/api.php) center/cover fixed';
        if (infoDiv) {
            infoDiv.innerHTML = '高清壁纸';
        }
    } else if (type.startsWith('loveanimer')) {
        const loveanimerCategoryNames = {
            '1': '美女',
            '2': '动漫',
            '3': '风景',
            '4': '游戏',
            '5': '明星',
            '6': '机械',
            '7': '动物',
            '8': '文字',
            '9': '城市',
            '10': '视觉',
            '11': '物语',
            '12': '情感',
            '13': '设计',
            '14': '男人'
        };

        let format = '';
        let categoryName = '';
        if (type.includes('-')) {
            format = type.split('-')[1];
            categoryName = loveanimerCategoryNames[format] || format;
        } else {
            categoryName = '随机';
        }

        try {
            let url = 'https://oiapi.net/api/Loveanimer?limit=1';
            if (format) {
                url += `&format=${format}`;
            }

            const response = await fetch(url);
            const data = await response.json();

            if (data.code === 1) {
                let imageUrl = '';

                if (data.data && data.data.length > 0) {
                    const wallpaper = data.data[0];
                    imageUrl = wallpaper.url;

                    if (infoDiv) {
                        let infoHtml = `<div>Loveanimer壁纸 - ${categoryName}</div>`;
                        if (wallpaper.width && wallpaper.height) {
                            infoHtml += `<div>尺寸: ${wallpaper.width}x${wallpaper.height}</div>`;
                        }
                        if (wallpaper.tag) {
                            infoHtml += `<div>标签: ${wallpaper.tag}</div>`;
                        }
                        infoDiv.innerHTML = infoHtml;
                    }
                } else if (data.message) {
                    const match = data.message.match(/img=([^\s]+)/);
                    if (match) {
                        imageUrl = match[1];
                    }

                    if (infoDiv) {
                        infoDiv.innerHTML = `<div>Loveanimer壁纸 - ${categoryName}</div>`;
                    }
                }

                if (imageUrl) {
                    imageUrl = imageUrl.trim().replace(/[`"\s]/g, '');
                    body.style.background = `url(${imageUrl}) center/cover fixed`;
                }
            }
        } catch (error) {
            console.error('获取Loveanimer壁纸失败:', error);
            if (infoDiv) {
                infoDiv.innerHTML = 'Loveanimer壁纸加载失败';
            }
        }
    } else if (type.startsWith('360-')) {
        const format = type.split('-')[1];
        const categoryNames = {
            '1': '4K专区',
            '2': '美女模特',
            '3': '爱情美图',
            '4': '风景',
            '5': '小清新',
            '6': '动漫',
            '7': '明星',
            '8': '萌宠',
            '9': '游戏',
            '10': '汽车',
            '11': '炫酷',
            '12': '军事',
            '13': '劲爆',
            '14': '纹理',
            '15': '文字',
            '16': '限时'
        };
        try {
            const response = await fetch(`https://oiapi.net/api/Wallpaper360?format=${format}`);
            const data = await response.json();
            if (data.code === 1 && data.message) {
                const imageUrl = data.message.replace(/[` ]/g, '');
                body.style.background = `url(${imageUrl}) center/cover fixed`;

                if (infoDiv && data.data && data.data.length > 0) {
                    const wallpaper = data.data[0];
                    let infoHtml = `<div>360壁纸 - ${categoryNames[format] || format}</div>`;
                    if (wallpaper.resolution) {
                        infoHtml += `<div>分辨率: ${wallpaper.resolution}</div>`;
                    }
                    if (wallpaper.tag) {
                        infoHtml += `<div>标签: ${wallpaper.tag}</div>`;
                    }
                    infoDiv.innerHTML = infoHtml;
                }
            }
        } catch (error) {
            console.error('获取360壁纸失败:', error);
            if (infoDiv) {
                infoDiv.innerHTML = '360壁纸加载失败';
            }
        }
    }
}

function selectImage() {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = 'image/*';
    input.onchange = function (e) {
        const file = e.target.files[0];
        if (file) {
            const reader = new FileReader();
            reader.onload = function (event) {
                const imgUrl = event.target.result;
                document.body.style.background = `url(${imgUrl}) center/cover fixed`;
                localStorage.setItem('chooseBg', 'custom');
                localStorage.setItem('customBg', imgUrl);
                const infoDiv = document.getElementById('wallpaperInfo');
                if (infoDiv) {
                    infoDiv.innerHTML = '<div>自定义壁纸</div>';
                }
            };
            reader.readAsDataURL(file);
        }
    };
    input.click();
}