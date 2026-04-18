# Runtime image for GitHub Copilot CLI agents.
# Extends agentic-base-java by default; remaps the node user to match host UID/GID for bind mount compatibility.
ARG BASE_IMAGE=agentic-base:latest
FROM ${BASE_IMAGE}
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG HOST_UID=1000
ARG HOST_GID=1000

ARG TOOL_USER=opencode
ARG TOOL_HOME=/home/${TOOL_USER}

RUN \
  # Remove any existing user/group at HOST_UID/HOST_GID to avoid conflicts
  # with base image users (e.g. the node user at uid 1000 in Node base images).
  existing=$(getent passwd ${HOST_UID} | cut -d: -f1); \
  if [ -n "$existing" ] && [ "$existing" != "${TOOL_USER}" ]; then \
  userdel -r "$existing" 2>/dev/null || true; \
  fi \
  # Add the container user.
  && groupadd -g ${HOST_GID} --non-unique ${TOOL_USER} \
  && useradd -l -u ${HOST_UID} -g ${HOST_GID} -m -s /bin/bash --non-unique ${TOOL_USER}

COPY ./entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

USER ${TOOL_USER}
ENV PATH="${TOOL_HOME}/.opencode/bin:${PATH}"
RUN curl -fsSL https://opencode.ai/install | bash -s -- --no-modify-path \
  && mkdir -p ${TOOL_HOME}/.config/opencode
ENV TOOL_HOME=${TOOL_HOME}

WORKDIR /workspace
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
