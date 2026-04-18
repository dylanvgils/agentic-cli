ARG NODE_VERSION=24
FROM node:${NODE_VERSION}-bookworm-slim

LABEL project="agentic-cli"

# Suppress apt prompts during build
ENV DEBIAN_FRONTEND=noninteractive

# Version script
RUN printf '#!/bin/sh\nnode --version\n' > /usr/local/bin/agentic-version-node \
  && chmod +x /usr/local/bin/agentic-version-node

RUN apt-get update -yq \
  && apt-get install -yq --no-install-recommends \
  curl \
  wget \
  jq \
  git \
  gpg \
  ca-certificates \
  apt-transport-https \
  && rm -rf /var/lib/apt/lists/*
