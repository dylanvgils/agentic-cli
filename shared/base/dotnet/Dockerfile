ARG BASE_IMAGE=agentic-base:latest
FROM ${BASE_IMAGE}

ARG DOTNET_VERSION=10

RUN \
  # Add dotnet repository
  wget "https://packages.microsoft.com/config/debian/$(. /etc/os-release && echo ${VERSION_ID})/packages-microsoft-prod.deb" \
  && dpkg -i packages-microsoft-prod.deb \
  && rm packages-microsoft-prod.deb \
  # Set version
  && case "${DOTNET_VERSION}" in \
  *.*) ;; \
  *) DOTNET_VERSION="${DOTNET_VERSION}.0" ;; \
  esac \
  # Install dotnet
  && apt-get update -yq \
  && apt-get install -yq --no-install-recommends dotnet-sdk-${DOTNET_VERSION} \
  && rm -rf /var/lib/apt/lists/*
