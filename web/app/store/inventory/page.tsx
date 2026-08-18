"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { getInventory, postInventoryAdjustment, putReorderPoint } from "@/lib/api";
import { useAuth } from "@/components/auth-provider";
import Modal from "@/components/modal";
import { SearchIcon } from "@/components/icons";
import type { InventoryStock } from "@/lib/types";

const number = (v: number) => new Intl.NumberFormat("fa-IR", { maximumFractionDigits: 2 }).format(v);
const money = (v: number) => new Intl.NumberFormat("fa-IR").format(v) + " تومان";

export default function InventoryPage() {
  const { session } = useAuth();
  const [rows, setRows] = useState<InventoryStock[]>([]);
  const [query, setQuery] = useState("");
  const [lowOnly, setLowOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [selected, setSelected] = useState<InventoryStock | null>(null);
  const [dialog, setDialog] = useState<"adjust" | "reorder" | null>(null);
  const [qtyDelta, setQtyDelta] = useState("");
  const [reason, setReason] = useState("");
  const [minQty, setMinQty] = useState("");
  const [targetQty, setTargetQty] = useState("");
  const [saving, setSaving] = useState(false);

  const canManage = Boolean(session?.roles.some(r => ["owner", "admin", "warehouse"].includes(r)));

  const load = useCallback(async () => {
    if (!session) return;
    setLoading(true);
    setError("");
    try {
      setRows(await getInventory(session, false));
    } catch (e) {
      setError(e instanceof Error ? e.message : "دریافت موجودی ناموفق بود.");
    } finally {
      setLoading(false);
    }
  }, [session]);

  useEffect(() => { void load(); }, [load]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return rows.filter(x => {
      if (lowOnly && !x.low_stock) return false;
      if (!q) return true;
      return [x.title, x.sku].filter(Boolean).some(v => String(v).toLowerCase().includes(q));
    });
  }, [rows, query, lowOnly]);

  const stats = useMemo(() => ({
    sku: rows.length,
    low: rows.filter(x => x.low_stock).length,
    units: rows.reduce((s, x) => s + x.available, 0),
    value: rows.reduce((s, x) => s + x.on_hand * x.avg_unit_cost, 0)
  }), [rows]);

  function openAdjust(row: InventoryStock) {
    setSelected(row);
    setQtyDelta("");
    setReason("");
    setDialog("adjust");
    setError("");
    setSuccess("");
  }

  function openReorder(row: InventoryStock) {
    setSelected(row);
    setMinQty(String(row.min_qty || 0));
    setTargetQty(String(row.target_qty || 0));
    setDialog("reorder");
    setError("");
    setSuccess("");
  }

  async function saveAdjustment() {
    if (!session || !selected) return;
    const delta = Number(qtyDelta);
    if (!Number.isFinite(delta) || delta === 0 || !reason.trim()) {
      setError("تعداد اصلاح و دلیل را کامل وارد کن.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await postInventoryAdjustment(session, selected.product_id, delta, reason.trim());
      setDialog(null);
      setSuccess(`موجودی «${selected.title}» با موفقیت اصلاح شد.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "اصلاح موجودی ناموفق بود.");
    } finally {
      setSaving(false);
    }
  }

  async function saveReorder() {
    if (!session || !selected) return;
    const min = Number(minQty);
    const target = Number(targetQty);
    if (!Number.isFinite(min) || !Number.isFinite(target) || min < 0 || target < min) {
      setError("حد هدف باید بزرگ‌تر یا مساوی حداقل موجودی باشد.");
      return;
    }
    setSaving(true);
    setError("");
    try {
      await putReorderPoint(session, selected.product_id, min, target);
      setDialog(null);
      setSuccess(`حد سفارش «${selected.title}» ذخیره شد.`);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "ثبت حد سفارش ناموفق بود.");
    } finally {
      setSaving(false);
    }
  }

  return <>
    <div className="page-head">
      <div>
        <span className="eyebrow">انبار اصلی</span>
        <h1>انبار و کالاها</h1>
        <p>موجودی واقعی، رزرو، بهای میانگین و نقطه سفارش را یکجا کنترل کن.</p>
      </div>
      <div className="head-actions"><Link className="primary-btn" href="/store/products/new">+ کالای جدید</Link><Link className="ghost-btn" href="/store/purchases">+ ثبت خرید</Link></div>
    </div>

    {success && <div className="success-box page-error">{success}</div>}
    {error && !dialog && <div className="error-box page-error">{error}</div>}

    <section className="inventory-stats">
      <article><span>کالاهای موجود</span><strong>{number(stats.sku)}</strong><small>ردیف موجودی</small></article>
      <article><span>رو به اتمام</span><strong className={stats.low ? "danger-text" : ""}>{number(stats.low)}</strong><small>نیازمند توجه</small></article>
      <article><span>قابل فروش</span><strong>{number(stats.units)}</strong><small>جمع مقادیر در واحد پایه هر کالا</small></article>
      <article><span>ارزش تقریبی انبار</span><strong>{money(stats.value)}</strong><small>بر اساس بهای میانگین</small></article>
    </section>

    <section className="panel inventory-panel">
      <div className="inventory-toolbar">
        <div className="inline-search"><SearchIcon/><input value={query} onChange={e => setQuery(e.target.value)} placeholder="نام یا کد کالا را جست‌وجو کن..."/></div>
        <label className="switch-filter"><input type="checkbox" checked={lowOnly} onChange={e => setLowOnly(e.target.checked)}/><span>فقط رو به اتمام</span></label>
        <button className="ghost-btn" type="button" onClick={() => void load()}>بروزرسانی</button>
      </div>

      <div className="table-wrap">
        <table className="inventory-table">
          <thead><tr><th>کالا</th><th>موجودی</th><th>رزرو</th><th>قابل فروش</th><th>بهای میانگین</th><th>حد سفارش</th><th>وضعیت</th><th></th></tr></thead>
          <tbody>
            {loading && <tr><td colSpan={8}><div className="table-state">در حال دریافت موجودی...</div></td></tr>}
            {!loading && !filtered.length && <tr><td colSpan={8}><div className="table-state">کالایی با این فیلتر پیدا نشد.</div></td></tr>}
            {!loading && filtered.map(row => <tr key={row.product_id}>
              <td><div className="product-cell"><b>{row.title}</b><span>{row.sku || "بدون کد کالا"}</span></div></td>
              <td>{number(row.on_hand)} <small>{row.base_unit_name||"واحد پایه"}</small></td>
              <td>{number(row.reserved)} <small>{row.base_unit_name||""}</small></td>
              <td><b>{number(row.available)}</b> <small>{row.base_unit_name||""}</small></td>
              <td>{money(row.avg_unit_cost)}</td>
              <td><span className="reorder-value">حداقل {number(row.min_qty)} / هدف {number(row.target_qty)}</span></td>
              <td>{row.low_stock ? <span className="status-pill danger">رو به اتمام</span> : <span className="status-pill ok">مناسب</span>}</td>
              <td>{canManage && <div className="row-actions"><Link href={`/store/products/${row.product_id}/units`}>واحدها/بارکد</Link><button onClick={() => openAdjust(row)}>اصلاح</button><button onClick={() => openReorder(row)}>حد سفارش</button></div>}</td>
            </tr>)}
          </tbody>
        </table>
      </div>
    </section>

    <Modal open={dialog === "adjust"} title="اصلاح موجودی" subtitle={selected?.title} onClose={() => setDialog(null)}>
      {error && <div className="error-box">{error}</div>}
      <div className="form-stack">
        <label>تغییر موجودی در واحد پایه {selected?.base_unit_name&&<b>({selected.base_unit_name})</b>} <small>برای کسری عدد منفی وارد کن؛ مثلاً ۲-</small><input inputMode="decimal" value={qtyDelta} onChange={e => setQtyDelta(e.target.value)} placeholder="-2 یا 5"/></label>
        <label>دلیل اصلاح<textarea value={reason} onChange={e => setReason(e.target.value)} placeholder="مثلاً شمارش فیزیکی انبار، شکستگی یا کسری"/></label>
      </div>
      <div className="modal-actions"><button className="ghost-btn" onClick={() => setDialog(null)}>انصراف</button><button className="primary-btn" disabled={saving} onClick={() => void saveAdjustment()}>{saving ? "در حال ثبت..." : "ثبت اصلاح"}</button></div>
    </Modal>

    <Modal open={dialog === "reorder"} title="حد سفارش کالا" subtitle={selected?.title} onClose={() => setDialog(null)}>
      {error && <div className="error-box">{error}</div>}
      <div className="two-field">
        <label>حداقل موجودی<input inputMode="decimal" value={minQty} onChange={e => setMinQty(e.target.value)}/></label>
        <label>موجودی هدف<input inputMode="decimal" value={targetQty} onChange={e => setTargetQty(e.target.value)}/></label>
      </div>
      <p className="form-help">وقتی موجودی قابل فروش به حداقل برسد، کالا در فهرست «رو به اتمام» دیده می‌شود.</p>
      <div className="modal-actions"><button className="ghost-btn" onClick={() => setDialog(null)}>انصراف</button><button className="primary-btn" disabled={saving} onClick={() => void saveReorder()}>{saving ? "در حال ذخیره..." : "ذخیره"}</button></div>
    </Modal>
  </>;
}
