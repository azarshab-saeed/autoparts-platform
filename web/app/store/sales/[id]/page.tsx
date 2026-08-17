"use client";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect,useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getSaleDetail } from "@/lib/api";
import type { SaleDetail } from "@/lib/types";
const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
export default function SaleInvoice(){
 const {session}=useAuth();const params=useParams<{id:string}>();const[data,setData]=useState<SaleDetail|null>(null);const[error,setError]=useState("");
 useEffect(()=>{if(!session||!params.id)return;getSaleDetail(session,params.id).then(setData).catch(e=>setError(e instanceof Error?e.message:"فاکتور پیدا نشد"))},[session,params.id]);
 if(error)return <div className="error-box">{error}</div>;if(!data)return <div className="panel table-state">در حال دریافت فاکتور...</div>;
 return <div className="invoice-page"><div className="page-head no-print"><div><span className="eyebrow">فاکتور فروش</span><h1>جزئیات فروش</h1><p>{data.id}</p></div><div className="head-actions"><Link className="ghost-btn" href={`/store/returns?kind=sale&id=${encodeURIComponent(data.id)}`}>ثبت مرجوعی</Link><button className="primary-btn" onClick={()=>window.print()}>چاپ فاکتور</button></div></div><section className="panel printable-invoice"><header className="invoice-header"><div><span className="eyebrow">فروشگاه هوشمند قطعات</span><h2>فاکتور فروش</h2></div><div><b>{new Date(data.created_at).toLocaleDateString("fa-IR")}</b><small>{data.id}</small></div></header><div className="invoice-meta"><div><span>مشتری</span><b>{data.customer_name||"مشتری نقدی"}</b></div><div><span>وضعیت</span><b>{data.status}</b></div><div><span>پرداخت‌شده</span><b>{money(data.paid_amount)}</b></div><div><span>مانده</span><b>{money(data.due_amount)}</b></div></div><div className="table-wrap"><table><thead><tr><th>کالا</th><th>تعداد</th><th>قیمت واحد</th><th>جمع</th><th>قابل مرجوع</th></tr></thead><tbody>{data.items.map(x=><tr key={x.id}><td><b>{x.title}</b></td><td>{x.qty}</td><td>{money(x.unit_price||0)}</td><td>{money(x.line_total)}</td><td>{x.returnable_qty}</td></tr>)}</tbody></table></div><footer className="invoice-total"><span>جمع فاکتور</span><strong>{money(data.total_amount)}</strong></footer></section></div>;
}
