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
    
    console.log('All components loaded successfully');
    
    const term = new Terminal({
        cursorBlink: true,
        fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', Menlo, Monaco, 'Courier New', monospace",
        fontSize: 14,
        theme: { 
            background: '#000000',
            foreground: '#f0f0f0',
            cursor: '#ffffff'
        },
        scrollback: 10000
    });
    
    const terminalContainer = document.getElementById('terminal-container');
    
    if (!terminalContainer) {
        console.error('Terminal container not found');
        return;
    }
    
    term.open(terminalContainer);
    term.focus();
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    // Create WebSocket with better error handling
    const socket = new WebSocket(wsUrl);
    
    // Handle WebSocket errors
    socket.addEventListener('error', (event) => {
        console.error('WebSocket error:', event);
        term.write('\r\n\x1b[31mWebSocket connection failed. Please check if the server is running.\x1b[0m\r\n');
    });
    
    // Handle WebSocket close
    socket.addEventListener('close', (event) => {
        console.log('WebSocket closed:', event.code, event.reason);
        if (event.code !== 1000) {
            term.write('\r\n\x1b[31mWebSocket connection closed unexpectedly. Trying to reconnect...\x1b[0m\r\n');
            // Simple reconnection logic
            setTimeout(() => {
                location.reload();
            }, 3000);
        }
    });
    
    // Create addons using the correct constructor access
    const fitAddon = new window.FitAddon.FitAddon();
    const attachAddon = new window.AttachAddon.AttachAddon(socket);
    
    term.loadAddon(fitAddon);
    term.loadAddon(attachAddon);
    
    function sendResize() {
        if (socket.readyState === WebSocket.OPEN) {
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
    
    socket.addEventListener('open', () => {
        console.log('WebSocket connected');
        fitAddon.fit();
        sendResize();
    });
    
    window.addEventListener('resize', () => {
        fitAddon.fit();
        sendResize();
    });
});