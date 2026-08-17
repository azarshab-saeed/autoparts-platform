export function downloadCSV(filename:string,rows:(string|number)[][]){
  const escape=(value:string|number)=>{const s=String(value??"");return /[",\n]/.test(s)?`"${s.replace(/"/g,'""')}"`:s;};
  const text="\uFEFF"+rows.map(row=>row.map(escape).join(",")).join("\r\n");
  const blob=new Blob([text],{type:"text/csv;charset=utf-8"});
  const url=URL.createObjectURL(blob);const a=document.createElement("a");a.href=url;a.download=filename;document.body.appendChild(a);a.click();a.remove();URL.revokeObjectURL(url);
}
