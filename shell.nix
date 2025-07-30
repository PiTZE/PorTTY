{ pkgs ? import <nixpkgs> {} }:

pkgs.mkShell {
  buildInputs = with pkgs; [
    # Core development tools
    go_1_24       # Go 1.24 (latest stable)
    git           # Version control
    tmux          # Required for terminal sessions
    
    # Build tools
    gnumake       # Make for build automation
    gcc           # C compiler (required for some Go dependencies)
    
    # Development utilities
    gopls         # Go language server for code intelligence
    go-tools      # Additional Go tools (goimports, etc.)
    delve         # Go debugger
    
    # Web development
    nodejs_20     # Node.js for web asset management (if needed)
    
    # Terminal utilities
    lsof          # For checking port usage
    curl          # For testing HTTP endpoints
    jq            # For JSON processing
  ];

  # Set environment variables
  shellHook = ''
    export GOPATH=$HOME/go
    export PATH=$GOPATH/bin:$PATH
    
    # Create GOPATH bin directory if it doesn't exist
    mkdir -p $GOPATH/bin
    
    echo "PorTTY development environment loaded!"
    echo "Go version: $(go version)"
    echo "Git version: $(git --version)"
    echo ""
    echo "To build PorTTY: go build -o portty ./cmd/portty"
    echo "To run PorTTY: ./portty --port 8080"
  '';
}