"use client";

import { useEffect, useMemo, useState } from "react";
import Modal from "@/components/modal";
import { useAuth } from "@/components/auth-provider";
import {
  getBankAccounts,
  getBankStatementLines,
  getCheckMaturityAverage,
  getChecks,
  getFinanceIntelligence,
  getReconciliationCandidates,
  getReconciliationMatches,
  importBankStatement,
  matchBankStatementLine,
  unmatchBankStatementLine,
} from "@/lib/api";
import type { BankAccount, BankStatementInput, BankStatementLine, CheckDirection, FinanceIntelligenceDashboard, MaturityAverageResult, ReconciliationCandidate, ReconciliationMatch, StoreCheck } from "@/lib/types";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(Math.abs(v));
const signed=(v:number)=>`${v>=0?"+":"-"} ${money(v)}`;
const faDate=(v:string)=>{try{return new Date(`${v}T00:00:00Z`).toLocaleDateString("fa-IR-u-ca-persian",{year:"numeric",month:"2-digit",day:"2-digit"})}catch{return v}};
const emptyDashboard:FinanceIntelligenceDashboard={generated_at:"",window_days:90,bank_balance:0,receivable_open_amount:0,payable_open_amount:0,overdue_receivable_amount:0,overdue_payable_amount:0,next_30_net:0,projected_bank_balance_30:0,unreconciled_bank_lines:0,unreconciled_bank_amount:0,maturity_buckets:[],cash_calendar:[],customer_risks:[]};
const refLabel:Record<string,string>={check_clear:"وصول چک دریافتی",check_payable_clear:"پاس چک پرداختی",customer_receipt:"دریافت از مشتری",supplier_payment:"پرداخت تأمین‌کننده",expense:"هزینه",sale_return:"مرجوعی فروش",purchase_return:"مرجوعی خرید"};
const riskLabel:Record<string,string>={low:"کم",medium:"متوسط",high:"زیاد"};

function normalizeDigits(v:string){return v.replace(/[۰-۹]/g,d=>String("۰۱۲۳۴۵۶۷۸۹".indexOf(d))).replace(/[٠-٩]/g,d=>String("٠١٢٣٤٥٦٧٨٩".indexOf(d)))}
function parseStatementText(text:string):BankStatementInput[]{
  const rows=text.split(/\r?\n/).map(x=>x.trim()).filter(Boolean);const out:BankStatementInput[]=[];
  for(let i=0;i<rows.length;i++){
    const parts=rows[i].split(/[\t;,]/).map(x=>x.trim());
    if(i===0&&/date|تاریخ/i.test(parts[0]||""))continue;
    if(parts.length<2)throw new Error(`سطر ${i+1}: تاریخ و مبلغ الزامی است.`);
    const amount=Number(normalizeDigits(parts[1]).replace(/[٬,\s]/g,""));
    if(!Number.isFinite(amount)||amount===0)throw new Error(`سطر ${i+1}: مبلغ باید عدد غیرصفر باشد.`);
    out.push({date:parts[0],amount:Math.trunc(amount),description:parts[2]||"",reference:parts[3]||"",external_id:parts[4]||""});
  }
  if(!out.length)throw new Error("حداقل یک سطر تراکنش وارد کن.");
  return out;
}

