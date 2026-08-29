"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";
import { apiPost, apiPatch, ApiError } from "@/lib/api";

// Routed through lib/api.js (not a raw fetch like actions/auth.js) since
// these endpoints are auth-required and benefit from its Authorization
// header + transparent refresh-and-retry on an expired access token.
export async function borrowAction(prevState, formData) {
  const bookId = formData.get("book_id")?.toString();
  if (!bookId) {
    return { error: "Missing book." };
  }

  try {
    await apiPost("/api/v1/borrows", { book_id: Number(bookId) });
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not borrow this book. Please try again." };
  }

  revalidatePath(`/books/${bookId}`);
  revalidatePath("/borrows");
  redirect("/borrows");
}

export async function returnAction(prevState, formData) {
  const borrowId = formData.get("borrow_id")?.toString();
  if (!borrowId) {
    return { error: "Missing borrow record." };
  }

  try {
    await apiPatch(`/api/v1/borrows/${borrowId}/return`);
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not return this book. Please try again." };
  }

  revalidatePath("/borrows");
  return { success: true };
}
