"use client";
import { useEffect, useState } from "react";
import { fulfillStoreReservation, getStoreReservations, transitionStoreReservation } from "@/lib/api";
import type { NetworkReservation } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";

const money = (v:number) => new Intl.NumberFormat("fa-IR").format(v) + " تومان";
const label:Record<string,string> = {pending:"در انتظار",accepted:"تأیید شده",ready:"آماده تحویل",fulfilled:"تحویل و فروخته شد",rejected:"رد شده",cancelled:"لغو مشتری",expired:"منقضی"};

export default function StoreOrders(){
  const {session}=useAuth();
  const [items,setItems]=useState<NetworkReservation[]>([]);
  const [status,setStatus]=useState("all");
  const [loading,setLoading]=useState(true);
  const [busy,setBusy]=useState("");
  const [error,setError]=useState("");

  async function load(next=status){
    if(!session)return;
    setLoading(true);setError("");
    try{setItems(await getStoreReservations(session,next));}
    catch(e){setError(e instanceof Error?e.message:"دریافت سفارش‌ها انجام نشد");}
    finally{setLoading(false);}
  }
  useEffect(()=>{void load(status)},[session,status]);

  async function move(id:string,next:"accepted"|"ready"|"rejected"){
    if(!session)return;
    setBusy(id);setError("");
    try{
      const x=await transitionStoreReservation(session,id,next);
      setItems(xs=>xs.map(r=>r.id===id?x:r).filter(r=>status==="all"||r.status===status));
    }catch(e){setError(e instanceof Error?e.message:"تغییر وضعیت انجام نشد");}
    finally{setBusy("");}
  }

  async function fulfill(r:NetworkReservation,method:"cash"|"card"){
    if(!session)return;
    const text=method==="cash"?"نقدی":"کارتخوان";
    if(!window.confirm(`تحویل ${r.product_title} و ثبت فروش ${text} به مبلغ ${money(r.total_amount)}؟`))return;
    setBusy(r.id);setError("");
    try{
      const out=await fulfillStoreReservation(session,r.id,method);
      setItems(xs=>xs.map(x=>x.id===r.id?{...x,status:"fulfilled" as const}:x).filter(x=>status==="all"||x.status===status));
      window.alert(`فروش ثبت شد. شناسه فروش: ${out.sale_id}`);
    }catch(e){setError(e instanceof Error?e.message:"تحویل و ثبت فروش انجام نشد");}
    finally{setBusy("");}
  }

  return <div>
    <div className="page-head"><div><span className="eyebrow">سفارش‌های شبکه</span><h1>رزروهای ورودی</h1><p>رزرو را تأیید کن، آماده تحویل بزن و هنگام مراجعه مکانیک همان رزرو را به فروش واقعی تبدیل کن.</p></div>
      <div className="order-tabs">{[["all","همه"],["pending","در انتظار"],["accepted","تأیید"],["ready","آماده"],["fulfilled","فروخته‌شده"]].map(([v,l])=><button key={v} className={status===v?"active":""} onClick={()=>setStatus(v)}>{l}</button>)}</div>
    </div>
    {error&&<div className="alert danger">{error}</div>}
    <section className="panel store-orders-panel">{loading?<div className="empty-state">در حال دریافت...</div>:items.length===0?<div className="empty-state">رزروی در این وضعیت ندارید.</div>:<div className="store-order-list">{items.map(r=><article className="store-order" key={r.id}>
      <div className="store-order-main"><div><span className={`reservation-status ${r.status}`}>{label[r.status]||r.status}</span><h3>{r.product_title}</h3><p>{r.buyer_name||r.buyer_email||"کاربر شبکه"} · {r.qty} عدد</p></div><div className="store-order-total"><b>{money(r.total_amount)}</b><span>{money(r.unit_price)} / عدد</span></div></div>
      <div className="store-order-foot"><small>ثبت: {new Date(r.created_at).toLocaleString("fa-IR")} · انقضا: {new Date(r.expires_at).toLocaleTimeString("fa-IR")}</small><div className="store-order-actions">
        {r.status==="pending"&&<><button disabled={busy===r.id} className="success-btn" onClick={()=>void move(r.id,"accepted")}>تأیید رزرو</button><button disabled={busy===r.id} className="danger-btn" onClick={()=>void move(r.id,"rejected")}>رد</button></>}
        {r.status==="accepted"&&<><button disabled={busy===r.id} className="primary-btn" onClick={()=>void move(r.id,"ready")}>آماده تحویل شد</button><button disabled={busy===r.id} className="danger-btn" onClick={()=>void move(r.id,"rejected")}>لغو و آزادسازی</button></>}
        {r.status==="ready"&&<><button disabled={busy===r.id} className="success-btn" onClick={()=>void fulfill(r,"cash")}>تحویل + فروش نقدی</button><button disabled={busy===r.id} className="primary-btn" onClick={()=>void fulfill(r,"card")}>تحویل + کارتخوان</button></>}
        {r.status==="fulfilled"&&<span className="ready-note">فروش ثبت شده و موجودی از رزرو مصرف شده است</span>}
      </div></div>
    </article>)}</div>}</section>
  </div>;
}
