import Link from "next/link";
import Container from "@/components/Container";
import Pagination from "@/components/Pagination";
import LibraryStatusForm from "@/components/LibraryStatusForm";
import { apiGet, ApiError } from "@/lib/api";

export const metadata = {
  title: "My library — Bibliotheca",
};

const filters = [
  { value: "", label: "All" },
  { value: "wishlist", label: "Wishlist" },
  { value: "to_read", label: "To read" },
  { value: "reading", label: "Reading" },
  { value: "completed", label: "Completed" },
  { value: "dropped", label: "Dropped" },
];

async function getLibrary(searchParams) {
  const query = new URLSearchParams();
  if (searchParams.page) query.set("page", searchParams.page);
  if (searchParams.status) query.set("status", searchParams.status);

  try {
    return {
      data: await apiGet(`/api/v1/users/me/library?${query.toString()}`),
      error: null,
    };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export default async function LibraryPage({ searchParams }) {
  const resolvedSearchParams = await searchParams;
  const { data, error } = await getLibrary(resolvedSearchParams);
  const activeStatus = resolvedSearchParams.status || "";

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-8">
        <h1 className="font-serif text-3xl font-semibold text-foreground sm:text-4xl">
          My library
        </h1>

        <div className="flex flex-wrap gap-2">
          {filters.map((f) => (
            <Link
              key={f.value}
              href={f.value ? `/library?status=${f.value}` : "/library"}
              className={`rounded-full border px-4 py-1.5 text-sm transition-colors ${
                activeStatus === f.value
                  ? "border-accent bg-accent/10 text-accent"
                  : "border-border text-muted hover:text-accent"
              }`}
            >
              {f.label}
            </Link>
          ))}
        </div>

        {error && <p className="text-sm text-alert">{error}</p>}

        {!error && data.items.length === 0 && (
          <p className="text-sm text-muted">
            Nothing here yet.{" "}
            <Link href="/" className="text-link hover:underline">
              Browse the catalog
            </Link>{" "}
            and add a book to your shelf.
          </p>
        )}

        {!error && data.items.length > 0 && (
          <>
            <div className="flex flex-col divide-y divide-border rounded-2xl border border-border bg-surface">
              {data.items.map((entry) => (
                <div
                  key={entry.book_id}
                  className="flex flex-wrap items-center justify-between gap-4 p-5"
                >
                  <Link
                    href={`/books/${entry.book_id}`}
                    className="font-serif text-lg text-foreground hover:text-accent"
                  >
                    {entry.book_title}
                  </Link>
                  <LibraryStatusForm
                    bookId={entry.book_id}
                    defaultStatus={entry.status}
                  />
                </div>
              ))}
            </div>

            <Pagination
              basePath="/library"
              searchParams={resolvedSearchParams}
              page={data.page}
              totalPages={data.total_pages}
              hasNextPage={data.has_next_page}
              hasPreviousPage={data.has_previous_page}
            />
          </>
        )}
      </Container>
    </section>
  );
}
