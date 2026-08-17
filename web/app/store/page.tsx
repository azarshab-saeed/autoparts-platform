"use client";
import Link from "next/link";
import { useEffect,useMemo,useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getDashboardSummary } from "@/lib/api";
import type { DashboardSummary } from "@/lib/types";

const toman=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const num=(v:number)=>new Intl.NumberFormat("fa-IR").format(v);

export default function Dashboard(){
 const {session}=useAuth();const[data,setData]=useState<DashboardSummary|null>(null);const[error,setError]=useState("");
 useEffect(()=>{if(!session)return;getDashboardSummary(session).then(setData).catch(e=>setError(e instanceof Error?e.message:"داشبورد دریافت نشد"))},[session]);
 const max=useMemo(()=>Math.max(1,...(data?.sales_last_seven_days||[]).map(x=>x.amount)),[data]);
 if(error)return <div className="error-box">{error}</div>;if(!data)return <div className="panel table-state dashboard-loading"><span className="loading-dot"/>در حال آماده‌سازی داشبورد...</div>;
 const actionCount=data.low_stock_count+data.open_reservations+data.open_selling_procurements+data.open_buying_procurements;
 const networkReady=data.network_enabled&&data.published_offers>0;
 return <>
 <div className="page-head dashboard-head"><div><span className="eyebrow">مرکز عملیات امروز</span><h1>داشبورد فروشگاه</h1><p>فروشگاهت را اداره کن و همزمان ببین شبکه قطعات چقدر فروش و درخواست تازه برایت ساخته است.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/import">⇧ ورود اطلاعات</Link><Link className="primary-btn" href="/store/sales">+ فروش جدید</Link></div></div>

 <section className="kpi-grid phase11-kpis polished-kpis">
   <article className="kpi tone-primary"><div className="kpi-top"><span>فروش خالص امروز</span><i>↗</i></div><strong>{toman(data.net_sales_today)}</strong><small>مرجوعی: {toman(data.sales_returns_today)}</small></article>
   <article className="kpi tone-success"><div className="kpi-top"><span>سود ناخالص امروز</span><i>٪</i></div><strong>{toman(data.gross_profit_today)}</strong><small>براساس بهای میانگین</small></article>
   <article className="kpi tone-violet"><div className="kpi-top"><span>خرید امروز</span><i>↓</i></div><strong>{toman(data.purchases_today)}</strong><small><Link href="/store/purchases/history">مشاهده خریدها</Link></small></article>
   <article className="kpi tone-cyan"><div className="kpi-top"><span>طلب مشتریان</span><i>⇢</i></div><strong>{toman(data.receivables)}</strong><small><Link href="/store/accounts">پیگیری مانده حساب‌ها</Link></small></article>
   <article className="kpi tone-warning"><div className="kpi-top"><span>بدهی تأمین‌کنندگان</span><i>⇠</i></div><strong>{toman(data.payables)}</strong><small><Link href="/store/accounts">تسویه و پرداخت</Link></small></article>
   <article className="kpi tone-slate"><div className="kpi-top"><span>ارزش موجودی</span><i>▣</i></div><strong>{toman(data.inventory_value)}</strong><small>{data.low_stock_count} قلم کم‌موجود</small></article>
 </section>

 <section className={`network-value-card${networkReady?" ready":" setup"}`}>
   <div className="network-value-main">
     <span className="network-value-badge">شبکه برای فروشگاه شما</span>
     {networkReady?<><h2>{data.network_sales_30d>0?`${toman(data.network_sales_30d)} فروش از شبکه در ۳۰ روز گذشته`:"فروشگاهت در شبکه دیده می‌شود؛ اولین فروش شبکه را بگیر."}</h2><p>{data.network_sales_count_30d>0?`${num(data.network_sales_count_30d)} فروش نهایی از ${num(data.network_requests_30d)} درخواست شبکه ثبت شده است.`:`${num(data.network_requests_30d)} درخواست در ۳۰ روز گذشته ثبت شده؛ موجودی و قیمت را تازه نگه دار.`}</p></>:data.network_enabled?<><h2>کالاهایت را منتشر کن تا مشتری خودش پیدایت کند.</h2><p>شبکه روشن است، اما هنوز پیشنهاد قابل نمایش نداری. قیمت فروش و موجودی چند کالای پرفروش را منتشر کن.</p></>:<><h2>فقط حسابداری نکن؛ از همین موجودی مشتری بگیر.</h2><p>شبکه قطعات را فعال کن تا مکانیک‌ها موجودی، قیمت و فاصله فروشگاهت را ببینند و رزرو ثبت کنند.</p></>}
     <div className="network-value-actions"><Link className="network-value-primary" href="/store/network">{networkReady?"مدیریت عرضه شبکه":"فعال‌سازی شبکه"}</Link><Link href="/store/orders">سفارش‌های شبکه</Link></div>
   </div>
   <div className="network-value-metrics">
     <div><span>درخواست ۳۰ روز</span><b>{num(data.network_requests_30d)}</b></div>
     <div><span>فروش نهایی</span><b>{num(data.network_sales_count_30d)}</b></div>
     <div><span>کالای منتشرشده</span><b>{num(data.published_offers)}</b></div>
     <div><span>درخواست باز فروش</span><b>{num(data.open_reservations+data.open_selling_procurements)}</b></div>
   </div>
 </section>

 <section className="dashboard-grid polished-dashboard">
   <article className="panel wide"><div className="panel-head"><div><span className="panel-kicker">فروش</span><h2>فروش‌های اخیر</h2><p>آخرین فاکتورهای ثبت‌شده</p></div><Link className="panel-link" href="/store/sales/history">همه فروش‌ها ←</Link></div>{!data.recent_sales.length?<div className="table-state empty-soft">هنوز فروشی ثبت نشده.</div>:<div className="table-wrap"><table><thead><tr><th>مشتری</th><th>مبلغ</th><th>مانده</th><th>زمان</th><th></th></tr></thead><tbody>{data.recent_sales.map(s=><tr key={s.id}><td><div className="table-primary">{s.customer_name||"مشتری نقدی"}{s.network_source&&<small className="block-note">فروش شبکه</small>}</div></td><td className="money-cell">{toman(s.total_amount)}</td><td>{s.due_amount?<span className="status-pill warn">{toman(s.due_amount)}</span>:<span className="status-pill ok">تسویه</span>}</td><td>{new Date(s.created_at).toLocaleString("fa-IR")}</td><td><Link className="table-action" href={`/store/sales/${s.id}`}>جزئیات</Link></td></tr>)}</tbody></table></div>}</article>

   <article className="panel action-center"><div className="action-center-head"><div><span className="panel-kicker attention">الان چه کار کنم؟</span><h2>مرکز اقدام</h2><p>{actionCount?`${num(actionCount)} مورد باز نیاز به تصمیم دارد.`:"مورد فوری بازی نداری؛ فروش و شبکه را ادامه بده."}</p></div><span className={actionCount?"action-count active":"action-count"}>{num(actionCount)}</span></div><div className="action-center-list">
     <Link href="/store/orders" className={data.open_reservations?"urgent":""}><span><i className="attention-dot info"/><div><b>رزروهای مشتری</b><small>تأیید یا آماده تحویل کن</small></div></span><strong>{num(data.open_reservations)}</strong></Link>
     <Link href="/store/procurement" className={data.open_selling_procurements?"urgent":""}><span><i className="attention-dot success"/><div><b>درخواست خرید از شما</b><small>فرصت فروش بین فروشگاه‌ها</small></div></span><strong>{num(data.open_selling_procurements)}</strong></Link>
     <Link href="/store/inventory" className={data.low_stock_count?"urgent warning":""}><span><i className="attention-dot danger"/><div><b>کالاهای کم‌موجود</b><small>قبل از از‌دست‌رفتن فروش تأمین کن</small></div></span><strong>{num(data.low_stock_count)}</strong></Link>
     <Link href="/store/procurement"><span><i className="attention-dot neutral"/><div><b>خریدهای شبکه در جریان</b><small>وضعیت دریافت از همکاران</small></div></span><strong>{num(data.open_buying_procurements)}</strong></Link>
   </div></article>

   <article className="panel wide trend-panel"><div className="panel-head"><div><span className="panel-kicker">روند</span><h2>فروش ۷ روز اخیر</h2><p>مبلغ فروش ثبت‌شده هر روز</p></div><Link className="panel-link" href="/store/reports">سود و زیان ←</Link></div><div className="bars real-bars">{data.sales_last_seven_days.map(x=><div key={x.date} title={toman(x.amount)}><b>{x.amount?toman(x.amount):"—"}</b><span style={{height:`${Math.max(5,Math.round(x.amount/max*100))}%`}}></span><small>{new Date(x.date+"T00:00:00").toLocaleDateString("fa-IR",{weekday:"short"})}</small></div>)}</div></article>

   <article className="panel quick-panel polished-quick"><div><span className="panel-kicker">میانبر</span><h2>کارهای سریع</h2></div><Link className="quick-primary" href="/store/sales"><span>+</span><div><b>فروش جدید</b><small>ثبت فاکتور و پرداخت</small></div></Link><Link href="/store/import"><span>⇧</span><div><b>ورود از فایل</b><small>مهاجرت کالا، موجودی و قیمت</small></div></Link><Link href="/store/purchases"><span>↓</span><div><b>خرید جدید</b><small>ورود کالا به انبار</small></div></Link><Link href="/store/procurement"><span>⌁</span><div><b>تأمین از شبکه</b><small>خرید از فروشگاه همکار</small></div></Link></article>
 </section></>;
}
