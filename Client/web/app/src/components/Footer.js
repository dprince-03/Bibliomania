import Container from "./Container";

export default function Footer() {
  return (
    <footer className="border-t border-border bg-surface">
      <Container className="py-6">
        <p className="text-xs text-muted">
          © {new Date().getFullYear()} Bibliotheca.
        </p>
      </Container>
    </footer>
  );
}
