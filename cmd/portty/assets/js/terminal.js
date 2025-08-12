// ============================================================================
// CONSTANTS AND CONFIGURATION
// ============================================================================

const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 1000;
const KEEP_ALIVE_INTERVAL = 30000;

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

function isRunningOnLocalhost() {
    const hostname = window.location.hostname;
    return ['localhost', '127.0.0.1', '::1'].includes(hostname);
}

function testWebGL2Support() {
    try {
        if (typeof window.WebGL2RenderingContext === 'undefined') {
            return false;
        }
        
        if (typeof window.OffscreenCanvas === 'undefined') {
            return false;
        }
        
        const offscreenCanvas = new OffscreenCanvas(1, 1);
        const gl = offscreenCanvas.getContext('webgl2');
        if (!gl) {
            return false;
        }
        
        return true;
    } catch (error) {
        console.warn('[PorTTY] WebGL2 support test failed:', error);
        return false;
    }
}

function getThemeFromCSS() {
    const rootStyles = getComputedStyle(document.documentElement);
    return {
        fontFamily: rootStyles.getPropertyValue('--font-family').trim() || "'JetBrains Mono', monospace",
        fontSize: parseInt(rootStyles.getPropertyValue('--font-size').trim()) || 14,
        backgroundColor: rootStyles.getPropertyValue('--background-color').trim() || '#000000',
        foregroundColor: rootStyles.getPropertyValue('--foreground-color').trim() || '#f0f0f0',
        cursorColor: rootStyles.getPropertyValue('--cursor-color').trim() || '#ffffff'
    };
}

function validateDependencies() {
    const requiredAddons = [
        { name: 'Terminal', check: () => typeof Terminal !== 'undefined' },
        { name: 'FitAddon', check: () => typeof window.FitAddon !== 'undefined' && typeof window.FitAddon.FitAddon !== 'undefined' },
        { name: 'AttachAddon', check: () => typeof window.AttachAddon !== 'undefined' && typeof window.AttachAddon.AttachAddon !== 'undefined' },
        { name: 'WebglAddon', check: () => typeof window.WebglAddon !== 'undefined' && typeof window.WebglAddon.WebglAddon !== 'undefined' },
        { name: 'SearchAddon', check: () => typeof window.SearchAddon !== 'undefined' && typeof window.SearchAddon.SearchAddon !== 'undefined' },
        { name: 'Unicode11Addon', check: () => typeof window.Unicode11Addon !== 'undefined' && typeof window.Unicode11Addon.Unicode11Addon !== 'undefined' },
        { name: 'WebLinksAddon', check: () => typeof window.WebLinksAddon !== 'undefined' && typeof window.WebLinksAddon.WebLinksAddon !== 'undefined' },
        { name: 'ClipboardAddon', check: () => typeof window.ClipboardAddon !== 'undefined' && typeof window.ClipboardAddon.ClipboardAddon !== 'undefined' }
    ];
    
    let allLoaded = true;
    
    for (const addon of requiredAddons) {
        if (!addon.check()) {
            console.error(`${addon.name} not loaded - check CDN link and network connectivity`);
            allLoaded = false;
        }
    }
    
    return allLoaded;
}

// ============================================================================
// CLASS DEFINITIONS
// ============================================================================

class ConnectionStatusManager {
    constructor() {
        this.statusIndicator = document.getElementById('status-indicator');
        this.statusText = document.getElementById('status-text');
        this.connectionStatus = document.getElementById('connection-status');
        this.isLocalhost = isRunningOnLocalhost();
        this.initialize();
    }
    
    initialize() {
        if (this.isLocalhost) {
            this.hideConnectionStatus();
            return;
        }
        
        if (this.connectionStatus) {
            this.makeVisible();
        } else {
            console.error('[PorTTY] Connection status element not found in DOM');
        }
        
        this.updateStatus('connecting');
        this.ensureVisibilityFallback();
    }
    
    makeVisible() {
        if (this.connectionStatus && !this.isLocalhost) {
            this.connectionStatus.classList.remove('hidden');
            this.connectionStatus.style.display = 'flex';
            this.connectionStatus.style.visibility = 'visible';
            this.connectionStatus.style.opacity = '1';
        }
    }
    
    hideConnectionStatus() {
        if (this.connectionStatus) {
            this.connectionStatus.style.display = 'none';
            this.connectionStatus.style.visibility = 'hidden';
            this.connectionStatus.classList.add('hidden');
        }
    }
    
