const ones=["","یک","دو","سه","چهار","پنج","شش","هفت","هشت","نه","ده","یازده","دوازده","سیزده","چهارده","پانزده","شانزده","هفده","هجده","نوزده"];
const tens=["","","بیست","سی","چهل","پنجاه","شصت","هفتاد","هشتاد","نود"];
const hundreds=["","صد","دویست","سیصد","چهارصد","پانصد","ششصد","هفتصد","هشتصد","نهصد"];
const scales=["","هزار","میلیون","میلیارد","تریلیون","کوادریلیون"];
function under1000(n:number){const out:string[]=[];const h=Math.floor(n/100);let r=n%100;if(h)out.push(hundreds[h]);if(r){if(r<20)out.push(ones[r]);else{const t=Math.floor(r/10);const o=r%10;out.push(tens[t]);if(o)out.push(ones[o]);}}return out.join(" و ");}
export function amountToPersianWords(value:number){let n=Math.round(Math.abs(value));if(!Number.isFinite(n))return "";if(n===0)return "صفر تومان";const chunks:string[]=[];let i=0;while(n>0&&i<scales.length){const c=n%1000;if(c){const text=under1000(c)+(scales[i]?` ${scales[i]}`:"");chunks.unshift(text);}n=Math.floor(n/1000);i++;}return `${value<0?"منفی ":""}${chunks.join(" و ")} تومان`;}
const L=["0001101","0011001","0010011","0111101","0100011","0110001","0101111","0111011","0110111","0001011"];
const G=["0100111","0110011","0011011","0100001","0011101","0111001","0000101","0010001","0001001","0010111"];
const R=["1110010","1100110","1101100","1000010","1011100","1001110","1010000","1000100","1001000","1110100"];
const PAR=["LLLLLL","LLGLGG","LLGGLG","LLGGGL","LGLLGG","LGGLLG","LGGGLL","LGLGLG","LGLGGL","LGGLGL"];
export function ean13Bits(raw:string){const d=raw.replace(/\D/g,"");if(d.length!==13)return "";let bits="101";const first=Number(d[0]);for(let i=1;i<=6;i++){const digit=Number(d[i]);bits+=PAR[first][i-1]==="L"?L[digit]:G[digit];}bits+="01010";for(let i=7;i<13;i++)bits+=R[Number(d[i])];return bits+"101";}
export function barcodeBars(raw:string){const bits=ean13Bits(raw);if(bits)return bits;const src=raw.trim();if(!src)return "";let out="101";for(const ch of src){const n=ch.charCodeAt(0);for(let i=0;i<7;i++)out+=((n>>i)&1)?"1":"0";out+="0";}return out+"101";}
