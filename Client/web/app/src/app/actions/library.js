"use server";

import { revalidatePath } from "next/cache";
import { apiPatch, ApiError } from "@/lib/api";

// PATCH /users/me/library/{bookId} is an upsert (Server/app's own doc
// comment on UpdateLibraryStatusRequest) — there's no separate "add to
// library" endpoint, so this same action both adds a book to the shelf and
// changes its status.
export async function updateLibraryStatusAction(prevState, formData) {
  const bookId = formData.get("book_id")?.toString();
  const status = formData.get("status")?.toString();
  if (!bookId || !status) {
    return { error: "Missing book or status." };
  }

  try {
    await apiPatch(`/api/v1/users/me/library/${bookId}`, { status });
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not update your library. Please try again." };
  }

  revalidatePath(`/books/${bookId}`);
  revalidatePath("/library");
  return { success: true };
}
