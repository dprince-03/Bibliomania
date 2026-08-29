import Container from "@/components/Container";
import BookCard from "@/components/BookCard";
import Pagination from "@/components/Pagination";
import SearchForm from "@/components/SearchForm";
import { apiGet, ApiError } from "@/lib/api";

function buildQuery(searchParams) {
  const params = new URLSearchParams();
  if (searchParams.page) params.set("page", searchParams.page);
  if (searchParams.q) params.set("q", searchParams.q);
  if (searchParams.genre) params.set("genre", searchParams.genre);
  if (searchParams.format) params.set("format", searchParams.format);
  if (searchParams.author) params.set("author", searchParams.author);
  if (searchParams.year) params.set("year", searchParams.year);
  return params;
}

function isSearching(searchParams) {
  return Boolean(
    searchParams.q ||
      searchParams.genre ||
      searchParams.format ||
      searchParams.author ||
      searchParams.year
  );
}

async function getCatalogPage(searchParams) {
  const query = buildQuery(searchParams);
  const path = isSearching(searchParams)
    ? `/api/v1/search?${query.toString()}`
    : `/api/v1/books?${query.toString()}`;

  try {
    return { data: await apiGet(path), error: null };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export default async function Home({ searchParams }) {
  const resolvedSearchParams = await searchParams;
  const { data, error } = await getCatalogPage(resolvedSearchParams);

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-10">
        <div className="text-center">
          <h1 className="font-serif text-4xl font-semibold text-foreground">
            The catalog
          </h1>
          <p className="mt-2 text-muted">
            Search by title, author, genre, or format.
          </p>
        </div>

        <SearchForm defaultValues={resolvedSearchParams} />

        {error && <p className="text-center text-sm text-alert">{error}</p>}

        {!error && data.items.length === 0 && (
          <p className="text-center text-sm text-muted">
            No books matched your search.
          </p>
        )}

        {!error && data.items.length > 0 && (
          <>
            <p className="text-sm text-muted">
              {data.total_count.toLocaleString()} book
              {data.total_count === 1 ? "" : "s"}
            </p>
            <div className="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
              {data.items.map((book) => (
                <BookCard key={book.id} book={book} />
              ))}
            </div>
            <Pagination
              basePath="/"
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
