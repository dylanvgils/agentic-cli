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
mounts = [
  "maven:$CONTAINER_HOME/.m2",
  "gradle:$CONTAINER_HOME/.gradle",
]
```

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

## Copilot without telemetry

Copilot CLI 1.0.100 and later skip their telemetry uploads with `--disable-client-telemetry`. Pass it on every run with [`[run.args]`](config.md#runargs):

```toml
# .agenticrc.toml
[run.args]
copilot = ["--disable-client-telemetry"]
```

Run `agentic update copilot` first if your image has an older Copilot.
