# Trust the egress proxy's CA for credential hosts
if [[ -n "${ {{- .ProxyCAEnv}}:-}" ]]; then
  { cat {{.SystemCABundle}}; printf '\n%s\n' "${{.ProxyCAEnv}}"; } > {{.ProxyCABundle}}
{{- range .TrustEnvNames}}
  export {{.}}={{$.ProxyCABundle}}
{{- end}}
  unset {{.ProxyCAEnv}}
fi
