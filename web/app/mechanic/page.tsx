"use client";

import { FormEvent, useMemo, useState } from "react";
import Link from "next/link";
import { createNetworkReservation, searchNetwork } from "@/lib/api";
import type { NetworkSearchResult } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import Modal from "@/components/modal";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const number=(v:number)=>new Intl.NumberFormat("fa-IR",{maximumFractionDigits:1}).format(v);
type Sort="best"|"price"|"distance"|"fresh";

export default function MechanicPage(){
  const {ready,session,login,logout}=useAuth();
  const [q,setQ]=useState("لنت 206");
  const [sort,setSort]=useState<Sort>("best");
  const [items,setItems]=useState<NetworkSearchResult[]>([]);
  const [loading,setLoading]=useState(false);
  const [error,setError]=useState("");
  const [notice,setNotice]=useState("");
  const [location,setLocation]=useState<{lat:number;lng:number}|null>(null);
  const [searched,setSearched]=useState(false);
  const [reserve,setReserve]=useState<NetworkSearchResult|null>(null);
  const [qty,setQty]=useState(1);
  const [reserving,setReserving]=useState(false);

  async function run(e?:FormEvent, overrideSort?:Sort){
    e?.preventDefault(); setError(""); setNotice("");
    if(q.trim().length<2){setError("حداقل دو حرف از نام یا کد قطعه را وارد کن.");return;}
    setLoading(true);
    try{setItems(await searchNetwork(q,{sort:overrideSort||sort,lat:location?.lat,lng:location?.lng,limit:40}));setSearched(true);}catch(err){setError(err instanceof Error?err.message:"جست‌وجو انجام نشد.");}finally{setLoading(false);}
  }
  function locate(){
    if(!navigator.geolocation){setError("مرورگر شما موقعیت مکانی را پشتیبانی نمی‌کند.");return;}
    navigator.geolocation.getCurrentPosition(p=>{setLocation({lat:p.coords.latitude,lng:p.coords.longitude});setError("");},()=>setError("اجازه دسترسی به موقعیت داده نشد."),{enableHighAccuracy:false,timeout:7000});
  }
  function openReserve(x:NetworkSearchResult){
    setError("");setNotice("");
    if(!session){void login("/mechanic");return;}
    if(session.role!=="mechanic"&&session.role!=="consumer"){setError("برای رزرو با حساب مکانیک یا کاربر عادی وارد شوید.");return;}
    setQty(1);setReserve(x);
  }
  async function confirmReserve(){
    if(!reserve||!session)return;
    setReserving(true);setError("");
    try{
      await createNetworkReservation(session,reserve.offer_id,qty);
      setItems(xs=>xs.map(x=>x.offer_id===reserve.offer_id?{...x,available:Math.max(0,x.available-qty)}:x));
      setNotice(`رزرو ${reserve.title} ثبت شد. فروشگاه باید آن را تأیید کند.`);setReserve(null);
    }catch(err){setError(err instanceof Error?err.message:"رزرو انجام نشد.");}finally{setReserving(false);}
  }
  const cheapest=useMemo(()=>items.length?Math.min(...items.map(x=>x.selling_price)):0,[items]);

  return <main className="mechanic-page">
    <header className="mechanic-header">
      <Link href="/mechanic" className="mechanic-brand"><span>ی</span><b>شبکه قطعات</b></Link>
      <div className="mechanic-account">
        {ready&&session ? <><span>{session.displayName}</span>{(session.role==="mechanic"||session.role==="consumer")&&<Link href="/mechanic/orders">رزروهای من</Link>}{session.role!=="mechanic"&&session.role!=="consumer"&&<Link href="/store">پنل فروشگاه</Link>}<button onClick={()=>void logout()}>خروج</button></>:
          <button className="mechanic-login" onClick={()=>void login("/mechanic")}>ورود مکانیک</button>}
      </div>
    </header>

    <section className="mechanic-hero">
      <div className="hero-copy"><span className="eyebrow">موجودی واقعی فروشگاه‌ها</span><h1>قطعه را پیدا کن، نه مغازه را.</h1><p>نام قطعه، خودرو، برند یا کد فنی را بنویس؛ موجودی، قیمت و امکان رزرو فروشگاه‌های شبکه را یکجا ببین.</p></div>
      <form className="mechanic-search" onSubmit={run}><input value={q} onChange={e=>setQ(e.target.value)} placeholder="مثلاً لنت جلو 206، واترپمپ EF7، 206-FB-01" autoFocus/><button disabled={loading}>{loading?"در حال جست‌وجو...":"جست‌وجو"}</button></form>
      <div className="mechanic-filters"><div className="sort-pills">{([['best','پیشنهاد بهتر'],['price','ارزان‌ترین'],['distance','نزدیک‌ترین'],['fresh','تازه‌ترین']] as [Sort,string][]).map(([v,l])=><button type="button" key={v} className={sort===v?"active":""} onClick={()=>{setSort(v);if(searched)void run(undefined,v);}}>{l}</button>)}</div><button type="button" className={location?"location-btn active":"location-btn"} onClick={locate}>{location?"✓ موقعیت فعال است":"⌖ استفاده از موقعیت من"}</button></div>
      {error&&<div className="mechanic-error">{error}</div>}{notice&&<div className="mechanic-success">{notice} <Link href="/mechanic/orders">مشاهده رزروها</Link></div>}
    </section>

    <section className="mechanic-results">
      {!searched&&!loading&&<div className="mechanic-start"><b>جست‌وجو را شروع کن</b><span>مثلاً «لنت 206» را بزن تا چند فروشگاه نمونه را ببینی.</span><button onClick={()=>void run()}>نمایش نمونه</button></div>}
      {searched&&<div className="results-head"><div><b>{items.length} نتیجه</b><span>{items.length?`شروع قیمت از ${money(cheapest)}`:"قطعه‌ای با موجودی قابل نمایش پیدا نشد."}</span></div>{location&&<small>فاصله‌ها با موقعیت فعلی شما محاسبه شده‌اند.</small>}</div>}
      <div className="network-cards">{items.map((x,i)=><article className="network-card" key={x.offer_id}>
        <div className="network-card-top"><div><div className="network-title-row"><h2>{x.title}</h2>{i===0&&sort==="best"&&<span className="best-tag">پیشنهاد</span>}</div><p>{[x.brand,x.oem_code].filter(Boolean).join(" · ")}</p></div><div className="network-price"><b>{money(x.selling_price)}</b>{x.selling_price===cheapest&&<span>کمترین قیمت</span>}</div></div>
        <div className="store-row"><div className="store-avatar">{x.store_name.slice(0,1)}</div><div className="store-info"><b>{x.store_name}</b><span>{x.address||x.city||"آدرس ثبت نشده"}</span></div>{x.distance_km!=null&&<div className="distance"><b>{number(x.distance_km)}</b><span>کیلومتر</span></div>}</div>
        <div className="network-meta"><span className="stock-ok">● {number(x.available)} عدد موجود</span><span className={`fresh ${x.freshness}`}>{x.freshness==="live"?"موجودی لحظه‌ای":x.freshness==="recent"?"بروزرسانی امروز":"نیاز به تأیید"}</span>{x.allow_reservation&&<span className="reserve-tag">قابل رزرو</span>}</div>
        <div className="network-actions">{x.allow_reservation&&<button className="reserve-action" onClick={()=>openReserve(x)}>رزرو قطعه</button>}{x.phone&&<a className="primary-action" href={`tel:${x.phone}`}>تماس</a>}<a className="secondary-action" target="_blank" rel="noreferrer" href={`https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(x.address||x.store_name)}`}>مسیریابی</a></div>
      </article>)}</div>
    </section>

    <Modal open={Boolean(reserve)} title="رزرو قطعه" subtitle={reserve?`${reserve.title} · ${reserve.store_name}`:""} onClose={()=>!reserving&&setReserve(null)}>
      {reserve&&<div className="reservation-modal"><div className="reservation-summary"><span>قیمت هر عدد</span><b>{money(reserve.selling_price)}</b><span>موجودی قابل رزرو</span><b>{number(reserve.available)} عدد</b></div><label>تعداد<input type="number" min={1} max={Math.floor(reserve.available)} step="1" value={qty} onChange={e=>setQty(Math.max(1,Math.min(Math.floor(reserve.available),Number(e.target.value)||1)))}/></label><div className="reservation-total"><span>مبلغ تقریبی</span><b>{money(reserve.selling_price*qty)}</b></div><p>رزرو برای ۳۰ دقیقه موجودی را نگه می‌دارد و فروشگاه باید درخواست را تأیید کند.</p><button className="primary-btn wide" disabled={reserving} onClick={()=>void confirmReserve()}>{reserving?"در حال ثبت...":"ثبت رزرو"}</button></div>}
    </Modal>
  </main>;
}
