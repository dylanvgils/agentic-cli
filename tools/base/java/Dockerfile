ARG BASE_IMAGE=agentic-base:latest
FROM ${BASE_IMAGE}

ARG JAVA_VERSION=21

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN \
  # Add Adoptium GPG key and repository
  wget -qO - https://packages.adoptium.net/artifactory/api/gpg/key/public \
  | gpg --dearmor | tee /etc/apt/trusted.gpg.d/adoptium.gpg > /dev/null \
  && echo "deb https://packages.adoptium.net/artifactory/deb \
  $(awk -F= '/^VERSION_CODENAME/{print$2}' /etc/os-release) main" \
  | tee /etc/apt/sources.list.d/adoptium.list \
  \
  # Install Temurin JDK and cleanup
  && apt-get update -yq \
  && apt-get install -yq --no-install-recommends temurin-${JAVA_VERSION}-jdk \
  && rm -rf /var/lib/apt/lists/* \
  \
  # Version script
  && printf '#!/bin/sh\njava --version\n' > /usr/local/bin/agentic-version-java \
  && chmod +x /usr/local/bin/agentic-version-java
