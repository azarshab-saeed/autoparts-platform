"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { getAuditLogs } from "@/lib/api";
import type { AuditLogEntry } from "@/lib/types";

const methodLabel: Record<string,string> = {POST:"ایجاد",PUT:"ویرایش",PATCH:"تغییر وضعیت",DELETE:"حذف"};

export default function AuditPage(){
  const {session}=useAuth();
  const [items,setItems]=useState<AuditLogEntry[]>([]);
  const [loading,setLoading]=useState(true);
  const [error,setError]=useState("");
  useEffect(()=>{if(!session)return;setLoading(true);setError("");void getAuditLogs(session).then(x=>setItems(x.items)).catch(e=>setError(e instanceof Error?e.message:"گزارش رویدادها آماده نشد")).finally(()=>setLoading(false));},[session]);
  return <div>
    <div className="page-head"><div><span className="eyebrow">امنیت و ممیزی</span><h1>رویدادهای حساس فروشگاه</h1><p>عملیات تغییردهنده داده با شناسه درخواست، کاربر، مسیر و نتیجه ثبت می‌شوند.</p></div></div>
    {error&&<div className="error-box">{error}</div>}
    <section className="panel audit-panel">
      {loading?<div className="table-state">در حال دریافت رویدادها...</div>:items.length===0?<div className="table-state">هنوز عملیات تغییردهنده‌ای ثبت نشده است.</div>:<div className="audit-table-wrap"><table className="audit-table"><thead><tr><th>زمان</th><th>عملیات</th><th>مسیر</th><th>نقش</th><th>نتیجه</th><th>IP</th><th>Request ID</th></tr></thead><tbody>{items.map(x=><tr key={x.id}><td>{new Date(x.occurred_at).toLocaleString("fa-IR")}</td><td><b>{methodLabel[x.method]||x.method}</b><small>{x.method}</small></td><td><code>{x.path}</code></td><td>{x.role}</td><td><span className={x.status<400?"audit-ok":"audit-fail"}>{x.status}</span></td><td><code>{x.remote_ip||"-"}</code></td><td><code title={x.request_id}>{x.request_id.slice(0,8)}…</code></td></tr>)}</tbody></table></div>}
    </section>
  </div>;
}
