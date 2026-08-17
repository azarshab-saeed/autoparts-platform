"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ApiError, postSale, searchCustomers, searchProducts } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import type { Customer, PaymentPart, Product, SaleItem } from "@/lib/types";
import { SearchIcon } from "@/components/icons";
import { getStoreEdgeStatus, queueOfflineSale } from "@/lib/edge";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v);
type PayMode="cash"|"card"|"credit"|"mixed";

export default function SalesPage(){
 const {session}=useAuth();
 const [query,setQuery]=useState(""); const [results,setResults]=useState<Product[]>([]); const [items,setItems]=useState<SaleItem[]>([]);
 const [customers,setCustomers]=useState<Customer[]>([]); const [customer,setCustomer]=useState<Customer|null>(null); const [customerQ,setCustomerQ]=useState("");
 const [method,setMethod]=useState<PayMode>("card"); const [cashAmount,setCashAmount]=useState(0); const [cardAmount,setCardAmount]=useState(0);
 const [busy,setBusy]=useState(false); const [success,setSuccess]=useState(""); const [error,setError]=useState(""); const [edgeReady,setEdgeReady]=useState(false);
 useEffect(()=>{if(session){searchProducts("",session).then(setResults);searchCustomers("",session).then(setCustomers)}},[session]);
 useEffect(()=>{void getStoreEdgeStatus().then(s=>setEdgeReady(Boolean(s.paired))).catch(()=>setEdgeReady(false))},[]);
 useEffect(()=>{if(!session)return;const t=setTimeout(()=>searchProducts(query,session).then(setResults).catch(()=>{}),180);return()=>clearTimeout(t)},[query,session]);
 useEffect(()=>{if(!session)return;const t=setTimeout(()=>searchCustomers(customerQ,session).then(setCustomers).catch(()=>{}),180);return()=>clearTimeout(t)},[customerQ,session]);
 const total=useMemo(()=>items.reduce((s,i)=>s+i.qty*i.unitPrice,0),[items]);
 const explicitPaid=method==="mixed"?Math.max(0,cashAmount)+Math.max(0,cardAmount):(method==="credit"?0:total);
 const paid=Math.min(total,explicitPaid); const due=Math.max(0,total-paid);
 function add(p:Product){setItems(prev=>{const f=prev.find(i=>i.product.id===p.id);return f?prev.map(i=>i.product.id===p.id?{...i,qty:i.qty+1}:i):[...prev,{product:p,qty:1,unitPrice:p.mockPrice||0}]});setQuery("")}
 function update(id:string,patch:Partial<SaleItem>){setItems(prev=>prev.map(i=>i.product.id===id?{...i,...patch}:i))}
 function choose(m:PayMode){setMethod(m); if(m!=="mixed"){setCashAmount(0);setCardAmount(0)}}
 function paymentParts():PaymentPart[]{if(method!=="mixed")return[];const parts:PaymentPart[]=[];if(cashAmount>0)parts.push({method:"cash",amount:cashAmount});if(cardAmount>0)parts.push({method:"card",amount:cardAmount});return parts}
 async function submit(){if(!session||!items.length)return;if(method==="mixed"&&cashAmount+cardAmount>total){setError("جمع پرداخت نقد و کارت از مبلغ فاکتور بیشتر است.");return}if(due>0&&!customer){setError("برای مانده نسیه باید مشتری انتخاب شود.");return}setBusy(true);setError("");setSuccess("");try{const legacy=method==="mixed"?"credit":method;const parts=method==="mixed"?paymentParts():undefined;const out=await postSale(session,items,customer?.id||null,legacy,parts);setSuccess(`فروش ثبت شد — ${money(out.total_amount)} تومان${out.due_amount?` | مانده ${money(out.due_amount)}`:""} — شناسه: ${out.id}`);setItems([]);setCustomer(null);setCashAmount(0);setCardAmount(0)}catch(e){const offlineFailure=!navigator.onLine||e instanceof TypeError||(e instanceof ApiError&&e.status>=500);if(offlineFailure&&(method==="cash"||method==="card")&&due===0){try{const st=await getStoreEdgeStatus();if(!st.paired)throw new Error("Store Edge روی این صندوق pair نشده است.");const local=await queueOfflineSale(items,method);setEdgeReady(true);setSuccess(`اینترنت/سرور در دسترس نبود؛ فروش با شماره ${local.local_number} روی Store Edge ذخیره شد و بعداً همگام می‌شود.`);setItems([]);setCustomer(null);setCashAmount(0);setCardAmount(0);return}catch(edgeErr){setError(`${e instanceof Error?e.message:"سرور در دسترس نیست"} — ذخیره آفلاین هم انجام نشد: ${edgeErr instanceof Error?edgeErr.message:"Store Edge در دسترس نیست"}`)}}else{setError(e instanceof Error?e.message:"ثبت فروش ناموفق بود")}}finally{setBusy(false)}}
 return <><div className="page-head"><div><span className="eyebrow">صندوق فروش</span><h1>فروش جدید</h1><p>فروش نقد، کارت، نسیه یا ترکیبی را در یک مرحله ثبت کن.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/sales/history">تاریخچه فروش</Link><div className="sale-number">فاکتور جدید</div></div></div>
 {edgeReady&&<div className="offline-ready-strip"><span>● Store Edge آماده</span><small>اگر اینترنت قطع شود، فروش کامل نقد/کارت روی همین صندوق ذخیره می‌شود.</small><Link href="/store/edge">مدیریت</Link></div>}
 {success&&<div className="success-box">{success}</div>}{error&&<div className="error-box page-error">{error}</div>}
 <div className="sale-layout"><section className="sale-main">
   <div className="search-card"><SearchIcon/><input autoFocus value={query} onChange={e=>setQuery(e.target.value)} placeholder="نام، کد، OEM یا بارکد کالا..."/>{query&&<div className="search-results">{results.slice(0,7).map(p=><button key={p.id} onClick={()=>add(p)}><div><b>{p.title}</b><span>{p.brand||"بدون برند"} · {p.sku||"بدون کد"}</span></div><div><strong>{money(p.mockPrice||0)}</strong><small>موجودی {p.mockQty??"—"}</small></div></button>)}</div>}</div>
   <article className="panel sale-items"><div className="panel-head"><div><h2>اقلام فاکتور</h2><p>{items.length?`${items.length} قلم انتخاب شده`:"هنوز کالایی اضافه نشده"}</p></div></div>{!items.length?<div className="empty-state">برای شروع، نام قطعه را در جست‌وجوی بالا بنویس.</div>:<div className="sale-lines">{items.map(i=><div className="sale-line" key={i.product.id}><div className="line-product"><b>{i.product.title}</b><span>{i.product.brand} · {i.product.sku}</span></div><div className="qty-control"><button onClick={()=>i.qty<=1?setItems(v=>v.filter(x=>x.product.id!==i.product.id)):update(i.product.id,{qty:i.qty-1})}>−</button><input value={i.qty} type="number" min="0.01" step="0.01" onChange={e=>update(i.product.id,{qty:Number(e.target.value)})}/><button onClick={()=>update(i.product.id,{qty:i.qty+1})}>+</button></div><label className="price-input"><input value={i.unitPrice} type="number" onChange={e=>update(i.product.id,{unitPrice:Number(e.target.value)})}/><span>تومان</span></label><strong className="line-total">{money(i.qty*i.unitPrice)}</strong><button className="remove" onClick={()=>setItems(v=>v.filter(x=>x.product.id!==i.product.id))}>×</button></div>)}</div>}</article>
 </section><aside className="checkout-card"><div><span>مشتری</span><div className="customer-picker"><input value={customerQ} onChange={e=>setCustomerQ(e.target.value)} placeholder={customer?customer.name:"مشتری را جست‌وجو کن..."}/>{customerQ&&<div className="customer-results">{customers.slice(0,6).map(c=><button key={c.id} onClick={()=>{setCustomer(c);setCustomerQ("")}}><b>{c.name}</b><span>{c.phone||""}</span></button>)}</div>}</div>{customer&&<button className="selected-customer" onClick={()=>setCustomer(null)}>{customer.name}<span>×</span></button>}</div><hr/>
 <div><span>روش پرداخت</span><div className="payment-grid payment-grid-4"><button onClick={()=>choose("cash")} className={method==="cash"?"selected":""}>نقد</button><button onClick={()=>choose("card")} className={method==="card"?"selected":""}>کارت</button><button onClick={()=>choose("credit")} className={method==="credit"?"selected":""}>نسیه</button><button onClick={()=>choose("mixed")} className={method==="mixed"?"selected":""}>ترکیبی</button></div></div>
 {method==="mixed"&&<div className="split-payment"><label>نقد<input type="number" min="0" value={cashAmount} onChange={e=>setCashAmount(Number(e.target.value)||0)}/></label><label>کارت<input type="number" min="0" value={cardAmount} onChange={e=>setCardAmount(Number(e.target.value)||0)}/></label></div>}
 <hr/><div className="payment-summary"><div><span>جمع فاکتور</span><b>{money(total)}</b></div><div><span>پرداخت‌شده</span><b>{money(paid)}</b></div><div className={due>0?"due":""}><span>مانده حساب</span><b>{money(due)}</b></div></div>
 <button className="submit-sale" onClick={submit} disabled={!items.length||busy}>{busy?"در حال ثبت...":"ثبت و نهایی کردن فروش"}</button><small className="checkout-note">مانده پرداخت‌نشده مستقیماً به حساب مشتری منتقل می‌شود.</small></aside></div></>
}
