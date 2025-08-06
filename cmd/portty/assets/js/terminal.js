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
    
    if (typeof window.WebglAddon === 'undefined' || typeof window.WebglAddon.WebglAddon === 'undefined') {
        console.error('WebglAddon not loaded');
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
        rendererType: 'webgl' // Use WebGL renderer for better performance
        // No cols/rows specified - let FitAddon handle all sizing
    });
    
    // Load addons BEFORE opening terminal
    const fitAddon = new window.FitAddon.FitAddon();
    const webglAddon = new window.WebglAddon.WebglAddon();
    
    term.loadAddon(fitAddon);
    
    // Open terminal first
    term.open(terminalContainer);
    
    // Load WebGL addon after opening with context loss handling
    try {
        // Handle WebGL context loss as per best practices
        webglAddon.onContextLoss(e => {
            webglAddon.dispose();
        });
        
        term.loadAddon(webglAddon);
    } catch (error) {
        console.warn('[PorTTY] WebGL addon failed to load, falling back to canvas:', error);
    }
    
    // Wait for container to be properly sized, then fit
    requestAnimationFrame(() => {
        requestAnimationFrame(() => {
            fitAddon.fit();
            term.focus();
        });
    });
    
    let socket = null;
    let reconnectAttempts = 0;
    
    const connectionManager = new ConnectionStatusManager();
    
    // Global references for reactive resize and debugging
    window.porttySocket = null;
    window.porttyTerminal = term;
    window.porttyFitAddon = fitAddon;
    window.porttyConnectionManager = connectionManager;
    
    setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts);
    setupReactiveResize(term, fitAddon);
    setupConnectionInfo(socket, reconnectAttempts, term);
}

// ============================================================================
// EVENT LISTENERS AND HANDLERS
// ============================================================================

// PWA installation is handled in index.html to avoid conflicts

function setupWebSocketConnection(term, fitAddon, connectionManager, socket, reconnectAttempts) {
    function connectWebSocket() {
        connectionManager.updateStatus('connecting');
        
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        socket = new WebSocket(wsUrl);
        window.porttySocket = socket; // Update global reference
        const attachAddon = new window.AttachAddon.AttachAddon(socket);
        term.loadAddon(attachAddon);
        
        socket.addEventListener('open', () => {
            connectionManager.updateStatus('connected');
            term.write('\r\n\x1b[32mConnected to PorTTY server\x1b[0m\r\n');
            reconnectAttempts = 0;
            
            // Send initial size (terminal already fitted during initialization)
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
    
    // High-performance resize function - no debouncing needed
    const performResize = () => {
        const container = document.getElementById('terminal-container');
        if (!container) {
            return;
        }
        
        const containerRect = container.getBoundingClientRect();
        
        // Skip resize if container has no dimensions (hidden, etc.)
        if (containerRect.width <= 0 || containerRect.height <= 0) {
            return;
        }
        
        // Skip resize if dimensions haven't actually changed (5px threshold)
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
    
    // Only use window resize for maximum performance
    window.addEventListener('resize', performResize);
    
    // Keyboard shortcuts and cleanup (non-resize related)
    document.addEventListener('keydown', (e) => {
        if ((e.ctrlKey && e.key === 'r') || e.key === 'F5') {
            if (window.porttySocket && window.porttySocket.readyState !== WebSocket.OPEN) {
                e.preventDefault();
                // Trigger reconnection logic would go here
            }
        }
    });
    
    window.addEventListener('beforeunload', () => {
        if (window.porttySocket && window.porttySocket.readyState === WebSocket.OPEN) {
            window.porttySocket.close(1000, 'Page unloaded');
        }
    });
    
    // Manual resize trigger for debugging
    window.porttyManualResize = performResize;
}

function setupConnectionInfo(socket, reconnectAttempts, term) {
    const connectionInfoBtn = document.getElementById('offline-mode-btn');
    if (connectionInfoBtn && !isRunningOnLocalhost()) {
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