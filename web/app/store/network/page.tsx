"use client";
import { useEffect,useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getNetworkStoreOffers,getNetworkStoreProfile,updateNetworkStoreOffer,updateNetworkStoreProfile } from "@/lib/api";
import type { NetworkStoreOffer,StoreNetworkProfile } from "@/lib/types";
const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v);

export default function StoreNetworkPage(){
 const {session}=useAuth(); const [profile,setProfile]=useState<StoreNetworkProfile|null>(null); const [items,setItems]=useState<NetworkStoreOffer[]>([]); const [busy,setBusy]=useState(""); const [message,setMessage]=useState("");
 async function load(){if(!session)return; const [p,o]=await Promise.all([getNetworkStoreProfile(session),getNetworkStoreOffers(session)]);setProfile(p);setItems(o);}
 useEffect(()=>{void load().catch(e=>setMessage(e.message));},[session]);
 async function saveProfile(){if(!session||!profile)return;setBusy("profile");setMessage("");try{await updateNetworkStoreProfile(session,profile);setMessage("تنظیمات شبکه ذخیره شد.");}catch(e){setMessage(e instanceof Error?e.message:"خطا");}finally{setBusy("");}}
 async function saveOffer(x:NetworkStoreOffer){if(!session)return;setBusy(x.product_id);setMessage("");try{await updateNetworkStoreOffer(session,x.product_id,x.selling_price,x.visible,x.allow_reservation);setMessage(`«${x.title}» بروزرسانی شد.`);}catch(e){setMessage(e instanceof Error?e.message:"خطا");}finally{setBusy("");}}
 function patch(id:string,v:Partial<NetworkStoreOffer>){setItems(xs=>xs.map(x=>x.product_id===id?{...x,...v}:x));}
 if(!profile)return <div className="table-state">در حال دریافت تنظیمات شبکه...</div>;
 return <div>
   <div className="page-head"><div><span className="eyebrow">شبکه فروشگاه‌ها</span><h1>نمایش موجودی در شبکه</h1><p>کنترل کن مکانیک‌ها چه کالا، قیمت و اطلاعات عمومی از فروشگاهت ببینند.</p></div><a className="ghost-btn" href="/mechanic" target="_blank">مشاهده صفحه مکانیک ↗</a></div>
   {message&&<div className="network-notice">{message}</div>}
   <section className="panel network-profile-panel"><div className="panel-head"><div><b>پروفایل عمومی فروشگاه</b><span>فقط این اطلاعات در جست‌وجوی مکانیک نمایش داده می‌شود.</span></div><label className="network-master"><input type="checkbox" checked={profile.network_enabled} onChange={e=>setProfile({...profile,network_enabled:e.target.checked})}/><span>{profile.network_enabled?"فروشگاه در شبکه فعال است":"فروشگاه از شبکه مخفی است"}</span></label></div>
     <div className="network-profile-grid"><label>شهر<input value={profile.city||""} onChange={e=>setProfile({...profile,city:e.target.value})}/></label><label>تلفن عمومی<input value={profile.phone||""} onChange={e=>setProfile({...profile,phone:e.target.value})}/></label><label className="span-2">آدرس عمومی<input value={profile.address||""} onChange={e=>setProfile({...profile,address:e.target.value})}/></label><label>عرض جغرافیایی<input type="number" step="0.000001" value={profile.latitude??""} onChange={e=>setProfile({...profile,latitude:e.target.value?Number(e.target.value):undefined})}/></label><label>طول جغرافیایی<input type="number" step="0.000001" value={profile.longitude??""} onChange={e=>setProfile({...profile,longitude:e.target.value?Number(e.target.value):undefined})}/></label></div>
     <div className="panel-actions"><button className="primary-btn" disabled={busy==="profile"} onClick={()=>void saveProfile()}>{busy==="profile"?"در حال ذخیره...":"ذخیره پروفایل شبکه"}</button></div>
   </section>
   <section className="panel network-offers-panel"><div className="panel-head"><div><b>کالاهای قابل نمایش</b><span>موجودی از انبار خوانده می‌شود؛ تو فقط قیمت عمومی و نمایش در شبکه را کنترل می‌کنی.</span></div><span className="network-count">{items.filter(x=>x.visible).length} کالای فعال</span></div>
     <div className="network-offer-list">{items.map(x=><div className="network-offer-row" key={x.product_id}><div className="offer-product"><b>{x.title}</b><span>{[x.brand,x.sku].filter(Boolean).join(" · ")}</span></div><div className="offer-stock"><span>قابل فروش</span><b>{x.available}</b></div><label className="offer-price"><span>قیمت فروش (تومان)</span><input type="number" min="1" value={x.selling_price||""} onChange={e=>patch(x.product_id,{selling_price:Number(e.target.value)})}/><small>{x.selling_price?money(x.selling_price)+" تومان":"قیمت وارد نشده"}</small></label><label className="offer-toggle"><input type="checkbox" checked={x.visible} onChange={e=>patch(x.product_id,{visible:e.target.checked})}/><span>نمایش</span></label><label className="offer-toggle"><input type="checkbox" checked={x.allow_reservation} onChange={e=>patch(x.product_id,{allow_reservation:e.target.checked})}/><span>رزرو</span></label><button className="table-action" disabled={busy===x.product_id||x.selling_price<=0} onClick={()=>void saveOffer(x)}>{busy===x.product_id?"...":"ذخیره"}</button></div>)}</div>
   </section>
 </div>
}
