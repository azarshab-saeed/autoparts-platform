"use client";

import { useEffect } from "react";

export default function StoreError({error,reset}:{error:Error & {digest?:string};reset:()=>void}){
  useEffect(()=>{console.error("store route error",error);},[error]);
  return <div className="panel route-error-card">
    <span className="eyebrow">خطای موقت</span>
    <h2>این بخش درست بارگذاری نشد</h2>
    <p>عملیات ثبت‌شده دوباره ارسال نمی‌شود. صفحه را دوباره امتحان کن؛ اگر خطا تکرار شد Request ID پاسخ API را برای بررسی نگه دار.</p>
    <button className="primary-btn" onClick={reset}>تلاش دوباره</button>
  </div>;
}
