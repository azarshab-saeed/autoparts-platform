"use client";

import { ChangeEvent, useMemo, useState } from "react";
import { exportProducts, importProducts } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import type { Product, ProductImportResult, ProductImportRow, ProductUnitInput } from "@/lib/types";

const money=(v:number)=>new Intl.NumberFormat("fa-IR").format(v)+" تومان";
const number=(v:number)=>new Intl.NumberFormat("fa-IR",{maximumFractionDigits:2}).format(v);

const digitMap:Record<string,string>={"۰":"0","۱":"1","۲":"2","۳":"3","۴":"4","۵":"5","۶":"6","۷":"7","۸":"8","۹":"9","٠":"0","١":"1","٢":"2","٣":"3","٤":"4","٥":"5","٦":"6","٧":"7","٨":"8","٩":"9"};
const ascii=(v:string)=>v.replace(/[۰-۹٠-٩]/g,ch=>digitMap[ch]||ch);
const numeric=(v:string)=>Number(ascii(v).replace(/[٬,\s]/g,"").replace(/٫/g,".").trim()||0);
const truthy=(v:string, fallback=false)=>{const x=ascii(v).trim().toLowerCase();if(!x)return fallback;return ["1","true","yes","y","on","بله","بلی","فعال","آری"].includes(x)};
const cleanHeader=(v:string)=>ascii(v).trim().toLowerCase().replace(/[ _\-]+/g," ");

const aliases:Record<string,string[]>={
  title:["نام کالا","عنوان کالا","عنوان","title","product","product title"],
  sku:["کد کالا","کد داخلی","sku","code"],
  brand:["برند","brand"],
  oem_code:["oem","کد oem","oem code","oem_code"],
  barcode:["بارکد","barcode"],
  unit:["واحد","unit"],
  fractional:["موجودی اعشاری","اعشاری","fractional","allow fractional","allow_fractional_base_qty"],
  packages:["بسته بندی","بسته‌بندی","واحدهای جایگزین","بسته ها","بسته‌ها","packages","units","alternate units"],
  on_hand:["موجودی","تعداد موجودی","on hand","on_hand","qty","quantity"],
  avg_unit_cost:["قیمت خرید","بهای خرید","میانگین خرید","avg unit cost","avg_unit_cost","cost"],
  selling_price:["قیمت فروش","selling price","selling_price","price"],
  visible:["انتشار شبکه","نمایش در شبکه","visible","network visible"],
  allow_reservation:["قابل رزرو","رزرو","allow reservation","allow_reservation"],
  allow_procurement:["تامین همکار","تأمین همکار","allow procurement","allow_procurement"],
};

function parseDelimited(text:string):string[][]{
  const normalized=text.replace(/^\uFEFF/,"").replace(/\r\n?/g,"\n");
  const first=normalized.split("\n",1)[0]||"";
  const candidates=[",",";","\t"];
  const delimiter=candidates.sort((a,b)=>(first.split(b).length-first.split(a).length))[0];
  const rows:string[][]=[];let row:string[]=[];let cell="";let quoted=false;
  for(let i=0;i<normalized.length;i++){
    const ch=normalized[i];
    if(ch==='"'){
      if(quoted&&normalized[i+1]==='"'){cell+='"';i++;}else quoted=!quoted;
    }else if(ch===delimiter&&!quoted){row.push(cell.trim());cell="";}
    else if(ch==='\n'&&!quoted){row.push(cell.trim());if(row.some(x=>x!==""))rows.push(row);row=[];cell="";}
    else cell+=ch;
  }
  row.push(cell.trim());if(row.some(x=>x!==""))rows.push(row);
  return rows;
}

function mapHeaders(headers:string[]){
  const out:Record<string,number>={};
  headers.forEach((h,i)=>{
    const normalized=cleanHeader(h);
    for(const [key,values] of Object.entries(aliases)){
      if(values.map(cleanHeader).includes(normalized)){out[key]=i;break;}
    }
  });
  return out;
}

