import { notFound } from "next/navigation";
import Container from "@/components/Container";
import BookCard from "@/components/BookCard";
import Pagination from "@/components/Pagination";
import { apiGet, ApiError } from "@/lib/api";

async function getAuthor(id) {
  try {
    return await apiGet(`/api/v1/authors/${id}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }
}

async function getAuthorBooks(id, page) {
  const query = new URLSearchParams();
  if (page) query.set("page", page);

  try {
    return {
      data: await apiGet(`/api/v1/authors/${id}/books?${query.toString()}`),
      error: null,
    };
  } catch (err) {
    const message = err instanceof ApiError ? err.message : "Could not reach the API.";
    return { data: null, error: message };
  }
}

export async function generateMetadata({ params }) {
  const { id } = await params;
  try {
    const author = await apiGet(`/api/v1/authors/${id}`);
    const name = [author.first_name, author.middle_name, author.last_name]
      .filter(Boolean)
      .join(" ");
    return { title: `${name} — Bibliomania` };
  } catch {
    return {};
  }
}

export default async function AuthorDetailPage({ params, searchParams }) {
  const { id } = await params;
  const resolvedSearchParams = await searchParams;

  const author = await getAuthor(id);
  const { data, error } = await getAuthorBooks(id, resolvedSearchParams.page);

  const name = [author.first_name, author.middle_name, author.last_name]
    .filter(Boolean)
    .join(" ");

  return (
    <section className="py-16">
      <Container className="flex flex-col gap-10">
        <div className="max-w-2xl">
          <h1 className="font-serif text-3xl font-semibold text-foreground sm:text-4xl">
            {name}
          </h1>
          {author.biography && (
            <p className="mt-4 leading-relaxed text-muted">{author.biography}</p>
          )}
        </div>

        <div>
          <h2 className="font-serif text-xl font-semibold text-foreground">
            Books
          </h2>

          {error && <p className="mt-4 text-sm text-alert">{error}</p>}

          {!error && data.items.length === 0 && (
            <p className="mt-4 text-sm text-muted">
              No books by this author yet.
            </p>
          )}

          {!error && data.items.length > 0 && (
            <>
              <div className="mt-4 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
                {data.items.map((book) => (
                  <BookCard key={book.id} book={book} />
                ))}
              </div>
              <div className="mt-8">
                <Pagination
                  basePath={`/authors/${id}`}
                  searchParams={resolvedSearchParams}
                  page={data.page}
                  totalPages={data.total_pages}
                  hasNextPage={data.has_next_page}
                  hasPreviousPage={data.has_previous_page}
                />
              </div>
            </>
          )}
        </div>
      </Container>
    </section>
  );
}
