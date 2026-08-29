"use client";

import { useEffect, useSyncExternalStore } from "react";

const STORAGE_KEY = "bibliomania:reading-theme";
const themes = [
  { value: "light", label: "Light" },
  { value: "sepia", label: "Sepia" },
  { value: "dark", label: "Dark" },
];

function subscribe(callback) {
  window.addEventListener("storage", callback);
  return () => window.removeEventListener("storage", callback);
}

function getSnapshot() {
  try {
    return window.localStorage.getItem(STORAGE_KEY) || "light";
  } catch {
    return "light";
  }
}

function getServerSnapshot() {
  return "light";
}

function setStoredTheme(next) {
  try {
    window.localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // Private browsing / storage blocked — the DOM attribute below still
    // applies for the rest of this session, it just won't persist.
  }
  // The native "storage" event only fires in *other* tabs, not the one that
  // made the write — dispatch one here so useSyncExternalStore re-reads the
  // snapshot and this tab's buttons update immediately too.
  window.dispatchEvent(new Event("storage"));
}

// Sets data-reading-theme on <html> only while this component is mounted
// (i.e. only on the reader page — see globals.css), and clears it on
// unmount so leaving the reader doesn't leak the choice onto the rest of
// the app. Dispatches a window event so EpubReader (a sibling client
// component, not a descendant) can recolor the book text itself to match,
// without needing a shared React context for one cross-component signal.
export default function ReadingThemeToggle() {
  const theme = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);

  useEffect(() => {
    document.documentElement.dataset.readingTheme = theme;
    window.dispatchEvent(new CustomEvent("readingthemechange", { detail: theme }));
    return () => {
      delete document.documentElement.dataset.readingTheme;
    };
  }, [theme]);

  return (
    <fieldset className="m-0 flex min-w-0 items-center gap-1 rounded-full border border-border p-1">
      <legend className="sr-only">Reading theme</legend>
      {themes.map((t) => (
        <button
          key={t.value}
          type="button"
          onClick={() => setStoredTheme(t.value)}
          aria-pressed={theme === t.value}
          className={`rounded-full px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent ${
            theme === t.value
              ? "bg-accent text-accent-foreground"
              : "text-muted hover:text-accent"
          }`}
        >
          {t.label}
        </button>
      ))}
    </fieldset>
  );
}
