document.addEventListener('DOMContentLoaded', () => {
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
    const socket = new WebSocket(wsUrl);
    
    const fitAddon = new FitAddon();
    const attachAddon = new AttachAddon(socket);
    
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
        fitAddon.fit();
        sendResize();
    });
    
    window.addEventListener('resize', () => {
        fitAddon.fit();
        sendResize();
    });
});