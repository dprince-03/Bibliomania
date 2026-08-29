import Link from "next/link";

function hrefForPage(basePath, searchParams, page) {
  const params = new URLSearchParams(searchParams);
  params.set("page", String(page));
  return `${basePath}?${params.toString()}`;
}

export default function Pagination({
  basePath,
  searchParams,
  page,
  totalPages,
  hasNextPage,
  hasPreviousPage,
}) {
  if (totalPages <= 1) return null;

  return (
    <nav className="flex items-center justify-center gap-4 text-sm">
      {hasPreviousPage ? (
        <Link
          href={hrefForPage(basePath, searchParams, page - 1)}
          className="text-link hover:underline"
        >
          ← Previous
        </Link>
      ) : (
        <span className="text-muted">← Previous</span>
      )}

      <span className="text-muted">
        Page {page} of {totalPages}
      </span>

      {hasNextPage ? (
        <Link
          href={hrefForPage(basePath, searchParams, page + 1)}
          className="text-link hover:underline"
        >
          Next →
        </Link>
      ) : (
        <span className="text-muted">Next →</span>
      )}
    </nav>
  );
}
