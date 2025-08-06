{ pkgs ? import <nixpkgs> {} }:
pkgs.mkShell {
  buildInputs = with pkgs; [
    go
    gopls
    go-tools
    delve

    git
    tmux
    curl
    jq
  ];

  shellHook = ''
    export GOPATH="$HOME/go"
    export GOPROXY="https://proxy.golang.org,direct"
    export GOSUMDB="sum.golang.org"
    export PATH="$GOPATH/bin:$PATH"
  '';

  GO111MODULE = "on";
}
