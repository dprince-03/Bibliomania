const common = {
  width: 20,
  height: 20,
  viewBox: "0 0 20 20",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.6,
  strokeLinecap: "round",
  strokeLinejoin: "round",
};

export function CatalogIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M4 3.5h9a2 2 0 0 1 2 2V17H6a2 2 0 0 1-2-2Z" />
      <path d="M4 15h11" />
      <path d="M7 7h5M7 9.5h5" />
    </svg>
  );
}

export function CloudIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M5.5 15A3.5 3.5 0 0 1 5 8.02 4.5 4.5 0 0 1 13.9 6.6 3.5 3.5 0 0 1 14.5 15Z" />
      <path d="M10 9v6M7.5 12.5 10 10l2.5 2.5" />
    </svg>
  );
}

export function ClockIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <circle cx="10" cy="10.5" r="6.5" />
      <path d="M10 6.5v4l3 2" />
    </svg>
  );
}

export function BookmarkIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M5.5 3.5h9v13l-4.5-3-4.5 3Z" />
    </svg>
  );
}

export function ShieldIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <path d="M10 2.5 16 5v5c0 4-2.5 6.5-6 7.5-3.5-1-6-3.5-6-7.5V5Z" />
      <path d="M7.5 10 9 11.5 12.5 8" />
    </svg>
  );
}

export function SearchIcon() {
  return (
    <svg {...common} aria-hidden="true">
      <circle cx="9" cy="9" r="5.5" />
      <path d="M17 17l-3.8-3.8" />
    </svg>
  );
}
