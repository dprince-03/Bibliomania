import Container from "@/components/Container";
import Button from "@/components/Button";

export default function Contact() {
  return (
    <section className="py-24">
      <Container className="max-w-xl text-center">
        <h1 className="font-serif text-4xl font-semibold text-foreground">
          Get in touch
        </h1>
        <p className="mt-6 leading-relaxed text-muted">
          Found something that isn&apos;t working, have a question, or just want
          to tell us what you think? We&apos;d love to hear it. The button below
          takes you to our public inbox on GitHub — leave a message there and
          we&apos;ll see it.
        </p>
        <div className="mt-8">
          <Button href="https://github.com/dprince-03/Bibliomania/issues">
            Send us a message
          </Button>
        </div>
      </Container>
    </section>
  );
}
