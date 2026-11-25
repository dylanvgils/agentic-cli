# Trust the proxy and msb CAs in Java, which ignores the PEM bundle vars
proxy_ca="${ {{- .ProxyCAEnv}}:-}"
msb_ca={{.MsbCA}}
java_trust_store={{.JavaTrustStore}}
java_store_password={{.JavaTrustStorePassword}}

import_ca() {
  keytool -importcert -noprompt -keystore "$java_trust_store" -storepass "$java_store_password" "$@" > /dev/null 2>&1
}

# set -e doesn't apply inside a function called from if, so each step returns on failure
build_java_trust_store() {
  local keytool
  keytool="$(readlink -f "$(command -v keytool)")" || return 1
  cp "${keytool%/bin/keytool}/lib/security/cacerts" "$java_trust_store" || return 1
  chmod u+w "$java_trust_store" || return 1
  if [[ -n "$proxy_ca" ]]; then
    printf '%s\n' "$proxy_ca" | import_ca -alias agentic-proxy || return 1
  fi
  if [[ -f "$msb_ca" ]]; then
    import_ca -alias agentic-msb -file "$msb_ca" || return 1
  fi
}

if [[ -n "$proxy_ca" || -f "$msb_ca" ]]; then
  if build_java_trust_store; then
    java_options="-Djavax.net.ssl.trustStore=$java_trust_store -Djavax.net.ssl.trustStorePassword=$java_store_password"
    export JDK_JAVA_OPTIONS="$java_options${JDK_JAVA_OPTIONS:+ $JDK_JAVA_OPTIONS}"
  else
    echo "warning: Java won't trust the proxy CA" >&2
  fi
fi
