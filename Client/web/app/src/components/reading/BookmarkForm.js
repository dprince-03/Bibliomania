"use client";

import { useActionState } from "react";
import Button from "../Button";
import { createBookmarkAction } from "@/app/actions/reading";

const colors = ["yellow", "green", "blue", "pink", "purple"];

export default function BookmarkForm({ bookId }) {
  const [state, action, pending] = useActionState(createBookmarkAction, undefined);

  return (
    <form action={action} className="flex flex-wrap items-end gap-3">
      <input type="hidden" name="book_id" value={bookId} />
      <div>
        <label htmlFor="page" className="block text-xs text-muted">
          Page
        </label>
        <input
          id="page"
          name="page"
          type="number"
          min={1}
          required
          className="mt-1 w-20 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        />
      </div>
      <div className="flex-1">
        <label htmlFor="note" className="block text-xs text-muted">
          Note (optional)
        </label>
        <input
          id="note"
          name="note"
          maxLength={500}
          className="mt-1 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        />
      </div>
      <div>
        <label htmlFor="color" className="block text-xs text-muted">
          Color
        </label>
        <select
          id="color"
          name="color"
          defaultValue="yellow"
          className="mt-1 rounded-lg border border-border bg-background px-2 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
        >
          {colors.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
      </div>
      <Button type="submit" variant="ghost" disabled={pending}>
        {pending ? "Adding…" : "Add bookmark"}
      </Button>
      {state?.error && <p className="text-xs text-alert">{state.error}</p>}
    </form>
  );
}
