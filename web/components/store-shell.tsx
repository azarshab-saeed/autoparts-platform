"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BoxIcon, CartIcon, ChartIcon, HomeIcon, SearchIcon, SettingsIcon, UsersIcon } from "./icons";
import { useEffect } from "react";
import { useAuth } from "@/components/auth-provider";
import EdgeStatus from "@/components/edge-status";

type NavItem = {
  href: string;
  label: string;
  icon: typeof HomeIcon;
  group: "عملیات" | "شبکه" | "مالی و کنترل";
  roles?: string[];
  mobile?: boolean;
  soon?: boolean;
};

const nav: NavItem[] = [
  {href:"/store",label:"داشبورد",icon:HomeIcon,group:"عملیات",mobile:true},
  {href:"/store/sales",label:"فروش",icon:CartIcon,group:"عملیات",mobile:true},
  {href:"/store/purchases",label:"خرید",icon:BoxIcon,group:"عملیات"},
  {href:"/store/inventory",label:"انبار و کالاها",icon:BoxIcon,group:"عملیات",mobile:true},
  {href:"/store/import",label:"ورود اطلاعات",icon:BoxIcon,group:"عملیات",roles:["owner","admin","warehouse"]},
  {href:"/store/edge",label:"فروش آفلاین / Edge",icon:SettingsIcon,group:"عملیات",roles:["owner","admin"]},
  {href:"/store/hardware",label:"چاپگر و سخت‌افزار",icon:SettingsIcon,group:"عملیات",roles:["owner","admin"]},
  {href:"/store/network",label:"شبکه قطعات",icon:SearchIcon,group:"شبکه"},
  {href:"/store/procurement",label:"تأمین از شبکه",icon:BoxIcon,group:"شبکه",mobile:true},
  {href:"/store/orders",label:"سفارش‌های شبکه",icon:CartIcon,group:"شبکه",mobile:true},
  {href:"/store/fitment",label:"کاتالوگ و فیتمنت",icon:SearchIcon,group:"شبکه"},
  {href:"/store/accounts",label:"حساب‌ها",icon:UsersIcon,group:"مالی و کنترل"},
  {href:"/store/expenses",label:"هزینه‌ها",icon:ChartIcon,group:"مالی و کنترل",roles:["owner","admin","accountant"]},
  {href:"/store/closing",label:"صندوق و بستن روز",icon:ChartIcon,group:"مالی و کنترل",roles:["owner","admin","cashier","accountant"]},
  {href:"/store/returns",label:"مرجوعی",icon:CartIcon,group:"مالی و کنترل"},
  {href:"/store/reports",label:"گزارش‌ها",icon:ChartIcon,group:"مالی و کنترل",roles:["owner","admin","accountant"]},
  {href:"/store/audit",label:"رویدادهای امنیتی",icon:SettingsIcon,group:"مالی و کنترل",roles:["owner","admin"]},
  {href:"#",label:"تنظیمات",icon:SettingsIcon,group:"مالی و کنترل",soon:true},
];

const roleLabel: Record<string,string> = {
  owner:"مالک فروشگاه",admin:"مدیر",cashier:"صندوقدار",warehouse:"انباردار",accountant:"حسابدار",
};

export default function StoreShell({children}:{children:React.ReactNode}){
  const path=usePathname();
  const router=useRouter();
  const {ready,session,logout}=useAuth();
  useEffect(()=>{
    if(ready&&!session) router.replace("/login");
    if(ready&&session&&(session.role==="mechanic"||session.role==="consumer")) router.replace("/mechanic");
  },[ready,session,router]);
  if(!ready||!session||session.role==="mechanic"||session.role==="consumer") return <div className="loading-screen"><span className="loading-dot"/>در حال ورود به فروشگاه...</div>;

  const visible=nav.filter(item=>!item.roles||item.roles.includes(session.role));
  const groups=(['عملیات','شبکه','مالی و کنترل'] as const);
  const current=visible.find(item=>item.href!=="#"&&(path===item.href||(item.href!=="/store"&&path.startsWith(item.href+"/"))))||visible[0];
  const CurrentIcon=current?.icon||HomeIcon;
  const renderItem=(item:NavItem,mobile=false)=>{
    const Icon=item.icon;
    const active=item.href!=="#"&&(path===item.href||(item.href!=="/store"&&path.startsWith(item.href+"/")));
    return <Link
      key={`${mobile?'m':'d'}-${item.href}`}
      href={item.href}
      title={item.label}
      onClick={e=>{if(item.href==="#")e.preventDefault()}}
      className={`${active?"nav-item active":"nav-item"}${mobile?" mobile-nav-item":""}`}
    ><span className="nav-icon"><Icon/></span><span className="nav-label">{item.label}</span>{item.soon&&<small>به‌زودی</small>}</Link>;
  };

  return <div className="app-shell">
    <aside className="sidebar">
      <div className="brand"><div className="brand-mark">ی</div><div><b>فروشگاه هوشمند</b><span>{session.storeName}</span></div></div>
      <div className="store-context"><span className="context-dot"/><div><b>{session.storeName}</b><span>{roleLabel[session.role]||session.role}</span></div></div>
      <nav className="desktop-nav">{groups.map(group=><div className="nav-group" key={group}><div className="nav-group-title">{group}</div>{visible.filter(item=>item.group===group).map(item=>renderItem(item))}</div>)}</nav>
      <div className="sidebar-footer"><div className="sidebar-help"><b>همه‌چیز آماده فروش است</b><span>موجودی و شبکه به‌صورت زنده همگام‌اند.</span></div><button className="logout" onClick={()=>void logout()}>خروج از حساب</button></div>
    </aside>

    <main className="main-area">
      <header className="topbar">
        <div className="topbar-leading"><div className="mobile-brand"><span>ی</span><div><b>{session.storeName}</b><small>پنل فروشگاه</small></div></div><div className="topbar-context"><span className="topbar-context-icon"><CurrentIcon/></span><div><b>{current?.label||"پنل فروشگاه"}</b><small>{current?.group||"عملیات روزانه"}</small></div></div></div>
        <div className="topbar-actions"><EdgeStatus/><Link className="top-sale-action" href="/store/sales">+ فروش جدید</Link><div className="top-user"><div><b>{session.displayName}</b><span>{roleLabel[session.role]||session.role}</span></div><div className="avatar">{session.displayName.slice(0,1)}</div></div></div>
      </header>
      <div className="page-content">{children}</div>
    </main>

    <nav className="mobile-bottom-nav">{visible.filter(item=>item.mobile).map(item=>renderItem(item,true))}</nav>
  </div>;
}
