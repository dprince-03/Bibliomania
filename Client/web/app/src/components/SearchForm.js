// Plain GET form — no client JS needed. Submitting always resets to page 1
// by omitting a page field, so a new search never lands on a stale page
// number from the previous query.
export default function SearchForm({ defaultValues = {} }) {
  return (
    <form className="grid gap-3 sm:grid-cols-[1fr_auto_auto_auto]" action="/">
      <input
        type="text"
        name="q"
        placeholder="Search by title, author, or description"
        defaultValue={defaultValues.q}
        className="rounded-lg border border-border bg-background px-3.5 py-2.5 text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent sm:col-span-1"
      />
      <input
        type="text"
        name="genre"
        placeholder="Genre"
        defaultValue={defaultValues.genre}
        className="rounded-lg border border-border bg-background px-3.5 py-2.5 text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
      />
      <select
        name="format"
        defaultValue={defaultValues.format || ""}
        className="rounded-lg border border-border bg-background px-3.5 py-2.5 text-foreground outline-none focus:border-accent focus:ring-1 focus:ring-accent"
      >
        <option value="">Any format</option>
        <option value="digital">Digital</option>
        <option value="physical">Physical</option>
      </select>
      <button
        type="submit"
        className="rounded-lg bg-accent px-5 py-2.5 text-sm font-medium text-accent-foreground transition-colors hover:bg-accent-strong"
      >
        Search
      </button>
    </form>
  );
}
