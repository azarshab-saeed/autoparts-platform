"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { cancelMyReservation, getMyReservations } from "@/lib/api";
import type { NetworkReservation } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const label:Record<string,string>={pending:"در انتظار فروشگاه",accepted:"تأیید شده",ready:"آماده تحویل",rejected:"رد شده",cancelled:"لغو شده",expired:"منقضی شده"};
export default function MechanicOrders(){
 const {ready,session,login,logout}=useAuth();const [items,setItems]=useState<NetworkReservation[]>([]);const [loading,setLoading]=useState(true);const [error,setError]=useState("");
 async function load(){if(!session||!(session.role==="mechanic"||session.role==="consumer")){setLoading(false);return;}setLoading(true);try{setItems(await getMyReservations(session));}catch(e){setError(e instanceof Error?e.message:"خطا در دریافت رزروها");}finally{setLoading(false);}}
 useEffect(()=>{void load()},[session]);
 async function cancel(id:string){if(!session)return;try{const x=await cancelMyReservation(session,id);setItems(xs=>xs.map(r=>r.id===id?x:r));}catch(e){setError(e instanceof Error?e.message:"لغو انجام نشد")}}
 if(ready&&!session)return <main className="mechanic-page"><div className="mechanic-auth-empty"><b>برای دیدن رزروها وارد شوید.</b><button onClick={()=>void login("/mechanic/orders")}>ورود مکانیک</button></div></main>;
 return <main className="mechanic-page"><header className="mechanic-header"><Link href="/mechanic" className="mechanic-brand"><span>ی</span><b>شبکه قطعات</b></Link><div className="mechanic-account"><Link href="/mechanic">جست‌وجو</Link>{session&&<><span>{session.displayName}</span><button onClick={()=>void logout()}>خروج</button></>}</div></header><section className="mechanic-orders"><div className="page-head"><div><span className="eyebrow">پیگیری درخواست‌ها</span><h1>رزروهای من</h1><p>وضعیت تأیید فروشگاه و آماده‌شدن قطعه را اینجا ببین.</p></div><Link className="primary-btn" href="/mechanic">+ جست‌وجوی قطعه</Link></div>{error&&<div className="mechanic-error">{error}</div>}{loading?<div className="mechanic-start">در حال دریافت رزروها...</div>:items.length===0?<div className="mechanic-start"><b>هنوز رزروی نداری</b><span>یک قطعه پیدا کن و از فروشگاه رزرو بگیر.</span></div>:<div className="reservation-list">{items.map(r=><article className="reservation-card" key={r.id}><div><span className={`reservation-status ${r.status}`}>{label[r.status]||r.status}</span><h2>{r.product_title}</h2><p>{r.store_name} · {r.address||"آدرس ثبت نشده"}</p></div><div className="reservation-numbers"><span>{r.qty} عدد</span><b>{money(r.total_amount)}</b></div><div className="reservation-foot"><small>انقضا: {new Date(r.expires_at).toLocaleString("fa-IR")}</small>{r.phone&&<a href={`tel:${r.phone}`}>تماس با فروشگاه</a>}{["pending","accepted","ready"].includes(r.status)&&<button onClick={()=>void cancel(r.id)}>لغو رزرو</button>}</div></article>)}</div>}</section></main>
}
