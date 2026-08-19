"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getInvoicePrintData } from "@/lib/api";
import type { OfficialInvoicePrintData } from "@/lib/types";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(Math.round(v))+" تومان";
const text=(obj:Record<string,unknown>,key:string)=>String(obj[key]??"");

export default function OfficialInvoicePage(){
  const {session}=useAuth(); const params=useParams<{id:string}>();
  const [data,setData]=useState<OfficialInvoicePrintData|null>(null);const[error,setError]=useState("");
  useEffect(()=>{if(!session||!params.id)return;getInvoicePrintData(session,params.id).then(setData).catch(e=>setError(e instanceof Error?e.message:"داده فاکتور دریافت نشد"))},[session,params.id]);
  if(error)return <div className="error-box">{error}</div>;if(!data)return <div className="panel table-state">در حال آماده‌سازی نسخه رسمی...</div>;
  return <div className="official-invoice-page"><div className="page-head no-print"><div><span className="eyebrow">فاکتور رسمی</span><h1>{data.invoice_number_display||data.sale_id}</h1><p>این نسخه از snapshot زمان صدور ساخته شده است.</p></div><div className="head-actions"><Link className="ghost-btn" href="/store/tax">بازگشت</Link><button className="primary-btn" onClick={()=>window.print()}>چاپ</button></div></div>
    <section className="panel official-invoice-sheet"><header><div><span>فروشنده</span><h2>{text(data.seller,"legal_name")||"—"}</h2><p>{text(data.seller,"address")}</p></div><div className="invoice-number-box"><span>شماره فاکتور</span><b dir="ltr">{data.invoice_number_display||"—"}</b><small>{data.issued_at?new Date(data.issued_at).toLocaleString("fa-IR"):""}</small></div></header>
      <div className="official-party-grid"><article><h3>مشخصات فروشنده</h3><div><span>شناسه ملی</span><b>{text(data.seller,"national_id")||"—"}</b></div><div><span>کد اقتصادی</span><b>{text(data.seller,"economic_code")||"—"}</b></div><div><span>شماره ثبت</span><b>{text(data.seller,"registration_number")||"—"}</b></div><div><span>کد پستی</span><b>{text(data.seller,"postal_code")||"—"}</b></div></article><article><h3>مشخصات خریدار</h3><div><span>نام</span><b>{text(data.buyer,"name")||"مشتری نقدی"}</b></div><div><span>شناسه ملی</span><b>{text(data.buyer,"national_id")||"—"}</b></div><div><span>کد اقتصادی</span><b>{text(data.buyer,"economic_code")||"—"}</b></div><div><span>کد پستی</span><b>{text(data.buyer,"postal_code")||"—"}</b></div></article></div>
      <div className="table-wrap"><table className="official-lines"><thead><tr><th>#</th><th>شرح کالا</th><th>واحد</th><th>تعداد</th><th>قیمت واحد</th><th>خالص</th><th>کد مالیاتی</th><th>نرخ</th><th>مالیات</th><th>جمع</th></tr></thead><tbody>{data.items.map((x,i)=><tr key={`${x.product_id}-${i}`}><td>{i+1}</td><td><b>{x.title}</b>{x.exemption_reason&&<small className="block-note">{x.exemption_reason}</small>}</td><td>{x.unit_name}</td><td>{x.qty}</td><td>{money(x.unit_price)}</td><td>{money(x.net_amount)}</td><td><span dir="ltr">{x.tax_code||"—"}</span></td><td>{x.tax_category==="taxable"?`${(x.tax_rate_bps/100).toLocaleString("fa-IR")}%`:"—"}</td><td>{money(x.tax_amount)}</td><td><b>{money(x.total_with_tax)}</b></td></tr>)}</tbody></table></div>
      <footer className="official-totals"><div><span>ناخالص قبل تخفیف</span><b>{money(data.gross_amount)}</b></div><div><span>تخفیف</span><b>{money(data.discount_amount)}</b></div><div><span>خالص</span><b>{money(data.net_amount)}</b></div><div><span>مشمول</span><b>{money(data.taxable_amount)}</b></div><div><span>معاف/غیرمشمول</span><b>{money(data.exempt_amount)}</b></div><div><span>مالیات و عوارض</span><b>{money(data.tax_amount)}</b></div><div className="grand"><span>مبلغ نهایی</span><strong>{money(data.total_amount)}</strong></div></footer>
      <div className="official-signatures"><div>مهر و امضای فروشنده</div><div>نام و امضای خریدار</div></div>
    </section>
  </div>;
}