    updateStatus(status) {
        if (this.isLocalhost) {
            return;
        }
        
        this.makeVisible();
        
        if (this.statusIndicator && this.statusText) {
            this.statusIndicator.className = `status-indicator ${status}`;
            
            const statusMessages = {
                'connecting': 'Connecting...',
                'connected': 'Connected',
                'disconnected': 'Disconnected',
                'reconnecting': 'Reconnecting...',
                'error': 'Connection Error',
                'failed': 'Connection Failed'
            };
            
            this.statusText.textContent = statusMessages[status] || status;
        }
    }
    
    ensureVisibilityFallback() {
        if (this.isLocalhost) {
            return;
        }
        
        setTimeout(() => {
            if (this.connectionStatus && this.connectionStatus.style.display === 'none') {
                this.makeVisible();
                this.updateStatus('connecting');
            }
        }, 500);
        
        setTimeout(() => {
            if (this.connectionStatus && !this.connectionStatus.offsetParent) {
                this.makeVisible();
            }
        }, 1000);
    }
}

class FontSizeManager {
    constructor(term) {
        this.term = term;
        this.currentSize = parseInt(getComputedStyle(document.documentElement).getPropertyValue('--font-size').trim()) || 14;
        this.minSize = 8;
        this.maxSize = 32;
    }
    
    increaseFontSize() {
        if (this.currentSize < this.maxSize) {
            this.currentSize += 1;
            this.updateFontSize();
        }
    }
    
    decreaseFontSize() {
        if (this.currentSize > this.minSize) {
            this.currentSize -= 1;
            this.updateFontSize();
        }
    }
    
    resetFontSize() {
        this.currentSize = 14;
        this.updateFontSize();
    }
    
    updateFontSize() {
        this.term.options.fontSize = this.currentSize;
        document.documentElement.style.setProperty('--font-size', `${this.currentSize}px`);
        
        if (window.porttyFitAddon) {
            requestAnimationFrame(() => {
                window.porttyFitAddon.fit();
                sendResize(this.term);
            });
        }
    }
}

class SearchManager {
    constructor(term, searchAddon) {
        this.term = term;
        this.searchAddon = searchAddon;
        this.isSearchVisible = false;
        this.createSearchOverlay();
    }
    
    createSearchOverlay() {
        this.searchOverlay = document.createElement('div');
        this.searchOverlay.className = 'search-overlay hidden';
        this.searchOverlay.innerHTML = `
            <div class="search-box">
                <input type="text" id="search-input" placeholder="Search terminal..." />
                <button id="search-prev" title="Previous match">↑</button>
                <button id="search-next" title="Next match">↓</button>
                <button id="search-close" title="Close search">×</button>
            </div>
        `;
        
        document.body.appendChild(this.searchOverlay);
        
        this.searchInput = document.getElementById('search-input');
        this.searchPrev = document.getElementById('search-prev');
        this.searchNext = document.getElementById('search-next');
        this.searchClose = document.getElementById('search-close');
        
        this.setupSearchEvents();
    }
    
    setupSearchEvents() {
        this.searchInput.addEventListener('input', (e) => {
            const query = e.target.value;
            if (query) {
                this.searchAddon.findNext(query);
            }
        });
        
        this.searchInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                if (e.shiftKey) {
                    this.findPrevious();
                } else {
                    this.findNext();
                }
            } else if (e.key === 'Escape') {
                this.hideSearch();
            }
        });
        
        this.searchPrev.addEventListener('click', () => this.findPrevious());
        this.searchNext.addEventListener('click', () => this.findNext());
        this.searchClose.addEventListener('click', () => this.hideSearch());
    }
    
    showSearch() {
        this.isSearchVisible = true;
        this.searchOverlay.classList.remove('hidden');
        this.searchInput.focus();
        this.searchInput.select();
    }
    
    hideSearch() {
        this.isSearchVisible = false;
        this.searchOverlay.classList.add('hidden');
        this.term.focus();
    }
    
    findNext() {
        const query = this.searchInput.value;
        if (query) {
            this.searchAddon.findNext(query);
        }
    }
    
    findPrevious() {
        const query = this.searchInput.value;
        if (query) {
            this.searchAddon.findPrevious(query);
        }
    }
    
    toggleSearch() {
        if (this.isSearchVisible) {
            this.hideSearch();
        } else {
            this.showSearch();
        }
    }
}

