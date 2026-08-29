import Container from "@/components/Container";
import AuthorCard from "@/components/AuthorCard";
import Pagination from "@/components/Pagination";
import { apiGet, ApiError } from "@/lib/api";

export const metadata = {
  title: "Authors — Bibliomania",
};

async function getAuthors(page) {
  const query = new URLSearchParams();
  if (page) query.set("page", page);

  try {
    return { data: await apiGet(`/api/v1/authors?${query.toString()}`), error: null };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export default async function AuthorsPage({ searchParams }) {
  const resolvedSearchParams = await searchParams;
  const { data, error } = await getAuthors(resolvedSearchParams.page);

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-10">
        <div className="text-center">
          <h1 className="font-serif text-4xl font-semibold text-foreground">
            Authors
          </h1>
        </div>

        {error && <p className="text-center text-sm text-alert">{error}</p>}

        {!error && data.items.length === 0 && (
          <p className="text-center text-sm text-muted">No authors yet.</p>
        )}

        {!error && data.items.length > 0 && (
          <>
            <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
              {data.items.map((author) => (
                <AuthorCard key={author.id} author={author} />
              ))}
            </div>
            <Pagination
              basePath="/authors"
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
