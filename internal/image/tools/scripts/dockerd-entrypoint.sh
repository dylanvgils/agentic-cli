#!/usr/bin/env bash
set -euo pipefail

entrypoint={{.Entrypoint}}
host_alias={{.HostAlias}}
config_dir={{.DockerdConfigDir}}

# Only a run through the proxy sets HTTPS_PROXY
[[ -n "${HTTPS_PROXY:-}" ]] || exec "$entrypoint" "$@"

# Inner containers resolve names through dockerd, not this VM's /etc/hosts, so they need the IP
host_ip="$(getent ahostsv4 "$host_alias" | awk 'NR == 1 { print $1 }' || true)"
if [[ -z "$host_ip" ]]; then
  echo "warning: can't resolve $host_alias, so inner containers get no proxy" >&2
  exec "$entrypoint" "$@"
fi

# Point inner containers and builds at the proxy
proxy="${HTTPS_PROXY/@$host_alias:/@$host_ip:}"
install -d -m 0700 "$config_dir"
jq -n --arg proxy "$proxy" --arg no_proxy "${NO_PROXY:-}" \
  '{proxies: {default: {httpProxy: $proxy, httpsProxy: $proxy, noProxy: $no_proxy}}}' \
  > "$config_dir/config.json"
export DOCKER_CONFIG="$config_dir"

exec "$entrypoint" "$@"