function parsePackages(raw:string,rowNo:number,errors:string[]):ProductUnitInput[]{
  if(!raw.trim())return[];
  const out:ProductUnitInput[]=[];
  for(const token of raw.split(";").map(x=>x.trim()).filter(Boolean)){
    const [code0,name0,factor0,barcode0,sale0,purchase0,price0]=token.split("|").map(x=>x.trim());
    const factor=numeric(factor0||"");const price=numeric(price0||"");
    if(!code0||!name0||!Number.isFinite(factor)||factor<=0){errors.push(`ردیف ${rowNo}: بسته‌بندی «${token}» باید code|name|factor داشته باشد.`);continue;}
    out.push({code:code0.toLowerCase(),name:name0,factor_to_base:factor,barcode:barcode0||undefined,allow_sale:truthy(sale0,true),allow_purchase:truthy(purchase0,true),retail_price:price>0?Math.round(price):undefined});
  }
  return out;
}

function parseImport(text:string):{rows:ProductImportRow[];errors:string[]}{
  const grid=parseDelimited(text);const errors:string[]=[];
  if(grid.length<2)return{rows:[],errors:["فایل باید یک ردیف عنوان و حداقل یک ردیف کالا داشته باشد."]};
  const map=mapHeaders(grid[0]);
  if(map.title==null)return{rows:[],errors:["ستون «نام کالا» یا title در فایل پیدا نشد."]};
  const rows:ProductImportRow[]=[];const seenSKU=new Map<string,number>();const seenBarcode=new Map<string,number>();
  grid.slice(1,2001).forEach((cells,index)=>{
    const rowNo=index+2;const get=(key:string)=>map[key]==null?"":(cells[map[key]]||"").trim();
    const title=get("title");const sku=get("sku");const barcode=get("barcode");const onHand=numeric(get("on_hand"));const cost=numeric(get("avg_unit_cost"));const price=numeric(get("selling_price"));const packages=parsePackages(get("packages"),rowNo,errors);
    if(!title){errors.push(`ردیف ${rowNo}: نام کالا خالی است.`);return;}
    if(!Number.isFinite(onHand)||onHand<0){errors.push(`ردیف ${rowNo}: موجودی معتبر نیست.`);return;}
    if(!Number.isFinite(cost)||cost<0){errors.push(`ردیف ${rowNo}: قیمت خرید معتبر نیست.`);return;}
    if(!Number.isFinite(price)||price<0){errors.push(`ردیف ${rowNo}: قیمت فروش معتبر نیست.`);return;}
    if(onHand>0&&cost===0){errors.push(`ردیف ${rowNo}: برای موجودی اولیه باید قیمت خرید وارد شود.`);return;}
    if(sku){const k=sku.toLowerCase();if(seenSKU.has(k)){errors.push(`ردیف ${rowNo}: کد کالا با ردیف ${seenSKU.get(k)} تکراری است.`);return;}seenSKU.set(k,rowNo);}
    if(barcode){const k=barcode.toLowerCase();if(seenBarcode.has(k)){errors.push(`ردیف ${rowNo}: بارکد با ردیف ${seenBarcode.get(k)} تکراری است.`);return;}seenBarcode.set(k,rowNo);}
    for(const u of packages){if(!u.barcode)continue;const k=u.barcode.toLowerCase();if(seenBarcode.has(k)){errors.push(`ردیف ${rowNo}: بارکد بسته «${u.name}» با ردیف ${seenBarcode.get(k)} تکراری است.`);return;}seenBarcode.set(k,rowNo);}
    rows.push({row_number:rowNo,sku:sku||undefined,title,brand:get("brand")||undefined,oem_code:get("oem_code")||undefined,barcode:barcode||undefined,unit:get("unit")||undefined,allow_fractional_base_qty:truthy(get("fractional"),false),units:packages,on_hand:onHand,avg_unit_cost:Math.round(cost),selling_price:Math.round(price),visible:price>0&&truthy(get("visible"),false),allow_reservation:truthy(get("allow_reservation"),true),allow_procurement:truthy(get("allow_procurement"),true)});
  });
  if(grid.length-1>2000)errors.push("فایل بیش از ۲۰۰۰ ردیف دارد؛ آن را به چند فایل تقسیم کنید.");
  return{rows,errors};
}

