# Bibliomania Desktop

Native desktop client (JavaFX), talking to the same Go API as the other clients.

**Status:** bare scaffold — a single window with a placeholder label, hand-written rather than generated via `mvn archetype:generate` because Maven isn't installed in the environment this was scaffolded in. Structure matches the standard `javafx-archetype-simple` output, so it should build as-is once Maven is available.

## Requirements

- JDK 21+ (targets Java 21 LTS via `maven.compiler.release`)
- Maven 3.9+

## Run

```bash
mvn javafx:run
```

## Package

```bash
mvn package
java -jar target/desktop-0.1.0-SNAPSHOT.jar   # via the Launcher entry point (see Launcher.java)
```

## Gitignore

Add a standard Maven `.gitignore` (`target/`) before committing further work here — not included yet since no build has run locally to generate one from.
