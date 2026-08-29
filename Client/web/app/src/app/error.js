"use client";

import Container from "@/components/Container";
import Button from "@/components/Button";

// Safety net for anything a page's own try/catch doesn't handle — e.g.
// books/[id] and authors/[id] only catch a 404 specially and re-throw
// everything else (a real API/network failure), which lands here instead
// of Next's default unstyled error screen.
export default function Error({ error, reset }) {
  return (
    <section className="py-24">
      <Container className="max-w-md text-center">
        <p className="font-serif text-6xl font-semibold text-alert">!</p>
        <h1 className="mt-4 font-serif text-2xl font-semibold text-foreground">
          Something went wrong
        </h1>
        <p className="mt-4 text-muted">
          {error?.message || "An unexpected error occurred. Please try again."}
        </p>
        <div className="mt-8 flex items-center justify-center gap-4">
          <Button type="button" onClick={() => reset()}>
            Try again
          </Button>
          <Button href="/" variant="ghost">
            Back to the catalog
          </Button>
        </div>
      </Container>
    </section>
  );
}
