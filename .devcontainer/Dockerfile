FROM mcr.microsoft.com/devcontainers/go:2-1.26-trixie

USER vscode

RUN curl -fsSL https://opencode.ai/install | bash \
 && curl -fsSL https://claude.ai/install.sh | bash \
 && curl -fsSL https://bun.com/install | bash

USER root

RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install ripgrep tmux \
    && rm -rf /var/lib/apt/lists/* \
    && /home/vscode/.bun/bin/bunx --bun playwright@1.61.0-alpha-1781023400000 install-deps chromium

USER vscode

RUN /home/vscode/.bun/bin/bunx --bun playwright@1.61.0-alpha-1781023400000 install chromium
