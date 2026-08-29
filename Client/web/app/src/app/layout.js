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
        <Navbar />
        <main className="flex-1">{children}</main>
        <Footer />
      </body>
    </html>
  );
}
