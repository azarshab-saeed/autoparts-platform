"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { createNetworkReservation, getVehicleCatalog, searchNetwork } from "@/lib/api";
import type { NetworkSearchResult, VehicleMake } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";
import Modal from "@/components/modal";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const number=(v:number)=>new Intl.NumberFormat("fa-IR",{maximumFractionDigits:1}).format(v);
type Sort="best"|"price"|"distance"|"fresh";
type SavedVehicle={makeId:string;modelId:string;variantId:string;year:string;label:string};
const vehicleKey="autoparts.saved-vehicle.v1";
const recentKey="autoparts.recent-searches.v1";

const reasonLabel=(v?:NetworkSearchResult["match_reason"])=>v==="exact_code"?"تطبیق دقیق کد":v==="exact_alias"?"نام معادل":v==="vehicle_fitment"?"سازگار با خودرو":v==="title"?"تطبیق نام قطعه":"تطبیق جست‌وجو";

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
  const [vehicles,setVehicles]=useState<VehicleMake[]>([]);
  const [makeId,setMakeId]=useState("");
  const [modelId,setModelId]=useState("");
  const [variantId,setVariantId]=useState("");
  const [year,setYear]=useState("");
  const [savedVehicle,setSavedVehicle]=useState<SavedVehicle|null>(null);
  const [recentSearches,setRecentSearches]=useState<string[]>([]);

  useEffect(()=>{void getVehicleCatalog().then(setVehicles).catch(()=>setVehicles([]));try{const raw=localStorage.getItem(recentKey);if(raw)setRecentSearches(JSON.parse(raw).filter((x:unknown)=>typeof x==="string").slice(0,5));}catch{}},[]);
  useEffect(()=>{
    if(!vehicles.length)return;
    try{
      const raw=localStorage.getItem(vehicleKey);if(!raw)return;const saved=JSON.parse(raw) as SavedVehicle;
      const make=vehicles.find(x=>x.id===saved.makeId);const model=make?.models.find(x=>x.id===saved.modelId);const variant=model?.variants.find(x=>x.id===saved.variantId);
      if(make&&model&&variant){setSavedVehicle(saved);setMakeId(saved.makeId);setModelId(saved.modelId);setVariantId(saved.variantId);setYear(saved.year||"");}
    }catch{}
  },[vehicles]);
  const selectedMake=useMemo(()=>vehicles.find(x=>x.id===makeId),[vehicles,makeId]);
  const selectedModel=useMemo(()=>selectedMake?.models.find(x=>x.id===modelId),[selectedMake,modelId]);
  const selectedVariant=useMemo(()=>selectedModel?.variants.find(x=>x.id===variantId),[selectedModel,variantId]);

  function rememberSearch(term:string){const clean=term.trim();if(clean.length<2)return;setRecentSearches(prev=>{const next=[clean,...prev.filter(x=>x!==clean)].slice(0,5);try{localStorage.setItem(recentKey,JSON.stringify(next));}catch{}return next;});}
  async function run(e?:FormEvent, overrideSort?:Sort, overrideQuery?:string){
    e?.preventDefault(); setError(""); setNotice("");
    const term=overrideQuery??q;
    if(term.trim().length<2&&!variantId){setError("حداقل دو حرف از نام یا کد قطعه را وارد کن یا خودرو را انتخاب کن.");return;}
    const parsedYear=year?Number(year):undefined;
    if(year&&(!Number.isInteger(parsedYear)||Number(parsedYear)<1200||Number(parsedYear)>2200)){setError("سال مدل را بین ۱۲۰۰ تا ۲۲۰۰ وارد کن.");return;}
    setLoading(true);
    try{
      setItems(await searchNetwork(term,{sort:overrideSort||sort,lat:location?.lat,lng:location?.lng,limit:40,vehicleVariantId:variantId||undefined,year:parsedYear}));
      setSearched(true);rememberSearch(term);
    }catch(err){setError(err instanceof Error?err.message:"جست‌وجو انجام نشد.");}finally{setLoading(false);}
  }
  function locate(){
    if(!navigator.geolocation){setError("مرورگر شما موقعیت مکانی را پشتیبانی نمی‌کند.");return;}
    navigator.geolocation.getCurrentPosition(p=>{setLocation({lat:p.coords.latitude,lng:p.coords.longitude});setError("");},()=>setError("اجازه دسترسی به موقعیت داده نشد."),{enableHighAccuracy:false,timeout:7000});
  }
  function saveCurrentVehicle(){
    if(!selectedMake||!selectedModel||!selectedVariant)return;
    const saved:SavedVehicle={makeId:selectedMake.id,modelId:selectedModel.id,variantId:selectedVariant.id,year,label:`${selectedMake.name} ${selectedModel.name} ${selectedVariant.name}${selectedVariant.engine_code?` · ${selectedVariant.engine_code}`:""}`};
    try{localStorage.setItem(vehicleKey,JSON.stringify(saved));}catch{}setSavedVehicle(saved);setNotice("خودرو ذخیره شد؛ دفعه بعد همین خودرو خودکار فعال می‌شود.");
  }
  function clearSavedVehicle(){try{localStorage.removeItem(vehicleKey);}catch{}setSavedVehicle(null);setMakeId("");setModelId("");setVariantId("");setYear("");}
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
  const suggestions=[...new Set([...recentSearches,"لنت 206","فیلتر روغن 206","واتر پمپ EF7","4254.97"])].slice(0,6);

  return <main className="mechanic-page polished-mechanic adoption-mechanic">
    <header className="mechanic-header">
      <Link href="/mechanic" className="mechanic-brand"><span>ی</span><div><b>شبکه قطعات</b><small>موجودی واقعی فروشگاه‌ها</small></div></Link>
      <div className="mechanic-account">
        {ready&&session ? <><span className="account-name">{session.displayName}</span>{(session.role==="mechanic"||session.role==="consumer")&&<Link className="orders-link" href="/mechanic/orders">رزروهای من</Link>}{session.role==="mechanic"&&<Link className="orders-link" href="/mechanic/vehicle-notebook">دفتر خودرو</Link>}{session.role==="mechanic"&&<Link className="orders-link" href="/mechanic/workshop">کارگاه من</Link>}{session.role==="mechanic"&&<Link className="orders-link" href="/mechanic/accounts">حساب فروشگاه‌ها</Link>}{session.role!=="mechanic"&&session.role!=="consumer"&&<Link className="orders-link" href="/store">پنل فروشگاه</Link>}<button onClick={()=>void logout()}>خروج</button></>:
          <button className="mechanic-login" onClick={()=>void login("/mechanic")}>ورود / رزرو قطعه</button>}
      </div>
    </header>

    <section className="mechanic-hero">
      <div className="hero-copy"><span className="hero-badge">جست‌وجوی هوشمند قطعه</span><h1>قطعه درست، نزدیک‌ترین فروشگاه.</h1><p>خودرو را یک بار ذخیره کن؛ بعد فقط اسم قطعه را بنویس و موجودی، قیمت و فاصله فروشگاه‌ها را ببین.</p></div>
      {savedVehicle&&<div className="saved-vehicle-banner"><div><span>🚘</span><div><small>خودروی ذخیره‌شده</small><b>{savedVehicle.label}</b></div></div><div>{savedVehicle.year&&<em>سال {new Intl.NumberFormat("fa-IR",{useGrouping:false}).format(Number(savedVehicle.year))}</em>}<button onClick={clearSavedVehicle}>تغییر خودرو</button></div></div>}
      <form className="mechanic-search" onSubmit={run}><div className="mechanic-search-input"><span>⌕</span><input value={q} onChange={e=>setQ(e.target.value)} placeholder={savedVehicle?"مثلاً لنت، فیلتر روغن، واتر پمپ":"مثلاً لنت 206، 4254.97، واتر پمپ EF7"} autoFocus/></div><button disabled={loading}>{loading?<><i className="button-spinner"/>در حال جست‌وجو</>:"جست‌وجوی قطعه"}</button></form>
      <div className="search-suggestions"><span>{recentSearches.length?"جست‌وجوهای اخیر:":"جست‌وجوی سریع:"}</span>{suggestions.map(v=><button type="button" key={v} onClick={()=>{setQ(v);void run(undefined,undefined,v)}}>{v}</button>)}</div>

      <div className="vehicle-filter-card">
        <div className="vehicle-filter-copy"><span className="vehicle-icon">🚘</span><div><b>{savedVehicle?"خودروی فعال":"خودروی مشتری"}</b><span>{savedVehicle?"نتایج با خودروی ذخیره‌شده فیلتر می‌شوند.":"یک بار انتخاب و ذخیره کن تا دفعات بعد تکرار نشود."}</span></div></div>
        <select aria-label="سازنده خودرو" value={makeId} onChange={e=>{setMakeId(e.target.value);setModelId("");setVariantId("");setYear("");}}><option value="">سازنده</option>{vehicles.map(x=><option key={x.id} value={x.id}>{x.name}</option>)}</select>
        <select aria-label="مدل خودرو" value={modelId} disabled={!selectedMake} onChange={e=>{setModelId(e.target.value);setVariantId("");setYear("");}}><option value="">مدل</option>{selectedMake?.models.map(x=><option key={x.id} value={x.id}>{x.name}</option>)}</select>
        <select aria-label="تیپ خودرو" value={variantId} disabled={!selectedModel} onChange={e=>{setVariantId(e.target.value);const v=selectedModel?.variants.find(x=>x.id===e.target.value);setYear(v?.year_to?String(v.year_to):"");}}><option value="">تیپ / موتور</option>{selectedModel?.variants.map(x=><option key={x.id} value={x.id}>{x.name}{x.engine_code?` · ${x.engine_code}`:""}</option>)}</select>
        <input aria-label="سال مدل" className="vehicle-year" inputMode="numeric" value={year} disabled={!variantId} onChange={e=>setYear(e.target.value)} placeholder="سال مدل"/>
        {selectedVariant&&!savedVehicle&&<button type="button" className="vehicle-save" onClick={saveCurrentVehicle}>ذخیره خودرو</button>}
        {(makeId||modelId||variantId)&&!savedVehicle&&<button type="button" className="vehicle-clear" onClick={()=>{setMakeId("");setModelId("");setVariantId("");setYear("");}}>پاک کردن</button>}
      </div>
      {selectedVariant&&<div className="selected-vehicle"><span>✓</span> خودرو: <b>{selectedMake?.name} {selectedModel?.name} {selectedVariant.name}</b>{selectedVariant.engine_code&&<em>موتور {selectedVariant.engine_code}</em>}{year&&<em>سال {new Intl.NumberFormat("fa-IR",{useGrouping:false}).format(Number(year))}</em>}{!savedVehicle&&<button onClick={saveCurrentVehicle}>ذخیره برای دفعه بعد</button>}</div>}
      <div className="mechanic-filters"><div className="sort-pills">{([['best','مرتبط‌ترین'],['price','ارزان‌ترین'],['distance','نزدیک‌ترین'],['fresh','تازه‌ترین']] as [Sort,string][]).map(([v,l])=><button type="button" key={v} className={sort===v?"active":""} onClick={()=>{setSort(v);if(searched)void run(undefined,v);}}>{l}</button>)}</div><button type="button" className={location?"location-btn active":"location-btn"} onClick={locate}>{location?"✓ موقعیت فعال است":"⌖ استفاده از موقعیت من"}</button></div>
      {error&&<div className="mechanic-error">{error}</div>}{notice&&<div className="mechanic-success">{notice} {notice.startsWith("رزرو")&&<Link href="/mechanic/orders">مشاهده رزروها ←</Link>}</div>}
    </section>

    <section className="mechanic-results">
      {!searched&&!loading&&<div className="mechanic-start"><span className="start-icon">⌕</span><b>{savedVehicle?`برای ${savedVehicle.label} چه قطعه‌ای می‌خواهی؟`:"با نام، کد یا خودرو جست‌وجو کن"}</b><span>{savedVehicle?"فقط نام قطعه را بنویس؛ سازگاری خودرو از قبل اعمال شده است.":"OEM «4254.97» یا خودروی «پژو 206 تیپ 5» را امتحان کن."}</span><button onClick={()=>void run()}>نمایش نتایج نمونه</button></div>}
      {searched&&<div className="results-head"><div><b>{items.length} نتیجه پیدا شد</b><span>{items.length?`شروع قیمت از ${money(cheapest)}`:"قطعه سازگار با موجودی قابل نمایش پیدا نشد."}</span></div>{location&&<small>⌖ فاصله‌ها با موقعیت فعلی شما محاسبه شده‌اند.</small>}</div>}
      <div className="network-cards">{items.map((x,i)=><article className={`network-card${i===0&&sort==="best"?" featured":""}`} key={x.offer_id}>
        <div className="network-card-top"><div><div className="network-title-row"><h2>{x.title}</h2>{i===0&&sort==="best"&&<span className="best-tag">★ پیشنهاد اول</span>}</div><p>{[x.brand,x.oem_code,x.sku].filter(Boolean).join(" · ")}</p></div><div className="network-price"><b>{money(x.selling_price)}</b>{x.selling_price===cheapest&&<span>کمترین قیمت</span>}</div></div>
        <div className="fitment-strip"><span className="match-reason">{reasonLabel(x.match_reason)}</span>{x.fitment_match&&<span className="fitment-ok">✓ سازگار با خودروی انتخابی</span>}{x.fitment_summary&&<span className="fitment-summary">{x.fitment_summary}</span>}</div>
        <div className="store-row"><div className="store-avatar">{x.store_name.slice(0,1)}</div><div className="store-info"><b>{x.store_name}</b><span>{x.address||x.city||"آدرس ثبت نشده"}</span></div>{x.distance_km!=null&&<div className="distance"><b>{number(x.distance_km)}</b><span>کیلومتر</span></div>}</div>
        <div className="network-meta"><span className="stock-ok"><i/> {number(x.available)} عدد موجود</span><span className={`fresh ${x.freshness}`}>{x.freshness==="live"?"موجودی لحظه‌ای":x.freshness==="recent"?"بروزرسانی امروز":"نیاز به تأیید"}</span>{x.allow_reservation&&<span className="reserve-tag">قابل رزرو</span>}</div>
        <div className="network-actions">{x.allow_reservation&&<button className="reserve-action" onClick={()=>openReserve(x)}>رزرو قطعه</button>}{x.phone&&<a className="primary-action" href={`tel:${x.phone}`}>☎ تماس</a>}<a className="secondary-action" target="_blank" rel="noreferrer" href={`https://www.google.com/maps/search/?api=1&query=${encodeURIComponent(x.address||x.store_name)}`}>⌖ مسیریابی</a></div>
      </article>)}</div>
    </section>

    <Modal open={Boolean(reserve)} title="رزرو قطعه" subtitle={reserve?`${reserve.title} · ${reserve.store_name}`:""} onClose={()=>!reserving&&setReserve(null)}>
      {reserve&&<div className="reservation-modal"><div className="reservation-summary"><span>قیمت هر عدد</span><b>{money(reserve.selling_price)}</b><span>موجودی قابل رزرو</span><b>{number(reserve.available)} عدد</b></div>{reserve.fitment_summary&&<div className="reservation-fitment">سازگاری ثبت‌شده: {reserve.fitment_summary}</div>}<label>تعداد<input type="number" min={1} max={Math.floor(reserve.available)} step="1" value={qty} onChange={e=>setQty(Math.max(1,Math.min(Math.floor(reserve.available),Number(e.target.value)||1)))}/></label><div className="reservation-total"><span>مبلغ تقریبی</span><b>{money(reserve.selling_price*qty)}</b></div><p>رزرو برای ۳۰ دقیقه موجودی را نگه می‌دارد و فروشگاه باید درخواست را تأیید کند.</p><button className="primary-btn wide" disabled={reserving} onClick={()=>void confirmReserve()}>{reserving?"در حال ثبت...":"ثبت رزرو"}</button></div>}
    </Modal>
  </main>;
}
