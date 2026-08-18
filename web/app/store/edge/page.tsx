"use client";
import {useCallback,useEffect,useState} from "react";
import {useAuth} from "@/components/auth-provider";
import {createEdgePairing,getEdgeDevices,revokeEdgeDevice} from "@/lib/api";
import type {EdgeDevice,EdgePairing} from "@/lib/types";
import {getStoreEdgeStatus,triggerStoreEdgeSync,type StoreEdgeStatus,STORE_EDGE_URL} from "@/lib/edge";
import {
  applyStoreEdgeUpdate,checkStoreEdgeUpdate,getEdgeLifecycleStatus,restartStoreEdge,startStoreEdge,stopStoreEdge,
  type EdgeLifecycleStatus,type EdgeUpdateCheck,
} from "@/lib/edge-manager";
import {
  detectEdgeInstallerTarget,installerTarget,selectableInstallerTargets,type EdgeInstallerTarget,
} from "@/lib/edge-bootstrap";

function workerLabel(state?:string){
  if(state==="running")return "در حال اجرا";
  if(state==="starting")return "در حال راه‌اندازی";
  if(state==="stopped")return "متوقف";
  return state||"نامشخص";
}
function osLabel(os?:string){return os==="windows"?"Windows":os==="linux"?"Linux":os||"—"}
function serviceLabel(mode?:string){
  if(mode==="windows")return "Windows Service";
  if(mode==="systemd-user")return "Linux user service";
  return "Development";
}

