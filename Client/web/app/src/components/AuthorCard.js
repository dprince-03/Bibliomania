import Link from "next/link";

export default function AuthorCard({ author }) {
  const name = [author.first_name, author.middle_name, author.last_name]
    .filter(Boolean)
    .join(" ");

  return (
    <Link
      href={`/authors/${author.id}`}
      className="block rounded-2xl border border-border bg-surface p-5 transition-colors hover:border-accent"
    >
      <h3 className="font-serif text-lg font-semibold text-foreground">
        {name}
      </h3>
      {author.biography && (
        <p className="mt-2 line-clamp-2 text-sm text-muted">
          {author.biography}
        </p>
      )}
    </Link>
  );
}
