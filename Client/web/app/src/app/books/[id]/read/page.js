import Link from "next/link";
import { notFound } from "next/navigation";
import Container from "@/components/Container";
import ReadingThemeToggle from "@/components/reading/ReadingThemeToggle";
import ProgressForm from "@/components/reading/ProgressForm";
import EpubReader from "@/components/reading/EpubReader";
import BookmarkForm from "@/components/reading/BookmarkForm";
import BookmarkList from "@/components/reading/BookmarkList";
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

async function getSession(id) {
  try {
    return await apiGet(`/api/v1/reading/${id}/session`);
  } catch (err) {
    if (err instanceof ApiError && err.status === 404) {
      return null;
    }
    throw err;
  }
}

async function getBookmarks(id) {
  try {
    return await apiGet(`/api/v1/reading/${id}/bookmarks`);
  } catch (err) {
    if (err instanceof ApiError) return [];
    throw err;
  }
}

export async function generateMetadata({ params }) {
  const { id } = await params;
  try {
    const book = await apiGet(`/api/v1/books/${id}`);
    return { title: `Reading: ${book.title} — Bibliomania` };
  } catch {
    return {};
  }
}

export default async function ReadBookPage({ params }) {
  const { id } = await params;
  const book = await getBook(id);

  if (!book.is_digital || !book.file_format) {
    return (
      <section className="py-16">
        <Container className="max-w-2xl text-center">
          <p className="text-sm text-muted">
            {book.title} isn&apos;t available as an e-book.{" "}
            <Link href={`/books/${id}`} className="text-link hover:underline">
              Back to the book
            </Link>
          </p>
        </Container>
      </section>
    );
  }

  const [session, bookmarks] = await Promise.all([
    getSession(id),
    getBookmarks(id),
  ]);

  const fileUrl = `/api/books/${id}/file?mode=inline`;
  // Our own convention (see EpubReader): an epub session's total_pages is
  // always 100, current_page IS the percentage. A PDF session carries real
  // page numbers instead.
  const initialPercentage =
    book.file_format === "epub" && session ? session.current_page / 100 : 0;

  return (
    <section className="py-10">
      <Container className="flex flex-col gap-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div>
            <Link
              href={`/books/${id}`}
              className="text-sm text-link hover:underline"
            >
              ← Back to {book.title}
            </Link>
          </div>
          <div className="flex items-center gap-4">
            <ReadingThemeToggle />
            <a
              href={`/api/books/${id}/file?mode=attachment`}
              download
              className="text-sm text-link hover:underline"
            >
              Download
            </a>
          </div>
        </div>

        {book.file_format === "pdf" ? (
          <div className="flex flex-col gap-4">
            <iframe
              src={fileUrl}
              title={book.title}
              className="h-[75vh] w-full rounded-2xl border border-border"
            />
            <ProgressForm
              bookId={id}
              currentPage={session?.current_page}
              totalPages={session?.total_pages}
            />
          </div>
        ) : (
          <EpubReader bookId={id} fileUrl={fileUrl} initialPercentage={initialPercentage} />
        )}

        <div className="rounded-2xl border border-border bg-surface p-5">
          <h2 className="font-serif text-lg font-semibold text-foreground">
            Bookmarks
          </h2>
          <div className="mt-4">
            <BookmarkForm bookId={id} />
          </div>
          <div className="mt-4">
            <BookmarkList bookId={id} bookmarks={bookmarks} />
          </div>
        </div>
      </Container>
    </section>
  );
}
