export default function FeatureCard({ icon, title, description }) {
  return (
    <div className="rounded-2xl border border-border bg-surface p-6">
      <div
        className="flex h-11 w-11 items-center justify-center rounded-full bg-background text-accent"
        aria-hidden="true"
      >
        {icon}
      </div>
      <h3 className="mt-4 font-serif text-lg font-semibold text-foreground">
        {title}
      </h3>
      <p className="mt-2 text-sm leading-relaxed text-muted">{description}</p>
    </div>
  );
}
