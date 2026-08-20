"use client";

import Link from "next/link";
import { FormEvent, useEffect, useMemo, useState } from "react";
import QRCode from "qrcode";
import { useAuth } from "@/components/auth-provider";
import {
  addVehicleNotebookEntry,
  createVehicleNotebook,
  getVehicleNotebookByToken,
  listVehicleNotebooks,
  rotateVehicleOwnerCode,
  type AddVehicleNotebookEntryInput,
  type VehicleNotebookDetail,
  type VehicleNotebookEntryKind,
  type VehicleNotebookVehicle,
} from "@/lib/vehicle-notebook";

const faNumber = (v?: number) => v == null ? "—" : new Intl.NumberFormat("fa-IR").format(v);
const faDate = (v?: string) => v ? new Intl.DateTimeFormat("fa-IR", { dateStyle: "medium" }).format(new Date(v)) : "—";
const today = () => new Date().toISOString().slice(0, 10);
const toISO = (v: string) => v ? `${v}T00:00:00Z` : undefined;

const quickServices = ["تعویض روغن", "تعویض تسمه تایم", "تعویض لنت", "تعویض شمع", "تعویض باتری", "سرویس دوره‌ای"];
const quickParts = ["تسمه تایم", "لنت ترمز", "فیلتر روغن", "فیلتر هوا", "شمع", "باتری"];

function vehicleTitle(v: VehicleNotebookVehicle) {
  const car = [v.make, v.model, v.trim].filter(Boolean).join(" ");
  return car || v.plate || v.vin || "خودرو";
}