export default function FinanceControlPage(){
  const {session}=useAuth();
  const [tab,setTab]=useState<"dashboard"|"maturity"|"reconcile">("dashboard");
  const [dashboard,setDashboard]=useState<FinanceIntelligenceDashboard>(emptyDashboard);
  const [banks,setBanks]=useState<BankAccount[]>([]);
  const [checks,setChecks]=useState<StoreCheck[]>([]);
  const [direction,setDirection]=useState<CheckDirection>("receivable");
  const [selected,setSelected]=useState<string[]>([]);
  const [referenceDate,setReferenceDate]=useState("");
  const [maturity,setMaturity]=useState<MaturityAverageResult|null>(null);
  const [bankId,setBankId]=useState("");
  const [statementLines,setStatementLines]=useState<BankStatementLine[]>([]);
  const [statementOpen,setStatementOpen]=useState(false);
  const [statementText,setStatementText]=useState("");
  const [candidateLine,setCandidateLine]=useState<BankStatementLine|null>(null);
  const [candidates,setCandidates]=useState<ReconciliationCandidate[]>([]);
  const [matchLine,setMatchLine]=useState<BankStatementLine|null>(null);
  const [matches,setMatches]=useState<ReconciliationMatch[]>([]);
  const [matchAmount,setMatchAmount]=useState(0);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const [success,setSuccess]=useState("");

  async function loadBase(){if(!session)return;setBusy(true);setError("");try{const [d,b]=await Promise.all([getFinanceIntelligence(session,90),getBankAccounts(session)]);setDashboard(d);setBanks(b);setBankId(current=>current||b.find(x=>x.is_default&&x.active)?.id||b.find(x=>x.active)?.id||"")}catch(e){setError(e instanceof Error?e.message:"دریافت کنترل مالی ناموفق بود")}finally{setBusy(false)}}
  async function loadChecks(){if(!session)return;setBusy(true);setError("");try{const out=await getChecks(session,{direction,limit:100});setChecks(out.items.filter(x=>direction==="receivable"?["held","deposited","returned"].includes(x.status):x.status==="issued"));setSelected([]);setMaturity(null)}catch(e){setError(e instanceof Error?e.message:"دریافت چک‌ها ناموفق بود")}finally{setBusy(false)}}
  async function loadStatement(){if(!session||!bankId){setStatementLines([]);return}setBusy(true);setError("");try{setStatementLines(await getBankStatementLines(session,bankId))}catch(e){setError(e instanceof Error?e.message:"دریافت صورت‌حساب ناموفق بود")}finally{setBusy(false)}}
  useEffect(()=>{void loadBase()},[session]);
  useEffect(()=>{if(tab==="maturity")void loadChecks()},[tab,direction,session]);
  useEffect(()=>{if(tab==="reconcile"&&bankId)void loadStatement()},[tab,bankId,session]);

  const selectedChecks=useMemo(()=>checks.filter(x=>selected.includes(x.id)),[checks,selected]);
  const selectedTotal=useMemo(()=>selectedChecks.reduce((s,x)=>s+x.amount,0),[selectedChecks]);
  const selectedBank=useMemo(()=>banks.find(x=>x.id===bankId),[banks,bankId]);
  const unreconciled=useMemo(()=>statementLines.filter(x=>x.status!=="matched"),[statementLines]);

  async function calculate(){if(!session||!selected.length)return;setBusy(true);setError("");try{setMaturity(await getCheckMaturityAverage(session,selected,referenceDate||undefined))}catch(e){setError(e instanceof Error?e.message:"راس‌گیری ناموفق بود")}finally{setBusy(false)}}
  async function importStatementNow(){if(!session||!bankId)return;setBusy(true);setError("");try{const result=await importBankStatement(session,bankId,parseStatementText(statementText));setSuccess(`${new Intl.NumberFormat("fa-IR").format(result.imported)} تراکنش وارد شد${result.duplicates?`؛ ${new Intl.NumberFormat("fa-IR").format(result.duplicates)} تکراری رد شد.`:"."}`);setStatementText("");setStatementOpen(false);await Promise.all([loadStatement(),loadBase()])}catch(e){setError(e instanceof Error?e.message:"ورود صورت‌حساب ناموفق بود")}finally{setBusy(false)}}
  async function openCandidates(line:BankStatementLine){if(!session||!bankId)return;setBusy(true);setError("");try{setCandidateLine(line);setCandidates(await getReconciliationCandidates(session,bankId,line.id));setMatchAmount(line.remaining_amount)}catch(e){setCandidateLine(null);setError(e instanceof Error?e.message:"پیشنهاد تطبیق دریافت نشد")}finally{setBusy(false)}}
  async function submitMatch(c:ReconciliationCandidate){if(!session||!candidateLine)return;const max=Math.min(candidateLine.remaining_amount,c.remaining_amount);const amount=matchAmount||max;if(amount<=0||amount>max){setError(`مبلغ تطبیق نمی‌تواند بیشتر از ${money(max)} تومان باشد.`);return}setBusy(true);setError("");try{await matchBankStatementLine(session,candidateLine.id,c.journal_entry_id,amount);setSuccess("تطبیق بانکی ثبت شد؛ سند حسابداری بدون تغییر باقی ماند.");setCandidateLine(null);await Promise.all([loadStatement(),loadBase()])}catch(e){setError(e instanceof Error?e.message:"تطبیق ناموفق بود")}finally{setBusy(false)}}
  async function openMatches(line:BankStatementLine){if(!session)return;setBusy(true);setError("");try{setMatchLine(line);setMatches(await getReconciliationMatches(session,line.id))}catch(e){setMatchLine(null);setError(e instanceof Error?e.message:"جزئیات تطبیق دریافت نشد")}finally{setBusy(false)}}
  async function undoMatch(row:ReconciliationMatch){if(!session||!matchLine)return;if(!window.confirm("این اتصال بانکی باز شود؟ سند حسابداری حذف یا تغییر نمی‌کند."))return;setBusy(true);setError("");try{await unmatchBankStatementLine(session,matchLine.id,row.id);const next=matches.filter(x=>x.id!==row.id);setMatches(next);setSuccess("تطبیق باز شد و در Audit ثبت شد؛ سند حسابداری بدون تغییر ماند.");await Promise.all([loadStatement(),loadBase()]);if(!next.length)setMatchLine(null)}catch(e){setError(e instanceof Error?e.message:"بازکردن تطبیق ناموفق بود")}finally{setBusy(false)}}

  return <>
    <div className="page-head"><div><span className="eyebrow">Phase 15.13</span><h1>کنترل مالی پیشرفته</h1><p>راس‌گیری چک، تقویم نقدینگی، ریسک مشتری و مغایرت بانکی برای مالک و حسابدار.</p></div><div className="head-actions"><button className="ghost-btn" disabled={busy} onClick={()=>void loadBase()}>به‌روزرسانی</button></div></div>
    {success&&<div className="success-box finance-flash">{success}<button onClick={()=>setSuccess("")}>×</button></div>}{error&&<div className="error-box finance-flash">{error}<button onClick={()=>setError("")}>×</button></div>}
    <div className="segmented finance-control-tabs"><button className={tab==="dashboard"?"active":""} onClick={()=>setTab("dashboard")}>داشبورد و ریسک</button><button className={tab==="maturity"?"active":""} onClick={()=>setTab("maturity")}>راس‌گیری چک</button><button className={tab==="reconcile"?"active":""} onClick={()=>setTab("reconcile")}>مغایرت بانکی</button></div>

    {tab==="dashboard"&&<>
      <section className="finance-intel-kpis"><article><span>مانده بانک‌ها</span><strong>{money(dashboard.bank_balance)} تومان</strong></article><article className={dashboard.next_30_net<0?"danger":"ok"}><span>خالص چک‌های ۳۰ روز آینده</span><strong>{signed(dashboard.next_30_net)}</strong></article><article><span>مانده پیش‌بینی ۳۰ روز</span><strong>{money(dashboard.projected_bank_balance_30)} تومان</strong></article><article className={dashboard.overdue_receivable_amount?"warn":""}><span>دریافتنی معوق</span><strong>{money(dashboard.overdue_receivable_amount)} تومان</strong></article><article className={dashboard.overdue_payable_amount?"danger":""}><span>پرداختنی معوق</span><strong>{money(dashboard.overdue_payable_amount)} تومان</strong></article><article className={dashboard.unreconciled_bank_lines?"warn":""}><span>بانکی تطبیق‌نشده</span><strong>{money(dashboard.unreconciled_bank_amount)} تومان</strong><small>{dashboard.unreconciled_bank_lines.toLocaleString("fa-IR")} تراکنش</small></article></section>
      <div className="finance-intel-grid"><section className="panel finance-intel-panel"><div className="panel-title"><div><h2>بازه‌های سررسید</h2><p>Exposure چک‌های دریافتی و پرداختی.</p></div></div><div className="maturity-buckets">{dashboard.maturity_buckets.map(x=><div key={x.key}><b>{x.label}</b><span className="incoming">دریافتی: {money(x.receivable_amount)} ({x.receivable_count.toLocaleString("fa-IR")})</span><span className="outgoing">پرداختی: {money(x.payable_amount)} ({x.payable_count.toLocaleString("fa-IR")})</span></div>)}</div></section><section className="panel finance-intel-panel"><div className="panel-title"><div><h2>تقویم جریان نقدی چک‌ها</h2><p>روزهای دارای ورود/خروج در ۹۰ روز آینده.</p></div></div><div className="cash-calendar-list">{dashboard.cash_calendar.slice(0,18).map(x=><div key={x.date}><div><b>{faDate(x.date)}</b><small>{x.date}</small></div><span className={x.net>=0?"incoming":"outgoing"}>{signed(x.net)}</span><small>مانده: {money(x.projected_balance)}</small></div>)}{!dashboard.cash_calendar.length&&<div className="table-state">چک بازی در بازه آینده وجود ندارد.</div>}</div></section></div>
      <section className="panel finance-intel-panel"><div className="panel-title"><div><h2>ریسک چک مشتریان</h2><p>بر اساس سابقه برگشتی و چک معوق؛ این شاخص اعتبارسنجی بانکی رسمی نیست.</p></div></div><div className="table-wrap"><table className="risk-table"><thead><tr><th>مشتری</th><th>ریسک</th><th>تعداد</th><th>برگشتی</th><th>معوق</th><th>بیشترین تأخیر</th></tr></thead><tbody>{dashboard.customer_risks.map(x=><tr key={x.customer_id}><td><b>{x.customer_name}</b><small>گردش: {money(x.total_amount)}</small></td><td><span className={`risk-badge ${x.risk_level}`}>{riskLabel[x.risk_level]}</span></td><td>{x.total_count.toLocaleString("fa-IR")}</td><td><b>{money(x.bounced_amount)}</b><small>{(x.bounce_rate_bps/100).toLocaleString("fa-IR")}%</small></td><td><b>{money(x.overdue_amount)}</b><small>{x.overdue_count.toLocaleString("fa-IR")} فقره</small></td><td>{x.max_overdue_days.toLocaleString("fa-IR")} روز</td></tr>)}</tbody></table>{!dashboard.customer_risks.length&&<div className="table-state">سابقه کافی برای شاخص ریسک وجود ندارد.</div>}</div></section>
    </>}

    {tab==="maturity"&&<section className="panel finance-intel-panel"><div className="panel-title maturity-head"><div><h2>راس‌گیری چک‌های {direction==="receivable"?"دریافتی":"پرداختی"}</h2><p>راس بر اساس وزن مبلغ هر چک روی تاریخ سررسید محاسبه می‌شود.</p></div><div className="maturity-actions"><select value={direction} onChange={e=>setDirection(e.target.value as CheckDirection)}><option value="receivable">دریافتی</option><option value="payable">پرداختی</option></select><input value={referenceDate} onChange={e=>setReferenceDate(e.target.value)} placeholder="تاریخ مبنا؛ ۱۴۰۵/۰۵/۲۷"/><button className="primary-btn" disabled={busy||!selected.length} onClick={()=>void calculate()}>محاسبه راس</button></div></div><div className="maturity-selection-meta"><span>{selected.length.toLocaleString("fa-IR")} انتخاب</span><b>{money(selectedTotal)} تومان</b><button onClick={()=>setSelected(checks.map(x=>x.id))}>انتخاب همه</button><button onClick={()=>setSelected([])}>پاک‌کردن</button></div><div className="table-wrap"><table className="risk-table"><thead><tr><th></th><th>شماره</th><th>شخص</th><th>مبلغ</th><th>سررسید</th><th>وضعیت</th></tr></thead><tbody>{checks.map(x=><tr key={x.id}><td><input type="checkbox" checked={selected.includes(x.id)} onChange={e=>setSelected(current=>e.target.checked?[...current,x.id]:current.filter(id=>id!==x.id))}/></td><td dir="ltr">{x.check_number}</td><td>{x.customer_name||x.supplier_name}</td><td>{money(x.amount)}</td><td>{faDate(x.due_date)}</td><td>{x.status}</td></tr>)}</tbody></table>{!checks.length&&<div className="table-state">چک باز برای راس‌گیری وجود ندارد.</div>}</div>{maturity&&<div className="maturity-result"><div><span>جمع مبلغ</span><b>{money(maturity.total_amount)} تومان</b></div><div><span>فاصله وزنی</span><b>{maturity.weighted_days.toLocaleString("fa-IR")} روز</b></div><div className="accent"><span>راس چک‌ها</span><b>{faDate(maturity.maturity_date)}</b><small>{maturity.maturity_date}</small></div></div>}</section>}

    {tab==="reconcile"&&<section className="panel finance-intel-panel"><div className="panel-title reconciliation-head"><div><h2>مغایرت بانکی</h2><p>صورت‌حساب بانک را به سندهای دفتر کل وصل کن؛ تطبیق خودش سند مالی نمی‌سازد.</p></div><div className="reconciliation-actions"><select value={bankId} onChange={e=>setBankId(e.target.value)}><option value="">انتخاب حساب...</option>{banks.filter(x=>x.active).map(x=><option key={x.id} value={x.id}>{x.bank_name} — {x.name}</option>)}</select><button className="primary-btn" disabled={!bankId} onClick={()=>setStatementOpen(true)}>ورود صورت‌حساب</button></div></div>{selectedBank&&<div className="reconcile-summary"><div><span>حساب</span><b>{selectedBank.bank_name} — {selectedBank.name}</b></div><div><span>مانده دفتر</span><b>{money(selectedBank.balance)} تومان</b></div><div><span>باز</span><b>{unreconciled.length.toLocaleString("fa-IR")} تراکنش</b></div><div><span>مبلغ باز</span><b>{money(unreconciled.reduce((s,x)=>s+x.remaining_amount,0))} تومان</b></div></div>}<div className="table-wrap"><table className="reconciliation-table"><thead><tr><th>تاریخ</th><th>شرح / مرجع</th><th>مبلغ بانک</th><th>تطبیق‌شده</th><th>مانده</th><th>وضعیت</th><th></th></tr></thead><tbody>{statementLines.map(x=><tr key={x.id}><td><b>{faDate(x.date)}</b><small>{x.date}</small></td><td><b>{x.description||"—"}</b><small>{x.reference||x.external_id||"بدون مرجع"}{x.duplicate_suspected&&" • احتمال تکراری"}</small></td><td className={x.amount>=0?"incoming":"outgoing"}>{signed(x.amount)}</td><td>{money(x.matched_amount)}</td><td>{money(x.remaining_amount)}</td><td><span className={`reconcile-status ${x.status}`}>{x.status==="matched"?"کامل":x.status==="partial"?"جزئی":"باز"}</span></td><td><div className="reconcile-row-actions">{x.status!=="matched"&&<button className="ghost-btn" onClick={()=>void openCandidates(x)}>پیشنهاد تطبیق</button>}{x.matched_amount>0&&<button className="ghost-btn" onClick={()=>void openMatches(x)}>مدیریت تطبیق</button>}</div></td></tr>)}</tbody></table>{!bankId&&<div className="table-state">حساب بانکی را انتخاب کن.</div>}{bankId&&!statementLines.length&&<div className="table-state">صورت‌حسابی برای این حساب وارد نشده است.</div>}</div></section>}

    <Modal open={statementOpen} onClose={()=>setStatementOpen(false)} title="ورود صورت‌حساب بانک" subtitle="هر سطر: تاریخ، مبلغ، شرح، مرجع، شناسه بانک. مثبت=ورودی، منفی=خروجی."><div className="form-stack statement-import"><textarea dir="ltr" value={statementText} onChange={e=>setStatementText(e.target.value)} placeholder={'1405/05/27,25000000,وصول چک,CHK-102\n1405/05/28,-12500000,پرداخت تامین‌کننده,TRX-551'}/><small>comma، semicolon و Tab پذیرفته می‌شود. رکورد دقیق تکراری دوباره وارد نمی‌شود.</small></div><div className="modal-actions"><button className="ghost-btn" onClick={()=>setStatementOpen(false)}>انصراف</button><button className="primary-btn" disabled={busy||!statementText.trim()} onClick={()=>void importStatementNow()}>ورود تراکنش‌ها</button></div></Modal>

    <Modal open={!!candidateLine} onClose={()=>setCandidateLine(null)} title="پیشنهادهای تطبیق" subtitle={candidateLine?`${faDate(candidateLine.date)} • ${signed(candidateLine.amount)} تومان • مانده ${money(candidateLine.remaining_amount)}`:undefined}>
      <div className="candidate-list">
        {candidates.map(c=>{
          const max=Math.min(candidateLine?.remaining_amount||0,c.remaining_amount);
          return <article key={c.journal_entry_id} className={c.exact_amount?"exact":""}>
            <div><b>{refLabel[c.reference_type]||c.reference_type}</b>{c.exact_amount&&<em>مبلغ دقیق</em>}<small>{new Date(c.posted_at).toLocaleString("fa-IR")}</small></div>
            <strong className={c.change>=0?"incoming":"outgoing"}>{signed(c.change)}</strong>
            <small>قابل تطبیق: {money(c.remaining_amount)}</small>
            <button className="ghost-btn" disabled={busy} onClick={()=>{setMatchAmount(max);void submitMatch(c);}}>تطبیق</button>
          </article>;
        })}
        {!candidates.length&&<div className="table-state">در بازه ±۱۴ روز سند بانکی هم‌جهت پیدا نشد.</div>}
      </div>
      {candidates.length>0&&<label className="partial-match">مبلغ تطبیق جزئی<input type="number" min="1" value={matchAmount||""} onChange={e=>setMatchAmount(Number(e.target.value)||0)}/></label>}
      <div className="modal-actions"><button className="primary-btn" onClick={()=>setCandidateLine(null)}>بستن</button></div>
    </Modal>

    <Modal open={!!matchLine} onClose={()=>setMatchLine(null)} title="مدیریت تطبیق‌های بانکی" subtitle={matchLine?`${faDate(matchLine.date)} • تطبیق‌شده ${money(matchLine.matched_amount)} تومان`:undefined}>
      <div className="candidate-list reconciliation-match-list">
        {matches.map(row=><article key={row.id}><div><b>{refLabel[row.reference_type||""]||row.reference_type||"سند مالی"}</b><small>{row.posted_at?new Date(row.posted_at).toLocaleString("fa-IR"):""}</small></div><strong>{money(row.matched_amount)} تومان</strong><button className="danger-btn" disabled={busy} onClick={()=>void undoMatch(row)}>بازکردن تطبیق</button></article>)}
        {!matches.length&&<div className="table-state">تطبیق فعالی باقی نمانده است.</div>}
      </div>
      <div className="modal-actions"><button className="primary-btn" onClick={()=>setMatchLine(null)}>بستن</button></div>
    </Modal>
  </>;
}
