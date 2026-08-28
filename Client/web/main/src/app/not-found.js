import Container from "@/components/Container";
import Button from "@/components/Button";

export default function NotFound() {
  return (
    <section className="py-24">
      <Container className="max-w-xl text-center">
        <p className="font-serif text-6xl font-semibold text-accent">404</p>
        <h1 className="mt-4 font-serif text-3xl font-semibold text-foreground">
          This page wandered off the shelf.
        </h1>
        <p className="mt-4 leading-relaxed text-muted">
          We couldn&apos;t find the page you were looking for. It might have
          moved, or the link might be out of date.
        </p>
        <div className="mt-8">
          <Button href="/">Back to home</Button>
        </div>
      </Container>
    </section>
  );
}
