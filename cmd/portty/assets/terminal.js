// Wait for all scripts to load before initializing
function waitForAddons() {
    return new Promise((resolve) => {
        const checkAddons = () => {
            if (typeof FitAddon !== 'undefined' && typeof AttachAddon !== 'undefined' && 
                typeof FitAddon.FitAddon === 'function' && typeof AttachAddon.AttachAddon === 'function') {
                console.log('Addons loaded successfully');
                resolve();
            } else {
                console.log('Waiting for addons to load...', {
                    FitAddon: typeof FitAddon,
                    AttachAddon: typeof AttachAddon,
                    'FitAddon.FitAddon': typeof (FitAddon && FitAddon.FitAddon),
                    'AttachAddon.AttachAddon': typeof (AttachAddon && AttachAddon.AttachAddon)
                });
                setTimeout(checkAddons, 100);
            }
        };
        checkAddons();
    });
}

document.addEventListener('DOMContentLoaded', async () => {
    console.log('DOM loaded, waiting for addons...');
    
    // Wait for addons to be available
    await waitForAddons();
    
    console.log('Initializing terminal with addons');
    const terminalContainer = document.getElementById('terminal-container');
    if (!terminalContainer) {
        console.error('Terminal container not found!');
        return;
    }

    // Initialize terminal with better defaults
    const term = new Terminal({
        cursorBlink: true,
        fontFamily: "'JetBrains Mono', 'Cascadia Code', 'Fira Code', Menlo, Monaco, 'Courier New', monospace",
        fontSize: 14,
        theme: { 
            background: '#000000',
            foreground: '#f0f0f0',
            cursor: '#ffffff',
            black: '#000000',
            red: '#cc0000',
            green: '#4e9a06',
            yellow: '#c4a000',
            blue: '#3465a4',
            magenta: '#75507b',
            cyan: '#06989a',
            white: '#d3d7cf',
            brightBlack: '#555753',
            brightRed: '#ef2929',
            brightGreen: '#8ae234',
            brightYellow: '#fce94f',
            brightBlue: '#729fcf',
            brightMagenta: '#ad7fa8',
            brightCyan: '#34e2e2',
            brightWhite: '#eeeeec'
        },
        allowTransparency: false,
        scrollback: 10000,
        convertEol: true
    });

    // Open terminal in the container
    term.open(terminalContainer);
    term.focus();
    term.write('Connecting to terminal...\r\n');

    // Connect to WebSocket
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws`;
    console.log(`Connecting to WebSocket at ${wsUrl}`);
    
    const socket = new WebSocket(wsUrl);
    
    // Initialize the fit addon
    const fitAddon = new FitAddon.FitAddon();
    term.loadAddon(fitAddon);

    // Initialize the attach addon immediately
    const attachAddon = new AttachAddon.AttachAddon(socket);
    term.loadAddon(attachAddon);

    // Handle terminal resize - send size changes to server for PTY resizing
    term.onResize(size => {
        if (socket.readyState === WebSocket.OPEN) {
            const resizeMessage = JSON.stringify({
                type: 'resize',
                cols: size.cols,
                rows: size.rows
            });
            socket.send(resizeMessage);
        }
    });

    // Handle WebSocket events
    socket.onopen = () => {
        console.log('WebSocket connection established');
        term.clear();
        
        // Trigger initial fit and send size to server
        fitAddon.fit();
        const initialSize = JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
        });
        socket.send(initialSize);
    };

    socket.onclose = () => {
        console.log('WebSocket connection closed');
        term.write('\r\n\r\nConnection closed. Refresh to reconnect.\r\n');
    };

    socket.onerror = error => {
        console.error('WebSocket error:', error);
        term.write('\r\n\r\nWebSocket error. Check console for details.\r\n');
    };

    // Handle window resize - FitAddon will automatically resize terminal and trigger onResize
    window.addEventListener('resize', () => {
        fitAddon.fit();
    });
    
    // Use ResizeObserver for more reliable size detection
    if (window.ResizeObserver) {
        const resizeObserver = new ResizeObserver(entries => {
            for (let entry of entries) {
                if (entry.target === terminalContainer) {
                    fitAddon.fit();
                }
            }
        });
        
        // Start observing the terminal container
        resizeObserver.observe(terminalContainer);
    }

    // Handle copy/paste (AttachAddon handles input automatically)
    document.addEventListener('copy', event => {
        const selection = term.getSelection();
        if (selection) {
            event.clipboardData.setData('text/plain', selection);
            event.preventDefault();
        }
    });
});