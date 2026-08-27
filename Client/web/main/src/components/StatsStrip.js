"use client";

import { useEffect, useState } from "react";
import { getTotalCount } from "@/lib/api";

function formatCount(value) {
  if (value === null) return "—";
  return value.toLocaleString();
}

export default function StatsStrip() {
  const [books, setBooks] = useState(null);
  const [authors, setAuthors] = useState(null);

  useEffect(() => {
    let cancelled = false;

    getTotalCount("/api/v1/books?limit=1").then((count) => {
      if (!cancelled) setBooks(count);
    });
    getTotalCount("/api/v1/authors?limit=1").then((count) => {
      if (!cancelled) setAuthors(count);
    });

    return () => {
      cancelled = true;
    };
  }, []);

  const stats = [
    { label: "Books in the catalog", value: books },
    { label: "Authors", value: authors },
  ];

  return (
    <div className="flex flex-wrap justify-center gap-10 sm:gap-16">
      {stats.map((stat) => (
        <div key={stat.label} className="text-center">
          <p className="font-serif text-3xl font-semibold text-accent sm:text-4xl">
            {formatCount(stat.value)}
          </p>
          <p className="mt-1 text-sm text-muted">{stat.label}</p>
        </div>
      ))}
    </div>
  );
}
