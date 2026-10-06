# Recipes

Worked setups for common cases.

## Java build tools

Maven and Gradle aren't in the Java image. Use the project's wrappers (`mvnw` / `gradlew`), which download the exact version the project needs.

To add a wrapper:

```bash
# Maven
mvn wrapper:wrapper

# Gradle
gradle wrapper
```

Keep the download cache in named volumes:

```bash
agentic run -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude
```

Or share it with the team through `.agenticrc.toml`:

```toml
# .agenticrc.toml
[run]
extra_mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
```

## Maven through the egress proxy

Maven ignores the proxy env vars, so with `--proxy` on, mount a `settings.xml` that points at `agentic-proxy:3128` ([why](egress-proxy.md#tools-that-ignore-proxy-env-vars)). Maven matches `<protocol>` against the repository URL, so add a `<proxy>` for each scheme:

```xml
<!-- settings.xml -->
<settings>
  <proxies>
    <proxy>
      <id>agentic-proxy-http</id>
      <active>true</active>
      <protocol>http</protocol>
      <host>agentic-proxy</host>
      <port>3128</port>
    </proxy>
    <proxy>
      <id>agentic-proxy-https</id>
      <active>true</active>
      <protocol>https</protocol>
      <host>agentic-proxy</host>
      <port>3128</port>
    </proxy>
  </proxies>
</settings>
```

```toml
# .agenticrc.toml
[run]
secrets = ["maven-settings:~/.m2/settings.xml:$CONTAINER_HOME/.m2/settings.xml"]

[run.proxy]
enabled = true
allowed_hosts = ["repo.maven.apache.org"]
```

Don't point `<proxies>` at an external corporate proxy, because that traffic would skip the allowlist.

## Per-project image set

Separate images for a Java project:

```toml
# ~/projects/java-app/.agenticrc.toml
namespace = "java-app"

[build]
bases = ["java"]
apt_packages = ["make"]

[build.versions]
java = "17"
```

`agentic build claude` then creates `java-app-claude` and leaves `agentic-claude` untouched.

## Devcontainers

Add Node.js and the devcontainer CLI, enable [DinD](docker-in-docker.md), then run `devcontainer up --workspace-folder /workspace` from inside the tool:

```toml
[build]
bases = ["node", "docker"]

[[build.custom_installs]]
name = "devcontainer-cli"
run = ["npm install -g --prefix /usr/local @devcontainers/cli"]

[run.dind]
enabled = true
```