export default function VehicleNotebookWorkspace({ mechanic = false }: { mechanic?: boolean }) {
  const { ready, session, login } = useAuth();
  const [items, setItems] = useState<VehicleNotebookVehicle[]>([]);
  const [q, setQ] = useState("");
  const [token, setToken] = useState("");
  const [detail, setDetail] = useState<VehicleNotebookDetail | null>(null);
  const [mode, setMode] = useState<"history" | "service" | "part">("history");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [ownerCode, setOwnerCode] = useState("");
  const [qrData, setQrData] = useState("");

  useEffect(() => {
    if (typeof window === "undefined") return;
    const fromQuery = new URLSearchParams(window.location.search).get("token") || "";
    if (fromQuery) setToken(fromQuery);
  }, []);

  useEffect(() => {
    if (!ready || !session || mechanic) return;
    void refreshList("");
  }, [ready, session, mechanic]);

  useEffect(() => {
    if (!ready || !session || !token) return;
    void openToken(token);
  }, [ready, session, token]);

  useEffect(() => {
    if (!detail || typeof window === "undefined") {
      setQrData("");
      return;
    }
    const url = `${window.location.origin}/vehicle/${detail.vehicle.public_token}`;
    void QRCode.toDataURL(url, { width: 260, margin: 1, errorCorrectionLevel: "M" }).then(setQrData).catch(() => setQrData(""));
  }, [detail]);

  async function refreshList(term: string) {
    if (!session) return;
    setLoading(true); setError("");
    try { setItems(await listVehicleNotebooks(session, term)); }
    catch (e) { setError(e instanceof Error ? e.message : "دفتر خودروها خوانده نشد."); }
    finally { setLoading(false); }
  }

  async function openToken(value: string) {
    if (!session || !value.trim()) return;
    setLoading(true); setError(""); setNotice("");
    try {
      const out = await getVehicleNotebookByToken(session, value.trim());
      setDetail(out); setMode("history"); setToken(out.vehicle.public_token);
    } catch (e) { setError(e instanceof Error ? e.message : "QR خودرو پیدا نشد."); }
    finally { setLoading(false); }
  }

  async function createVehicle(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!session) return;
    const fd = new FormData(e.currentTarget);
    const year = String(fd.get("model_year") || "").trim();
    setLoading(true); setError(""); setNotice("");
    try {
      const v = await createVehicleNotebook(session, {
        owner_name: text(fd, "owner_name"), owner_phone: text(fd, "owner_phone"), plate: text(fd, "plate"), vin: text(fd, "vin"),
        make: text(fd, "make"), model: text(fd, "model"), trim: text(fd, "trim"), model_year: year ? Number(year) : undefined,
      });
      setOwnerCode(v.owner_code || ""); setShowCreate(false); setToken(v.public_token);
      await refreshList("");
      setNotice("دفتر خودرو ساخته شد. QR را به مالک بدهید.");
    } catch (err) { setError(err instanceof Error ? err.message : "ثبت خودرو انجام نشد."); }
    finally { setLoading(false); }
  }

  async function saveEntry(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    if (!session || !detail) return;
    const fd = new FormData(e.currentTarget);
    const mileage = numberField(fd, "mileage");
    const nextMileage = numberField(fd, "next_due_mileage");
    const input: AddVehicleNotebookEntryInput = {
      kind: mode as VehicleNotebookEntryKind,
      title: String(fd.get("title") || "").trim(),
      mileage,
      occurred_on: toISO(String(fd.get("occurred_on") || "")),
      next_due_mileage: nextMileage,
      next_due_date: toISO(String(fd.get("next_due_date") || "")),
      notes: text(fd, "notes"),
    };
    if (!input.title) { setError("اسم سرویس یا قطعه را وارد کنید."); return; }
    setLoading(true); setError("");
    try {
      await addVehicleNotebookEntry(session, detail.vehicle.public_token, input);
      setDetail(await getVehicleNotebookByToken(session, detail.vehicle.public_token));
      setMode("history"); setNotice("در دفتر خودرو ثبت شد.");
    } catch (err) { setError(err instanceof Error ? err.message : "ثبت سابقه انجام نشد."); }
    finally { setLoading(false); }
  }

  async function newOwnerCode() {
    if (!session || !detail) return;
    setLoading(true); setError("");
    try { setOwnerCode(await rotateVehicleOwnerCode(session, detail.vehicle.id)); setNotice("کد جدید مالک ساخته شد؛ کد قبلی دیگر معتبر نیست."); }
    catch (err) { setError(err instanceof Error ? err.message : "ساخت کد مالک انجام نشد."); }
    finally { setLoading(false); }
  }

  if (!ready) return <div className="vehicle-book-loading">در حال آماده‌سازی دفتر خودرو...</div>;
  if (!session) return <div className="vehicle-book-login"><b>برای ثبت سابقه وارد حساب شوید.</b><button onClick={() => void login(mechanic ? "/mechanic/vehicle-notebook" : "/store/vehicle-notebook")}>ورود</button></div>;

  return <div className="vehicle-book-page">
    <section className="vehicle-book-hero">
      <div><span className="vehicle-book-kicker">دفتر خودرو</span><h1>QR را بگیر، سابقه را فراموش نکن.</h1><p>{mechanic ? "QR مشتری را اسکن کن و در چند ثانیه سرویس یا قطعه مصرف‌شده را ثبت کن." : "برای هر خودرو یک دفتر ساده: سرویس، قطعه، کیلومتر و یادآوری بعدی."}</p></div>
      {!mechanic && <button className="primary" onClick={() => setShowCreate(v => !v)}>+ خودرو جدید</button>}
    </section>

    {error && <div className="vehicle-book-alert error">{error}</div>}
    {notice && <div className="vehicle-book-alert success">{notice}</div>}

    {showCreate && !mechanic && <form className="vehicle-book-create" onSubmit={createVehicle}>
      <div className="vehicle-book-section-title"><div><b>خودرو جدید</b><span>فقط یکی از پلاک، VIN یا موبایل مالک کافی است.</span></div></div>
      <div className="vehicle-book-grid compact">
        <label><span>پلاک</span><input name="plate" placeholder="12الف345 ایران 67" autoFocus /></label>
        <label><span>موبایل مالک</span><input name="owner_phone" inputMode="tel" placeholder="0912..." /></label>
        <label><span>نام مالک</span><input name="owner_name" /></label>
        <label><span>سازنده</span><input name="make" placeholder="پژو" /></label>
        <label><span>مدل</span><input name="model" placeholder="206" /></label>
        <label><span>تیپ</span><input name="trim" placeholder="تیپ 5" /></label>
        <label><span>سال</span><input name="model_year" inputMode="numeric" placeholder="1399" /></label>
        <label><span>VIN / شاسی</span><input name="vin" dir="ltr" /></label>
      </div>
      <div className="vehicle-book-form-actions"><button className="primary" disabled={loading}>ساخت دفتر و QR</button><button type="button" onClick={() => setShowCreate(false)}>انصراف</button></div>
    </form>}

    <section className="vehicle-book-lookup">
      {mechanic ? <>
        <div className="vehicle-book-section-title"><div><b>اسکن یا وارد کردن QR</b><span>بعد از اسکن، لینک خودرو مستقیم همین صفحه را باز می‌کند.</span></div></div>
        <div className="vehicle-book-search"><input value={token} onChange={e => setToken(e.target.value)} placeholder="شناسه QR خودرو" dir="ltr" /><button onClick={() => void openToken(token)} disabled={loading || !token.trim()}>باز کردن دفتر</button></div>
      </> : <>
        <div className="vehicle-book-section-title"><div><b>خودروهای فروشگاه</b><span>با پلاک، موبایل، نام مالک یا مدل پیدا کنید.</span></div></div>
        <form className="vehicle-book-search" onSubmit={e => { e.preventDefault(); void refreshList(q); }}><input value={q} onChange={e => setQ(e.target.value)} placeholder="مثلاً 206 یا 0912..." /><button disabled={loading}>جست‌وجو</button></form>
        <div className="vehicle-book-list">{items.map(v => <button key={v.id} className={detail?.vehicle.public_token === v.public_token ? "active" : ""} onClick={() => setToken(v.public_token)}><b>{vehicleTitle(v)}</b><span>{v.plate || "بدون پلاک"}{v.owner_name ? ` · ${v.owner_name}` : ""}</span></button>)}{!loading && !items.length && <div className="vehicle-book-empty">هنوز خودرویی ثبت نشده است.</div>}</div>
      </>}
    </section>

    {detail && <section className="vehicle-book-detail">
      <div className="vehicle-book-card-head">
        <div><span>خودرو</span><h2>{vehicleTitle(detail.vehicle)}</h2><p>{detail.vehicle.plate || "بدون پلاک"}{detail.vehicle.model_year ? ` · مدل ${faNumber(detail.vehicle.model_year)}` : ""}</p></div>
        <div className="vehicle-book-head-actions"><Link href={`/vehicle/${detail.vehicle.public_token}`} target="_blank">نمای مالک</Link>{!mechanic && <button onClick={() => void newOwnerCode()}>کد جدید مالک</button>}</div>
      </div>

      {!mechanic && (detail.vehicle.owner_name || detail.vehicle.owner_phone) && <div className="vehicle-book-owner"><span>مالک</span><b>{detail.vehicle.owner_name || "—"}</b><small>{detail.vehicle.owner_phone || ""}</small></div>}

      {ownerCode && <div className="vehicle-book-owner-code"><div><b>کد مالک: <strong>{ownerCode}</strong></b><span>این کد را فقط به مالک بدهید؛ برای ثبت کیلومتر از صفحه QR استفاده می‌شود.</span></div><button onClick={() => navigator.clipboard?.writeText(ownerCode)}>کپی</button></div>}

      <div className="vehicle-book-actions">
        <button className={mode === "service" ? "active" : ""} onClick={() => setMode("service")}><span>🔧</span><b>ثبت سرویس</b><small>کار انجام‌شده</small></button>
        <button className={mode === "part" ? "active" : ""} onClick={() => setMode("part")}><span>⚙️</span><b>ثبت قطعه</b><small>قطعه مصرف‌شده</small></button>
        <button className={mode === "history" ? "active" : ""} onClick={() => setMode("history")}><span>🕘</span><b>سوابق</b><small>{faNumber(detail.entries.length)} مورد</small></button>
      </div>

      {(mode === "service" || mode === "part") && <EntryForm kind={mode} loading={loading} onSubmit={saveEntry} />}
      {mode === "history" && <History entries={detail.entries} />}

      {!mechanic && <div className="vehicle-book-qr-card">
        <div>{qrData ? <img src={qrData} alt="QR دفتر خودرو" /> : <div className="qr-placeholder">QR</div>}</div>
        <div><b>QR ثابت این خودرو</b><p>مالک این QR را نگه می‌دارد. با اسکن، سوابق و موعد سرویس‌ها باز می‌شود.</p><code>{detail.vehicle.public_token}</code></div>
      </div>}
    </section>}
  </div>;
}

