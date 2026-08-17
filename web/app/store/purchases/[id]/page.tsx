"use client";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect,useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getPurchaseDetail } from "@/lib/api";
import type { PurchaseDetail } from "@/lib/types";
const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
export default function PurchaseDocument(){
 const {session}=useAuth();const params=useParams<{id:string}>();const[data,setData]=useState<PurchaseDetail|null>(null);const[error,setError]=useState("");
 useEffect(()=>{if(!session||!params.id)return;getPurchaseDetail(session,params.id).then(setData).catch(e=>setError(e instanceof Error?e.message:"سند خرید پیدا نشد"))},[session,params.id]);
 if(error)return <div className="error-box">{error}</div>;if(!data)return <div className="panel table-state">در حال دریافت سند خرید...</div>;
 return <div className="invoice-page"><div className="page-head no-print"><div><span className="eyebrow">سند خرید</span><h1>جزئیات خرید</h1><p>{data.id}</p></div><div className="head-actions"><Link className="ghost-btn" href={`/store/returns?kind=purchase&id=${encodeURIComponent(data.id)}`}>ثبت مرجوعی</Link><button className="primary-btn" onClick={()=>window.print()}>چاپ سند</button></div></div><section className="panel printable-invoice"><header className="invoice-header"><div><span className="eyebrow">فروشگاه هوشمند قطعات</span><h2>سند خرید</h2></div><div><b>{new Date(data.created_at).toLocaleDateString("fa-IR")}</b><small>{data.id}</small></div></header><div className="invoice-meta"><div><span>تأمین‌کننده</span><b>{data.supplier_name}</b></div><div><span>وضعیت</span><b>{data.status}</b></div><div><span>پرداخت‌شده</span><b>{money(data.paid_amount)}</b></div><div><span>بدهی</span><b>{money(data.due_amount)}</b></div></div><div className="table-wrap"><table><thead><tr><th>کالا</th><th>تعداد</th><th>بهای واحد</th><th>جمع</th><th>قابل مرجوع</th></tr></thead><tbody>{data.items.map(x=><tr key={x.id}><td><b>{x.title}</b></td><td>{x.qty}</td><td>{money(x.unit_cost)}</td><td>{money(x.line_total)}</td><td>{x.returnable_qty}</td></tr>)}</tbody></table></div><footer className="invoice-total"><span>جمع خرید</span><strong>{money(data.total_amount)}</strong></footer></section></div>;
}
