import { Cinzel, EB_Garamond, UnifrakturMaguntia } from "next/font/google";
import "./globals.css";
import Navbar from "@/components/Navbar";
import Footer from "@/components/Footer";

const cinzel = Cinzel({
  variable: "--font-cinzel",
  subsets: ["latin"],
});

const garamond = EB_Garamond({
  variable: "--font-garamond",
  subsets: ["latin"],
});

const blackletter = UnifrakturMaguntia({
  variable: "--font-blackletter-face",
  weight: "400",
  subsets: ["latin"],
});

export const metadata = {
  title: "Bibliotheca — Your Library",
  description:
    "Search the catalog, borrow a book, and read online or offline — the Bibliotheca product app.",
};

export default function RootLayout({ children }) {
  return (
    <html
      lang="en"
      className={`${cinzel.variable} ${garamond.variable} ${blackletter.variable} h-full antialiased`}
    >
      <body className="flex min-h-full flex-col bg-background text-foreground">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded-lg focus:bg-accent focus:px-4 focus:py-2 focus:text-accent-foreground"
        >
          Skip to content
        </a>
        <Navbar />
        <main id="main-content" className="flex-1">
          {children}
        </main>
        <Footer />
      </body>
    </html>
  );
}
