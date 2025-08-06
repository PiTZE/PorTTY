// ============================================================================
// CONSTANTS AND CONFIGURATION
// ============================================================================

const CACHE_NAME = 'portty-v0.2';
const STATIC_CACHE = 'portty-static-v0.2';
const DYNAMIC_CACHE = 'portty-dynamic-v0.2';

const STATIC_ASSETS = [
    '/',
    '/terminal.css',
    '/terminal.js',
    '/manifest.json',
    '/icon.svg',
    'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.css',
    'https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.js',
    'https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js',
    'https://cdn.jsdelivr.net/npm/@xterm/addon-attach@0.11.0/lib/addon-attach.js',
    'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap'
];

// ============================================================================
// SERVICE WORKER LIFECYCLE
// ============================================================================

self.addEventListener('install', (event) => {
    console.log('[SW] Installing service worker...');
    
    event.waitUntil(
        caches.open(STATIC_CACHE)
            .then((cache) => {
                console.log('[SW] Caching static assets');
                return cache.addAll(STATIC_ASSETS);
            })
            .then(() => {
                console.log('[SW] Static assets cached successfully');
                return self.skipWaiting();
            })
            .catch((error) => {
                console.error('[SW] Failed to cache static assets:', error);
            })
    );
});

self.addEventListener('activate', (event) => {
    console.log('[SW] Activating service worker...');
    
    event.waitUntil(
        caches.keys()
            .then((cacheNames) => {
                return Promise.all(
                    cacheNames.map((cacheName) => {
                        if (cacheName !== STATIC_CACHE && cacheName !== DYNAMIC_CACHE) {
                            console.log('[SW] Deleting old cache:', cacheName);
                            return caches.delete(cacheName);
                        }
                    })
                );
            })
            .then(() => {
                console.log('[SW] Service worker activated');
                return self.clients.claim();
            })
    );
});

// ============================================================================
// FETCH HANDLING - CACHE STRATEGIES
// ============================================================================

self.addEventListener('fetch', (event) => {
    const { request } = event;
    const url = new URL(request.url);
    
    // Skip non-GET requests
    if (request.method !== 'GET') {
        return;
    }
    
    // Skip WebSocket upgrade requests
    if (request.headers.get('upgrade') === 'websocket') {
        return;
    }
    
    // Handle different types of requests with appropriate strategies
    if (isStaticAsset(request.url)) {
        // Static assets: Cache First strategy
        event.respondWith(cacheFirst(request));
    } else if (url.pathname === '/') {
        // Main page: Network First with cache fallback
        event.respondWith(networkFirst(request));
    } else if (url.pathname.startsWith('/ws')) {
        // WebSocket endpoint: Don't intercept
        return;
    } else {
        // Other requests: Network First
        event.respondWith(networkFirst(request));
    }
});

// ============================================================================
// CACHE STRATEGIES
// ============================================================================

async function cacheFirst(request) {
    try {
        const cachedResponse = await caches.match(request);
        if (cachedResponse) {
            return cachedResponse;
        }
        
        const networkResponse = await fetch(request);
        if (networkResponse.ok) {
            const cache = await caches.open(STATIC_CACHE);
            cache.put(request, networkResponse.clone());
        }
        return networkResponse;
    } catch (error) {
        console.error('[SW] Cache first failed:', error);
        return new Response('Offline - Asset not available', { 
            status: 503,
            statusText: 'Service Unavailable'
        });
    }
}

