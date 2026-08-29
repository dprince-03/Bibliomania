import Container from "@/components/Container";
import Button from "@/components/Button";
import { apiGet, ApiError } from "@/lib/api";

async function getBookCount() {
  try {
    const data = await apiGet("/api/v1/books?limit=1");
    return { count: data?.total_count ?? null, error: null };
  } catch (err) {
    if (err instanceof ApiError) {
      return { count: null, error: err.message };
    }
    return { count: null, error: "Could not reach the API." };
  }
}

export default async function Home() {
  const { count, error } = await getBookCount();

  return (
    <section className="py-24">
      <Container className="flex flex-col items-center gap-8 text-center">
        <h1 className="font-serif text-4xl font-semibold tracking-wide text-foreground sm:text-5xl">
          Welcome to Bibliotheca
        </h1>
        <p className="max-w-xl text-lg text-muted">
          This is the foundation for the product app — layout, palette,
          typography, and a working connection to the API. Real pages start
          arriving in the next step.
        </p>

        <div className="rounded-2xl border border-border bg-surface px-8 py-6">
          {error ? (
            <p className="text-sm text-alert">
              Couldn&apos;t reach the API: {error}
            </p>
          ) : (
            <p className="font-serif text-2xl text-accent">
              {count !== null ? count.toLocaleString() : "—"}{" "}
              <span className="text-base font-sans text-muted">
                books in the catalog
              </span>
            </p>
          )}
        </div>

        <Button href="/">Placeholder button</Button>
      </Container>
    </section>
  );
}
