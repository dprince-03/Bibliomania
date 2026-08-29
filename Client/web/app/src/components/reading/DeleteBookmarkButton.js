"use client";

import { useActionState } from "react";
import { deleteBookmarkAction } from "@/app/actions/reading";

export default function DeleteBookmarkButton({ bookId, bookmarkId, page }) {
  const [state, action, pending] = useActionState(deleteBookmarkAction, undefined);

  return (
    <form action={action}>
      <input type="hidden" name="book_id" value={bookId} />
      <input type="hidden" name="bookmark_id" value={bookmarkId} />
      <button
        type="submit"
        disabled={pending}
        aria-label={`Remove bookmark for page ${page}`}
        className="text-xs text-muted hover:text-alert"
      >
        {pending ? "Removing…" : "Remove"}
      </button>
      {state?.error && <p className="text-xs text-alert">{state.error}</p>}
    </form>
  );
}
