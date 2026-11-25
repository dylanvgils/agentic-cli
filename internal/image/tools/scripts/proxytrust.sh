# Trust the egress proxy's CA for credential hosts
proxy_ca="${ {{- .ProxyCAEnv}}:-}"
system_bundle={{.SystemCABundle}}
ca_bundle={{.ProxyCABundle}}
trust_vars=({{join .TrustEnvNames " "}})

if [[ -n "$proxy_ca" ]]; then
  { cat "$system_bundle"; printf '\n%s\n' "$proxy_ca"; } > "$ca_bundle"
  for name in "${trust_vars[@]}"; do
    export "$name=$ca_bundle"
  done
  unset {{.ProxyCAEnv}}
fi
