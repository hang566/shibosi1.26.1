const CACHE_NAME = 'shibosi-v1.26.1-sync';
const RUNTIME_CACHE_NAME = 'shibosi-runtime-v1';

const CORE_ASSETS = [
    './',
    './index.html',
    './manifest.json',
    './favicon.ico',
    './favicon.png',
    './favicon-1.ico',

    './css/style.css',
    './css/search.css',
    './css/weather.css',
    './css/theme.css',
    './css/apps.css',
    './css/QuickSettings.css',

    './ui/css/modules/base.css',
    './ui/css/modules/display.css',
    './ui/css/modules/feedback.css',
    './ui/css/modules/forms.css',
    './ui/css/modules/layout.css',
    './ui/css/modules/navigation.css',
    './ui/css/modules/tabs-browser.css',
    './ui/css/modules/variables.css',
    './ui/css/input.css',
    './ui/css/table.css',
    './ui/css/Global.css',
    './ui/css/Typesetting.css',
    './ui/css/color.css',

    './js/titleManager.js',
    './js/background.js',
    './js/search/search.js',
    './js/pointsSystem.js',
    './js/app.js',
    './js/pwa.js',
    './js/search/instruction/go.js',
    './js/search/instruction/Email.js',
    './js/search/utils.js',
    './js/card/weather.js',

    './ui/js/modules/modal.js',
    './ui/js/modules/feedback.js',
    './ui/js/modules/captcha.js',
    './ui/js/modules/tabs.js',
    './ui/js/modules/scrollspy.js',
    './ui/js/modules/carousel.js',
    './ui/js/modules/theme.js',
    './ui/js/foolproof/VerificationCode/code.js',

    './Service/game/home.html',
    './Service/game/gacha.html',
    './Service/game/Tetris/Tetris.html',
    './Service/game/Tetris/style.css',
    './Service/game/Tetris/game.js',
    './Service/game/Tic-tac-toe/Tic-Tac-Toe.html',
    './Service/game/Tic-tac-toe/style.css',
    './Service/game/Tic-tac-toe/game.js',
    './Service/game/LightCorridor/LightCorridor.html',
    './Service/game/Star Luster/Star Luster.html',

    './help/home.html',

    './img/app/earth-box.svg',
    './img/app/earth.svg'
];

const FALLBACK_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24"><path fill="none" stroke="#ccc" stroke-width="2" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/></svg>';

self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then((cache) => {
            const addPromises = CORE_ASSETS.map((asset) => {
                return cache.add(asset).catch((error) => {
                    console.warn('Failed to cache asset:', asset, error);
                });
            });
            return Promise.allSettled(addPromises);
        }).then(() => {
            self.skipWaiting();
        }).catch((error) => {
            console.error('Cache installation failed:', error);
        })
    );
});

self.addEventListener('activate', (event) => {
    const currentCaches = [CACHE_NAME, RUNTIME_CACHE_NAME];
    event.waitUntil(
        caches.keys().then((cacheNames) => {
            return Promise.all(
                cacheNames.map((cacheName) => {
                    if (!currentCaches.includes(cacheName)) {
                        return caches.delete(cacheName);
                    }
                })
            );
        }).then(() => {
            self.clients.claim();
        })
    );
});

self.addEventListener('fetch', (event) => {
    const request = event.request;

    if (request.method !== 'GET') {
        return;
    }

    if (/\/img\/weather\//.test(request.url)) {
        event.respondWith(
            caches.open(RUNTIME_CACHE_NAME).then((cache) => {
                return cache.match(request).then((cachedResponse) => {
                    const fetchPromise = fetch(request).then((networkResponse) => {
                        cache.put(request, networkResponse.clone());
                        return networkResponse;
                    }).catch(() => {
                        return cachedResponse || new Response(FALLBACK_SVG, { headers: { 'Content-Type': 'image/svg+xml' } });
                    });
                    return cachedResponse || fetchPromise;
                });
            })
        );
        return;
    }

    if (/\.html$/.test(request.url) || request.url === self.location.origin + '/' || request.url === self.location.origin) {
        event.respondWith(
            fetch(request).then((networkResponse) => {
                caches.open(CACHE_NAME).then((cache) => {
                    cache.put(request, networkResponse.clone());
                });
                return networkResponse;
            }).catch(() => {
                return caches.match(request).then((cachedResponse) => {
                    return cachedResponse || caches.match('./index.html');
                });
            })
        );
        return;
    }

    event.respondWith(
        caches.match(request).then((cachedResponse) => {
            if (cachedResponse) {
                fetch(request).then((networkResponse) => {
                    caches.open(CACHE_NAME).then((cache) => {
                        cache.put(request, networkResponse.clone());
                    });
                }).catch(() => {});
                return cachedResponse;
            }
            return fetch(request).then((networkResponse) => {
                caches.open(RUNTIME_CACHE_NAME).then((cache) => {
                    cache.put(request, networkResponse.clone());
                });
                return networkResponse;
            }).catch(() => {
                if (/\.css$/.test(request.url)) {
                    return new Response('', { headers: { 'Content-Type': 'text/css' } });
                }
                if (/\.js$/.test(request.url)) {
                    return new Response('console.log("Script failed to load");', { headers: { 'Content-Type': 'application/javascript' } });
                }
                if (/\.svg$/.test(request.url)) {
                    return new Response(FALLBACK_SVG, { headers: { 'Content-Type': 'image/svg+xml' } });
                }
            });
        })
    );
});

self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }
});

fun