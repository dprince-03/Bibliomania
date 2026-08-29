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
  type = "button",
  children,
  ...rest
}) {
  const classes = `inline-flex items-center justify-center rounded-full px-6 py-2.5 text-sm font-medium transition-colors font-serif tracking-wide ${variants[variant]} ${className}`;

  if (href) {
    return (
      <Link href={href} className={classes}>
        {children}
      </Link>
    );
  }

  return (
    <button type={type} className={classes} {...rest}>
      {children}
    </button>
  );
}