function EntryForm({ kind, loading, onSubmit }: { kind: "service" | "part"; loading: boolean; onSubmit: (e: FormEvent<HTMLFormElement>) => void }) {
  const [title, setTitle] = useState("");
  const [more, setMore] = useState(false);
  const chips = kind === "service" ? quickServices : quickParts;
  return <form className="vehicle-book-entry-form" onSubmit={onSubmit}>
    <div className="vehicle-book-entry-main">
      <label><span>{kind === "service" ? "چه کاری انجام شد؟" : "چه قطعه‌ای مصرف شد؟"}</span><input name="title" value={title} onChange={e => setTitle(e.target.value)} placeholder={kind === "service" ? "مثلاً تعویض تسمه تایم" : "مثلاً تسمه تایم INA"} autoFocus /></label>
      <label><span>کیلومتر</span><input name="mileage" inputMode="numeric" placeholder="87400" /></label>
      <label><span>تاریخ</span><input name="occurred_on" type="date" defaultValue={today()} /></label>
    </div>
    <div className="vehicle-book-chips">{chips.map(x => <button type="button" key={x} onClick={() => setTitle(x)}>{x}</button>)}</div>
    <button type="button" className="vehicle-book-more" onClick={() => setMore(v => !v)}>{more ? "جزئیات کمتر" : "+ یادآوری / توضیحات"}</button>
    {more && <div className="vehicle-book-grid compact more">
      <label><span>کیلومتر بعدی</span><input name="next_due_mileage" inputMode="numeric" placeholder="147400" /></label>
      <label><span>تاریخ بعدی</span><input name="next_due_date" type="date" /></label>
      <label className="wide"><span>توضیحات</span><input name="notes" placeholder="برند، ضمانت یا نکته کوتاه" /></label>
    </div>}
    <button className="primary save-entry" disabled={loading || !title.trim()}>ذخیره در دفتر خودرو</button>
  </form>;
}

