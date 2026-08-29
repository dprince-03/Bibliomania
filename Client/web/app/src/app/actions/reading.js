"use server";

import { revalidatePath } from "next/cache";
import { apiPatch, apiPost, apiDelete, ApiError } from "@/lib/api";

// The plain, always-online PATCH /reading/{bookId}/progress — not the
// offline Sync variant (that needs a client_updated_at clock this app has
// no offline story to generate yet).
export async function updateProgressAction(prevState, formData) {
  const bookId = formData.get("book_id")?.toString();
  const currentPage = Number(formData.get("current_page"));
  const totalPages = Number(formData.get("total_pages"));

  if (!bookId || !Number.isFinite(currentPage) || !Number.isFinite(totalPages) || totalPages < 1) {
    return { error: "Enter a valid page and total." };
  }

  try {
    await apiPatch(`/api/v1/reading/${bookId}/progress`, {
      current_page: currentPage,
      total_pages: totalPages,
    });
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not save your progress. Please try again." };
  }

  revalidatePath(`/books/${bookId}/read`);
  revalidatePath("/history");
  return { success: true };
}

export async function createBookmarkAction(prevState, formData) {
  const bookId = formData.get("book_id")?.toString();
  const page = Number(formData.get("page"));
  const note = formData.get("note")?.toString().trim();
  const color = formData.get("color")?.toString() || "yellow";

  if (!bookId || !Number.isFinite(page) || page < 1) {
    return { error: "Enter a valid page number." };
  }

  try {
    await apiPost(`/api/v1/reading/${bookId}/bookmarks`, {
      page,
      note: note || undefined,
      color,
    });
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not add the bookmark. Please try again." };
  }

  revalidatePath(`/books/${bookId}/read`);
  return { success: true };
}

export async function deleteBookmarkAction(prevState, formData) {
  const bookId = formData.get("book_id")?.toString();
  const bookmarkId = formData.get("bookmark_id")?.toString();

  if (!bookId || !bookmarkId) {
    return { error: "Missing bookmark." };
  }

  try {
    await apiDelete(`/api/v1/reading/${bookId}/bookmarks/${bookmarkId}`);
  } catch (err) {
    if (err instanceof ApiError) {
      return { error: err.message };
    }
    return { error: "Could not delete the bookmark. Please try again." };
  }

  revalidatePath(`/books/${bookId}/read`);
  return { success: true };
}
