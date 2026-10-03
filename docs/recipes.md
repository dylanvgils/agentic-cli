# Recipes

Worked setups for common cases.

## Java build tools

Maven and Gradle are **not** included in the Java base image. Instead, use the wrappers that come with your project (`mvnw` / `gradlew`). Wrappers are committed to the repo and download the exact build tool version the project requires on first run - this avoids version mismatches and keeps the image lean.

To generate a wrapper if your project doesn't have one yet:

```bash
# Maven
mvn wrapper:wrapper

# Gradle
gradle wrapper
```

Use named volumes to persist the download cache across container runs:

```bash
agentic run -v 'maven:$CONTAINER_HOME/.m2' -v 'gradle:$CONTAINER_HOME/.gradle' claude
```

Or add to `.agenticrc.toml` in the repo root so the whole team picks it up:

```toml
# .agenticrc.toml
[run]
extra_mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
```

## Maven through the egress proxy

With `--proxy` on, Maven needs its own proxy setting pointed at the `agentic-proxy:3128` sidecar (background: [Pointing a tool's own proxy setting at the egress proxy](config.md#pointing-a-tools-own-proxy-setting-at-the-egress-proxy)).

Maven only reads proxy settings from `settings.xml`'s `<proxies>` section, not `MAVEN_OPTS` or the standard proxy env vars. Mount a `settings.xml` pointing at `agentic-proxy:3128`, with a `<proxy>` entry per URL scheme - Maven matches `<protocol>` against the repository URL (not the connection to the proxy itself), and most registries including Maven Central serve over `https`:

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

Pointing `<proxies>` at an _external_ corporate proxy instead would bypass agentic's egress allowlist entirely, since that traffic never reaches the `agentic-proxy` sidecar - only routing through `agentic-proxy` keeps Maven's traffic subject to `allowed_hosts`.

## Per-project image set

Building separate images for a Java project:

```toml
# ~/projects/java-app/.agenticrc.toml
namespace = "java-app"

[build]
bases = ["java"]
apt_packages = ["make"]

[build.versions]
java = "17"
```

Then `agentic build claude` creates `java-app-claude` with the Java layer, while the default `agentic-claude` remains untouched.

## Devcontainers

To test devcontainers, add Node.js and the devcontainer CLI, then run `devcontainer up --workspace-folder /workspace` from inside the tool:

```toml
[build]
bases = ["node", "docker"]

[[build.custom_installs]]
name = "devcontainer-cli"
run = ["npm install -g --prefix /usr/local @devcontainers/cli"]

[run.dind]
enabled = true
```
