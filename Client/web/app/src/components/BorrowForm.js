"use client";

import { useActionState } from "react";
import Button from "./Button";
import { borrowAction } from "@/app/actions/borrow";

export default function BorrowForm({ bookId }) {
  const [state, action, pending] = useActionState(borrowAction, undefined);

  return (
    <form action={action} className="mt-8">
      <input type="hidden" name="book_id" value={bookId} />
      {state?.error && (
        <p className="mb-3 text-sm text-alert">{state.error}</p>
      )}
      <Button type="submit" disabled={pending}>
        {pending ? "Borrowing…" : "Borrow this book"}
      </Button>
    </form>
  );
}
