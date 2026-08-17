import type { Metadata } from "next";
import "@fontsource-variable/vazirmatn/wght.css";
import "./globals.css";
import "./ui-polish.css";
import "./ui-polish-15-5-1.css";
import "./ui-adoption.css";
import { AuthProvider } from "@/components/auth-provider";

export const metadata: Metadata = {
  title: "فروشگاه هوشمند قطعات",
  description: "فروش، انبار و شبکه قطعات خودرو",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="fa" dir="rtl">
      <body><AuthProvider>{children}</AuthProvider></body>
    </html>
  );
}
