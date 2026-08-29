"use client";

import { useActionState } from "react";
import Button from "../Button";
import { updateProgressAction } from "@/app/actions/reading";

// PDFs render in the browser's own native viewer (see the reader page),
// which exposes no JS API to read the current page back out — so progress
// here is reported by the reader themselves rather than tracked
// automatically the way the epub.js reader can.
export default function ProgressForm({ bookId, currentPage, totalPages }) {
  const [state, action, pending] = useActionState(updateProgressAction, undefined);

  return (
    <form action={action} className="flex flex-wrap items-end gap-3">
      <input type="hidden" name="book_id" value={bookId} />
      <div>
        <label htmlFor="current_page" className="block text-xs text-muted">
          Current page
        </label>
        <input
          id="current_page"
          name="current_page"
          type="number"
          min={0}
          defaultValue={currentPage ?? 0}
          className="mt-1 w-24 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        />
      </div>
      <div>
        <label htmlFor="total_pages" className="block text-xs text-muted">
          Total pages
        </label>
        <input
          id="total_pages"
          name="total_pages"
          type="number"
          min={1}
          defaultValue={totalPages ?? ""}
          required
          className="mt-1 w-24 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        />
      </div>
      <Button type="submit" variant="ghost" disabled={pending}>
        {pending ? "Saving…" : "Save progress"}
      </Button>
      {state?.error && <p className="text-xs text-alert">{state.error}</p>}
      {state?.success && <p className="text-xs text-muted">Saved.</p>}
    </form>
  );
}
