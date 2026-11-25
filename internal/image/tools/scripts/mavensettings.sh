# Render the Maven proxy settings for the maven integration
maven_port="${ {{- .MavenPortEnv}}:-}"
maven_port_placeholder={{.MavenPortPlaceholder}}
maven_template={{.MavenSettingsTemplate}}
maven_settings={{.MavenSettings}}

if [[ -n "$maven_port" && -f "$maven_template" ]]; then
  sed "s/$maven_port_placeholder/$maven_port/" "$maven_template" > "$maven_settings"
  export MAVEN_ARGS="--global-settings $maven_settings${MAVEN_ARGS:+ $MAVEN_ARGS}"
fi
