"use client";
import Link from "next/link";
import { mockDashboard } from "@/lib/mock";

const toman=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
export default function Dashboard(){const d=mockDashboard;return <>
  <div className="page-head"><div><span className="eyebrow">امروز</span><h1>داشبورد فروشگاه</h1><p>وضعیت امروز را در یک نگاه ببین.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/purchases">+ خرید جدید</Link><Link className="primary-btn" href="/store/sales">+ فروش جدید</Link></div></div>
  <section className="kpi-grid">
    <article className="kpi"><span>فروش امروز</span><strong>{toman(d.sales)}</strong><small className="up">۱۲٪ بیشتر از دیروز</small></article>
    <article className="kpi"><span>سود تقریبی</span><strong>{toman(d.profit)}</strong><small>براساس بهای میانگین</small></article>
    <article className="kpi"><span>خرید امروز</span><strong>{toman(d.purchases)}</strong><small>۳ سند خرید</small></article>
    <article className="kpi"><span>طلب از مشتری‌ها</span><strong>{toman(d.receivables)}</strong><small>۱۸ حساب باز</small></article>
    <article className="kpi"><span>بدهی به تأمین‌کننده</span><strong>{toman(d.payables)}</strong><small>۹ حساب باز</small></article>
  </section>
  <section className="dashboard-grid">
    <article className="panel wide"><div className="panel-head"><div><h2>فروش‌های اخیر</h2><p>آخرین تراکنش‌های ثبت‌شده امروز</p></div><Link href="/store/sales">ثبت فروش</Link></div><div className="table-wrap"><table><thead><tr><th>شماره</th><th>مشتری</th><th>مبلغ</th><th>پرداخت</th><th>ساعت</th></tr></thead><tbody>{d.recentSales.map(s=><tr key={s.no}><td>{s.no}</td><td>{s.customer}</td><td>{toman(s.amount)}</td><td><span className="pill">{s.method}</span></td><td>{s.time}</td></tr>)}</tbody></table></div></article>
    <article className="panel"><div className="panel-head"><div><h2>رو به اتمام</h2><p>کالاهایی که نیاز به توجه دارند</p></div><Link href="/store/inventory">مشاهده انبار</Link></div><div className="low-list">{d.lowStock.map(x=><div key={x.title}><div><b>{x.title}</b><span>موجودی فعلی</span></div><strong>{x.qty}</strong></div>)}</div></article>
    <article className="panel wide"><div className="panel-head"><div><h2>فروش ۷ روز اخیر</h2><p>نمای ساده عملکرد فروشگاه</p></div></div><div className="bars">{[42,58,51,76,63,88,72].map((h,i)=><div key={i}><span style={{height:`${h}%`}}></span><small>{["ش","ی","د","س","چ","پ","ج"][i]}</small></div>)}</div></article>
    <article className="panel quick-panel"><h2>کارهای سریع</h2><Link href="/store/sales">+ فروش جدید</Link><Link href="/store/purchases">+ خرید جدید</Link><Link href="/store/inventory">مدیریت انبار</Link><Link href="/store/accounts">دریافت / پرداخت</Link><Link href="/store/returns">مرجوعی</Link></article>
  </section>
</>}
