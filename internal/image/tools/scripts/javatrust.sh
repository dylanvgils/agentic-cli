# Trust the proxy and msb CAs in Java, which ignores the PEM bundle vars
{{/* set -e doesn't apply inside a function called from if, so each step returns on failure. */ -}}
build_java_trust_store() {
  local keytool
  keytool="$(readlink -f "$(command -v keytool)")" || return 1
  cp "${keytool%/bin/keytool}/lib/security/cacerts" {{.JavaTrustStore}} || return 1
  chmod u+w {{.JavaTrustStore}} || return 1
  if [[ -n "${ {{- .ProxyCAEnv}}:-}" ]]; then
    printf '%s\n' "${{.ProxyCAEnv}}" | keytool -importcert -noprompt -keystore {{.JavaTrustStore}} -storepass {{.JavaTrustStorePassword}} -alias agentic-proxy > /dev/null 2>&1 || return 1
  fi
  if [[ -f {{.MsbCA}} ]]; then
    keytool -importcert -noprompt -keystore {{.JavaTrustStore}} -storepass {{.JavaTrustStorePassword}} -alias agentic-msb -file {{.MsbCA}} > /dev/null 2>&1 || return 1
  fi
}
if [[ -n "${ {{- .ProxyCAEnv}}:-}" || -f {{.MsbCA}} ]]; then
  if build_java_trust_store; then
    export JDK_JAVA_OPTIONS="-Djavax.net.ssl.trustStore={{.JavaTrustStore}} -Djavax.net.ssl.trustStorePassword={{.JavaTrustStorePassword}}${JDK_JAVA_OPTIONS:+ $JDK_JAVA_OPTIONS}"
  else
    echo "warning: Java won't trust the proxy CA" >&2
  fi
fi
