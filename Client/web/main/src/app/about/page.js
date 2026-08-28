import Link from "next/link";
import Container from "@/components/Container";

export default function About() {
  return (
    <>
      <section className="border-b border-border py-20">
        <Container className="max-w-2xl">
          <h1 className="font-serif text-4xl font-semibold text-foreground">
            Why we built Bibliotheca
          </h1>
          <p className="mt-6 leading-relaxed text-muted">
            Finding a book, borrowing it, and actually sitting down to read it
            shouldn&apos;t mean juggling three different apps — and a library card
            you can never find when you need it. Bibliotheca puts all of it
            in one place: search the catalog, reserve a copy or grab the
            e-book, and keep reading right where you left off, on whatever
            device you happen to have on you.
          </p>
          <p className="mt-4 leading-relaxed text-muted">
            It&apos;s for anyone who just wants an easier way to read — students,
            lifelong readers, and everyone who&apos;s ever shown up to the library
            only to find the book they wanted was already checked out.
          </p>
        </Container>
      </section>

      <section className="py-20">
        <Container className="max-w-2xl">
          <div className="rounded-2xl border border-dashed border-accent/60 bg-surface p-8">
            <p className="text-xs font-semibold uppercase tracking-wide text-accent">
              Coming soon
            </p>
            <h2 className="mt-3 font-serif text-2xl font-semibold text-foreground">
              A place for authors too
            </h2>
            <p className="mt-4 leading-relaxed text-muted">
              We&apos;re also building a way for authors to bring their books
              straight to readers — no print run to pay for, no gatekeeper to
              get past — and for libraries to discover independent writers
              worth sharing with the people who&apos;ll love them. It isn&apos;t here
              yet, but it&apos;s exactly where we&apos;re headed.
            </p>
            <Link
              href="/coming-soon/"
              className="mt-4 inline-block text-sm font-medium text-accent hover:text-accent-strong"
            >
              Learn more about what&apos;s next →
            </Link>
          </div>
        </Container>
      </section>
    </>
  );
}
