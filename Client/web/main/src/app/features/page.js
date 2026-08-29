import Container from "@/components/Container";
import Button from "@/components/Button";
import {
  SearchIcon,
  CloudIcon,
  ClockIcon,
  BookmarkIcon,
  ShieldIcon,
} from "@/components/icons";

const appUrl = process.env.NEXT_PUBLIC_APP_URL || "http://app.bibliomania.local";

const sections = [
  {
    icon: <SearchIcon />,
    title: "Find any book, fast",
    description:
      "Type a title, an author's name, or just the genre you're in the mood for — search finds it instantly, so you spend your time reading, not hunting.",
  },
  {
    icon: <CloudIcon />,
    title: "Read anywhere, anytime",
    description:
      "Download a book and start reading right away. Switch from your phone to your laptop without losing your place — and if you were reading with no signal, everything catches up the moment you're back online.",
  },
  {
    icon: <ClockIcon />,
    title: "Borrow without the guesswork",
    description:
      "See exactly what's available before you make the trip. Reserve a copy, get a clear due date, and return it when you're done — no surprises, no mystery fees creeping up on you.",
  },
  {
    icon: <BookmarkIcon />,
    title: "Your reading life, all in one place",
    description:
      "Every bookmark, everything you've finished, everything you're halfway through — it's all here, so you never lose track of a story again.",
  },
  {
    icon: <ShieldIcon />,
    title: "Built for libraries too",
    description:
      "Behind the scenes, librarians and staff have simple tools to keep the shelves organized and look after members' accounts — so the experience on your end just works.",
  },
];

export default function Features() {
  return (
    <>
      <section className="border-b border-border py-20 text-center">
        <Container>
          <h1 className="font-serif text-4xl font-semibold text-foreground">
            Everything you can do with Bibliomania
          </h1>
          <p className="mx-auto mt-4 max-w-xl text-muted">
            One app for finding, borrowing, and reading books — here&apos;s what&apos;s
            waiting for you inside.
          </p>
        </Container>
      </section>

      <section className="py-20">
        <Container className="space-y-16">
          {sections.map((section) => (
            <div
              key={section.title}
              className="grid gap-6 sm:grid-cols-[auto_1fr] sm:items-start"
            >
              <div
                className="flex h-12 w-12 items-center justify-center rounded-full bg-surface text-accent"
                aria-hidden="true"
              >
                {section.icon}
              </div>
              <div>
                <h2 className="font-serif text-2xl font-semibold text-foreground">
                  {section.title}
                </h2>
                <p className="mt-2 max-w-2xl leading-relaxed text-muted">
                  {section.description}
                </p>
              </div>
            </div>
          ))}
        </Container>
      </section>

      <section className="border-t border-border bg-surface py-20">
        <Container className="flex flex-col items-center gap-6 text-center">
          <h2 className="font-serif text-3xl font-semibold text-foreground">
            See it for yourself
          </h2>
          <Button href={appUrl}>Open the App</Button>
        </Container>
      </section>
    </>
  );
}
