import type { UserSession } from "./types";
import type { CreateVehicleNotebookInput, VehicleNotebookVehicle } from "./vehicle-notebook";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
async function request<T>(session:UserSession,path:string,init:RequestInit={}):Promise<T>{const h=new Headers(init.headers);h.set("Authorization",`Bearer ${session.token}`);if(init.body)h.set("Content-Type","application/json");const r=await fetch(`${API_URL}${path}`,{...init,headers:h});const b=await r.json().catch(()=>({}));if(!r.ok)throw new Error(b?.error?.message||`HTTP ${r.status}`);return b as T;}
export async function listMechanicVehicleNotebooks(session:UserSession,q=""){const x=await request<{items:VehicleNotebookVehicle[]}>(session,`/v1/mechanic/vehicles?q=${encodeURIComponent(q)}`);return x.items;}
export async function createMechanicVehicleNotebook(session:UserSession,input:CreateVehicleNotebookInput){return request<VehicleNotebookVehicle>(session,"/v1/mechanic/vehicles",{method:"POST",body:JSON.stringify(input)});}
export async function rotateMechanicOwnerCode(session:UserSession,id:string){const x=await request<{owner_code:string}>(session,`/v1/mechanic/vehicles/${id}/owner-code`,{method:"POST"});return x.owner_code;}
