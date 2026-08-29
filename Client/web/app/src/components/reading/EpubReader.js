"use client";

import { useEffect, useRef, useState } from "react";
import Button from "../Button";
import { updateProgressAction } from "@/app/actions/reading";

// epub.js paginates virtually — there's no fixed "page 12 of 340" the way a
// PDF has. The Go API's ReadingSession only has current_page/total_pages,
// so this reader maps epub.js's 0-1 location percentage onto that as
// current_page out of a fixed total_pages of 100 (i.e. current_page IS the
// percentage). PDFs instead report real page numbers — see ProgressForm.
const READING_THEME_STYLES = {
  light: { body: { background: "#ffffff", color: "#0d1b3e" } },
  sepia: { body: { background: "#f4ecd8", color: "#3b2f1e" } },
  dark: { body: { background: "#0a1633", color: "#f2ede1" } },
};

export default function EpubReader({ bookId, fileUrl, initialPercentage = 0 }) {
  const viewerRef = useRef(null);
  const renditionRef = useRef(null);
  const [percentage, setPercentage] = useState(initialPercentage);
  const [locationsReady, setLocationsReady] = useState(false);
  const [loadError, setLoadError] = useState(null);
  const [saveState, setSaveState] = useState(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    let cancelled = false;
    let book;

    import("epubjs").then(({ default: ePub }) => {
      if (cancelled || !viewerRef.current) return;

      // openAs: "epub" forces epub.js to fetch fileUrl as a single binary
      // archive to unzip client-side — without it, epub.js sniffs the type
      // from the URL's file extension, and our proxy URL (no .epub suffix)
      // gets misread as an already-unpacked directory tree, 404ing on
      // META-INF/container.xml relative to it.
      book = ePub(fileUrl, { openAs: "epub" });
      const rendition = book.renderTo(viewerRef.current, {
        width: "100%",
        height: "100%",
        flow: "paginated",
      });
      renditionRef.current = rendition;

      Object.entries(READING_THEME_STYLES).forEach(([name, styles]) => {
        rendition.themes.register(name, styles);
      });
      rendition.themes.select(document.documentElement.dataset.readingTheme || "light");

      rendition.display().catch((err) => {
        if (!cancelled) setLoadError(err?.message || "Could not load this book.");
      });

      rendition.on("relocated", (location) => {
        if (location?.start?.percentage != null) {
          setPercentage(location.start.percentage);
        }
      });

      book.ready
        .then(() => book.locations.generate(1600))
        .then(() => {
          if (cancelled) return;
          setLocationsReady(true);
          if (initialPercentage > 0) {
            rendition.display(book.locations.cfiFromPercentage(initialPercentage));
          }
        })
        .catch((err) => {
          if (!cancelled) setLoadError(err?.message || "Could not load this book.");
        });
    });

    function handleThemeChange(event) {
      renditionRef.current?.themes.select(event.detail);
    }
    window.addEventListener("readingthemechange", handleThemeChange);

    return () => {
      cancelled = true;
      window.removeEventListener("readingthemechange", handleThemeChange);
      book?.destroy();
    };
    // initialPercentage is only meant to apply once, at mount — re-running
    // this effect on every relocation would fight the reader's own navigation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fileUrl]);

  async function handleSaveProgress() {
    setSaving(true);
    const formData = new FormData();
    formData.set("book_id", bookId);
    formData.set("current_page", String(Math.round(percentage * 100)));
    formData.set("total_pages", "100");
    const result = await updateProgressAction(undefined, formData);
    setSaveState(result);
    setSaving(false);
  }

  return (
    <div className="flex flex-col gap-3">
      {loadError && <p className="text-sm text-alert">{loadError}</p>}
      <div
        ref={viewerRef}
        className="h-[70vh] w-full overflow-hidden rounded-2xl border border-border bg-background"
      />
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Button type="button" variant="ghost" onClick={() => renditionRef.current?.prev()}>
          ← Prev
        </Button>
        <p className="text-sm text-muted">
          {locationsReady ? `${Math.round(percentage * 100)}% through` : "Loading…"}
        </p>
        <Button type="button" variant="ghost" onClick={() => renditionRef.current?.next()}>
          Next →
        </Button>
      </div>
      <div className="flex items-center gap-3">
        <Button type="button" variant="ghost" disabled={saving} onClick={handleSaveProgress}>
          {saving ? "Saving…" : "Save progress"}
        </Button>
        {saveState?.error && <p className="text-xs text-alert">{saveState.error}</p>}
        {saveState?.success && <p className="text-xs text-muted">Saved.</p>}
      </div>
    </div>
  );
}
