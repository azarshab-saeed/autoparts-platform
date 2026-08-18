"use client";

import { useEffect, useMemo, useState } from "react";
import Modal from "@/components/modal";
import { useAuth } from "@/components/auth-provider";
import {
  createBankAccount,
  createStoreCheck,
  getBankAccounts,
  getBankLedger,
  getChecks,
  getCheckSummary,
  getCustomerBalances,
  getSupplierBalances,
  transitionStoreCheck,
} from "@/lib/api";
import type { BankAccount, BankLedger, CheckAction, CheckDirection, CheckSummary, PartyBalance, StoreCheck } from "@/lib/types";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(Math.abs(v));
const faDate=(v:string)=>{try{return new Date(`${v}T00:00:00Z`).toLocaleDateString("fa-IR-u-ca-persian",{year:"numeric",month:"2-digit",day:"2-digit"})}catch{return v}};
const todayFa=()=>{const parts=new Intl.DateTimeFormat("fa-IR-u-ca-persian",{year:"numeric",month:"2-digit",day:"2-digit"}).formatToParts(new Date());const get=(t:string)=>parts.find(x=>x.type===t)?.value||"";return `${get("year")}/${get("month")}/${get("day")}`};
const friendlyError=(e:unknown)=>{const msg=e instanceof Error?e.message:String(e||"");const balance=msg.match(/(?:check amount exceeds supplier |amount exceeds )outstanding balance \((\d+)\)/);if(balance)return `مبلغ چک از مانده باز شخص بیشتر است. حداکثر مبلغ قابل ثبت ${money(Number(balance[1]))} تومان است.`;if(msg.includes("party has no outstanding balance"))return "برای شخص انتخاب‌شده مانده بازی وجود ندارد.";if(msg.includes("limit must be between 1 and 100"))return "خطای دریافت فهرست چک‌ها برطرف نشده است؛ صفحه را تازه‌سازی کن.";return msg||"عملیات ناموفق بود"};

const statusLabel:Record<string,string>={held:"در صندوق",deposited:"واگذار به بانک",cleared:"وصول / پاس‌شده",bounced:"برگشتی",endorsed:"خرج‌شده",returned:"برگشت‌داده‌شده",cancelled:"باطل",issued:"صادرشده"};
const statusClass:Record<string,string>={held:"warn",issued:"warn",deposited:"info",cleared:"ok",bounced:"danger",endorsed:"info",returned:"warn",cancelled:"muted"};

type CheckForm={partyId:string;checkNumber:string;sayadId:string;bankName:string;branchName:string;amount:number;issueDate:string;dueDate:string;note:string};
type BankForm={name:string;bankName:string;accountNumber:string;cardNumber:string;iban:string;openingBalance:number;isDefault:boolean};
type ActionState={check:StoreCheck;action:CheckAction}|null;

const emptyCheck=():CheckForm=>({partyId:"",checkNumber:"",sayadId:"",bankName:"",branchName:"",amount:0,issueDate:todayFa(),dueDate:"",note:""});
const emptyBank=():BankForm=>({name:"",bankName:"",accountNumber:"",cardNumber:"",iban:"",openingBalance:0,isDefault:false});

