export default function Logo({ className = "" }) {
  return (
    <span className={`inline-flex items-center gap-2 ${className}`}>
      <svg
        width="26"
        height="26"
        viewBox="0 0 26 26"
        fill="none"
        aria-hidden="true"
        className="shrink-0 text-accent"
      >
        <path
          d="M13 5.5C11 4 7.5 3.3 4 3.5V19c3.5-.2 7 .5 9 2 2-1.5 5.5-2.2 9-2V3.5c-3.5-.2-7 .5-9 2Z"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinejoin="round"
        />
        <path
          d="M13 5.5V21.5"
          stroke="currentColor"
          strokeWidth="1.6"
          strokeLinejoin="round"
        />
      </svg>
      <span className="font-blackletter text-2xl leading-none text-foreground">
        Bibliomania
      </span>
    </span>
  );
}