// ============================================================================
// MAIN INITIALIZATION LOGIC
// ============================================================================

function initializePorTTY() {
    if (!validateDependencies()) {
        return;
    }
    
    const supportsWebgl2InWorker = testWebGL2Support();
    console.log('[PorTTY] WebGL2 support detected:', supportsWebgl2InWorker);
    
    const theme = getThemeFromCSS();
    const terminalContainer = document.getElementById('terminal-container');
    
    if (!terminalContainer) {
        console.error('Terminal container not found');
        return;
    }
    
    const term = new Terminal({
        cursorBlink: true,
        fontFamily: theme.fontFamily,
        fontSize: theme.fontSize,
        theme: {
            background: theme.backgroundColor,
            foreground: theme.foregroundColor,
            cursor: theme.cursorColor
        },
        scrollback: 10000,
        allowTransparency: false,
        fastScrollModifier: 'alt',
        disableStdin: false,
        screenReaderMode: false,
        rendererType: supportsWebgl2InWorker ? 'webgl' : 'canvas',
        allowProposedApi: true
    });
    
    const fitAddon = new window.FitAddon.FitAddon();
    const searchAddon = new window.SearchAddon.SearchAddon();
    const unicode11Addon = new window.Unicode11Addon.Unicode11Addon();
    const webLinksAddon = new window.WebLinksAddon.WebLinksAddon();
    const clipboardAddon = new window.ClipboardAddon.ClipboardAddon();
    
    term.open(terminalContainer);
    
    term.loadAddon(fitAddon);
    term.loadAddon(searchAddon);
    term.loadAddon(unicode11Addon);
    term.loadAddon(webLinksAddon);
    term.loadAddon(clipboardAddon);
    
    if (supportsWebgl2InWorker) {
        try {
            const webglAddon = new window.WebglAddon.WebglAddon();
            webglAddon.onContextLoss(() => {
                console.warn('WebGL context lost, falling back to canvas');
                webglAddon.dispose();
                term.setOption('rendererType', 'canvas');
            });
            term.loadAddon(webglAddon);
        } catch (err) {
            console.warn('WebGL addon failed to load, falling back to canvas:', err);
            term.setOption('rendererType', 'canvas');
        }
    }
    
    const fontSizeManager = new FontSizeManager(term);
    const searchManager = new SearchManager(term, searchAddon);
    
    const performInitialFit = () => {
        const container = document.getElementById('terminal-container');
        if (container && container.offsetWidth > 0 && container.offsetHeight > 0) {
            try {
                fitAddon.fit();
                term.focus();
                sendResize(term);
            } catch (error) {
                console.error('[PorTTY] Error during initial fit:', error);
                setTimeout(() => {
                    try {
                        fitAddon.fit();
                        sendResize(term);
                    } catch (retryError) {
                        console.error('[PorTTY] Retry fit also failed:', retryError);
                    }
                }, 100);
            }
        } else {
            setTimeout(performInitialFit, 10);
        }
    };
    
    requestAnimationFrame(() => {
        requestAnimationFrame(() => {
            performInitialFit();
            
            setTimeout(() => {
                const container = document.getElementById('terminal-container');
                if (container && container.offsetWidth > 0 && container.offsetHeight > 0) {
                    fitAddon.fit();
                    sendResize(term);
                }
            }, 250);
        });
    });
    
    let socket = null;
    let reconnectAttempts = 0;
    
    const connectionManager = new ConnectionStatusManager();
    
    window.porttySocket = null;
    window.porttyTerminal = term;
    window.porttyFitAddon = fitAddon;
    window.porttyConnectionManager = connectionManager;
    window.porttyFontSizeManager = fontSizeManager;
    window.porttySearchManager = searchManager;
    
    setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts);
    setupReactiveResize(term, fitAddon);
    setupKeyboardShortcuts(fontSizeManager, searchManager, term);
}

// ============================================================================
// EVENT LISTENERS AND HANDLERS
// ============================================================================

function setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts) {
    function connectWebSocket() {
        connectionManager.updateStatus('connecting');
        
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        socket = new WebSocket(wsUrl);
        window.porttySocket = socket;
        const attachAddon = new window.AttachAddon.AttachAddon(socket);
        term.loadAddon(attachAddon);
        
        socket.addEventListener('open', () => {
            connectionManager.updateStatus('connected');
            reconnectAttempts = 0;
            
            sendResize(term);
        });
        
        socket.addEventListener('error', (event) => {
            console.error('WebSocket error:', event);
            connectionManager.updateStatus('error');
            term.write('\r\n\x1b[31mWebSocket connection error\x1b[0m\r\n');
        });
        
        socket.addEventListener('close', (event) => {
            connectionManager.updateStatus('disconnected');
            
            if (event.code !== 1000 && reconnectAttempts < MAX_RECONNECT_ATTEMPTS) {
                reconnectAttempts++;
                const delay = RECONNECT_DELAY * Math.pow(1.5, reconnectAttempts - 1);
                
                connectionManager.updateStatus('reconnecting');
                term.write(`\r\n\x1b[33mConnection closed. Reconnecting in ${Math.round(delay/1000)} seconds...\x1b[0m\r\n`);
                
                setTimeout(() => {
                    term.write('\r\n\x1b[33mAttempting to reconnect...\x1b[0m\r\n');
                    connectWebSocket();
                }, delay);
            } else if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
                connectionManager.updateStatus('failed');
                term.write('\r\n\x1b[31mFailed to reconnect after multiple attempts. Please refresh the page.\x1b[0m\r\n');
            }
        });
        
        const keepAliveInterval = setInterval(() => {
            if (socket && socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({ type: 'keepalive' }));
            } else {
                clearInterval(keepAliveInterval);
            }
        }, KEEP_ALIVE_INTERVAL);
    }
    
    connectWebSocket();
}

function sendResize(term) {
    const socket = window.porttySocket;
    if (socket && socket.readyState === WebSocket.OPEN) {
        const resizeMessage = JSON.stringify({
            type: 'resize',
            dimensions: {
                cols: term.cols,
                rows: term.rows
            }
        });
        socket.send(resizeMessage);
    }
}

function setupReactiveResize(term, fitAddon) {
    let lastDimensions = { width: 0, height: 0 };
    
    const performResize = () => {
        const container = document.getElementById('terminal-container');
        if (!container) {
            return;
        }
        
        const containerRect = container.getBoundingClientRect();
        
        if (containerRect.width <= 0 || containerRect.height <= 0) {
            return;
        }
        
        if (Math.abs(containerRect.width - lastDimensions.width) < 5 &&
            Math.abs(containerRect.height - lastDimensions.height) < 5) {
            return;
        }
        
        lastDimensions = { width: containerRect.width, height: containerRect.height };
        
        try {
            fitAddon.fit();
            sendResize(term);
        } catch (error) {
            console.error('[PorTTY] Error during fit:', error);
        }
    };
    
    window.addEventListener('resize', performResize);
    window.addEventListener('beforeunload', () => {
        if (window.porttySocket && window.porttySocket.readyState === WebSocket.OPEN) {
            window.porttySocket.close(1000, 'Page unloaded');
        }
    });
    
    window.porttyManualResize = performResize;
}


function setupKeyboardShortcuts(fontSizeManager, searchManager, term) {
    const handleKeydown = (e) => {
        if (e.ctrlKey && (e.key === '=' || e.key === '+')) {
            e.preventDefault();
            e.stopPropagation();
            fontSizeManager.increaseFontSize();
            return;
        }
        
        if (e.ctrlKey && e.key === '-') {
            e.preventDefault();
            e.stopPropagation();
            fontSizeManager.decreaseFontSize();
            return;
        }
        
        if (e.ctrlKey && e.key === '0') {
            e.preventDefault();
            e.stopPropagation();
            fontSizeManager.resetFontSize();
            return;
        }
        
        if (e.ctrlKey && (e.key === 'f' || e.key === 'F')) {
            e.preventDefault();
            e.stopPropagation();
            searchManager.toggleSearch();
            return;
        }
        
        if ((e.ctrlKey && e.key === 'r') || e.key === 'F5') {
            if (window.porttySocket && window.porttySocket.readyState !== WebSocket.OPEN) {
                e.preventDefault();
            }
        }
    };
    
    document.addEventListener('keydown', handleKeydown, true);
    term.element.addEventListener('keydown', handleKeydown, true);
}

// ============================================================================
// DOM READY INITIALIZATION
// ============================================================================

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializePorTTY);
} else {
    initializePorTTY();
}