async function networkFirst(request) {
    try {
        const networkResponse = await fetch(request);
        if (networkResponse.ok) {
            const cache = await caches.open(DYNAMIC_CACHE);
            cache.put(request, networkResponse.clone());
        }
        return networkResponse;
    } catch (error) {
        console.log('[SW] Network failed, trying cache:', error.message);
        const cachedResponse = await caches.match(request);
        
        if (cachedResponse) {
            return cachedResponse;
        }
        
        // Return offline page for main requests
        if (request.destination === 'document') {
            return createOfflinePage();
        }
        
        return new Response('Offline', { 
            status: 503,
            statusText: 'Service Unavailable'
        });
    }
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

function isStaticAsset(url) {
    return STATIC_ASSETS.some(asset => url.includes(asset)) ||
           url.includes('.css') ||
           url.includes('.js') ||
           url.includes('.ico') ||
           url.includes('fonts.googleapis.com') ||
           url.includes('cdn.jsdelivr.net');
}

function createOfflinePage() {
    const offlineHTML = `
        <!DOCTYPE html>
        <html lang="en">
        <head>
            <meta charset="UTF-8">
            <meta name="viewport" content="width=device-width, initial-scale=1.0">
            <title>PorTTY - Offline Mode</title>
            <style>
                body {
                    margin: 0;
                    padding: 0;
                    background: #000;
                    color: #f0f0f0;
                    font-family: 'JetBrains Mono', monospace;
                    display: flex;
                    flex-direction: column;
                    justify-content: center;
                    align-items: center;
                    height: 100vh;
                    text-align: center;
                }
                .offline-container {
                    max-width: 600px;
                    padding: 2rem;
                }
                .terminal-icon {
                    font-size: 4rem;
                    margin-bottom: 1rem;
                }
                h1 {
                    color: #80ff80;
                    margin-bottom: 1rem;
                }
                .status {
                    color: #ffff80;
                    margin: 1rem 0;
                }
                .actions {
                    margin-top: 2rem;
                }
                button {
                    background: #333;
                    color: #f0f0f0;
                    border: 1px solid #666;
                    padding: 0.5rem 1rem;
                    margin: 0.5rem;
                    cursor: pointer;
                    font-family: inherit;
                }
                button:hover {
                    background: #555;
                }
                .offline-terminal {
                    margin-top: 2rem;
                    text-align: left;
                    background: #111;
                    padding: 1rem;
                    border-radius: 4px;
                    font-size: 0.9rem;
                }
            </style>
        </head>
        <body>
            <div class="offline-container">
                <div class="terminal-icon">💻</div>
                <h1>PorTTY - Connection Error</h1>
                <div class="status">
                    ⚠️ Cannot connect to PorTTY server<br>
                    Please check your connection and try again
                </div>
                
                <div class="actions">
                    <button onclick="location.reload()">🔄 Retry Connection</button>
                </div>
                
                <div class="offline-terminal" id="offline-info">
                    <div>$ connection status</div>
                    <div style="color: #ff8080;">✗ WebSocket connection failed</div>
                    <div style="color: #80ff80;">✓ PWA cached and available</div>
                    <div style="color: #ffff80;">ℹ Auto-retry in progress...</div>
                </div>
            </div>
            
            <script>
                // Auto-retry connection every 10 seconds
                setInterval(() => {
                    console.log('Auto-retry: attempting reconnection...');
                    location.reload();
                }, 10000);
                
                // Listen for online events
                window.addEventListener('online', () => {
                    console.log('Network restored, reloading...');
                    location.reload();
                });
            </script>
        </body>
        </html>
    `;
    
    return new Response(offlineHTML, {
        headers: { 'Content-Type': 'text/html' }
    });
}

// ============================================================================
// BACKGROUND SYNC (Simplified for PWA caching only)
// ============================================================================

self.addEventListener('sync', (event) => {
    console.log('[SW] Background sync triggered:', event.tag);
    
    if (event.tag === 'cache-update') {
        event.waitUntil(updateCache());
    }
});

async function updateCache() {
    try {
        console.log('[SW] Updating cache in background');
        const cache = await caches.open(STATIC_CACHE);
        
        const criticalAssets = [
            '/',
            '/terminal.js',
            '/terminal.css'
        ];
        
        for (const asset of criticalAssets) {
            try {
                const response = await fetch(asset);
                if (response.ok) {
                    await cache.put(asset, response);
                }
            } catch (error) {
                console.error(`[SW] Failed to update ${asset}:`, error);
            }
        }
    } catch (error) {
        console.error('[SW] Background cache update failed:', error);
    }
}

// ============================================================================
// PUSH NOTIFICATIONS
// ============================================================================

self.addEventListener('push', (event) => {
    console.log('[SW] Push notification received');
    
    const options = {
        body: 'Terminal activity detected',
        icon: '/icon.svg',
        badge: '/icon.svg',
        tag: 'terminal-activity',
        requireInteraction: false,
        actions: [
            {
                action: 'open',
                title: 'Open Terminal'
            },
            {
                action: 'dismiss',
                title: 'Dismiss'
            }
        ]
    };
    
    event.waitUntil(
        self.registration.showNotification('PorTTY', options)
    );
});

self.addEventListener('notificationclick', (event) => {
    event.notification.close();
    
    if (event.action === 'open') {
        event.waitUntil(
            clients.openWindow('/')
        );
    }
});

// ============================================================================
// MESSAGE HANDLING
// ============================================================================

self.addEventListener('message', (event) => {
    const { type, data } = event.data;
    
    switch (type) {
        case 'SKIP_WAITING':
            self.skipWaiting();
            break;
            
        case 'GET_CACHE_STATUS':
            getCacheStatus().then(status => {
                event.ports[0].postMessage(status);
            });
            break;
            
        case 'UPDATE_CACHE':
            // Trigger background cache update
            self.registration.sync.register('cache-update');
            break;
            
        default:
            console.log('[SW] Unknown message type:', type);
    }
});

// ============================================================================
// CACHE MANAGEMENT HELPERS
// ============================================================================

async function getCacheStatus() {
    const staticCache = await caches.open(STATIC_CACHE);
    const dynamicCache = await caches.open(DYNAMIC_CACHE);
    
    const staticKeys = await staticCache.keys();
    const dynamicKeys = await dynamicCache.keys();
    
    return {
        staticCached: staticKeys.length,
        dynamicCached: dynamicKeys.length,
        totalSize: staticKeys.length + dynamicKeys.length,
        version: CACHE_NAME
    };
}