const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://api.bibliotheca.local";

/**
 * Fetches a paginated list endpoint and returns its total_count, or null
 * if the API is unreachable / the response doesn't match the expected
 * envelope shape. Never throws — callers render a placeholder on null.
 */
export async function getTotalCount(path) {
  try {
    const res = await fetch(`${API_URL}${path}`, { cache: "no-store" });
    if (!res.ok) return null;

    const body = await res.json();
    const totalCount = body?.data?.total_count;
    return typeof totalCount === "number" ? totalCount : null;
  } catch {
    return null;
  }
}
