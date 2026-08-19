"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { createProduct } from "@/lib/api";
import { printEdgeLabel } from "@/lib/edge";
import type { Product, ProductUnitInput } from "@/lib/types";

type UnitDraft=ProductUnitInput&{key:string};
const unitOptions=[
  ["pcs","عدد"],["pair","جفت"],["set","دست"],["pack","بسته"],["box","جعبه"],["carton","کارتن"],["roll","رول"],["liter","لیتر"],["kg","کیلوگرم"]
] as const;
const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(Math.round(v));
const ptr=(v:string)=>v.trim()||undefined;

function ean13Check(first12:string){let sum=0;for(let i=0;i<12;i++)sum+=Number(first12[i])*(i%2===0?1:3);return String((10-(sum%10))%10)}
function internalBarcode(){const body=`29${Array.from({length:10},()=>Math.floor(Math.random()*10)).join("")}`;return body+ean13Check(body)}
function unitName(code:string){return unitOptions.find(x=>x[0]===code)?.[1]||code}

export default function NewProductPage(){
 const {session}=useAuth();
 const [title,setTitle]=useState("");const [sku,setSku]=useState("");const [brand,setBrand]=useState("");const [oem,setOem]=useState("");
 const [baseCode,setBaseCode]=useState("pcs");const [baseName,setBaseName]=useState("عدد");const [baseBarcode,setBaseBarcode]=useState("");const [retailPrice,setRetailPrice]=useState(0);const [fractional,setFractional]=useState(false);
 const [units,setUnits]=useState<UnitDraft[]>([]);const [busy,setBusy]=useState(false);const [error,setError]=useState("");const [message,setMessage]=useState("");const [created,setCreated]=useState<Product|null>(null);
 const duplicateBarcode=useMemo(()=>{const all=[baseBarcode,...units.map(x=>x.barcode||"")].map(x=>x.trim()).filter(Boolean);return new Set(all).size!==all.length},[baseBarcode,units]);
 const duplicateCode=useMemo(()=>{const all=[baseCode,...units.map(x=>x.code)].map(x=>x.trim().toLowerCase()).filter(Boolean);return new Set(all).size!==all.length},[baseCode,units]);
 function setBase(code:string){setBaseCode(code);setBaseName(unitName(code))}
 function addUnit(){const option=unitOptions.find(([code])=>code!==baseCode&&!units.some(x=>x.code===code))||unitOptions.find(([code])=>code!==baseCode);if(!option){setError("واحد جایگزین دیگری برای افزودن وجود ندارد.");return}const [code,name]=option;setUnits(v=>[...v,{key:crypto.randomUUID(),code,name,factor_to_base:code==="carton"?12:1,barcode:"",allow_sale:true,allow_purchase:true,retail_price:0}])}
 function patchUnit(key:string,patch:Partial<UnitDraft>){setUnits(v=>v.map(x=>x.key===key?{...x,...patch}:x))}
 function removeUnit(key:string){setUnits(v=>v.filter(x=>x.key!==key))}
 async function submit(){
  if(!session)return;if(!title.trim()){setError("نام کالا الزامی است.");return}if(duplicateBarcode){setError("یک بارکد در چند واحد تکرار شده است.");return}if(duplicateCode){setError("کد واحد پایه یا یکی از بسته‌ها تکراری است.");return}
  if(units.some(x=>!x.code.trim()||!x.name.trim()||x.factor_to_base<=0)){setError("نام، کد و ضریب تبدیل همه بسته‌ها را کامل کن.");return}
  if(units.some(x=>!x.allow_sale&&!x.allow_purchase)){setError("هر بسته باید حداقل برای خرید یا فروش فعال باشد.");return}
  if(!fractional&&units.some(x=>Math.abs(x.factor_to_base-Math.round(x.factor_to_base))>1e-9)){setError("برای کالای عددی ضریب بسته‌بندی باید عدد صحیح باشد؛ برای وزن/حجم موجودی اعشاری را فعال کن.");return}
  setBusy(true);setError("");setMessage("");
  try{
   const out=await createProduct(session,{sku:ptr(sku),title:title.trim(),brand:ptr(brand),oem_code:ptr(oem),barcode:ptr(baseBarcode),unit:baseCode,base_unit_name:baseName.trim()||unitName(baseCode),allow_fractional_base_qty:fractional,retail_price:Math.max(0,retailPrice),units:units.map(({key,...u})=>({...u,code:u.code.trim().toLowerCase(),name:u.name.trim(),barcode:ptr(u.barcode||""),retail_price:Math.max(0,u.retail_price||0)}))});
   setCreated(out);setMessage("کالا، واحد پایه، بسته‌بندی‌ها و بارکدها با موفقیت ساخته شدند.");
  }catch(e){setError(e instanceof Error?e.message:"ساخت کالا ناموفق بود.")}finally{setBusy(false)}
 }
 async function printLabel(name:string,barcode?:string,price?:number){if(!created)return;if(!barcode){setError("برای این واحد بارکد ثبت نشده است.");return}try{await printEdgeLabel({title:`${created.title} — ${name}`,sku:created.sku,barcode,price,copies:1});setMessage(`لیبل ${name} برای چاپگر ارسال شد.`)}catch(e){setError(e instanceof Error?e.message:"ارسال لیبل به چاپگر ناموفق بود.")}}
 return <>
  <div className="page-head"><div><span className="eyebrow">تعریف سریع کالا</span><h1>کالای جدید + بارکد و بسته‌بندی</h1><p>کالا را یک‌بار کامل بساز: واحد پایه، کارتن/بسته، ضریب تبدیل، بارکد و قیمت اولیه.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/inventory">بازگشت به انبار</Link><Link className="ghost-btn" href="/store/pricing">قیمت‌گذاری</Link></div></div>
  {message&&<div className="success-box page-error">{message}</div>}{error&&<div className="error-box page-error">{error}</div>}
  <div className="product-wizard-grid">
   <section className="panel product-wizard-main"><div className="panel-head"><div><h2>۱. مشخصات کالا</h2><p>SKU و OEM اختیاری‌اند، اما برای لوازم یدکی پیشنهاد می‌شوند.</p></div></div>
    <div className="wizard-fields"><label className="span-2">نام کالا<input autoFocus value={title} onChange={e=>setTitle(e.target.value)} placeholder="مثلاً لنت جلو پژو ۲۰۶ تیپ ۵"/></label><label>کد داخلی / SKU<input dir="ltr" value={sku} onChange={e=>setSku(e.target.value)} placeholder="BRK-206-TXT"/></label><label>برند<input value={brand} onChange={e=>setBrand(e.target.value)} placeholder="Textar"/></label><label>کد OEM<input dir="ltr" value={oem} onChange={e=>setOem(e.target.value)} placeholder="4254.89"/></label><label>قیمت خرده واحد پایه<input type="number" min="0" value={retailPrice||""} onChange={e=>setRetailPrice(Number(e.target.value)||0)} placeholder="تومان"/></label></div>
   </section>
   <section className="panel"><div className="panel-head"><div><h2>۲. واحد پایه</h2><p>موجودی انبار همیشه با این واحد نگهداری می‌شود.</p></div></div>
    <div className="wizard-fields"><label>نوع واحد<select value={baseCode} onChange={e=>setBase(e.target.value)}>{unitOptions.map(([code,name])=><option key={code} value={code}>{name}</option>)}</select></label><label>نام نمایشی<input value={baseName} onChange={e=>setBaseName(e.target.value)}/></label><label className="span-2">بارکد واحد پایه<div className="barcode-field"><input dir="ltr" value={baseBarcode} onChange={e=>setBaseBarcode(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")e.currentTarget.blur()}} placeholder="اسکن بارکد + Enter یا ورود دستی"/><button type="button" onClick={()=>setBaseBarcode(internalBarcode())}>ساخت بارکد داخلی</button></div><small>بارکدخوان USB مثل کیبورد عمل می‌کند؛ داخل این فیلد اسکن کن.</small></label><label className="span-2 wizard-check"><input type="checkbox" checked={fractional} onChange={e=>setFractional(e.target.checked)}/><span><b>موجودی اعشاری مجاز باشد</b><small>برای لیتر/کیلو فعال کن. برای عدد، جفت و قطعات معمولی خاموش بماند.</small></span></label></div>
   </section>
  </div>
  <section className="panel package-builder"><div className="panel-head"><div><h2>۳. بسته‌بندی و واحدهای جایگزین</h2><p>مثلاً «کارتن ۱۲ عددی» یا «دست ۴ عددی». هر واحد می‌تواند بارکد و قیمت مستقل داشته باشد.</p></div><button className="primary-btn" type="button" onClick={addUnit}>+ افزودن بسته</button></div>
   {!units.length?<div className="empty-state compact"><b>بسته‌بندی دیگری نداری؟</b><span>اگر کالا فقط با {baseName} خرید و فروش می‌شود، این بخش را خالی بگذار.</span></div>:<div className="package-rows">{units.map((u,index)=><article key={u.key} className="package-row"><div className="package-row-head"><b>بسته {index+1}</b><button type="button" onClick={()=>removeUnit(u.key)}>حذف</button></div><div className="wizard-fields"><label>نوع<select value={u.code} onChange={e=>patchUnit(u.key,{code:e.target.value,name:unitName(e.target.value)})}>{unitOptions.filter(x=>x[0]!==baseCode).map(([code,name])=><option key={code} value={code}>{name}</option>)}</select></label><label>نام نمایشی<input value={u.name} onChange={e=>patchUnit(u.key,{name:e.target.value})}/></label><label>ضریب به {baseName}<input type="number" min="0.001" step="0.001" value={u.factor_to_base} onChange={e=>patchUnit(u.key,{factor_to_base:Number(e.target.value)||0})}/><small>۱ {u.name||"بسته"} = {u.factor_to_base||0} {baseName}</small></label><label>قیمت خرده این واحد<input type="number" min="0" value={u.retail_price||""} onChange={e=>patchUnit(u.key,{retail_price:Number(e.target.value)||0})} placeholder={`خالی = قیمت ${baseName} × ضریب`}/></label><label className="span-2">بارکد {u.name}<div className="barcode-field"><input dir="ltr" value={u.barcode||""} onChange={e=>patchUnit(u.key,{barcode:e.target.value})} placeholder="بارکد روی کارتن/بسته یا بارکد داخلی"/><button type="button" onClick={()=>patchUnit(u.key,{barcode:internalBarcode()})}>ساخت بارکد داخلی</button></div></label><div className="span-2 unit-permissions"><label><input type="checkbox" checked={u.allow_sale} onChange={e=>patchUnit(u.key,{allow_sale:e.target.checked})}/> قابل فروش</label><label><input type="checkbox" checked={u.allow_purchase} onChange={e=>patchUnit(u.key,{allow_purchase:e.target.checked})}/> قابل خرید</label></div></div></article>)}</div>}
  </section>
  <section className="panel wizard-review"><div><h2>۴. ثبت نهایی</h2><p>موجودی با {baseName} نگهداری می‌شود. خرید/فروش بسته‌ای خودکار به واحد پایه تبدیل می‌شود.</p>{duplicateBarcode&&<strong className="danger-text">بارکد تکراری داخل فرم پیدا شد.</strong>}{duplicateCode&&<strong className="danger-text">کد واحد تکراری داخل فرم پیدا شد.</strong>}</div><button className="primary-btn" disabled={busy||!title.trim()||duplicateBarcode||duplicateCode} onClick={()=>void submit()}>{busy?"در حال ساخت...":"ساخت کالا"}</button></section>
  {created&&<section className="panel created-product-card"><div className="panel-head"><div><h2>کالا آماده استفاده است</h2><p>{created.title} — از همین حالا در خرید، فروش و جست‌وجوی بارکد دیده می‌شود.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/documents">قالب و چاپ گروهی لیبل</Link><Link className="primary-btn" href={`/store/products/${created.id}/units`}>مدیریت واحدها</Link></div></div><div className="created-unit-list">{created.units?.map((u,i)=><div key={u.id}><div><b>{u.name}{u.is_base?" (پایه)":""}</b><span>{u.is_base?"ضریب ۱":`هر ${u.name} = ${u.factor_to_base} ${baseName}`} {u.barcode?`• ${u.barcode}`:"• بدون بارکد"}</span></div>{u.barcode&&<button className="ghost-btn" onClick={()=>void printLabel(u.name,u.barcode,i===0?retailPrice:units.find(x=>x.code===u.code)?.retail_price)}>چاپ لیبل</button>}</div>)}</div>{retailPrice>0&&<small>قیمت پایه اولیه: {money(retailPrice)} تومان</small>}</section>}
 </>
}
