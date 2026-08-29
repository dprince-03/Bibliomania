import Link from "next/link";
import { notFound } from "next/navigation";
import Container from "@/components/Container";
import BorrowForm from "@/components/BorrowForm";
import LibraryStatusForm from "@/components/LibraryStatusForm";
import { apiGet, ApiError } from "@/lib/api";
import { getRefreshToken } from "@/lib/session";

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

// A second apiGet call, separate from the one BookDetailPage below makes —
// generateMetadata and the page component don't share a cache here, so
// this is a deliberate extra round-trip in exchange for a real per-book
// <title> instead of every book sharing the root layout's title.
export async function generateMetadata({ params }) {
  const { id } = await params;
  try {
    const book = await apiGet(`/api/v1/books/${id}`);
    return { title: `${book.title} — Bibliomania` };
  } catch {
    return {};
  }
}

export default async function BookDetailPage({ params }) {
  const { id } = await params;
  const book = await getBook(id);
  const hasSession = Boolean(await getRefreshToken());
  const canBorrow = book.is_digital || book.available_copies > 0;

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

        {hasSession ? (
          canBorrow ? (
            <BorrowForm bookId={book.id} />
          ) : (
            <p className="mt-10 text-sm text-alert">
              No copies of this book are currently available to borrow.
            </p>
          )
        ) : (
          <p className="mt-10 text-sm text-muted">
            <Link href="/login" className="text-link hover:underline">
              Sign in
            </Link>{" "}
            to borrow this book.
          </p>
        )}

        {hasSession && (
          <div className="mt-6">
            <LibraryStatusForm bookId={book.id} label="Add" />
          </div>
        )}

        {book.is_digital && book.file_format && (
          <div className="mt-6 flex items-center gap-4 text-sm">
            {hasSession ? (
              <>
                <Link
                  href={`/books/${book.id}/read`}
                  className="text-link hover:underline"
                >
                  Read online
                </Link>
                <a
                  href={`/api/books/${book.id}/file?mode=attachment`}
                  download
                  className="text-link hover:underline"
                >
                  Download
                </a>
              </>
            ) : (
              <p className="text-muted">
                <Link href="/login" className="text-link hover:underline">
                  Sign in
                </Link>{" "}
                to read or download this book.
              </p>
            )}
          </div>
        )}
      </Container>
    </section>
  );
}
