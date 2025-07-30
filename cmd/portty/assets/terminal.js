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

    // Initialize terminal
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

    // Open terminal in the container
    term.open(terminalContainer);
    term.focus();

    // Connect to WebSocket
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const host = window.location.host;
    const wsUrl = `${protocol}//${host}/ws`;
    console.log(`Connecting to WebSocket at ${wsUrl}`);
    
    const socket = new WebSocket(wsUrl);
    
    // Load addons - let them handle everything automatically
    const fitAddon = new FitAddon.FitAddon();
    const attachAddon = new AttachAddon.AttachAddon(socket);
    
    term.loadAddon(fitAddon);
    term.loadAddon(attachAddon);
});