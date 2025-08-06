// ============================================================================
// CONSTANTS AND CONFIGURATION
// ============================================================================

const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAY = 1000;
const KEEP_ALIVE_INTERVAL = 30000;
const RESIZE_DEBOUNCE_DELAY = 100;

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

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
    if (typeof Terminal === 'undefined') {
        console.error('Terminal not loaded');
        return false;
    }
    
    if (typeof window.FitAddon === 'undefined' || typeof window.FitAddon.FitAddon === 'undefined') {
        console.error('FitAddon not loaded');
        return false;
    }
    
    if (typeof window.AttachAddon === 'undefined' || typeof window.AttachAddon.AttachAddon === 'undefined') {
        console.error('AttachAddon not loaded');
        return false;
    }
    
    return true;
}

// ============================================================================
// CLASS DEFINITIONS
// ============================================================================

class ConnectionStatusManager {
    constructor() {
        this.statusIndicator = document.getElementById('status-indicator');
        this.statusText = document.getElementById('status-text');
        this.connectionStatus = document.getElementById('connection-status');
        this.initialize();
    }
    
    initialize() {
        console.log('[PorTTY] Initializing connection status, element found:', !!this.connectionStatus);
        
        if (this.connectionStatus) {
            this.makeVisible();
            console.log('[PorTTY] Connection status made visible');
        } else {
            console.error('[PorTTY] Connection status element not found in DOM');
        }
        
        this.updateStatus('connecting');
        this.ensureVisibilityFallback();
    }
    
    makeVisible() {
        if (this.connectionStatus) {
            this.connectionStatus.classList.remove('hidden');
            this.connectionStatus.style.display = 'flex';
            this.connectionStatus.style.visibility = 'visible';
            this.connectionStatus.style.opacity = '1';
        }
    }
    
    updateStatus(status) {
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
        setTimeout(() => {
            if (this.connectionStatus && this.connectionStatus.style.display === 'none') {
                console.log('[PorTTY] Fallback: Making connection status visible');
                this.makeVisible();
                this.updateStatus('connecting');
            }
        }, 500);
        
        setTimeout(() => {
            if (this.connectionStatus && !this.connectionStatus.offsetParent) {
                console.log('[PorTTY] Final fallback: Connection status not visible, forcing visibility');
                this.makeVisible();
            }
        }, 1000);
    }
}

// ============================================================================
// MAIN INITIALIZATION LOGIC
// ============================================================================

function initializePorTTY() {
    if (!validateDependencies()) {
        return;
    }
    
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
        rendererType: 'canvas',
        cols: 100,
        rows: 40
    });
    
    term.open(terminalContainer);
    term.focus();
    
    let socket = null;
    let reconnectAttempts = 0;
    
    const fitAddon = new window.FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    
    const connectionManager = new ConnectionStatusManager();
    
    setupPWAInstallation();
    setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts);
    setupEventListeners(term, fitAddon, socket);
    setupConnectionInfo(socket, reconnectAttempts, term);
    
    window.porttyTerminal = term;
    window.porttyConnectionManager = connectionManager;
}

// ============================================================================
// EVENT LISTENERS AND HANDLERS
// ============================================================================

function setupPWAInstallation() {
    let deferredPrompt;
    
    window.addEventListener('beforeinstallprompt', (e) => {
        e.preventDefault();
        deferredPrompt = e;
        showInstallButton(deferredPrompt);
    });
}

function showInstallButton(deferredPrompt) {
    const installButton = document.getElementById('install-button');
    if (installButton) {
        installButton.style.display = 'block';
        installButton.addEventListener('click', async () => {
            if (deferredPrompt) {
                deferredPrompt.prompt();
                const { outcome } = await deferredPrompt.userChoice;
                console.log(`User response to the install prompt: ${outcome}`);
                deferredPrompt = null;
                installButton.style.display = 'none';
            }
        });
    }
}

function setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts) {
    function connectWebSocket() {
        connectionManager.updateStatus('connecting');
        
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        socket = new WebSocket(wsUrl);
        const attachAddon = new window.AttachAddon.AttachAddon(socket);
        term.loadAddon(attachAddon);
        
        socket.addEventListener('open', () => {
            console.log('WebSocket connected');
            connectionManager.updateStatus('connected');
            term.write('\r\n\x1b[32mConnected to PorTTY server\x1b[0m\r\n');
            reconnectAttempts = 0;
            
            fitAddon.fit();
            sendResize(socket, term);
        });
        
        socket.addEventListener('error', (event) => {
            console.error('WebSocket error:', event);
            connectionManager.updateStatus('error');
            term.write('\r\n\x1b[31mWebSocket connection error\x1b[0m\r\n');
        });
        
        socket.addEventListener('close', (event) => {
            console.log('WebSocket closed:', event.code, event.reason);
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

function sendResize(socket, term) {
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

function setupEventListeners(term, fitAddon, socket) {
    let resizeTimer;
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => {
            fitAddon.fit();
            sendResize(socket, term);
        }, RESIZE_DEBOUNCE_DELAY);
    });
    
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey && e.key === 'r') || e.key === 'F5') {
            if (socket && socket.readyState !== WebSocket.OPEN) {
                e.preventDefault();
                setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts);
            }
        }
    });
    
    window.addEventListener('beforeunload', () => {
        if (socket && socket.readyState === WebSocket.OPEN) {
            socket.close(1000, 'Page unloaded');
        }
    });
}

function setupConnectionInfo(socket, reconnectAttempts, term) {
    const connectionInfoBtn = document.getElementById('offline-mode-btn');
    if (connectionInfoBtn) {
        connectionInfoBtn.addEventListener('click', () => {
            const info = {
                status: socket ? socket.readyState : 'No socket',
                url: socket ? socket.url : 'N/A',
                reconnectAttempts: reconnectAttempts,
                maxReconnectAttempts: MAX_RECONNECT_ATTEMPTS,
                keepAliveActive: socket && socket.readyState === WebSocket.OPEN
            };
            
            const statusText = {
                0: 'CONNECTING',
                1: 'OPEN',
                2: 'CLOSING',
                3: 'CLOSED'
            };
            
            alert(`Connection Information:
Status: ${statusText[info.status] || info.status}
URL: ${info.url}
Reconnect Attempts: ${info.reconnectAttempts}/${info.maxReconnectAttempts}
Keep-Alive: ${info.keepAliveActive ? 'Active' : 'Inactive'}
Terminal: ${term.cols}x${term.rows}`);
            
            setTimeout(() => {
                term.focus();
            }, 100);
        });
    }
}

// ============================================================================
// DOM READY INITIALIZATION
// ============================================================================

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initializePorTTY);
} else {
    initializePorTTY();
}