import Link from "next/link";

export default function Home(){
  return <main className="role-home">
    <section className="role-home-copy">
      <div className="role-logo">ی</div>
      <span>شبکه هوشمند قطعات خودرو</span>
      <h1>یک سیستم، دو تجربه ساده.</h1>
      <p>فروشگاه مغازه‌اش را مدیریت می‌کند؛ مکانیک قطعه را بین فروشگاه‌ها پیدا می‌کند.</p>
    </section>
    <section className="role-cards">
      <Link href="/mechanic" className="role-card mechanic-role"><div className="role-icon">⌕</div><div><small>برای مکانیک و خریدار</small><h2>پیدا کردن قطعه</h2><p>موجودی، قیمت و فاصله فروشگاه‌ها را مقایسه کن.</p></div><b>شروع جست‌وجو ←</b></Link>
      <Link href="/login" className="role-card store-role"><div className="role-icon">▦</div><div><small>برای فروشگاه لوازم یدکی</small><h2>مدیریت فروشگاه</h2><p>فروش، خرید، انبار، حساب‌ها و حضور در شبکه.</p></div><b>ورود به پنل ←</b></Link>
    </section>
  </main>
}
