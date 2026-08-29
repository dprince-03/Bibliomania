import Link from "next/link";
import Container from "@/components/Container";
import Pagination from "@/components/Pagination";
import { apiGet, ApiError } from "@/lib/api";

function formatDate(value) {
  return new Date(value).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

async function getHistory(searchParams) {
  const query = new URLSearchParams();
  if (searchParams.page) query.set("page", searchParams.page);

  try {
    return {
      data: await apiGet(`/api/v1/users/me/history?${query.toString()}`),
      error: null,
    };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export default async function HistoryPage({ searchParams }) {
  const resolvedSearchParams = await searchParams;
  const { data, error } = await getHistory(resolvedSearchParams);

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-8">
        <h1 className="font-serif text-3xl font-semibold text-foreground sm:text-4xl">
          Reading history
        </h1>

        {error && <p className="text-sm text-alert">{error}</p>}

        {!error && data.items.length === 0 && (
          <p className="text-sm text-muted">
            You haven&apos;t started reading anything yet.
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
                  <div>
                    <Link
                      href={`/books/${entry.book_id}`}
                      className="font-serif text-lg text-foreground hover:text-accent"
                    >
                      {entry.book_title}
                    </Link>
                    <p className="mt-1 text-sm text-muted">
                      Page {entry.current_page} of {entry.total_pages} · Last
                      read {formatDate(entry.last_read_at)}
                    </p>
                  </div>

                  <div className="flex items-center gap-3">
                    <div className="h-2 w-32 overflow-hidden rounded-full bg-border/50">
                      <div
                        className="h-full bg-accent"
                        style={{ width: `${Math.min(100, entry.progress_pct)}%` }}
                      />
                    </div>
                    {entry.is_completed && (
                      <span className="rounded-full bg-link/10 px-3 py-1 text-xs font-medium text-link">
                        Completed
                      </span>
                    )}
                  </div>
                </div>
              ))}
            </div>

            <Pagination
              basePath="/history"
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
