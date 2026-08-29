import Container from "@/components/Container";

// Route-level Suspense fallback while a page's Server Component data
// fetches resolve — without this, navigation to any page just hangs with
// no feedback until the fetch finishes.
export default function Loading() {
  return (
    <section className="py-24">
      <Container className="flex flex-col items-center gap-4 text-center">
        <div
          className="h-8 w-8 animate-spin rounded-full border-2 border-border border-t-accent"
          role="status"
          aria-label="Loading"
        />
        <p className="text-sm text-muted">Loading…</p>
      </Container>
    </section>
  );
}
