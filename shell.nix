{
  pkgs ? import <nixpkgs> { },
}:

pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    git
    tmux
    curl
  ];

  # Set environment variables
  shellHook = ''
    export GOPATH="$(pwd)/.go"
    mkdir -p "$GOPATH"
    export PATH="$GOPATH/bin:$PATH"
    export GO111MODULE=on
    echo "Go development environment ready!"
    echo "GOPATH set to: $GOPATH"
    echo "To build PorTTY: ./build.sh"
    echo "To run PorTTY: ./portty --port 8080"
  '';
}