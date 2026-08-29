package com.bibliomania.desktop;

/**
 * Separate launcher class with a plain main() — running the fat/shaded jar
 * directly via `java -jar` on a class that extends Application fails with
 * "JavaFX runtime components are missing" because the JVM can't see it's a
 * JavaFX app until Application.launch() is already on the stack.
 */
public class Launcher {
    public static void main(String[] args) {
        App.main(args);
    }
}
