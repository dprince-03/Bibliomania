import Container from "@/components/Container";
import Button from "@/components/Button";

export default function NotFound() {
  return (
    <section className="py-24">
      <Container className="max-w-md text-center">
        <p className="font-serif text-6xl font-semibold text-accent">404</p>
        <h1 className="mt-4 font-serif text-2xl font-semibold text-foreground">
          Not found in the catalog
        </h1>
        <p className="mt-4 text-muted">
          Whatever you were looking for isn&apos;t here.
        </p>
        <div className="mt-8">
          <Button href="/">Back to the catalog</Button>
        </div>
      </Container>
    </section>
  );
}