function downloadTemplate(){
  const content=[
    "نام کالا,کد کالا,برند,OEM,بارکد,واحد,موجودی اعشاری,بسته‌بندی,موجودی,قیمت خرید,قیمت فروش,انتشار شبکه,قابل رزرو,تأمین همکار",
    '"لنت جلو پژو 206 تیپ 5",BRK-206-TXT,Textar,4254.97,2900000000017,pcs,خیر,"carton|کارتن|12|2900000000024|بله|بله|20500000;pair|جفت|2||بله|بله|3400000",12,1350000,1780000,بله,بله,بله',
    '"فیلتر روغن پژو 206",FLT-206-SRK,سرکان,206-OF-02,,pcs,خیر,"box|جعبه|10||بله|بله|",24,210000,290000,خیر,بله,بله'
  ].join("\n");
  const blob=new Blob(["\uFEFF"+content],{type:"text/csv;charset=utf-8"});const url=URL.createObjectURL(blob);const a=document.createElement("a");a.href=url;a.download="autoparts-import-template.csv";a.click();URL.revokeObjectURL(url);
}

function csvCell(v:unknown){const text=String(v??"");return /[",\n]/.test(text)?`"${text.replace(/"/g,'""')}"`:text}
function packageCell(p:Product){return (p.units||[]).filter(u=>!u.is_base).map(u=>[u.code,u.name,u.factor_to_base,u.barcode||"",u.allow_sale?"بله":"خیر",u.allow_purchase?"بله":"خیر"].join("|")).join(";")}
function downloadCatalogRows(products:Product[]){const header=["نام کالا","کد کالا","برند","OEM","بارکد","واحد","موجودی اعشاری","بسته‌بندی"];const lines=[header.join(","),...products.map(p=>[p.title,p.sku||"",p.brand||"",p.oem_code||"",p.barcode||"",p.unit,p.allow_fractional_base_qty?"بله":"خیر",packageCell(p)].map(csvCell).join(","))];const blob=new Blob(["\uFEFF"+lines.join("\n")],{type:"text/csv;charset=utf-8"});const url=URL.createObjectURL(blob);const a=document.createElement("a");a.href=url;a.download="autoparts-catalog-units.csv";a.click();URL.revokeObjectURL(url)}

export default function ImportCenter(){
  const {session}=useAuth();const[fileName,setFileName]=useState("");const[rows,setRows]=useState<ProductImportRow[]>([]);const[errors,setErrors]=useState<string[]>([]);const[busy,setBusy]=useState(false);const[result,setResult]=useState<ProductImportResult|null>(null);const[serverError,setServerError]=useState("");
  const stats=useMemo(()=>({count:rows.length,stock:rows.reduce((s,x)=>s+x.on_hand,0),value:rows.reduce((s,x)=>s+x.on_hand*x.avg_unit_cost,0),offers:rows.filter(x=>x.selling_price>0).length,published:rows.filter(x=>x.selling_price>0&&x.visible).length}),[rows]);
  async function onFile(e:ChangeEvent<HTMLInputElement>){const file=e.target.files?.[0];if(!file)return;setFileName(file.name);setResult(null);setServerError("");try{const parsed=parseImport(await file.text());setRows(parsed.rows);setErrors(parsed.errors);}catch{setRows([]);setErrors(["فایل خوانده نشد. آن را با UTF-8 و فرمت CSV ذخیره کنید."]);}}
  async function commit(){if(!session||!rows.length||errors.length)return;setBusy(true);setServerError("");try{setResult(await importProducts(session,rows));}catch(e){setServerError(e instanceof Error?e.message:"ورود اطلاعات انجام نشد.");}finally{setBusy(false);}}
  async function exportCatalog(){if(!session)return;setBusy(true);setServerError("");try{downloadCatalogRows(await exportProducts(session));}catch(e){setServerError(e instanceof Error?e.message:"خروجی کالاها ناموفق بود.");}finally{setBusy(false)}}
  return <div className="import-center">
    <div className="page-head"><div><span className="eyebrow">شروع بدون ورود دستی</span><h1>Import Center</h1><p>فهرست کالا، موجودی اولیه و قیمت‌ها را از خروجی CSV اکسل وارد کن؛ موجودی‌های زنده قبلی هرگز بازنویسی نمی‌شوند.</p></div><div className="head-actions"><button className="ghost-btn" type="button" onClick={downloadTemplate}>دانلود فایل نمونه</button><button className="ghost-btn" type="button" disabled={busy} onClick={()=>void exportCatalog()}>خروجی کالا + واحدها</button></div></div>

    <section className="adoption-callout"><div><span className="adoption-callout-icon">⇧</span><div><b>از نرم‌افزار قبلی مهاجرت می‌کنی؟</b><p>فایل Excel را با گزینه Save As → CSV UTF-8 ذخیره کن. ستون‌ها لازم نیست دقیقاً به همین ترتیب باشند.</p></div></div><div className="adoption-steps"><span><b>۱</b> خروجی CSV</span><span><b>۲</b> بررسی پیش‌نمایش</span><span><b>۳</b> ورود امن</span></div></section>

    <section className="panel import-upload-panel">
      <label className="import-dropzone"><input type="file" accept=".csv,.txt,text/csv" onChange={onFile}/><span className="import-drop-icon">CSV</span><b>{fileName||"فایل CSV را انتخاب کن"}</b><small>حداکثر ۲۰۰۰ ردیف در هر فایل · UTF-8</small><em>انتخاب فایل</em></label>
      <div className="import-rules"><b>ستون‌های قابل تشخیص</b><div><span>نام کالا <i>الزامی</i></span><span>کد کالا / SKU</span><span>OEM</span><span>برند</span><span>بارکد</span><span>بسته‌بندی / ضریب / بارکد بسته</span><span>موجودی</span><span>قیمت خرید</span><span>قیمت فروش</span><span>انتشار شبکه</span></div></div>
    </section>

    {errors.length>0&&<section className="error-box import-errors"><b>{errors.length} مورد باید اصلاح شود</b>{errors.slice(0,12).map(x=><span key={x}>{x}</span>)}{errors.length>12&&<small>و {number(errors.length-12)} خطای دیگر...</small>}</section>}
    {serverError&&<div className="error-box">{serverError}</div>}

    {rows.length>0&&<>
      <section className="import-preview-stats"><article><span>ردیف آماده</span><b>{number(rows.length)}</b></article><article><span>موجودی اولیه</span><b>{number(stats.stock)}</b></article><article><span>ارزش موجودی</span><b>{money(stats.value)}</b></article><article><span>قیمت فروش دارد</span><b>{number(stats.offers)}</b></article><article><span>انتشار در شبکه</span><b>{number(stats.published)}</b></article></section>
      <section className="panel import-preview"><div className="panel-head"><div><span className="panel-kicker">پیش‌نمایش</span><h2>قبل از ورود بررسی کن</h2><p>۲۰ ردیف اول نمایش داده شده‌اند. قیمت‌ها با واحد تومان هستند.</p></div><button className="primary-btn" disabled={busy||errors.length>0} onClick={()=>void commit()}>{busy?"در حال ورود...":`ورود ${number(rows.length)} کالا`}</button></div><div className="table-wrap"><table><thead><tr><th>ردیف</th><th>کالا</th><th>SKU / OEM</th><th>موجودی</th><th>خرید</th><th>فروش</th><th>شبکه</th></tr></thead><tbody>{rows.slice(0,20).map(x=><tr key={x.row_number}><td>{number(x.row_number)}</td><td><div className="product-cell"><b>{x.title}</b><span>{x.brand||"بدون برند"}</span></div></td><td><span className="latin-token">{[x.sku,x.oem_code].filter(Boolean).join(" / ")||"—"}</span></td><td>{number(x.on_hand)}</td><td>{money(x.avg_unit_cost)}</td><td>{x.selling_price?money(x.selling_price):"—"}</td><td>{x.visible?<span className="status-pill ok">منتشر می‌شود</span>:<span className="status-pill">خصوصی</span>}</td></tr>)}</tbody></table></div></section>
      <div className="import-safety-note"><b>محافظت از اطلاعات زنده</b><span>اگر یک کالا قبلاً گردش انبار یا رزرو داشته باشد، Import Center فقط مشخصات و قیمت عرضه را بروزرسانی می‌کند و موجودی فعلی را دست نمی‌زند.</span></div>
    </>}

    {result&&<section className="import-result"><div className="import-result-icon">✓</div><div><span>ورود اطلاعات کامل شد</span><h2>{number(result.row_count)} ردیف پردازش شد</h2><p>{number(result.created_count)} کالای جدید، {number(result.updated_count)} کالای بروزشده، {number(result.inventory_initialized_count)} موجودی اولیه و {number(result.offers_upserted_count)} قیمت عرضه ثبت شد.</p>{result.inventory_preserved_count>0&&<strong>{number(result.inventory_preserved_count)} موجودی زنده برای جلوگیری از بازنویسی حفظ شد.</strong>}<small>شناسه عملیات: <span className="latin-token">{result.batch_id}</span></small></div></section>}
  </div>;
}
