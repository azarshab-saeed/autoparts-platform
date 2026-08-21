import type { Metadata } from "next";
import "@fontsource-variable/vazirmatn/wght.css";
import "./globals.css";
import "./ui-documents-15-15.css";
import "./ui-polish.css";
import "./ui-polish-15-5-1.css";
import "./ui-adoption.css";
import "./ui-finance-15-9.css";
import "./ui-finance-15-13.css";
import "./ui-phase15-12.css";
import "./ui-tax-15-14.css";
import "./ui-vehicle-notebook-15-16.css";
import "./ui-workshop-network-15-17.css";
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
