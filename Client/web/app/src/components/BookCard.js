import Link from "next/link";

export default function BookCard({ book }) {
  const authorNames = book.authors
    ?.map((a) => `${a.first_name} ${a.last_name}`)
    .join(", ");

  return (
    <Link
      href={`/books/${book.id}`}
      className="block rounded-2xl border border-border bg-surface p-5 transition-colors hover:border-accent"
    >
      <p className="text-xs font-medium uppercase tracking-wide text-accent">
        {book.genre}
      </p>
      <h3 className="mt-2 font-serif text-lg font-semibold text-foreground">
        {book.title}
      </h3>
      {authorNames && <p className="mt-1 text-sm text-muted">{authorNames}</p>}
      <div className="mt-4 flex items-center gap-2 text-xs">
        <span
          className={`rounded-full px-2.5 py-1 font-medium ${
            book.is_digital
              ? "bg-link/10 text-link"
              : "bg-accent/10 text-accent"
          }`}
        >
          {book.is_digital ? "Digital" : "Physical"}
        </span>
        {!book.is_digital && (
          <span className="text-muted">
            {book.available_copies > 0
              ? `${book.available_copies} available`
              : "None available"}
          </span>
        )}
      </div>
    </Link>
  );
}
