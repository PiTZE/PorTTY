# PorTTY Common Tasks

This document contains step-by-step instructions for common maintenance and development tasks.

## Update Go Dependencies

**Last performed:** Not yet performed
**Files to modify:**
- `/go.mod` - Update dependency versions
- `/go.sum` - Updated automatically by go mod

**Steps:**
1. Check for outdated dependencies:
   ```bash
   go list -u -m all
   ```

2. Update specific dependency:
   ```bash
   go get -u github.com/gorilla/websocket
   go get -u github.com/creack/pty
   ```

3. Update all dependencies:
   ```bash
   go get -u ./...
   ```

4. Tidy and verify:
   ```bash
   go mod tidy
   go mod verify
   ```

5. Test the application:
   ```bash
   go test ./...
   ./build.sh
   ./portty run
   ```

**Important notes:**
- Always test thoroughly after updating dependencies
- Check changelogs for breaking changes
- Update frontend CDN versions in `index.html` if needed

## Update Frontend Dependencies (xterm.js)

**Last performed:** Not yet performed
**Files to modify:**
- `/cmd/portty/assets/index.html` - Update CDN URLs with new versions
- `/cmd/portty/assets/terminal.js` - Adjust for any API changes

**Steps:**
1. Check latest xterm.js version at https://github.com/xtermjs/xterm.js/releases

2. Update CDN URLs in index.html:
   ```html
   <!-- Update version numbers in these lines -->
   <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/css/xterm.css">
   <script src="https://cdn.jsdelivr.net/npm/@xterm/xterm@5.5.0/lib/xterm.js"></script>
   <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-fit@0.10.0/lib/addon-fit.js"></script>
   <script src="https://cdn.jsdelivr.net/npm/@xterm/addon-attach@0.11.0/lib/addon-attach.js"></script>
   ```

3. Test in browser:
   - Clear browser cache
   - Test all terminal functionality
   - Verify WebSocket connection
   - Test resize functionality

**Important notes:**
- Check xterm.js migration guides for breaking changes
- Addon versions must be compatible with core xterm.js version
- Test thoroughly in multiple browsers

## Create New Release

**Last performed:** v0.1
**Files to modify:**
- `/RELEASE_NOTES.md` - Add new release notes
- Git tags for version

**Steps:**
1. Update version in code if hardcoded anywhere

2. Create release notes:
   ```bash
   # Edit RELEASE_NOTES.md with new version section
   ```

3. Commit all changes:
   ```bash
   git add .
   git commit -m "Prepare for v0.2 release"
   ```

4. Create and push tag:
   ```bash
   git tag -a v0.2 -m "Release version 0.2"
   git push origin main
   git push origin v0.2
   ```

5. Build release artifacts:
   ```bash
   ./build.sh
   # This creates platform-specific archives
   ```

6. Create GitHub release:
   - Go to GitHub releases page
   - Create release from tag
   - Upload artifacts
   - Copy release notes

**Important notes:**
- Version in build.sh is automatically detected from git tags
- Test the binary on target platforms before release
- Update installation script if needed

## Debug WebSocket Connection Issues

**Last performed:** Not yet performed
**Files to check:**
- `/internal/websocket/websocket.go` - WebSocket handler
- `/cmd/portty/assets/terminal.js` - Client-side WebSocket
- Browser console for errors

**Steps:**
1. Enable verbose logging in websocket.go:
   - Add more log.Printf statements in HandleWS function
   - Log connection establishment and errors

2. Check browser console:
   - Open Developer Tools (F12)
   - Look for WebSocket errors in Console
   - Check Network tab for WS connection

3. Test with curl:
   ```bash
   # Test HTTP endpoint
   curl -I http://localhost:7314
   
   # Test WebSocket upgrade (should return 400)
   curl -I http://localhost:7314/ws
   ```

4. Check tmux session:
   ```bash
   tmux list-sessions
   tmux attach -t PorTTY
   ```

5. Common fixes:
   - Restart PorTTY service
   - Kill orphaned tmux sessions
   - Check firewall rules
   - Verify port availability

**Important notes:**
- WebSocket connections require proper upgrade headers
- Check for proxy/reverse proxy configuration issues
- Ensure tmux is installed and accessible

## Add New Terminal Theme

**Last performed:** Not yet performed
**Files to modify:**
- `/cmd/portty/assets/terminal.js` - Update theme configuration
- `/cmd/portty/assets/terminal.css` - Adjust CSS if needed

**Steps:**
1. Define theme colors in terminal.js:
   ```javascript
   theme: {
       background: '#1e1e1e',
       foreground: '#d4d4d4',
       cursor: '#ffffff',
       black: '#000000',
       red: '#cd3131',
       green: '#0dbc79',
       yellow: '#e5e510',
       blue: '#2472c8',
       magenta: '#bc3fbc',
       cyan: '#11a8cd',
       white: '#e5e5e5',
       // Add bright colors...
   }
   ```

2. Test color output:
   ```bash
   # In terminal, run color test
   for i in {0..255}; do
       printf "\x1b[38;5;${i}mcolour${i}\x1b[0m\n"
   done
   ```

3. Adjust CSS for consistency:
   - Update background colors in terminal.css
   - Ensure selection colors match theme

**Important notes:**
- Test with various terminal applications (vim, htop, etc.)
- Ensure sufficient contrast for readability
- Consider adding theme switching functionality

## Performance Profiling

**Last performed:** Not yet performed
**Files to analyze:**
- `/internal/websocket/websocket.go` - Connection handling
- `/internal/ptybridge/ptybridge.go` - PTY operations

**Steps:**
1. Add CPU profiling:
   ```go
   import _ "net/http/pprof"
   // In main.go, the pprof server is already included with default mux
   ```

2. Run with profiling:
   ```bash
   ./portty run &
   go tool pprof http://localhost:7314/debug/pprof/profile
   ```

3. Analyze memory usage:
   ```bash
   go tool pprof http://localhost:7314/debug/pprof/heap
   ```

4. Check goroutine leaks:
   ```bash
   curl http://localhost:7314/debug/pprof/goroutine?debug=1
   ```

5. Benchmark WebSocket throughput:
   - Send large amounts of data
   - Monitor CPU and memory usage
   - Check for message drops

**Important notes:**
- Remove pprof imports in production builds
- Profile under realistic load conditions
- Focus on hot paths identified by profiler