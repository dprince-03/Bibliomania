import Container from "@/components/Container";
import Button from "@/components/Button";

export const metadata = {
  title: "Coming Soon — Bibliotheca",
  description:
    "This part of Bibliotheca isn't live yet — here's what we're building toward.",
};

export default function ComingSoon() {
  return (
    <section className="py-24">
      <Container className="max-w-xl text-center">
        <p className="text-xs font-semibold uppercase tracking-wide text-accent">
          Coming soon
        </p>
        <h1 className="mt-3 font-serif text-4xl font-semibold text-foreground">
          We&apos;re not there yet — but we&apos;re headed there.
        </h1>
        <p className="mt-6 leading-relaxed text-muted">
          This part of Bibliotheca is still being built. Today, the app is
          for finding, borrowing, and reading books. What&apos;s next is a
          way for authors to bring their books straight to readers — no
          print run to pay for, no gatekeeper to get past — and for real
          libraries to discover independent writers worth sharing with the
          people who&apos;ll love them.
        </p>
        <p className="mt-4 leading-relaxed text-muted">
          None of that is available yet, so there&apos;s nothing to click
          through to here. Check back soon, or head back to what&apos;s live
          today.
        </p>
        <div className="mt-8 flex flex-wrap items-center justify-center gap-4">
          <Button href="/">Back to home</Button>
          <Button href="/features/" variant="ghost">
            See what&apos;s live today
          </Button>
        </div>
      </Container>
    </section>
  );
}
