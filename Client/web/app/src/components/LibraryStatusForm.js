"use client";

import { useActionState } from "react";
import Button from "./Button";
import { updateLibraryStatusAction } from "@/app/actions/library";

// Matches UpdateLibraryStatusRequest's validate:"oneof=..." in
// Server/app/internal/modules/user/dto.go exactly.
const statusOptions = [
  { value: "wishlist", label: "Wishlist" },
  { value: "to_read", label: "To read" },
  { value: "reading", label: "Reading" },
  { value: "completed", label: "Completed" },
  { value: "dropped", label: "Dropped" },
];

export default function LibraryStatusForm({ bookId, defaultStatus, label = "Update" }) {
  const [state, action, pending] = useActionState(updateLibraryStatusAction, undefined);

  return (
    <form action={action} className="flex flex-wrap items-center gap-3">
      <input type="hidden" name="book_id" value={bookId} />
      <select
        name="status"
        aria-label="Library status"
        defaultValue={defaultStatus ?? ""}
        required
        className="rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
      >
        {!defaultStatus && (
          <option value="" disabled>
            Add to my library…
          </option>
        )}
        {statusOptions.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
      <Button type="submit" variant="ghost" disabled={pending}>
        {pending ? "Saving…" : label}
      </Button>
      {state?.error && <p className="text-xs text-alert">{state.error}</p>}
      {state?.success && <p className="text-xs text-muted">Saved.</p>}
    </form>
  );
}
