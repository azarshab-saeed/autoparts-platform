"use client";
import {useEffect,useState} from "react";
import {getStoreEdgeStatus,type StoreEdgeStatus} from "@/lib/edge";

export default function EdgeStatus(){
  const [s,setS]=useState<StoreEdgeStatus|null>(null);
  useEffect(()=>{
    let live=true;
    const poll=()=>void getStoreEdgeStatus().then(v=>live&&setS(v)).catch(()=>live&&setS(null));
    poll();const id=setInterval(poll,10000);return()=>{live=false;clearInterval(id)};
  },[]);
  if(!s?.paired)return null;
  const problem=s.conflicts>0||Boolean(s.last_sync_error);
  return <a href="/store/edge" className={problem?"edge-chip warning":"edge-chip"} title={s.last_sync_error||"Store Edge آماده است"}>
    <i/><span>{problem?`Edge: ${s.pending_sales} در صف${s.conflicts?` / ${s.conflicts} تعارض`:""}`:"Edge آماده"}</span>
  </a>;
}