function History({ entries }: { entries: VehicleNotebookDetail["entries"] }) {
  const latestMileage = useMemo(() => entries.find(x => x.mileage != null)?.mileage, [entries]);
  const next = useMemo(() => entries.find(x => x.next_due_mileage != null || x.next_due_date), [entries]);
  return <div className="vehicle-book-history">
    <div className="vehicle-book-summary"><div><span>آخرین کیلومتر</span><b>{faNumber(latestMileage)}</b></div><div><span>سرویس بعدی</span><b>{next?.next_due_mileage ? `${faNumber(next.next_due_mileage)} km` : next?.next_due_date ? faDate(next.next_due_date) : "ثبت نشده"}</b></div></div>
    <div className="vehicle-book-timeline">{entries.map(e => <article key={e.id}>
      <div className={`entry-dot ${e.kind}`}>{e.kind === "service" ? "🔧" : e.kind === "part" ? "⚙️" : e.kind === "mileage" ? "◉" : "•"}</div>
      <div className="entry-body"><div className="entry-title"><b>{e.title}</b><span>{faDate(e.occurred_on)}</span></div><div className="entry-meta">{e.mileage != null && <span>{faNumber(e.mileage)} کیلومتر</span>}<span>{e.actor_name}</span>{e.owner_reported && <span className="owner-report">ثبت مالک</span>}</div>{e.notes && <p>{e.notes}</p>}{(e.next_due_mileage || e.next_due_date) && <div className="entry-next">یادآوری: {e.next_due_mileage ? `${faNumber(e.next_due_mileage)} کیلومتر` : ""}{e.next_due_mileage && e.next_due_date ? " یا " : ""}{e.next_due_date ? faDate(e.next_due_date) : ""}</div>}</div>
    </article>)}{!entries.length && <div className="vehicle-book-empty">هنوز سابقه‌ای ثبت نشده است.</div>}</div>
  </div>;
}

function text(fd: FormData, key: string) { const v = String(fd.get(key) || "").trim(); return v || undefined; }
function numberField(fd: FormData, key: string) { const v = String(fd.get(key) || "").trim(); if (!v) return undefined; const n = Number(v); return Number.isFinite(n) ? n : undefined; }
