import Link from "next/link";
import Container from "@/components/Container";
import Pagination from "@/components/Pagination";
import ReturnForm from "@/components/ReturnForm";
import { apiGet, ApiError } from "@/lib/api";

export const metadata = {
  title: "My borrows — Bibliomania",
};

const statusStyles = {
  active: "bg-link/10 text-link",
  overdue: "bg-alert/10 text-alert",
  returned: "bg-muted/10 text-muted",
};

function formatDate(value) {
  return new Date(value).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

async function getMyBorrows(searchParams) {
  const query = new URLSearchParams();
  if (searchParams.page) query.set("page", searchParams.page);

  try {
    return { data: await apiGet(`/api/v1/borrows/my?${query.toString()}`), error: null };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export default async function MyBorrowsPage({ searchParams }) {
  const resolvedSearchParams = await searchParams;
  const { data, error } = await getMyBorrows(resolvedSearchParams);

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-8">
        <h1 className="font-serif text-3xl font-semibold text-foreground sm:text-4xl">
          My borrows
        </h1>

        {error && <p className="text-sm text-alert">{error}</p>}

        {!error && data.items.length === 0 && (
          <p className="text-sm text-muted">
            You haven&apos;t borrowed any books yet.{" "}
            <Link href="/" className="text-link hover:underline">
              Browse the catalog
            </Link>
            .
          </p>
        )}

        {!error && data.items.length > 0 && (
          <>
            <div className="flex flex-col divide-y divide-border rounded-2xl border border-border bg-surface">
              {data.items.map((borrow) => (
                <div
                  key={borrow.id}
                  className="flex flex-wrap items-center justify-between gap-4 p-5"
                >
                  <div>
                    <Link
                      href={`/books/${borrow.book_id}`}
                      className="font-serif text-lg text-foreground hover:text-accent"
                    >
                      {borrow.book_title}
                    </Link>
                    <p className="mt-1 text-sm text-muted">
                      Borrowed {formatDate(borrow.borrowed_at)} · Due{" "}
                      {formatDate(borrow.due_at)}
                      {borrow.returned_at &&
                        ` · Returned ${formatDate(borrow.returned_at)}`}
                    </p>
                  </div>

                  <div className="flex items-center gap-4">
                    <span
                      className={`rounded-full px-3 py-1 text-xs font-medium capitalize ${
                        statusStyles[borrow.status] ?? "bg-muted/10 text-muted"
                      }`}
                    >
                      {borrow.status}
                    </span>
                    {borrow.status !== "returned" && (
                      <ReturnForm borrowId={borrow.id} />
                    )}
                  </div>
                </div>
              ))}
            </div>

            <Pagination
              basePath="/borrows"
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
