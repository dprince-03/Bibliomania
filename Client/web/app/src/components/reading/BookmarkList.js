import DeleteBookmarkButton from "./DeleteBookmarkButton";

const colorDots = {
  yellow: "bg-yellow-400",
  green: "bg-green-500",
  blue: "bg-blue-500",
  pink: "bg-pink-400",
  purple: "bg-purple-500",
};

export default function BookmarkList({ bookId, bookmarks }) {
  if (bookmarks.length === 0) {
    return <p className="text-sm text-muted">No bookmarks yet.</p>;
  }

  return (
    <ul className="flex flex-col divide-y divide-border">
      {bookmarks.map((bookmark) => (
        <li key={bookmark.id} className="flex items-start justify-between gap-4 py-3">
          <div className="flex items-start gap-2">
            <span
              className={`mt-1.5 h-2.5 w-2.5 shrink-0 rounded-full ${
                colorDots[bookmark.color] ?? "bg-muted"
              }`}
              aria-hidden="true"
            />
            <div>
              <p className="text-sm font-medium text-foreground">
                Page {bookmark.page}
              </p>
              {bookmark.note && (
                <p className="text-sm text-muted">{bookmark.note}</p>
              )}
            </div>
          </div>
          <DeleteBookmarkButton bookId={bookId} bookmarkId={bookmark.id} />
        </li>
      ))}
    </ul>
  );
}
