"use client";

import Link from "next/link";
import { FormEvent, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { addOwnerMileage, getPublicVehicleNotebook, type PublicVehicleNotebookDetail } from "@/lib/vehicle-notebook";

const faNumber=(v?:number)=>v==null?"—":new Intl.NumberFormat("fa-IR").format(v);
const faDate=(v?:string)=>v?new Intl.DateTimeFormat("fa-IR",{dateStyle:"medium"}).format(new Date(v)):"—";
const today=()=>new Date().toISOString().slice(0,10);

export default function VehiclePublicPage(){
  const params=useParams<{token:string}>();
  const token=String(params?.token||"");
  const {ready,session}=useAuth();
  const [data,setData]=useState<PublicVehicleNotebookDetail|null>(null);
  const [loading,setLoading]=useState(true);
  const [error,setError]=useState("");
  const [notice,setNotice]=useState("");
  const [showMileage,setShowMileage]=useState(false);
  const [ownerCode,setOwnerCode]=useState("");

  async function load(){
    if(!token)return;
    setLoading(true);setError("");
    try{setData(await getPublicVehicleNotebook(token));}
    catch(e){setError(e instanceof Error?e.message:"دفتر خودرو پیدا نشد.");}
    finally{setLoading(false);}
  }
  useEffect(()=>{void load();},[token]);
  useEffect(()=>{if(!token||typeof window==="undefined")return;setOwnerCode(localStorage.getItem(`vehicle-owner-code:${token}`)||"");},[token]);

  const latestMileage=useMemo(()=>data?.entries.find(x=>x.mileage!=null)?.mileage,[data]);
  const next=useMemo(()=>data?.entries.find(x=>x.next_due_mileage!=null||x.next_due_date),[data]);
  const title=[data?.vehicle.make,data?.vehicle.model,data?.vehicle.trim].filter(Boolean).join(" ")||"دفتر خودرو";
  const workHref=session?.role==="mechanic"?`/mechanic/vehicle-notebook?token=${token}`:session&&session.role!=="consumer"?`/store/vehicle-notebook?token=${token}`:"";

  async function saveMileage(e:FormEvent<HTMLFormElement>){
    e.preventDefault();setError("");setNotice("");
    const fd=new FormData(e.currentTarget);const code=String(fd.get("owner_code")||"").trim();const mileage=Number(fd.get("mileage"));const date=String(fd.get("occurred_on")||"");
    if(!code||!Number.isFinite(mileage)||mileage<0){setError("کد مالک و کیلومتر را درست وارد کنید.");return;}
    setLoading(true);
    try{
      await addOwnerMileage(token,code,mileage,date?`${date}T00:00:00Z`:undefined);
      if(typeof window!=="undefined")localStorage.setItem(`vehicle-owner-code:${token}`,code);
      setOwnerCode(code);setShowMileage(false);setNotice("کیلومتر ثبت شد.");await load();
    }catch(err){setError(err instanceof Error?err.message:"ثبت کیلومتر انجام نشد.");setLoading(false);}
  }

  return <main className="vehicle-owner-page">
    <header className="vehicle-owner-header"><Link href="/">یدکی‌نت</Link><span>دفتر خودرو</span></header>
    <section className="vehicle-owner-card">
      {loading&&!data&&<div className="vehicle-book-loading">در حال باز کردن دفتر خودرو...</div>}
      {error&&<div className="vehicle-book-alert error">{error}</div>}
      {notice&&<div className="vehicle-book-alert success">{notice}</div>}
      {data&&<>
        <div className="vehicle-owner-identity"><span>🚘</span><div><small>خودروی من</small><h1>{title}</h1><p>{data.vehicle.plate_masked||"پلاک نمایش داده نمی‌شود"}{data.vehicle.model_year?` · مدل ${faNumber(data.vehicle.model_year)}`:""}</p></div></div>
        <div className="vehicle-owner-stats"><div><span>آخرین کیلومتر</span><b>{faNumber(latestMileage)}</b></div><div><span>موعد بعدی</span><b>{next?.next_due_mileage?`${faNumber(next.next_due_mileage)} km`:next?.next_due_date?faDate(next.next_due_date):"ثبت نشده"}</b></div></div>
        <div className="vehicle-owner-actions"><button onClick={()=>setShowMileage(v=>!v)}>ثبت کیلومتر</button>{workHref&&<Link href={workHref}>ثبت سرویس / قطعه</Link>}</div>
        {showMileage&&<form className="vehicle-owner-mileage" onSubmit={saveMileage}><label><span>کیلومتر فعلی</span><input name="mileage" inputMode="numeric" autoFocus placeholder="مثلاً 92000"/></label><label><span>کد مالک</span><input name="owner_code" inputMode="numeric" defaultValue={ownerCode} placeholder="۶ رقمی"/></label><label><span>تاریخ</span><input name="occurred_on" type="date" defaultValue={today()}/></label><button className="primary" disabled={loading}>ثبت</button><small>کد مالک را فروشگاه هنگام ساخت QR به شما می‌دهد و بعد از اولین ثبت روی همین دستگاه ذخیره می‌شود.</small></form>}

        <section className="vehicle-owner-history"><div className="vehicle-book-section-title"><div><b>سوابق خودرو</b><span>نام و اطلاعات شخصی مالک برای دیگران نمایش داده نمی‌شود.</span></div></div>{data.entries.map(e=><article key={e.id}><div className={`entry-dot ${e.kind}`}>{e.kind==="service"?"🔧":e.kind==="part"?"⚙️":e.kind==="mileage"?"◉":"•"}</div><div><div className="entry-title"><b>{e.title}</b><span>{faDate(e.occurred_on)}</span></div><div className="entry-meta">{e.mileage!=null&&<span>{faNumber(e.mileage)} کیلومتر</span>}<span>{e.actor_name}</span>{e.owner_reported&&<span className="owner-report">ثبت مالک</span>}</div>{(e.next_due_mileage||e.next_due_date)&&<div className="entry-next">موعد بعدی: {e.next_due_mileage?`${faNumber(e.next_due_mileage)} کیلومتر`:""}{e.next_due_mileage&&e.next_due_date?" یا ":""}{e.next_due_date?faDate(e.next_due_date):""}</div>}</div></article>)}{!data.entries.length&&<div className="vehicle-book-empty">هنوز سابقه‌ای ثبت نشده است.</div>}</section>
        {!ready&&<div/>}
      </>}
    </section>
  </main>;
}
