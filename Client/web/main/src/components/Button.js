import Link from "next/link";

const variants = {
  primary:
    "bg-accent text-accent-foreground hover:bg-accent-strong border border-transparent",
  ghost:
    "bg-transparent text-foreground border border-border hover:border-accent hover:text-accent",
};

export default function Button({
  href,
  variant = "primary",
  className = "",
  children,
}) {
  const classes = `inline-flex items-center justify-center rounded-full px-6 py-2.5 text-sm font-medium transition-colors ${variants[variant]} ${className}`;
  const isExternal = href.startsWith("http");

  if (isExternal) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className={classes}
      >
        {children}
      </a>
    );
  }

  return (
    <Link href={href} className={classes}>
      {children}
    </Link>
  );
}
