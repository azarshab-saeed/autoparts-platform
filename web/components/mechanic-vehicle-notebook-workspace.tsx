"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import QRCode from "qrcode";
import { useAuth } from "@/components/auth-provider";
import { addVehicleNotebookEntry, getVehicleNotebookByToken, type VehicleNotebookDetail } from "@/lib/vehicle-notebook";
import { createMechanicVehicleNotebook, listMechanicVehicleNotebooks } from "@/lib/mechanic-vehicle-notebook";
import type { VehicleNotebookVehicle } from "@/lib/vehicle-notebook";

const n=(v?:number)=>v==null?"—":new Intl.NumberFormat("fa-IR").format(v);
const title=(v:VehicleNotebookVehicle)=>[v.make,v.model,v.trim].filter(Boolean).join(" ")||v.plate||"خودرو";

export default function MechanicVehicleNotebookWorkspace(){
  const {ready,session,login}=useAuth();
  const [items,setItems]=useState<VehicleNotebookVehicle[]>([]);
  const [detail,setDetail]=useState<VehicleNotebookDetail|null>(null);
  const [q,setQ]=useState("");
  const [token,setToken]=useState("");
  const [creating,setCreating]=useState(false);
  const [ownerCode,setOwnerCode]=useState("");
  const [qr,setQr]=useState("");
  const [error,setError]=useState("");
  const [notice,setNotice]=useState("");

  async function load(term=""){if(!session)return;try{setItems(await listMechanicVehicleNotebooks(session,term));}catch(e){setError(e instanceof Error?e.message:"خواندن خودروها ناموفق بود.");}}
  async function open(value:string){if(!session||!value.trim())return;setError("");try{const d=await getVehicleNotebookByToken(session,value.trim());setDetail(d);setToken(d.vehicle.public_token);if(typeof window!=="undefined")setQr(await QRCode.toDataURL(`${window.location.origin}/vehicle/${d.vehicle.public_token}`,{width:230,margin:1}));}catch(e){setError(e instanceof Error?e.message:"دفتر خودرو پیدا نشد.");}}
  useEffect(()=>{if(ready&&session?.role==="mechanic")void load();},[ready,session]);
  useEffect(()=>{if(typeof window==="undefined"||!session)return;const t=new URLSearchParams(location.search).get("token");if(t)void open(t);},[session]);

  async function create(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!session)return;const fd=new FormData(e.currentTarget);const year=String(fd.get("model_year")||"").trim();try{const v=await createMechanicVehicleNotebook(session,{plate:s(fd,"plate"),vin:s(fd,"vin"),owner_name:s(fd,"owner_name"),owner_phone:s(fd,"owner_phone"),make:s(fd,"make"),model:s(fd,"model"),trim:s(fd,"trim"),model_year:year?Number(year):undefined});setOwnerCode(v.owner_code||"");setCreating(false);await load();await open(v.public_token);setNotice("دفتر خودرو و QR ساخته شد.");}catch(err){setError(err instanceof Error?err.message:"ثبت خودرو ناموفق بود.");}}
  async function entry(e:FormEvent<HTMLFormElement>){e.preventDefault();if(!session||!detail)return;const fd=new FormData(e.currentTarget);const kind=String(fd.get("kind")||"service") as "service"|"part"|"mileage"|"note";const mileage=num(fd,"mileage");const next=num(fd,"next_due_mileage");try{await addVehicleNotebookEntry(session,detail.vehicle.public_token,{kind,title:String(fd.get("title")||"").trim(),mileage,next_due_mileage:next,notes:s(fd,"notes")});await open(detail.vehicle.public_token);setNotice("در دفتر خودرو ثبت شد.");(e.currentTarget as HTMLFormElement).reset();}catch(err){setError(err instanceof Error?err.message:"ثبت سابقه ناموفق بود.");}}

  if(!ready)return <div className="nw-page">در حال بارگذاری...</div>;
  if(!session)return <div className="nw-page"><div className="nw-panel"><b>برای استفاده از دفتر مکانیک وارد شوید.</b><button onClick={()=>void login("/mechanic/vehicle-notebook")}>ورود</button></div></div>;
  if(session.role!=="mechanic")return <div className="nw-page">این صفحه مخصوص مکانیک است.</div>;

  return <main className="nw-page">
    <header className="nw-header"><div><span>دفتر خودرو مکانیک</span><h1>خودرو را بساز یا QR را اسکن کن</h1><p>ساخت دفتر، ثبت سرویس و قطعه و تحویل QR به مالک.</p></div><div className="nw-header-actions"><Link href="/mechanic/workshop">کارگاه من</Link><Link href="/mechanic/accounts">حساب فروشگاه‌ها</Link><button className="primary" onClick={()=>setCreating(v=>!v)}>+ خودرو جدید</button></div></header>
    {error&&<div className="nw-alert error">{error}</div>}{notice&&<div className="nw-alert success">{notice}</div>}
    {creating&&<form className="nw-form-card" onSubmit={create}><h2>خودرو جدید</h2><div className="nw-grid"><label><span>پلاک *</span><input name="plate" autoFocus/></label><label><span>VIN</span><input name="vin"/></label><label><span>نام مالک</span><input name="owner_name"/></label><label><span>موبایل مالک</span><input name="owner_phone"/></label><label><span>سازنده</span><input name="make"/></label><label><span>مدل</span><input name="model"/></label><label><span>تیپ</span><input name="trim"/></label><label><span>سال</span><input name="model_year" inputMode="numeric"/></label></div><div className="nw-header-actions"><button className="primary">ساخت دفتر و QR</button><button type="button" onClick={()=>setCreating(false)}>انصراف</button></div></form>}
    {ownerCode&&<div className="nw-alert success">کد ۶ رقمی مالک: <b>{ownerCode}</b> — فقط به مالک بده.</div>}
    <section className="nw-columns"><div className="nw-panel"><div className="nw-panel-title"><h2>خودروهای من</h2></div><form className="nw-inline-form" style={{gridTemplateColumns:"1fr auto"}} onSubmit={e=>{e.preventDefault();void load(q)}}><input value={q} onChange={e=>setQ(e.target.value)} placeholder="پلاک، مالک یا مدل"/><button>جست‌وجو</button></form><div className="nw-list">{items.map(v=><button className={`nw-row ${detail?.vehicle.id===v.id?"active":""}`} key={v.id} onClick={()=>void open(v.public_token)}><div><b>{title(v)}</b><span>{v.plate||"بدون پلاک"}{v.owner_name?` · ${v.owner_name}`:""}</span></div><strong>باز کردن</strong></button>)}{!items.length&&<div className="nw-empty">هنوز خودرویی نساختی.</div>}</div></div>
      <div className="nw-panel"><div className="nw-panel-title"><h2>اسکن / شناسه QR</h2></div><div className="nw-inline-form" style={{gridTemplateColumns:"1fr auto"}}><input dir="ltr" value={token} onChange={e=>setToken(e.target.value)} placeholder="UUID QR token"/><button onClick={()=>void open(token)}>باز کردن</button></div>{detail?<><div className="nw-money-strip"><span>خودرو<b>{title(detail.vehicle)}</b></span><span>پلاک<b>{detail.vehicle.plate||"—"}</b></span><span>سوابق<b>{n(detail.entries.length)}</b></span><span>مالک<b>{detail.vehicle.owner_name||"خصوصی"}</b></span></div><form className="nw-form-card" onSubmit={entry}><h2>ثبت سریع</h2><div className="nw-grid"><label><span>نوع</span><select name="kind"><option value="service">سرویس</option><option value="part">قطعه</option><option value="mileage">کیلومتر</option><option value="note">یادداشت</option></select></label><label className="wide"><span>عنوان</span><input name="title" required placeholder="تعویض تسمه تایم"/></label><label><span>کیلومتر</span><input name="mileage" inputMode="numeric"/></label><label><span>سرویس بعدی در کیلومتر</span><input name="next_due_mileage" inputMode="numeric"/></label><label className="wide"><span>توضیح</span><input name="notes"/></label></div><button className="primary">ثبت در دفتر خودرو</button></form><div className="nw-panel" style={{marginTop:12}}><div className="nw-panel-title"><h2>سوابق</h2><Link href={`/vehicle/${detail.vehicle.public_token}`} target="_blank">نمای مالک</Link></div><div className="nw-list">{detail.entries.map(x=><div className="nw-row static" key={x.id}><div><b>{x.title}</b><span>{x.actor_name} · {x.mileage!=null?`${n(x.mileage)} km`:"بدون کیلومتر"}</span></div><strong>{x.kind}</strong></div>)}</div></div>{qr&&<div className="nw-form-card" style={{marginTop:12,display:"flex",gap:16,alignItems:"center",flexWrap:"wrap"}}><img src={qr} alt="QR خودرو" width={180} height={180}/><div><b>QR ثابت خودرو</b><p>مالک این QR را نگه می‌دارد و هر مکانیک بعدی می‌تواند تاریخچه را ببیند.</p></div></div>}</>:<div className="nw-empty">یک خودرو را انتخاب کن یا QR را وارد کن.</div>}</div></section>
  </main>;
}
function s(fd:FormData,k:string){const v=String(fd.get(k)||"").trim();return v||undefined}
function num(fd:FormData,k:string){const v=String(fd.get(k)||"").trim();return v?Number(v):undefined}
