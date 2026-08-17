"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { getProductSearchMetadata, getVehicleCatalog, searchProducts, updateProductSearchMetadata } from "@/lib/api";
import type { Product, ProductFitmentInput, ProductSearchMetadata, ProductSearchTerm, VehicleMake } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";

const yearText=(from?:number,to?:number)=>from&&to?`${from} تا ${to}`:from?`از ${from}`:to?`تا ${to}`:"همه سال‌ها";

export default function FitmentPage(){
  const {session}=useAuth();
  const [products,setProducts]=useState<Product[]>([]);
  const [query,setQuery]=useState("");
  const [selected,setSelected]=useState<Product|null>(null);
  const [metadata,setMetadata]=useState<ProductSearchMetadata|null>(null);
  const [vehicles,setVehicles]=useState<VehicleMake[]>([]);
  const [loading,setLoading]=useState(true);
  const [saving,setSaving]=useState(false);
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");
  const [termKind,setTermKind]=useState<ProductSearchTerm["kind"]>("alias");
  const [term,setTerm]=useState("");
  const [makeId,setMakeId]=useState("");
  const [modelId,setModelId]=useState("");
  const [variantId,setVariantId]=useState("");
  const [yearFrom,setYearFrom]=useState("");
  const [yearTo,setYearTo]=useState("");
  const [notes,setNotes]=useState("");

  const loadProducts=useCallback(async(q="")=>{if(!session)return;setLoading(true);try{setProducts(await searchProducts(q,session));}catch(e){setError(e instanceof Error?e.message:"دریافت کالاها ناموفق بود.");}finally{setLoading(false);}},[session]);
  useEffect(()=>{void Promise.all([loadProducts(),getVehicleCatalog().then(setVehicles)]);},[loadProducts]);
  useEffect(()=>{if(!session||!selected)return;setMetadata(null);setError("");void getProductSearchMetadata(session,selected.id).then(setMetadata).catch(e=>setError(e instanceof Error?e.message:"دریافت فیتمنت ناموفق بود."));},[session,selected]);

  const selectedMake=useMemo(()=>vehicles.find(x=>x.id===makeId),[vehicles,makeId]);
  const selectedModel=useMemo(()=>selectedMake?.models.find(x=>x.id===modelId),[selectedMake,modelId]);
  const selectedVariant=useMemo(()=>selectedModel?.variants.find(x=>x.id===variantId),[selectedModel,variantId]);

  function addTerm(){
    if(!metadata||term.trim().length<2)return;
    const next={kind:termKind,term:term.trim()};
    if(metadata.terms.some(x=>x.kind===next.kind&&x.term.toLowerCase()===next.term.toLowerCase()))return;
    setMetadata({...metadata,terms:[...metadata.terms,next]});setTerm("");
  }
  function removeTerm(i:number){if(metadata)setMetadata({...metadata,terms:metadata.terms.filter((_,idx)=>idx!==i)});}
  function addFitment(){
    if(!metadata||!selectedVariant)return;
    const from=yearFrom?Number(yearFrom):undefined,to=yearTo?Number(yearTo):undefined;
    if((from&&!Number.isInteger(from))||(to&&!Number.isInteger(to))||(from&&to&&to<from)){setError("بازه سال فیتمنت معتبر نیست.");return;}
    if(metadata.fitments.some(x=>x.vehicle_variant_id===selectedVariant.id&&x.year_from===from&&x.year_to===to))return;
    setMetadata({...metadata,fitments:[...metadata.fitments,{vehicle_variant_id:selectedVariant.id,make_name:selectedMake?.name||"",model_name:selectedModel?.name||"",variant_name:selectedVariant.name,engine_code:selectedVariant.engine_code,year_from:from,year_to:to,notes:notes.trim()}]});
    setVariantId("");setYearFrom("");setYearTo("");setNotes("");
  }
  function removeFitment(i:number){if(metadata)setMetadata({...metadata,fitments:metadata.fitments.filter((_,idx)=>idx!==i)});}
  async function save(){
    if(!session||!selected||!metadata)return;setSaving(true);setError("");setMessage("");
    const fitments:ProductFitmentInput[]=metadata.fitments.map(x=>({vehicle_variant_id:x.vehicle_variant_id,...(x.year_from?{year_from:x.year_from}:{}),...(x.year_to?{year_to:x.year_to}:{}),...(x.notes?{notes:x.notes}:{})}));
    try{setMetadata(await updateProductSearchMetadata(session,selected.id,metadata.terms,fitments));setMessage("نام‌های معادل و سازگاری خودرو ذخیره شد.");}catch(e){setError(e instanceof Error?e.message:"ذخیره فیتمنت ناموفق بود.");}finally{setSaving(false);}
  }

  return <div>
    <div className="page-head"><div><span className="eyebrow">کاتالوگ هوشمند</span><h1>نام‌های معادل و سازگاری خودرو</h1><p>برای هر کالا OEMهای جایگزین، اصطلاحات رایج و خودروهای سازگار را ثبت کن تا جست‌وجوی شبکه دقیق‌تر شود.</p></div><a className="ghost-btn" href="/mechanic" target="_blank">تست جست‌وجوی مکانیک ↗</a></div>
    {message&&<div className="success-box page-error">{message}</div>}{error&&<div className="error-box page-error">{error}</div>}
    <div className="fitment-layout">
      <section className="panel fitment-products"><div className="panel-head"><div><b>انتخاب کالا</b><span>کالا را با نام، SKU، OEM یا alias پیدا کن.</span></div></div><div className="inline-search"><input value={query} onChange={e=>setQuery(e.target.value)} placeholder="مثلاً لنت 206 یا 4254.97"/><button className="table-action" onClick={()=>void loadProducts(query)}>جست‌وجو</button></div><div className="fitment-product-list">{loading&&<div className="table-state">در حال دریافت...</div>}{!loading&&!products.length&&<div className="table-state">کالایی پیدا نشد.</div>}{products.map(p=><button key={p.id} className={selected?.id===p.id?"fitment-product active":"fitment-product"} onClick={()=>setSelected(p)}><b>{p.title}</b><span>{[p.brand,p.sku,p.oem_code].filter(Boolean).join(" · ")}</span></button>)}</div></section>

      <section className="panel fitment-editor">{!selected?<div className="table-state tall">یک کالا را از ستون کناری انتخاب کن.</div>:!metadata?<div className="table-state tall">در حال دریافت اطلاعات «{selected.title}»...</div>:<>
        <div className="panel-head"><div><b>{selected.title}</b><span>{[selected.brand,selected.sku,selected.oem_code].filter(Boolean).join(" · ")}</span></div><button className="primary-btn" disabled={saving} onClick={()=>void save()}>{saving?"در حال ذخیره...":"ذخیره تغییرات"}</button></div>
        <div className="fitment-block"><div className="fitment-block-title"><div><b>نام‌های جایگزین و کدهای معادل</b><span>برای جست‌وجو با اصطلاح بازار، OEM دیگر یا کد معادل.</span></div></div><div className="term-add"><select value={termKind} onChange={e=>setTermKind(e.target.value as ProductSearchTerm["kind"])}><option value="alias">نام رایج</option><option value="oem">OEM دیگر</option><option value="equivalent">کد معادل</option></select><input value={term} onChange={e=>setTerm(e.target.value)} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();addTerm();}}} placeholder="مثلاً لنت تیپ 5 یا 4254.97"/><button onClick={addTerm}>+ افزودن</button></div><div className="term-chips">{metadata.terms.map((x,i)=><span key={`${x.kind}-${x.term}-${i}`}><small>{x.kind==="alias"?"نام":x.kind==="oem"?"OEM":"معادل"}</small>{x.term}<button onClick={()=>removeTerm(i)}>×</button></span>)}{!metadata.terms.length&&<em>هنوز نام معادلی ثبت نشده.</em>}</div></div>

        <div className="fitment-block"><div className="fitment-block-title"><div><b>خودروهای سازگار</b><span>تیپ، موتور و در صورت نیاز بازه سال را مشخص کن.</span></div></div><div className="fitment-add-grid"><select value={makeId} onChange={e=>{setMakeId(e.target.value);setModelId("");setVariantId("");}}><option value="">سازنده</option>{vehicles.map(x=><option key={x.id} value={x.id}>{x.name}</option>)}</select><select value={modelId} disabled={!selectedMake} onChange={e=>{setModelId(e.target.value);setVariantId("");}}><option value="">مدل</option>{selectedMake?.models.map(x=><option key={x.id} value={x.id}>{x.name}</option>)}</select><select value={variantId} disabled={!selectedModel} onChange={e=>{setVariantId(e.target.value);const v=selectedModel?.variants.find(x=>x.id===e.target.value);setYearFrom(v?.year_from?String(v.year_from):"");setYearTo(v?.year_to?String(v.year_to):"");}}><option value="">تیپ / موتور</option>{selectedModel?.variants.map(x=><option key={x.id} value={x.id}>{x.name}{x.engine_code?` · ${x.engine_code}`:""}</option>)}</select><input inputMode="numeric" value={yearFrom} disabled={!variantId} onChange={e=>setYearFrom(e.target.value)} placeholder="از سال"/><input inputMode="numeric" value={yearTo} disabled={!variantId} onChange={e=>setYearTo(e.target.value)} placeholder="تا سال"/><input value={notes} disabled={!variantId} onChange={e=>setNotes(e.target.value)} placeholder="یادداشت؛ مثلاً محور جلو"/><button disabled={!variantId} onClick={addFitment}>+ افزودن خودرو</button></div><div className="fitment-list">{metadata.fitments.map((x,i)=><article key={`${x.vehicle_variant_id}-${i}`}><div><b>{x.make_name} {x.model_name} {x.variant_name}</b><span>{[x.engine_code,yearText(x.year_from,x.year_to),x.notes].filter(Boolean).join(" · ")}</span></div><button onClick={()=>removeFitment(i)}>حذف</button></article>)}{!metadata.fitments.length&&<div className="table-state">هنوز خودروی سازگاری ثبت نشده.</div>}</div></div>
      </>}</section>
    </div>
  </div>;
}
