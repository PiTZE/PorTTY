document.addEventListener('DOMContentLoaded', () => {
    console.log('DOM loaded, initializing terminal');
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

    // Handle WebSocket events
    socket.onopen = () => {
        console.log('WebSocket connection established');
        term.clear();

        // Handle terminal input
        term.onData(data => {
            if (socket.readyState === WebSocket.OPEN) {
                socket.send(data);
            }
        });

        // Handle terminal resize
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

        // Trigger initial resize
        const initialSize = JSON.stringify({
            type: 'resize',
            cols: term.cols,
            rows: term.rows
        });
        socket.send(initialSize);
    };

    socket.onmessage = event => {
        // Handle binary data
        if (typeof event.data === 'string') {
            term.write(event.data);
        } else {
            const reader = new FileReader();
            reader.onload = () => {
                term.write(new Uint8Array(reader.result));
            };
            reader.readAsArrayBuffer(event.data);
        }
    };

    socket.onclose = () => {
        console.log('WebSocket connection closed');
        term.write('\r\n\r\nConnection closed. Refresh to reconnect.\r\n');
    };

    socket.onerror = error => {
        console.error('WebSocket error:', error);
        term.write('\r\n\r\nWebSocket error. Check console for details.\r\n');
    };

    // Handle window resize
    window.addEventListener('resize', () => {
        fitTerminal();
    });
    
    // Use ResizeObserver for more reliable size detection
    if (window.ResizeObserver) {
        const resizeObserver = new ResizeObserver(entries => {
            for (let entry of entries) {
                if (entry.target === terminalContainer) {
                    fitTerminal();
                }
            }
        });
        
        // Start observing the terminal element
        resizeObserver.observe(terminalContainer);
    }

    // Function to fit terminal to window
    function fitTerminal() {
        // Calculate available space
        const availableWidth = terminalContainer.clientWidth;
        const availableHeight = terminalContainer.clientHeight;

        // Get character dimensions - fallback to reasonable defaults if not available
        let charWidth = 9;
        let charHeight = 17;
        
        try {
            if (term._core && term._core._renderService && term._core._renderService.dimensions) {
                charWidth = term._core._renderService.dimensions.actualCellWidth || charWidth;
                charHeight = term._core._renderService.dimensions.actualCellHeight || charHeight;
            }
        } catch (e) {
            console.warn('Could not get terminal dimensions, using defaults', e);
        }

        // Calculate new dimensions
        const cols = Math.max(10, Math.floor(availableWidth / charWidth));
        const rows = Math.max(5, Math.floor(availableHeight / charHeight));

        // Resize terminal
        if (cols > 0 && rows > 0) {
            console.log(`Resizing terminal to ${cols}x${rows}`);
            term.resize(cols, rows);

            // Send resize event to server
            if (socket.readyState === WebSocket.OPEN) {
                socket.send(JSON.stringify({
                    type: 'resize',
                    cols: cols,
                    rows: rows
                }));
            }
        }
    }

    // Initial fit after a short delay to ensure terminal is fully initialized
    setTimeout(fitTerminal, 100);

    // Handle copy/paste
    document.addEventListener('copy', event => {
        const selection = term.getSelection();
        if (selection) {
            event.clipboardData.setData('text/plain', selection);
            event.preventDefault();
        }
    });

    document.addEventListener('paste', event => {
        if (document.activeElement === terminalContainer || terminalContainer.contains(document.activeElement)) {
            const text = event.clipboardData.getData('text/plain');
            if (socket.readyState === WebSocket.OPEN) {
                socket.send(text);
            }
            event.preventDefault();
        }
    });
});