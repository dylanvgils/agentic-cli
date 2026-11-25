# Render the Maven proxy settings for the maven integration
if [[ -n "${ {{- .MavenPortEnv}}:-}" && -f {{.MavenSettingsTemplate}} ]]; then
  sed "s/{{.MavenPortPlaceholder}}/${ {{- .MavenPortEnv}}}/" {{.MavenSettingsTemplate}} > {{.MavenSettings}}
  export MAVEN_ARGS="--global-settings {{.MavenSettings}}${MAVEN_ARGS:+ $MAVEN_ARGS}"
fi
