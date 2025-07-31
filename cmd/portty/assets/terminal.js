document.addEventListener('DOMContentLoaded', () => {
    // Check if required components are loaded
    if (typeof Terminal === 'undefined') {
        console.error('Terminal not loaded');
        return;
    }
    
    // The addons are loaded as window.FitAddon.FitAddon and window.AttachAddon.AttachAddon
    if (typeof window.FitAddon === 'undefined' || typeof window.FitAddon.FitAddon === 'undefined') {
        console.error('FitAddon not loaded');
        return;
    }
    
    if (typeof window.AttachAddon === 'undefined' || typeof window.AttachAddon.AttachAddon === 'undefined') {
        console.error('AttachAddon not loaded');
        return;
    }
    
    // Terminal configuration with optimized settings
    const term = new Terminal({
        cursorBlink: true,
        fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', Menlo, Monaco, 'Courier New', monospace",
        fontSize: 14,
        theme: { 
            background: '#000000',
            foreground: '#f0f0f0',
            cursor: '#ffffff'
        },
        scrollback: 10000,
        // Performance optimizations
        allowTransparency: false,
        fastScrollModifier: 'alt',
        disableStdin: false,
        screenReaderMode: false,
        rendererType: 'canvas',
        // Increase buffer size for better performance
        cols: 100,
        rows: 40
    });
    
    const terminalContainer = document.getElementById('terminal-container');
    
    if (!terminalContainer) {
        console.error('Terminal container not found');
        return;
    }
    
    term.open(terminalContainer);
    term.focus();
    
    // Connection management
    let socket = null;
    let reconnectAttempts = 0;
    const maxReconnectAttempts = 5;
    const reconnectDelay = 1000; // Start with 1 second delay
    
    // Create addons
    const fitAddon = new window.FitAddon.FitAddon();
    term.loadAddon(fitAddon);
    
    // Function to connect WebSocket
    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        // Create a new WebSocket connection
        socket = new WebSocket(wsUrl);
        
        // Create attach addon with the new socket
        const attachAddon = new window.AttachAddon.AttachAddon(socket);
        term.loadAddon(attachAddon);
        
        // Handle WebSocket open event
        socket.addEventListener('open', () => {
            console.log('WebSocket connected');
            term.write('\r\n\x1b[32mConnected to terminal server\x1b[0m\r\n');
            reconnectAttempts = 0; // Reset reconnect attempts on successful connection
            
            // Fit terminal to container and send resize event
            fitAddon.fit();
            sendResize();
        });
        
        // Handle WebSocket errors
        socket.addEventListener('error', (event) => {
            console.error('WebSocket error:', event);
            term.write('\r\n\x1b[31mWebSocket connection error\x1b[0m\r\n');
        });
        
        // Handle WebSocket close
        socket.addEventListener('close', (event) => {
            console.log('WebSocket closed:', event.code, event.reason);
            
            // Only attempt to reconnect if it wasn't a normal closure
            if (event.code !== 1000 && reconnectAttempts < maxReconnectAttempts) {
                reconnectAttempts++;
                const delay = reconnectDelay * Math.pow(1.5, reconnectAttempts - 1); // Exponential backoff
                
                term.write(`\r\n\x1b[33mConnection closed. Reconnecting in ${Math.round(delay/1000)} seconds...\x1b[0m\r\n`);
                
                setTimeout(() => {
                    term.write('\r\n\x1b[33mAttempting to reconnect...\x1b[0m\r\n');
                    connectWebSocket();
                }, delay);
            } else if (reconnectAttempts >= maxReconnectAttempts) {
                term.write('\r\n\x1b[31mFailed to reconnect after multiple attempts. Please refresh the page.\x1b[0m\r\n');
            }
        });
    }
    
    // Function to send resize events
    function sendResize() {
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
    
    // Handle window resize events with debouncing
    let resizeTimer;
    window.addEventListener('resize', () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => {
            fitAddon.fit();
            sendResize();
        }, 100);
    });
    
    // Initial connection
    connectWebSocket();
    
    // Cleanup on page unload
    window.addEventListener('beforeunload', () => {
        if (socket && socket.readyState === WebSocket.OPEN) {
            // Send a clean close frame
            socket.close(1000, 'Page unloaded');
        }
    });
});