"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BoxIcon, CartIcon, ChartIcon, HomeIcon, SearchIcon, SettingsIcon, UsersIcon } from "./icons";
import { useEffect } from "react";
import { useAuth } from "@/components/auth-provider";

const nav = [
  ["/store", "داشبورد", HomeIcon],
  ["/store/sales", "فروش", CartIcon],
  ["/store/purchases", "خرید", BoxIcon],
  ["/store/inventory", "انبار و کالاها", BoxIcon],
  ["/store/accounts", "حساب‌ها", UsersIcon],
  ["/store/expenses", "هزینه‌ها", ChartIcon],
  ["/store/closing", "صندوق و بستن روز", ChartIcon],
  ["/store/returns", "مرجوعی", CartIcon],
  ["/store/network", "شبکه قطعات", SearchIcon],
  ["/store/fitment", "کاتالوگ و فیتمنت", SearchIcon],
  ["/store/procurement", "تأمین از شبکه", BoxIcon],
  ["/store/orders", "سفارش‌های شبکه", CartIcon],
  ["/store/reports", "گزارش‌ها", ChartIcon],
  ["/store/audit", "رویدادهای امنیتی", SettingsIcon],
  ["#", "تنظیمات", SettingsIcon]
] as const;

export default function StoreShell({children}:{children:React.ReactNode}){
  const path=usePathname();
  const router=useRouter();
  const {ready,session,logout}=useAuth();
  useEffect(()=>{
    if(ready&&!session) router.replace("/login");
    if(ready&&session&&(session.role==="mechanic"||session.role==="consumer")) router.replace("/mechanic");
  },[ready,session,router]);
  if(!ready||!session||session.role==="mechanic"||session.role==="consumer") return <div className="loading-screen">در حال ورود به فروشگاه...</div>;
  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark">ی</div><div><b>فروشگاه هوشمند</b><span>{session.storeName}</span></div></div>
      <nav>{nav.map(([href,label,Icon],i)=>{
        if ((href==="/store/expenses"||href==="/store/reports") && !["owner","admin","accountant"].includes(session.role)) return null;
        if (href==="/store/closing" && !["owner","admin","cashier","accountant"].includes(session.role)) return null;
        if (href==="/store/audit" && !["owner","admin"].includes(session.role)) return null;
        const active=href!== "#" && (path===href || (href!=="/store" && path.startsWith(href+"/")));
        return <Link key={i} href={href} onClick={e=>{if(href==="#")e.preventDefault()}} className={active?"nav-item active":"nav-item"}><Icon/><span>{label}</span>{href==="#"&&<small>به‌زودی</small>}</Link>
      })}</nav>
      <button className="logout" onClick={()=>void logout()}>خروج از حساب</button>
    </aside>
    <main className="main-area">
      <header className="topbar"><div className="global-search"><SearchIcon/><input placeholder="جست‌وجوی کالا، مشتری یا فاکتور..."/></div><div className="top-user"><div><b>{session.displayName}</b><span>{session.role}</span></div><div className="avatar">{session.displayName.slice(0,1)}</div></div></header>
      <div className="page-content">{children}</div>
    </main>
  </div>;
}
