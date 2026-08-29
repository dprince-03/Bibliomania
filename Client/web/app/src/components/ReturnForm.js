"use client";

import { useActionState } from "react";
import Button from "./Button";
import { returnAction } from "@/app/actions/borrow";

export default function ReturnForm({ borrowId }) {
  const [state, action, pending] = useActionState(returnAction, undefined);

  return (
    <form action={action} className="flex flex-col items-end gap-2">
      <input type="hidden" name="borrow_id" value={borrowId} />
      <Button type="submit" variant="ghost" disabled={pending}>
        {pending ? "Returning…" : "Return"}
      </Button>
      {state?.error && (
        <p className="text-xs text-alert">{state.error}</p>
      )}
    </form>
  );
}
