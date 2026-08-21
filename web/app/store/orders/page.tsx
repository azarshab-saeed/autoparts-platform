"use client";
import { useEffect, useMemo, useState } from "react";
import { fulfillStoreReservation, getStoreReservations, transitionStoreReservation } from "@/lib/api";
import type { NetworkReservation, PaymentPart } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import Modal from "@/components/modal";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const label:Record<string,string>={pending:"در انتظار",accepted:"تأیید شده",ready:"آماده تحویل",fulfilled:"تحویل و فروخته شد",rejected:"رد شده",cancelled:"لغو مشتری",expired:"منقضی"};
type CreditMode="credit"|"partial";

export default function StoreOrders(){
  const {session}=useAuth();
  const [items,setItems]=useState<NetworkReservation[]>([]);
  const [status,setStatus]=useState("all");
  const [loading,setLoading]=useState(true);
  const [busy,setBusy]=useState("");
  const [error,setError]=useState("");
  const [credit,setCredit]=useState<{reservation:NetworkReservation;mode:CreditMode}|null>(null);
  const [partialMethod,setPartialMethod]=useState<"cash"|"card">("card");
  const [partialPaid,setPartialPaid]=useState(0);

  async function load(next=status){if(!session)return;setLoading(true);setError("");try{setItems(await getStoreReservations(session,next));}catch(e){setError(e instanceof Error?e.message:"دریافت سفارش‌ها انجام نشد");}finally{setLoading(false)}}
  useEffect(()=>{void load(status)},[session,status]);

  async function move(id:string,next:"accepted"|"ready"|"rejected"){if(!session)return;setBusy(id);setError("");try{const x=await transitionStoreReservation(session,id,next);setItems(xs=>xs.map(r=>r.id===id?x:r).filter(r=>status==="all"||r.status===status));}catch(e){setError(e instanceof Error?e.message:"تغییر وضعیت انجام نشد");}finally{setBusy("")}}

  async function fulfillImmediate(r:NetworkReservation,method:"cash"|"card"){
    if(!session)return;const text=method==="cash"?"نقدی":"کارتخوان";
    if(!window.confirm(`تحویل ${r.product_title} و ثبت فروش ${text} به مبلغ ${money(r.total_amount)}؟`))return;
    await doFulfill(r,method);
  }

  async function doFulfill(r:NetworkReservation,method:"cash"|"card"|"credit",payments?:PaymentPart[]){
    if(!session)return;setBusy(r.id);setError("");
    try{
      const out=await fulfillStoreReservation(session,r.id,method,undefined,payments);
      setItems(xs=>xs.map(x=>x.id===r.id?{...x,status:"fulfilled" as const,sale_id:out.sale_id,paid_amount:out.paid_amount,due_amount:out.due_amount}:x).filter(x=>status==="all"||x.status===status));
      setCredit(null);setPartialPaid(0);
      if(out.due_amount>0) window.alert(`فروش ثبت شد. ${money(out.paid_amount)} دریافت و ${money(out.due_amount)} به‌عنوان مانده برای تأیید مکانیک ثبت شد.`);
      else window.alert(`فروش تسویه کامل ثبت شد. شناسه فروش: ${out.sale_id}`);
    }catch(e){setError(e instanceof Error?e.message:"تحویل و ثبت فروش انجام نشد");}finally{setBusy("")}
  }

  const partialDue=useMemo(()=>credit?.mode==="partial"?Math.max(0,credit.reservation.total_amount-partialPaid):0,[credit,partialPaid]);
  async function confirmCredit(){if(!credit)return;const r=credit.reservation;if(credit.mode==="credit"){await doFulfill(r,"credit");return;}if(partialPaid<=0||partialPaid>=r.total_amount){setError("برای پرداخت بخشی، مبلغ دریافتی باید بیشتر از صفر و کمتر از مبلغ کل باشد.");return;}await doFulfill(r,"credit",[{method:partialMethod,amount:partialPaid}]);}

  return <div>
    <div className="page-head"><div><span className="eyebrow">سفارش‌های شبکه</span><h1>رزروهای ورودی</h1><p>رزرو را تأیید کن، آماده تحویل بزن و هنگام مراجعه مکانیک روش تسویه واقعی را انتخاب کن.</p></div><div className="order-tabs">{[["all","همه"],["pending","در انتظار"],["accepted","تأیید"],["ready","آماده"],["fulfilled","فروخته‌شده"]].map(([v,l])=><button key={v} className={status===v?"active":""} onClick={()=>setStatus(v)}>{l}</button>)}</div></div>
    {error&&<div className="alert danger">{error}</div>}
    <section className="panel store-orders-panel">{loading?<div className="empty-state">در حال دریافت...</div>:items.length===0?<div className="empty-state">رزروی در این وضعیت ندارید.</div>:<div className="store-order-list">{items.map(r=><article className="store-order" key={r.id}>
      <div className="store-order-main"><div><span className={`reservation-status ${r.status}`}>{label[r.status]||r.status}</span><h3>{r.product_title}</h3><p>{r.buyer_name||r.buyer_email||"کاربر شبکه"} · {r.qty} عدد{r.buyer_role==="mechanic"?" · مکانیک":""}</p></div><div className="store-order-total"><b>{money(r.total_amount)}</b><span>{money(r.unit_price)} / عدد</span>{r.status==="fulfilled"&&<span>{r.due_amount>0?`مانده ${money(r.due_amount)}`:"تسویه کامل"}</span>}</div></div>
      <div className="store-order-foot"><small>ثبت: {new Date(r.created_at).toLocaleString("fa-IR")} · انقضا: {new Date(r.expires_at).toLocaleTimeString("fa-IR")}</small><div className="store-order-actions">
        {r.status==="pending"&&<><button disabled={busy===r.id} className="success-btn" onClick={()=>void move(r.id,"accepted")}>تأیید رزرو</button><button disabled={busy===r.id} className="danger-btn" onClick={()=>void move(r.id,"rejected")}>رد</button></>}
        {r.status==="accepted"&&<><button disabled={busy===r.id} className="primary-btn" onClick={()=>void move(r.id,"ready")}>آماده تحویل شد</button><button disabled={busy===r.id} className="danger-btn" onClick={()=>void move(r.id,"rejected")}>لغو و آزادسازی</button></>}
        {r.status==="ready"&&<><button disabled={busy===r.id} className="success-btn" onClick={()=>void fulfillImmediate(r,"cash")}>تحویل + نقدی</button><button disabled={busy===r.id} className="primary-btn" onClick={()=>void fulfillImmediate(r,"card")}>تحویل + کارتخوان</button>{r.buyer_role==="mechanic"&&<><button disabled={busy===r.id} onClick={()=>{setError("");setCredit({reservation:r,mode:"credit"})}}>تحویل + اعتباری</button><button disabled={busy===r.id} onClick={()=>{setError("");setPartialPaid(0);setCredit({reservation:r,mode:"partial"})}}>پرداخت بخشی</button></>}</>}
        {r.status==="fulfilled"&&<span className="ready-note">{r.due_amount>0?`فروش ثبت شده · مانده ${money(r.due_amount)}`:"فروش ثبت شده · تسویه کامل"}</span>}
      </div></div>
    </article>)}</div>}</section>

    <Modal open={!!credit} onClose={()=>setCredit(null)} title={credit?.mode==="credit"?"تحویل اعتباری به مکانیک":"تحویل با پرداخت بخشی"} subtitle={credit?.reservation.product_title}>
      {credit&&<><div className="form-stack"><div className="settlement-balance"><span>مبلغ سفارش</span><b>{money(credit.reservation.total_amount)}</b></div>{credit.mode==="credit"?<div className="form-help">کل مبلغ به‌عنوان مانده ثبت می‌شود. بعد از تحویل، یک درخواست بدهی از طرف فروشگاه برای تأیید مکانیک ساخته خواهد شد.</div>:<><label>روش مبلغ دریافتی<select value={partialMethod} onChange={e=>setPartialMethod(e.target.value as "cash"|"card")}><option value="card">کارتخوان</option><option value="cash">نقد</option></select></label><label>مبلغ دریافتی الآن<input type="number" min="1" max={credit.reservation.total_amount-1} value={partialPaid||""} onChange={e=>setPartialPaid(Number(e.target.value)||0)}/></label><div className="settlement-balance"><span>مانده پس از تحویل</span><b>{money(partialDue)}</b></div><div className="form-help">فقط مانده واقعی برای تأیید مکانیک وارد حساب شبکه می‌شود.</div></>}</div><div className="modal-actions"><button className="ghost-btn" onClick={()=>setCredit(null)}>انصراف</button><button className="primary-btn" disabled={busy===credit.reservation.id} onClick={()=>void confirmCredit()}>{busy===credit.reservation.id?"در حال ثبت...":"تأیید تحویل و فروش"}</button></div></>}
    </Modal>
  </div>;
}