export default function ChecksPage(){
  const {session}=useAuth();
  const [tab,setTab]=useState<"receivable"|"payable"|"banks">("receivable");
  const [checks,setChecks]=useState<StoreCheck[]>([]);
  const [total,setTotal]=useState(0);
  const [next,setNext]=useState("");
  const [loading,setLoading]=useState(false);
  const [summary,setSummary]=useState<CheckSummary>({receivable_open_amount:0,payable_open_amount:0,due_today_count:0,due_next_7_count:0,overdue_count:0,bounced_count:0});
  const [banks,setBanks]=useState<BankAccount[]>([]);
  const [customers,setCustomers]=useState<PartyBalance[]>([]);
  const [suppliers,setSuppliers]=useState<PartyBalance[]>([]);
  const [q,setQ]=useState("");
  const [status,setStatus]=useState("");
  const [checkOpen,setCheckOpen]=useState(false);
  const [bankOpen,setBankOpen]=useState(false);
  const [direction,setDirection]=useState<CheckDirection>("receivable");
  const [checkForm,setCheckForm]=useState<CheckForm>(emptyCheck());
  const [bankForm,setBankForm]=useState<BankForm>(emptyBank());
  const [action,setAction]=useState<ActionState>(null);
  const [actionBank,setActionBank]=useState("");
  const [actionSupplier,setActionSupplier]=useState("");
  const [actionNote,setActionNote]=useState("");
  const [ledger,setLedger]=useState<BankLedger|null>(null);
  const [busy,setBusy]=useState(false);
  const [error,setError]=useState("");
  const [success,setSuccess]=useState("");

  const canManagePayable=session?.role!=="cashier";
  const canCreateBank=!!session&&["owner","admin","accountant"].includes(session.role);

  async function load(cursor="",append=false){
    if(!session)return;
    setLoading(true);setError("");
    try{
      if(!append){
        const [cs,bs,cus,sups]=await Promise.all([getCheckSummary(session),getBankAccounts(session),getCustomerBalances(session),getSupplierBalances(session)]);
        setSummary(cs);setBanks(bs);setCustomers(cus);setSuppliers(sups);
      }
      if(tab==="banks"){if(!append){setChecks([]);setTotal(0);setNext("")}return;}
      const out=await getChecks(session,{direction:tab,q,status,limit:100,cursor});
      setChecks(current=>append?[...current,...(out.items??[])]:out.items??[]);setTotal(out.total??0);setNext(out.next_cursor??"");
    }finally{setLoading(false)}
  }
  useEffect(()=>{void load().catch(e=>setError(friendlyError(e)))},[session,tab,status]);
  async function search(){if(!session||tab==="banks")return;try{await load()}catch(e){setError(friendlyError(e))}}

  const parties=direction==="receivable"?customers:suppliers;
  const eligibleParties=useMemo(()=>parties.filter(x=>x.balance>0),[parties]);
  const selectedParty=useMemo(()=>parties.find(x=>x.id===checkForm.partyId),[parties,checkForm.partyId]);
  const amountTooHigh=!!selectedParty&&checkForm.amount>selectedParty.balance;
  const defaultBank=useMemo(()=>banks.find(x=>x.is_default&&x.active)||banks.find(x=>x.active),[banks]);

  function openCheck(d:CheckDirection){setDirection(d);setCheckForm(emptyCheck());setError("");setCheckOpen(true)}
  async function submitCheck(){
    if(!session||!checkForm.partyId||!checkForm.checkNumber||!checkForm.dueDate||checkForm.amount<=0)return;
    if(amountTooHigh&&selectedParty){setError(`مبلغ چک نمی‌تواند بیشتر از مانده باز ${selectedParty.name} باشد. حداکثر ${money(selectedParty.balance)} تومان.`);return;}
    setBusy(true);setError("");
    try{
      await createStoreCheck(session,direction,{partyId:checkForm.partyId,checkNumber:checkForm.checkNumber,sayadId:checkForm.sayadId,bankName:checkForm.bankName,branchName:checkForm.branchName,amount:checkForm.amount,issueDate:checkForm.issueDate,dueDate:checkForm.dueDate,note:checkForm.note});
      setSuccess(direction==="receivable"?"چک دریافتی ثبت شد و مانده مشتری کاهش یافت.":"چک پرداختی صادر شد و مانده تأمین‌کننده کاهش یافت.");setCheckOpen(false);await load();
    }catch(e){setError(friendlyError(e))}finally{setBusy(false)}
  }
  async function submitBank(){
    if(!session||!bankForm.name||!bankForm.bankName)return;setBusy(true);setError("");
    try{await createBankAccount(session,{name:bankForm.name,bankName:bankForm.bankName,accountNumber:bankForm.accountNumber,cardNumber:bankForm.cardNumber,iban:bankForm.iban,openingBalance:bankForm.openingBalance,isDefault:bankForm.isDefault});setSuccess("حساب بانکی ایجاد شد.");setBankOpen(false);setBankForm(emptyBank());await load()}catch(e){setError(e instanceof Error?e.message:"ایجاد حساب بانکی ناموفق بود")}finally{setBusy(false)}
  }
  function beginAction(check:StoreCheck,a:CheckAction){setAction({check,action:a});setActionBank(check.bank_account_id||defaultBank?.id||"");setActionSupplier("");setActionNote("");setError("")}
  async function submitAction(){
    if(!session||!action)return;const needsBank=action.action==="deposit"||action.action==="clear";const needsSupplier=action.action==="endorse";if(needsBank&&!actionBank)return setError("یک حساب بانکی انتخاب کن.");if(needsSupplier&&!actionSupplier)return setError("تأمین‌کننده را انتخاب کن.");setBusy(true);setError("");
    try{await transitionStoreCheck(session,action.check.id,action.action,{bankAccountId:needsBank?actionBank:undefined,supplierId:needsSupplier?actionSupplier:undefined,note:actionNote});setSuccess("وضعیت چک با ثبت حسابداری به‌روزرسانی شد.");setAction(null);await load()}catch(e){setError(friendlyError(e))}finally{setBusy(false)}
  }
  async function openLedger(bank:BankAccount){if(!session)return;setBusy(true);setError("");try{setLedger(await getBankLedger(session,bank.id))}catch(e){setError(e instanceof Error?e.message:"گردش بانک دریافت نشد")}finally{setBusy(false)}}

  function actionButtons(x:StoreCheck){
    if(x.direction==="receivable"){
      if(x.status==="held"||x.status==="returned")return <div className="check-actions"><button onClick={()=>beginAction(x,"deposit")}>واگذاری به بانک</button><button onClick={()=>beginAction(x,"clear")}>وصول مستقیم</button><button onClick={()=>beginAction(x,"endorse")}>خرج کردن</button><button className="danger-link" onClick={()=>beginAction(x,"bounce")}>برگشتی</button>{x.status==="held"&&<button onClick={()=>beginAction(x,"cancel")}>ابطال</button>}</div>;
      if(x.status==="deposited")return <div className="check-actions"><button className="primary-mini" onClick={()=>beginAction(x,"clear")}>وصول شد</button><button className="danger-link" onClick={()=>beginAction(x,"bounce")}>برگشت خورد</button></div>;
      if(x.status==="endorsed")return <div className="check-actions"><button onClick={()=>beginAction(x,"return_endorsement")}>برگشت از تأمین‌کننده</button></div>;
      return null;
    }
    if(x.status==="issued"&&canManagePayable)return <div className="check-actions"><button className="primary-mini" onClick={()=>beginAction(x,"clear")}>پاس شد</button><button onClick={()=>beginAction(x,"return")}>برگشت/عودت</button><button onClick={()=>beginAction(x,"cancel")}>ابطال</button></div>;
    return null;
  }

  return <>
    <div className="page-head"><div><span className="eyebrow">مالی و کنترل</span><h1>چک و بانک</h1><p>چک‌های دریافتی و پرداختی، سررسیدها، وصول، برگشت و خرج‌کردن چک را با سند حسابداری مدیریت کن.</p></div><div className="head-actions">{tab!=="banks"&&<button className="primary-btn" onClick={()=>openCheck(tab==="payable"?"payable":"receivable")} disabled={tab==="payable"&&!canManagePayable}>{tab==="receivable"?"+ چک دریافتی":"+ چک پرداختی"}</button>}{tab==="banks"&&canCreateBank&&<button className="primary-btn" onClick={()=>setBankOpen(true)}>+ حساب بانکی</button>}</div></div>
    {success&&<div className="success-box">{success}</div>}{error&&!checkOpen&&!bankOpen&&!action&&!ledger&&<div className="error-box">{error}</div>}
    <section className="check-kpis">
      <article><span>چک‌های دریافتنی باز</span><strong>{money(summary.receivable_open_amount)} تومان</strong></article>
      <article><span>چک‌های پرداختنی باز</span><strong>{money(summary.payable_open_amount)} تومان</strong></article>
      <article className={summary.due_today_count?"attention":""}><span>سررسید امروز</span><strong>{money(summary.due_today_count)} فقره</strong></article>
      <article className={summary.overdue_count?"danger":""}><span>سررسید گذشته</span><strong>{money(summary.overdue_count)} فقره</strong></article>
      <article><span>۷ روز آینده</span><strong>{money(summary.due_next_7_count)} فقره</strong></article>
      <article className={summary.bounced_count?"danger":""}><span>برگشتی</span><strong>{money(summary.bounced_count)} فقره</strong></article>
    </section>
    <section className="panel checks-panel">
      <div className="checks-toolbar"><div className="segmented"><button className={tab==="receivable"?"active":""} onClick={()=>{setTab("receivable");setStatus("")}}>دریافتی</button><button className={tab==="payable"?"active":""} onClick={()=>{setTab("payable");setStatus("")}}>پرداختی</button><button className={tab==="banks"?"active":""} onClick={()=>setTab("banks")}>بانک‌ها</button></div>{tab!=="banks"&&<><input value={q} onChange={e=>setQ(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")void search()}} placeholder="شماره چک، صیاد یا نام شخص..."/><select value={status} onChange={e=>setStatus(e.target.value)}><option value="">همه وضعیت‌ها</option>{tab==="receivable"?<><option value="held">در صندوق</option><option value="deposited">واگذار به بانک</option><option value="cleared">وصول‌شده</option><option value="bounced">برگشتی</option><option value="endorsed">خرج‌شده</option><option value="returned">عودت‌شده</option></>:<><option value="issued">صادرشده</option><option value="cleared">پاس‌شده</option><option value="returned">برگشتی</option><option value="cancelled">باطل</option></>}</select><button className="ghost-btn" disabled={loading} onClick={()=>void search()}>{loading?"در حال دریافت...":"جست‌وجو"}</button></>}</div>
      {tab!=="banks"&&<div className="checks-list-meta"><span>{new Intl.NumberFormat("fa-IR").format(total)} چک</span><small>{new Intl.NumberFormat("fa-IR").format(checks.length)} مورد نمایش داده شده</small></div>}
      {tab==="banks"?<div className="bank-card-grid">{banks.map(b=><article key={b.id} className="bank-card"><div><span>{b.bank_name}</span><h3>{b.name}</h3>{b.is_default&&<em>پیش‌فرض</em>}</div><strong>{money(b.balance)} تومان</strong><small>{b.iban||b.account_number||"شماره حساب ثبت نشده"}</small><button className="ghost-btn" onClick={()=>void openLedger(b)} disabled={busy}>گردش حساب</button></article>)}{!banks.length&&<div className="table-state">هنوز حساب بانکی ثبت نشده است. برای وصول چک‌ها یک حساب بانکی بساز.</div>}</div>:<div className="table-wrap"><table className="checks-table"><thead><tr><th>شماره / صیاد</th><th>{tab==="receivable"?"مشتری":"تأمین‌کننده"}</th><th>مبلغ</th><th>سررسید</th><th>وضعیت</th><th>بانک / مقصد</th><th></th></tr></thead><tbody>{checks.map(x=><tr key={x.id} className={x.status==="bounced"?"check-row-danger":""}><td><b dir="ltr">{x.check_number}</b>{x.sayad_id&&<small dir="ltr">صیاد: {x.sayad_id}</small>}</td><td><b>{x.customer_name||x.supplier_name}</b>{x.bank_name&&<small>{x.bank_name}</small>}</td><td><strong>{money(x.amount)}</strong><small>تومان</small></td><td><b>{faDate(x.due_date)}</b><small>{x.due_date}</small></td><td><span className={`check-status ${statusClass[x.status]||"muted"}`}>{statusLabel[x.status]||x.status}</span></td><td>{x.bank_account_name||x.endorsed_supplier_name||"—"}</td><td>{actionButtons(x)}</td></tr>)}</tbody></table>{loading&&!checks.length&&<div className="table-state">در حال دریافت چک‌ها...</div>}{!loading&&!checks.length&&<div className="table-state">چکی در این وضعیت پیدا نشد.</div>}</div>}
      {tab!=="banks"&&next&&<button className="load-more" disabled={loading} onClick={()=>void load(next,true).catch(e=>setError(friendlyError(e)))}>{loading?"در حال دریافت...":"نمایش بیشتر"}</button>}
    </section>

    <Modal open={checkOpen} onClose={()=>setCheckOpen(false)} title={direction==="receivable"?"ثبت چک دریافتی":"صدور چک پرداختی"} subtitle="ثبت چک هم‌زمان مانده شخص و حساب چک‌ها را به‌روزرسانی می‌کند."><div className="form-stack check-form"><label>{direction==="receivable"?"مشتری":"تأمین‌کننده"}<select value={checkForm.partyId} onChange={e=>setCheckForm({...checkForm,partyId:e.target.value})}><option value="">انتخاب...</option>{eligibleParties.map(x=><option key={x.id} value={x.id}>{x.name} — مانده {money(x.balance)}</option>)}</select></label><div className="check-form-grid"><label>شماره چک<input dir="ltr" value={checkForm.checkNumber} onChange={e=>setCheckForm({...checkForm,checkNumber:e.target.value})}/></label><label>شناسه صیاد<input dir="ltr" maxLength={16} value={checkForm.sayadId} onChange={e=>setCheckForm({...checkForm,sayadId:e.target.value})} placeholder="۱۶ رقم - اختیاری"/></label><label>بانک روی چک<input value={checkForm.bankName} onChange={e=>setCheckForm({...checkForm,bankName:e.target.value})}/></label><label>شعبه<input value={checkForm.branchName} onChange={e=>setCheckForm({...checkForm,branchName:e.target.value})}/></label><label>مبلغ (تومان)<input type="number" min="1" max={selectedParty?.balance||undefined} value={checkForm.amount||""} onChange={e=>setCheckForm({...checkForm,amount:Number(e.target.value)||0})}/>{selectedParty&&<small className={amountTooHigh?"field-error-hint":"field-hint"}>حداکثر قابل ثبت: {money(selectedParty.balance)} تومان{amountTooHigh?" — مبلغ واردشده بیشتر از مانده است.":""}</small>}</label><label>تاریخ صدور<input dir="ltr" value={checkForm.issueDate} onChange={e=>setCheckForm({...checkForm,issueDate:e.target.value})} placeholder="۱۴۰۵/۰۵/۲۷"/></label><label>تاریخ سررسید<input dir="ltr" value={checkForm.dueDate} onChange={e=>setCheckForm({...checkForm,dueDate:e.target.value})} placeholder="۱۴۰۵/۰۶/۲۷"/></label></div><small className="date-hint">تاریخ شمسی با ارقام فارسی/انگلیسی و قالب ۱۴۰۵/۰۵/۲۷ پذیرفته می‌شود.</small><label>توضیح<textarea value={checkForm.note} onChange={e=>setCheckForm({...checkForm,note:e.target.value})}/></label>{error&&<div className="error-box">{error}</div>}</div><div className="modal-actions"><button className="ghost-btn" onClick={()=>setCheckOpen(false)}>انصراف</button><button className="primary-btn" disabled={busy||!checkForm.partyId||!checkForm.checkNumber||!checkForm.dueDate||checkForm.amount<=0||amountTooHigh} onClick={()=>void submitCheck()}>{busy?"در حال ثبت...":"ثبت چک"}</button></div></Modal>

    <Modal open={bankOpen} onClose={()=>setBankOpen(false)} title="حساب بانکی فروشگاه" subtitle="حسابی که وصول چک‌ها و گردش بانکی روی آن ثبت می‌شود."><div className="form-stack check-form"><div className="check-form-grid"><label>نام حساب<input value={bankForm.name} onChange={e=>setBankForm({...bankForm,name:e.target.value})} placeholder="مثلاً جاری فروشگاه"/></label><label>نام بانک<input value={bankForm.bankName} onChange={e=>setBankForm({...bankForm,bankName:e.target.value})} placeholder="ملت"/></label><label>شماره حساب<input dir="ltr" value={bankForm.accountNumber} onChange={e=>setBankForm({...bankForm,accountNumber:e.target.value})}/></label><label>شماره کارت<input dir="ltr" value={bankForm.cardNumber} onChange={e=>setBankForm({...bankForm,cardNumber:e.target.value})}/></label><label>شبا<input dir="ltr" value={bankForm.iban} onChange={e=>setBankForm({...bankForm,iban:e.target.value})} placeholder="IR..."/></label><label>مانده افتتاحیه (تومان)<input type="number" min="0" value={bankForm.openingBalance||""} onChange={e=>setBankForm({...bankForm,openingBalance:Number(e.target.value)||0})}/></label></div><label className="hardware-check"><input type="checkbox" checked={bankForm.isDefault} onChange={e=>setBankForm({...bankForm,isDefault:e.target.checked})}/> حساب پیش‌فرض برای وصول چک</label>{error&&<div className="error-box">{error}</div>}</div><div className="modal-actions"><button className="ghost-btn" onClick={()=>setBankOpen(false)}>انصراف</button><button className="primary-btn" disabled={busy||!bankForm.name||!bankForm.bankName} onClick={()=>void submitBank()}>{busy?"در حال ثبت...":"ایجاد حساب"}</button></div></Modal>

    <Modal open={!!action} onClose={()=>setAction(null)} title="تغییر وضعیت چک" subtitle={action?`${action.check.check_number} • ${money(action.check.amount)} تومان`:undefined}><div className="form-stack check-form">{action&&(action.action==="deposit"||action.action==="clear")&&<label>حساب بانکی<select value={actionBank} onChange={e=>setActionBank(e.target.value)}><option value="">انتخاب...</option>{banks.filter(x=>x.active).map(x=><option key={x.id} value={x.id}>{x.bank_name} — {x.name}</option>)}</select></label>}{action?.action==="endorse"&&<label>خرج به نام تأمین‌کننده<select value={actionSupplier} onChange={e=>setActionSupplier(e.target.value)}><option value="">انتخاب...</option>{suppliers.filter(x=>x.balance>=(action?.check.amount||0)).map(x=><option key={x.id} value={x.id}>{x.name} — بدهی {money(x.balance)}</option>)}</select></label>}<label>توضیح<textarea value={actionNote} onChange={e=>setActionNote(e.target.value)} placeholder="اختیاری"/></label>{action?.action==="bounce"&&<div className="check-warning">با ثبت برگشتی، طلب مشتری دوباره به حساب او برمی‌گردد و سند چک دریافتنی معکوس می‌شود.</div>}{error&&<div className="error-box">{error}</div>}</div><div className="modal-actions"><button className="ghost-btn" onClick={()=>setAction(null)}>انصراف</button><button className="primary-btn" disabled={busy} onClick={()=>void submitAction()}>{busy?"در حال ثبت...":"تأیید و ثبت سند"}</button></div></Modal>

    <Modal open={!!ledger} onClose={()=>setLedger(null)} title="گردش حساب بانکی" subtitle={ledger?`${ledger.account.bank_name} — ${ledger.account.name}`:undefined}><>{ledger&&<div className="bank-ledger"><div className="settlement-balance"><span>مانده فعلی</span><b>{money(ledger.account.balance)} تومان</b></div>{ledger.items.length?<div className="statement-list">{ledger.items.map(x=><div className="statement-row" key={x.id}><div><b>{x.reference_type}</b><small>{new Date(x.posted_at).toLocaleString("fa-IR")}</small></div><div className={x.change>=0?"statement-debit":"statement-credit"}><strong>{x.change>=0?"+":"-"} {money(x.change)}</strong><small>مانده: {money(x.balance)}</small></div></div>)}</div>:<div className="table-state">هنوز گردش بانکی ثبت نشده است.</div>}</div>}</><div className="modal-actions"><button className="primary-btn" onClick={()=>setLedger(null)}>بستن</button></div></Modal>
  </>;
}
