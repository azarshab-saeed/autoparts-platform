"use client";

import { useEffect, useMemo, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import {
  cancelBuyingProcurement,
  createNetworkProcurement,
  getBuyingProcurements,
  getSellingProcurements,
  receiveNetworkProcurement,
  searchNetworkProcurement,
  searchProducts,
  transitionSellingProcurement,
} from "@/lib/api";
import type { NetworkProcurement, NetworkSearchResult, Product } from "@/lib/types";

const money = (v:number) => new Intl.NumberFormat("fa-IR").format(v) + " تومان";
const qty = (v:number) => new Intl.NumberFormat("fa-IR",{maximumFractionDigits:3}).format(v);
const statusLabel:Record<string,string> = {
  requested:"در انتظار فروشنده", accepted:"تأیید فروشنده", ready:"آماده دریافت", received:"دریافت و ثبت شد",
  rejected:"رد شده", cancelled:"لغو شده", expired:"منقضی",
};

export default function ProcurementPage(){
  const {session}=useAuth();
  const [q,setQ]=useState("");
  const [localProducts,setLocalProducts]=useState<Product[]>([]);
  const [selected,setSelected]=useState<Product|null>(null);
  const [offers,setOffers]=useState<NetworkSearchResult[]>([]);
  const [buying,setBuying]=useState<NetworkProcurement[]>([]);
  const [selling,setSelling]=useState<NetworkProcurement[]>([]);
  const [qtyByOffer,setQtyByOffer]=useState<Record<string,number>>({});
  const [busy,setBusy]=useState("");
  const [message,setMessage]=useState("");
  const [loading,setLoading]=useState(true);

  async function loadOrders(){
    if(!session)return;
    const [b,s]=await Promise.all([getBuyingProcurements(session),getSellingProcurements(session)]);
    setBuying(b);setSelling(s);
  }
  useEffect(()=>{if(!session)return;setLoading(true);void loadOrders().catch(e=>setMessage(e instanceof Error?e.message:"خطا در دریافت درخواست‌ها")).finally(()=>setLoading(false));},[session]);

  async function findLocal(){
    if(!session||q.trim().length<2)return;
    setBusy("search-local");setMessage("");setSelected(null);setOffers([]);
    try{setLocalProducts(await searchProducts(q,session));}
    catch(e){setMessage(e instanceof Error?e.message:"جست‌وجوی کالا انجام نشد");}
    finally{setBusy("");}
  }
  async function chooseProduct(p:Product){
    if(!session)return;
    setSelected(p);setBusy("search-network");setMessage("");
    try{
      const term=(p.sku||p.oem_code||p.title).trim();
      const out=await searchNetworkProcurement(session,term,"price");
      setOffers(out);
      setQtyByOffer(Object.fromEntries(out.map(x=>[x.offer_id,1])));
    }catch(e){setOffers([]);setMessage(e instanceof Error?e.message:"جست‌وجوی شبکه انجام نشد");}
    finally{setBusy("");}
  }
  async function requestSupply(offer:NetworkSearchResult){
    if(!session||!selected)return;
    const amount=qtyByOffer[offer.offer_id]||1;
    if(amount<=0||amount>offer.available){setMessage("تعداد درخواستی با موجودی فروشنده سازگار نیست.");return;}
    if(!window.confirm(`درخواست ${qty(amount)} عدد از ${offer.store_name} به مبلغ ${money(amount*offer.selling_price)} ثبت شود؟`))return;
    setBusy(offer.offer_id);setMessage("");
    try{
      await createNetworkProcurement(session,{offerId:offer.offer_id,buyerProductId:selected.id,qty:amount});
      setMessage("درخواست تأمین ثبت شد و موجودی فروشنده برای این درخواست نگه داشته شد.");
      await Promise.all([loadOrders(),chooseProduct(selected)]);
    }catch(e){setMessage(e instanceof Error?e.message:"ثبت درخواست تأمین انجام نشد");}
    finally{setBusy("");}
  }
  async function sellerMove(id:string,next:"accepted"|"ready"|"rejected"){
    if(!session)return;setBusy(id);setMessage("");
    try{await transitionSellingProcurement(session,id,next);await loadOrders();}
    catch(e){setMessage(e instanceof Error?e.message:"تغییر وضعیت انجام نشد");}
    finally{setBusy("");}
  }
  async function cancelOrder(id:string){
    if(!session||!window.confirm("درخواست تأمین لغو شود و موجودی فروشنده آزاد شود؟"))return;
    setBusy(id);setMessage("");
    try{await cancelBuyingProcurement(session,id);await loadOrders();}
    catch(e){setMessage(e instanceof Error?e.message:"لغو درخواست انجام نشد");}
    finally{setBusy("");}
  }
  async function receiveOrder(x:NetworkProcurement){
    if(!session||!window.confirm(`دریافت ${qty(x.qty)} عدد از ${x.seller_store_name} ثبت شود؟ این کار خرید شما و فروش فروشنده را به‌صورت اعتباری ثبت می‌کند.`))return;
    setBusy(x.id);setMessage("");
    try{
      const out=await receiveNetworkProcurement(session,x.id);
      setMessage(`کالا وارد موجودی شد. سند خرید: ${out.buyer_purchase_id}`);
      await loadOrders();
    }catch(e){setMessage(e instanceof Error?e.message:"دریافت کالا انجام نشد");}
    finally{setBusy("");}
  }

  const activeBuying=useMemo(()=>buying.filter(x=>["requested","accepted","ready"].includes(x.status)).length,[buying]);
  const activeSelling=useMemo(()=>selling.filter(x=>["requested","accepted","ready"].includes(x.status)).length,[selling]);

  return <div>
    <div className="page-head"><div><span className="eyebrow">تأمین از شبکه</span><h1>خرید از فروشگاه‌های همکار</h1><p>کالای خودت را انتخاب کن، موجودی همکاران را ببین و درخواست تأمین را تا ورود خودکار به انبار پیگیری کن.</p></div><div className="order-tabs"><span className="network-count">{activeBuying} خرید باز</span><span className="network-count">{activeSelling} فروش باز</span></div></div>
    {message&&<div className="network-notice">{message}</div>}

    <section className="panel network-profile-panel">
      <div className="panel-head"><div><b>۱. کالای مورد نیاز فروشگاه</b><span>برای جلوگیری از ورود کالای اشتباه، ابتدا کالای مقصد در کاتالوگ خودت را انتخاب کن.</span></div></div>
      <div className="global-search" style={{maxWidth:620}}><input value={q} onChange={e=>setQ(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")void findLocal()}} placeholder="مثلاً لنت ۲۰۶ یا BRK-206-TXT"/><button className="primary-btn" disabled={busy==="search-local"||q.trim().length<2} onClick={()=>void findLocal()}>{busy==="search-local"?"...":"جست‌وجو"}</button></div>
      {localProducts.length>0&&<div className="network-offer-list" style={{marginTop:16}}>{localProducts.map(p=><button key={p.id} className={selected?.id===p.id?"table-action active":"table-action"} onClick={()=>void chooseProduct(p)}>{p.title} {p.sku?`· ${p.sku}`:""}</button>)}</div>}
    </section>

    <section className="panel network-offers-panel">
      <div className="panel-head"><div><b>۲. پیشنهادهای شبکه</b><span>{selected?`برای «${selected.title}»؛ فروشگاه خودت و پیشنهادهای غیرفعال حذف شده‌اند.`:"ابتدا کالای مقصد را انتخاب کن."}</span></div></div>
      {busy==="search-network"?<div className="empty-state">در حال جست‌وجوی شبکه...</div>:selected&&offers.length===0?<div className="empty-state">پیشنهاد قابل تأمینی برای این کالا پیدا نشد.</div>:<div className="network-offer-list">{offers.map(x=><div className="network-offer-row procurement-offer-row" key={x.offer_id}>
        <div className="offer-product"><b>{x.store_name}</b><span>{x.title} · {x.city||""}</span></div>
        <div className="offer-stock"><span>قابل تأمین</span><b>{qty(x.available)}</b></div>
        <div className="offer-price"><span>قیمت واحد</span><b>{money(x.selling_price)}</b><small>{x.freshness==="live"?"موجودی تازه":"آخرین بروزرسانی "+new Date(x.last_updated_at).toLocaleString("fa-IR")}</small></div>
        <label className="offer-price"><span>تعداد</span><input type="number" min="0.001" max={x.available} step="1" value={qtyByOffer[x.offer_id]??1} onChange={e=>setQtyByOffer(v=>({...v,[x.offer_id]:Number(e.target.value)}))}/><small>{money((qtyByOffer[x.offer_id]||1)*x.selling_price)}</small></label>
        <button className="primary-btn" disabled={busy===x.offer_id} onClick={()=>void requestSupply(x)}>{busy===x.offer_id?"...":"درخواست تأمین"}</button>
      </div>)}</div>}
    </section>

    <section className="panel store-orders-panel">
      <div className="panel-head"><div><b>خریدهای شبکه من</b><span>بعد از «آماده» شدن فروشنده، دریافت کالا همزمان خرید اعتباری و ورود موجودی را ثبت می‌کند.</span></div></div>
      {loading?<div className="empty-state">در حال دریافت...</div>:buying.length===0?<div className="empty-state">هنوز درخواست خرید شبکه ندارید.</div>:<div className="store-order-list">{buying.map(x=><article className="store-order" key={x.id}>
        <div className="store-order-main"><div><span className={`reservation-status ${x.status}`}>{statusLabel[x.status]}</span><h3>{x.buyer_product_title}</h3><p>از {x.seller_store_name} · {qty(x.qty)} عدد</p></div><div className="store-order-total"><b>{money(x.total_amount)}</b><span>{money(x.unit_price)} / عدد</span></div></div>
        <div className="store-order-foot"><small>مهلت: {new Date(x.expires_at).toLocaleString("fa-IR")}</small><div className="store-order-actions">
          {["requested","accepted","ready"].includes(x.status)&&<button className="danger-btn" disabled={busy===x.id} onClick={()=>void cancelOrder(x.id)}>لغو درخواست</button>}
          {x.status==="ready"&&<button className="success-btn" disabled={busy===x.id} onClick={()=>void receiveOrder(x)}>دریافت + ورود به انبار</button>}
          {x.status==="received"&&x.buyer_purchase_id&&<a className="table-action" href={`/store/purchases/${x.buyer_purchase_id}`}>مشاهده سند خرید</a>}
        </div></div>
      </article>)}</div>}
    </section>

    <section className="panel store-orders-panel">
      <div className="panel-head"><div><b>درخواست‌های خرید از فروشگاه من</b><span>تأیید، آماده‌سازی یا رد کن. موجودی از لحظه درخواست نگه داشته شده است.</span></div></div>
      {loading?<div className="empty-state">در حال دریافت...</div>:selling.length===0?<div className="empty-state">درخواست فروش شبکه ندارید.</div>:<div className="store-order-list">{selling.map(x=><article className="store-order" key={x.id}>
        <div className="store-order-main"><div><span className={`reservation-status ${x.status}`}>{statusLabel[x.status]}</span><h3>{x.seller_product_title}</h3><p>خریدار: {x.buyer_store_name} · {qty(x.qty)} عدد</p></div><div className="store-order-total"><b>{money(x.total_amount)}</b><span>فروش اعتباری بین فروشگاه‌ها</span></div></div>
        <div className="store-order-foot"><small>ثبت: {new Date(x.created_at).toLocaleString("fa-IR")}</small><div className="store-order-actions">
          {x.status==="requested"&&<><button className="success-btn" disabled={busy===x.id} onClick={()=>void sellerMove(x.id,"accepted")}>تأیید درخواست</button><button className="danger-btn" disabled={busy===x.id} onClick={()=>void sellerMove(x.id,"rejected")}>رد</button></>}
          {x.status==="accepted"&&<><button className="primary-btn" disabled={busy===x.id} onClick={()=>void sellerMove(x.id,"ready")}>آماده تحویل شد</button><button className="danger-btn" disabled={busy===x.id} onClick={()=>void sellerMove(x.id,"rejected")}>رد و آزادسازی</button></>}
          {x.status==="received"&&x.seller_sale_id&&<a className="table-action" href={`/store/sales/${x.seller_sale_id}`}>مشاهده سند فروش</a>}
        </div></div>
      </article>)}</div>}
    </section>
  </div>;
}