export default function StoreEdgePage(){
  const {session}=useAuth();
  const [devices,setDevices]=useState<EdgeDevice[]>([]);const[pairing,setPairing]=useState<EdgePairing|null>(null);
  const [local,setLocal]=useState<StoreEdgeStatus|null>(null);const[lifecycle,setLifecycle]=useState<EdgeLifecycleStatus|null>(null);
  const [updateCheck,setUpdateCheck]=useState<EdgeUpdateCheck|null>(null);const[message,setMessage]=useState("");const[busy,setBusy]=useState(false);
  const [installer,setInstaller]=useState<EdgeInstallerTarget|null>(null);const[installWatching,setInstallWatching]=useState(false);

  const load=useCallback(async()=>{
    if(!session)return;
    const [ds,ls,lm]=await Promise.all([
      getEdgeDevices(session),
      getStoreEdgeStatus().catch(()=>null),
      getEdgeLifecycleStatus().catch(()=>null),
    ]);
    setDevices(ds);setLocal(ls);setLifecycle(lm);
  },[session]);

  useEffect(()=>{setInstaller(detectEdgeInstallerTarget())},[]);
  useEffect(()=>{void load()},[load]);
  useEffect(()=>{
    const timer=setInterval(()=>{void getEdgeLifecycleStatus().then(setLifecycle).catch(()=>setLifecycle(null))},2500);
    return()=>clearInterval(timer);
  },[]);
  useEffect(()=>{
    if(!installWatching||!lifecycle)return;
    setInstallWatching(false);
    setMessage(`Lifecycle Manager نسخه ${lifecycle.manager_version} شناسایی شد؛ Agent از همین صفحه مدیریت می‌شود.`);
    void load();
  },[installWatching,lifecycle,load]);

  async function makePair(){if(!session)return;setBusy(true);setMessage("");try{setPairing(await createEdgePairing(session));}catch(e){setMessage(e instanceof Error?e.message:"کد اتصال ساخته نشد")}finally{setBusy(false)}}
  async function revoke(id:string){if(!session||!confirm("دسترسی این Store Edge قطع شود؟ فروش‌های محلی همگام‌نشده روی همان دستگاه باقی می‌مانند."))return;setBusy(true);try{await revokeEdgeDevice(session,id);await load()}catch(e){setMessage(e instanceof Error?e.message:"قطع دسترسی انجام نشد")}finally{setBusy(false)}}
  async function sync(){setBusy(true);setMessage("");try{await triggerStoreEdgeSync();setMessage("همگام‌سازی Store Edge انجام شد.");await load()}catch(e){setMessage(e instanceof Error?e.message:"Store Edge در دسترس نیست")}finally{setBusy(false)}}
  async function lifecycleAction(action:"start"|"stop"|"restart"){
    if(action==="stop"&&local?.pending_sales&& !confirm(`${local.pending_sales.toLocaleString("fa-IR")} فروش هنوز در صف Sync است. Agent متوقف شود؟ اطلاعات محلی حذف نمی‌شود.`))return;
    if(action==="restart"&&!confirm("Store Agent چند ثانیه Restart شود؟ فروش‌های ذخیره‌شده محلی حفظ می‌شوند."))return;
    setBusy(true);setMessage("");
    try{
      if(action==="start")await startStoreEdge();
      if(action==="stop")await stopStoreEdge();
      if(action==="restart")await restartStoreEdge();
      setMessage(action==="start"?"راه‌اندازی Store Agent درخواست شد.":action==="stop"?"Store Agent متوقف شد. Manager همچنان برای راه‌اندازی مجدد فعال است.":"Store Agent Restart شد.");
      await new Promise(r=>setTimeout(r,700));await load();
    }catch(e){setMessage(e instanceof Error?e.message:"عملیات Agent انجام نشد")}finally{setBusy(false)}
  }
  async function checkUpdate(){setBusy(true);setMessage("");try{const c=await checkStoreEdgeUpdate();setUpdateCheck(c);setMessage(c.update_available?`نسخه ${c.latest_version} آماده بروزرسانی است.`:"Store Agent به‌روز است.");await load()}catch(e){setMessage(e instanceof Error?e.message:"بررسی بروزرسانی انجام نشد")}finally{setBusy(false)}}
  async function applyUpdate(){
    const target=updateCheck?.latest_version||lifecycle?.latest_version||"نسخه جدید";
    if(!confirm(`Store Agent به ${target} بروزرسانی شود؟ Manager عملیات را امن انجام می‌دهد و Agent چند ثانیه در دسترس نخواهد بود.`))return;
    setBusy(true);setMessage("بروزرسانی در حال آماده‌سازی است…");
    try{const r=await applyStoreEdgeUpdate();setMessage(`بروزرسانی ${r.version} شروع شد. صفحه وضعیت تا بازگشت Agent خودکار بررسی می‌شود.`);setUpdateCheck(null);setTimeout(()=>void load(),3500)}catch(e){setMessage(e instanceof Error?e.message:"بروزرسانی انجام نشد")}finally{setBusy(false)}
  }
  async function probeManager(){
    setBusy(true);setMessage("در حال بررسی Lifecycle Manager…");
    try{const lm=await getEdgeLifecycleStatus();setLifecycle(lm);setMessage(`Manager نسخه ${lm.manager_version} متصل است.`);await load()}
    catch{setMessage("Manager هنوز در دسترس نیست. اگر Installer باز است، نصب را کامل کن؛ این صفحه خودکار دوباره بررسی می‌کند.")}
    finally{setBusy(false)}
  }
  function beginInstallerDownload(target:EdgeInstallerTarget){
    setInstaller(target);setInstallWatching(true);
    setMessage(`${target.label}: Installer دانلود می‌شود. نصب سیستم‌عامل را تأیید کن و به همین صفحه برگرد؛ اتصال Manager خودکار تشخیص داده می‌شود.`);
  }

  const managerInstalled=!!lifecycle;
  const workerRunning=lifecycle?lifecycle.worker.state==="running":!!local;
  const alternatives=selectableInstallerTargets();
  const selectedInstaller=installer||installerTarget("unsupported","unknown");
  return <>
    <div className="page-head"><div><span className="eyebrow">تداوم فروش</span><h1>Store Edge و فروش آفلاین</h1><p>نصب اولیه با Installer سیستم‌عامل انجام می‌شود؛ بعد از آن Start، Stop، Restart و Update از همین پنل است و کاربر نیازی به Terminal یا Services ندارد.</p></div><a className="ghost-btn" target="_blank" rel="noreferrer" href={STORE_EDGE_URL}>باز کردن صندوق آفلاین</a></div>
    {message&&<div className="network-notice">{message}</div>}

    <section className="edge-manager-card panel">
      <div className="panel-head"><div><span className="panel-kicker">Lifecycle Manager</span><h2>کنترل Store Agent این کامپیوتر</h2><p>Manager سبک و محلی باقی می‌ماند تا حتی وقتی Agent متوقف است بتوانی دوباره آن را Start یا Update کنی.</p></div>
        <span className={managerInstalled?(workerRunning?"status-pill ok":"status-pill warn"):"status-pill"}>{managerInstalled?`Agent: ${workerLabel(lifecycle?.worker.state)}`:"Manager نصب نیست"}</span>
      </div>
      {managerInstalled?<>
        <div className="edge-manager-stats">
          <div><span>سیستم‌عامل</span><b>{osLabel(lifecycle.os)} / {lifecycle.arch}</b></div>
          <div><span>Manager</span><b>v{lifecycle.manager_version}</b></div>
          <div><span>Agent</span><b>{lifecycle.worker.version?`v${lifecycle.worker.version}`:workerLabel(lifecycle.worker.state)}</b></div>
          <div><span>حالت سرویس</span><b>{serviceLabel(lifecycle.service_mode)}</b></div>
        </div>
        {lifecycle.worker.last_exit_error&&<div className="edge-manager-error">آخرین خطای Agent: <code>{lifecycle.worker.last_exit_error}</code></div>}
        <div className="edge-manager-actions">
          <button className="primary-btn" disabled={busy||workerRunning||lifecycle.worker.state==="starting"} onClick={()=>void lifecycleAction("start")}>Start Agent</button>
          <button className="ghost-btn" disabled={busy||!workerRunning} onClick={()=>void lifecycleAction("restart")}>Restart</button>
          <button className="ghost-btn danger-text" disabled={busy||lifecycle.worker.state==="stopped"} onClick={()=>void lifecycleAction("stop")}>Stop Agent</button>
          <span className="edge-manager-divider"/>
          <button className="ghost-btn" disabled={busy||!lifecycle.update_enabled} onClick={()=>void checkUpdate()}>بررسی بروزرسانی</button>
          {(updateCheck?.update_available||lifecycle.update_available)&&<button className="primary-btn" disabled={busy} onClick={()=>void applyUpdate()}>نصب نسخه {updateCheck?.latest_version||lifecycle.latest_version}</button>}
        </div>
        {!lifecycle.update_enabled&&<small className="edge-update-hint">کانال بروزرسانی امضاشده روی این build تنظیم نشده است. Release رسمی Manifest URL و Ed25519 public key را داخل Manager دارد.</small>}
        {updateCheck?.release_notes&&<div className="edge-update-notes"><b>تغییرات {updateCheck.latest_version}</b><p>{updateCheck.release_notes}</p></div>}
      </>:<div className="edge-bootstrap-card">
        <div className="edge-bootstrap-copy"><span className="panel-kicker">One-click bootstrap</span><b>Store Agent هنوز روی این کامپیوتر شناسایی نشده است.</b><p>Installer مناسب سیستم‌عامل را دانلود و نصب کن. Windows Service یا Linux user service خودکار فعال می‌شود و این صفحه بدون Refresh دستی Manager را پیدا می‌کند.</p></div>
        <div className="edge-bootstrap-detected"><span>سیستم تشخیص‌داده‌شده</span><b>{selectedInstaller.label}</b><small>{selectedInstaller.note}</small></div>
        {selectedInstaller.supported&&selectedInstaller.url?<div className="edge-bootstrap-actions"><a className="primary-btn" href={selectedInstaller.url} onClick={()=>beginInstallerDownload(selectedInstaller)}>دانلود Installer</a><button className="ghost-btn" disabled={busy} onClick={()=>void probeManager()}>نصب شد؛ بررسی کن</button>{installWatching&&<span className="status-pill warn">در انتظار نصب…</span>}</div>:<div className="edge-bootstrap-warning">تشخیص خودکار برای این سیستم قطعی نیست. نسخه مناسب را دستی انتخاب کن.</div>}
        <div className="edge-installer-options">{alternatives.map(target=><button key={`${target.os}-${target.arch}`} className={selectedInstaller.os===target.os&&selectedInstaller.arch===target.arch?"edge-installer-option active":"edge-installer-option"} onClick={()=>setInstaller(target)}><b>{target.label}</b><span>{target.os==="windows"?"Setup.exe":"DEB"}</span></button>)}</div>
        <small className="edge-bootstrap-security">مرورگر اجازه نصب سرویس را به‌تنهایی ندارد؛ تأیید Installer توسط خود سیستم‌عامل فقط در نصب اول لازم است. بعد از نصب، Lifecycle و Update از همین صفحه انجام می‌شود.</small>
      </div>}
    </section>

    <section className="edge-overview-grid"><article className="panel edge-live-card"><div className="panel-head"><div><span className="panel-kicker">این دستگاه</span><h2>وضعیت عملیاتی Agent</h2></div><span className={local?.paired?local.last_sync_error?"status-pill warn":"status-pill ok":"status-pill"}>{local?.paired?(local.last_sync_error?"آفلاین / صف محلی":"متصل و آماده"):workerRunning?"در انتظار Pair / Sync":"Agent متوقف"}</span></div>{local?.paired?<div className="edge-live-stats"><div><span>دستگاه</span><b>{local.device_name||"Store Edge"}</b></div><div><span>کاتالوگ محلی</span><b>{local.catalog_items.toLocaleString("fa-IR")} کالا</b></div><div><span>در انتظار Sync</span><b>{local.pending_sales.toLocaleString("fa-IR")}</b></div><div><span>تعارض</span><b className={local.conflicts?"danger-text":""}>{local.conflicts.toLocaleString("fa-IR")}</b></div></div>:<div className="edge-install-help"><b>{managerInstalled&&!workerRunning?"Agent فعلاً متوقف است.":"Store Agent Pair نشده یا در دسترس نیست."}</b><p>{managerInstalled&&!workerRunning?"از کارت Lifecycle بالا Start Agent را بزن؛ نیازی به systemctl یا Services نیست.":"بعد از نصب و Start شدن Agent، این دستگاه را Pair کن."}</p></div>}<div className="edge-actions"><button className="primary-btn" disabled={busy||!workerRunning} onClick={()=>void sync()}>همگام‌سازی الان</button><a className="ghost-btn" href={STORE_EDGE_URL} target="_blank" rel="noreferrer">صندوق محلی</a></div></article>
      <article className="panel"><div className="panel-head"><div><span className="panel-kicker">اتصال امن</span><h2>Pair کردن کامپیوتر جدید</h2><p>کد یک‌بارمصرف ۱۰ دقیقه اعتبار دارد و secret دستگاه فقط یک‌بار به Agent تحویل می‌شود.</p></div></div>{pairing?<div className="pair-code-box"><span>کد اتصال</span><code>{pairing.pair_code}</code><small>انقضا: {new Date(pairing.expires_at).toLocaleString("fa-IR")}</small><p>این کد را در صفحه محلی Store Edge وارد کن.</p></div>:<button className="primary-btn" disabled={busy||!workerRunning} onClick={()=>void makePair()}>ساخت کد اتصال</button>}</article>
    </section>

    <section className="panel"><div className="panel-head"><div><span className="panel-kicker">دستگاه‌ها</span><h2>Store Edgeهای متصل</h2><p>دستگاه گم‌شده یا قدیمی را فوراً revoke کن.</p></div></div><div className="table-wrap"><table><thead><tr><th>نام دستگاه</th><th>وضعیت</th><th>آخرین ارتباط</th><th>تاریخ اتصال</th><th></th></tr></thead><tbody>{devices.length?devices.map(d=><tr key={d.id}><td><b>{d.name}</b></td><td>{d.active?<span className="status-pill ok">فعال</span>:<span className="status-pill">قطع‌شده</span>}</td><td>{d.last_seen_at?new Date(d.last_seen_at).toLocaleString("fa-IR"):"—"}</td><td>{new Date(d.created_at).toLocaleString("fa-IR")}</td><td>{d.active&&<button className="table-action danger-text" disabled={busy} onClick={()=>void revoke(d.id)}>قطع دسترسی</button>}</td></tr>):<tr><td colSpan={5}><div className="table-state">هنوز Store Edge متصل نشده است.</div></td></tr>}</tbody></table></div></section>
    <section className="edge-policy panel"><b>سیاست Lifecycle نسخه 15.8.1.1</b><p><strong>نصب اول</strong> فقط با تأیید Installer سیستم‌عامل انجام می‌شود. بعد از آن <strong>Stop Agent</strong> فقط Worker فروش/سخت‌افزار را متوقف می‌کند؛ Manager محلی روشن می‌ماند و Start/Restart/Update از همین صفحه ممکن است. بروزرسانی فقط با SHA-256 و امضای Ed25519 معتبر نصب می‌شود.</p></section>
  </>;
}
