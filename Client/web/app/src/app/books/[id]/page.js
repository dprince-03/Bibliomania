import Link from "next/link";
import { notFound } from "next/navigation";
import Container from "@/components/Container";
import { apiGet, ApiError } from "@/lib/api";

async function getBook(id) {
  try {
    return await apiGet(`/api/v1/books/${id}`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      notFound();
    }
    throw err;
  }
}

export default async function BookDetailPage({ params }) {
  const { id } = await params;
  const book = await getBook(id);

  return (
    <section className="py-16">
      <Container className="max-w-2xl">
        <p className="text-xs font-medium uppercase tracking-wide text-accent">
          {book.genre}
        </p>
        <h1 className="mt-2 font-serif text-3xl font-semibold text-foreground sm:text-4xl">
          {book.title}
        </h1>

        {book.authors?.length > 0 && (
          <p className="mt-3 text-muted">
            by{" "}
            {book.authors.map((author, i) => (
              <span key={author.id}>
                {i > 0 && ", "}
                <Link
                  href={`/authors/${author.id}`}
                  className="text-link hover:underline"
                >
                  {author.first_name} {author.last_name}
                </Link>
              </span>
            ))}
          </p>
        )}

        <div className="mt-6 flex flex-wrap items-center gap-3 text-sm">
          <span
            className={`rounded-full px-3 py-1 font-medium ${
              book.is_digital
                ? "bg-link/10 text-link"
                : "bg-accent/10 text-accent"
            }`}
          >
            {book.is_digital ? "Digital" : "Physical"}
          </span>
          {!book.is_digital && (
            <span className="text-muted">
              {book.available_copies} of {book.total_copies} copies available
            </span>
          )}
          {book.published_year && (
            <span className="text-muted">Published {book.published_year}</span>
          )}
        </div>

        {book.description && (
          <p className="mt-8 leading-relaxed text-foreground">
            {book.description}
          </p>
        )}

        <p className="mt-10 text-sm text-muted">
          Borrowing and reading are coming in a later step — this page is
          catalog information only for now.
        </p>
      </Container>
    </section>
  );
}
