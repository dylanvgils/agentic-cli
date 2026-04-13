ARG NODE_VERSION=24
FROM node:${NODE_VERSION}-bookworm-slim

LABEL project="agentic-cli"

# Suppress apt prompts during build
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update -yq \
  && apt-get install -yq --no-install-recommends \
  curl \
  wget \
  git \
  gpg \
  ca-certificates \
  apt-transport-https \
  && rm -rf /var/lib/apt/lists/*
