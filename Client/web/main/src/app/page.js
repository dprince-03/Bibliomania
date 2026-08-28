import Container from "@/components/Container";
import Button from "@/components/Button";
import FeatureCard from "@/components/FeatureCard";
import StatsStrip from "@/components/StatsStrip";
import { CatalogIcon, CloudIcon, ClockIcon, BookmarkIcon } from "@/components/icons";

const appUrl = process.env.NEXT_PUBLIC_APP_URL || "http://app.bibliotheca.local";

const teasers = [
  {
    icon: <CatalogIcon />,
    title: "Find it in seconds",
    description:
      "Search by title, author, or genre and get straight to the book you want — no wandering the shelves, no guessing where it's filed.",
  },
  {
    icon: <CloudIcon />,
    title: "Read wherever you are",
    description:
      "Start a book on your phone at lunch, pick it up on your laptop tonight. Even offline, your place is saved and waiting when you're back.",
  },
  {
    icon: <ClockIcon />,
    title: "Borrow without the hassle",
    description:
      "Reserve a copy, know exactly when it's due, and bring it back when you're ready. We'll give you a nudge before you're running late.",
  },
  {
    icon: <BookmarkIcon />,
    title: "Everything you're reading, in one place",
    description:
      "Bookmarks, what you've finished, what you're mid-way through — your whole reading life on one shelf, always where you left it.",
  },
];

export default function Home() {
  return (
    <>
      <section className="border-b border-border">
        <Container className="flex flex-col items-center gap-8 py-24 text-center sm:py-32">
          <h1 className="max-w-2xl font-serif text-4xl font-semibold leading-tight text-foreground sm:text-5xl">
            A library, wherever you are.
          </h1>
          <p className="max-w-xl text-lg text-muted">
            Search for a book, borrow it, and start reading — today, tonight,
            on whatever you have with you. No queues, no lost library card,
            no forgetting where you put it down.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-4">
            <Button href={appUrl}>Open the App</Button>
            <Button href="/features/" variant="ghost">
              See what&apos;s inside
            </Button>
          </div>
        </Container>
      </section>

      <section className="border-b border-border bg-surface py-14">
        <Container>
          <StatsStrip />
        </Container>
      </section>

      <section className="py-24">
        <Container>
          <div className="mx-auto max-w-xl text-center">
            <h2 className="font-serif text-3xl font-semibold text-foreground">
              Everything you need, already here
            </h2>
            <p className="mt-3 text-muted">
              No waitlist, no &ldquo;coming soon&rdquo; — just open the app and use it.
            </p>
          </div>

          <div className="mt-12 grid gap-6 sm:grid-cols-2">
            {teasers.map((teaser) => (
              <FeatureCard key={teaser.title} {...teaser} />
            ))}
          </div>
        </Container>
      </section>

      <section className="border-t border-border bg-surface py-20">
        <Container className="flex flex-col items-center gap-6 text-center">
          <h2 className="font-serif text-3xl font-semibold text-foreground">
            Your next book is waiting.
          </h2>
          <p className="max-w-md text-muted">
            Search the catalog, borrow something new, or jump back into
            whatever you were reading last.
          </p>
          <Button href={appUrl}>Open the App</Button>
        </Container>
      </section>
    </>
  );
}
