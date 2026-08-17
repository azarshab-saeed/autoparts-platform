"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { MOCK_MODE } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const { ready, authenticated, session, login } = useAuth();
  const showDev = process.env.NEXT_PUBLIC_SHOW_DEV_CREDENTIALS === "true";

  useEffect(() => {
    if (ready && authenticated) router.replace(session?.role === "mechanic" || session?.role === "consumer" ? "/mechanic" : "/store");
  }, [ready, authenticated, session, router]);

  return <main className="login-page">
    <section className="login-card">
      <div className="login-logo">ی</div>
      <h1>ورود به فروشگاه</h1>
      <p>ورود امن از طریق سامانه هویت مرکزی</p>
      {MOCK_MODE && <div className="mock-badge">حالت نمایشی فعال است.</div>}
      {!ready ? <div className="loading-auth">در حال آماده‌سازی ورود...</div> :
        <button className="keycloak-login" onClick={() => void login()}>ورود امن به پنل</button>}
      {showDev && !MOCK_MODE && <div className="dev-credentials">
        <b>کاربران نمونه توسعه</b>
        <code>یدکی رضایی: owner@example.com / ChangeMe123!</code>
        <code>یدکی مرکزی: central@example.com / ChangeMe123!</code>
        <code>پارس یدک: pars@example.com / ChangeMe123!</code>
        <code>مکانیک: mechanic@example.com / ChangeMe123!</code>
        <code>مصرف‌کننده: consumer@example.com / ChangeMe123!</code>
      </div>}
      <small className="login-security-note">رمز عبور داخل اپلیکیشن فروشگاه دریافت یا ذخیره نمی‌شود.</small>
      <a className="mechanic-entry-link" href="/mechanic">جست‌وجوی قطعه برای مکانیک‌ها ←</a>
    </section>
    <section className="login-visual">
      <div><span>فروش سریع</span><span>موجودی دقیق</span><span>حساب مشتری</span><span>شبکه قطعات</span></div>
      <h2>مغازه را ساده‌تر اداره کن.</h2>
      <p>عملیات واقعی مغازه را انجام بده؛ حسابداری پشت صحنه انجام می‌شود.</p>
    </section>
  </main>;
}
