export type EdgeInstallerOS = "windows" | "linux" | "unsupported";
export type EdgeInstallerArch = "amd64" | "arm64" | "unknown";

export type EdgeInstallerTarget = {
  os: EdgeInstallerOS;
  arch: EdgeInstallerArch;
  label: string;
  filename: string;
  url: string;
  supported: boolean;
  note: string;
};

const RELEASE_BASE = "https://github.com/azarshab-saeed/autoparts-platform/releases/latest/download";
const LEGACY_INSTALLER_URL = process.env.NEXT_PUBLIC_EDGE_INSTALLER_URL || "";
const WINDOWS_AMD64_URL = process.env.NEXT_PUBLIC_EDGE_INSTALLER_WINDOWS_AMD64_URL || `${RELEASE_BASE}/AutoParts-Store-Agent-Setup-windows-x64.exe`;
const LINUX_AMD64_URL = process.env.NEXT_PUBLIC_EDGE_INSTALLER_LINUX_AMD64_URL || `${RELEASE_BASE}/autoparts-store-agent-linux-amd64.deb`;
const LINUX_ARM64_URL = process.env.NEXT_PUBLIC_EDGE_INSTALLER_LINUX_ARM64_URL || `${RELEASE_BASE}/autoparts-store-agent-linux-arm64.deb`;

export function installerTarget(os: EdgeInstallerOS, arch: EdgeInstallerArch): EdgeInstallerTarget {
  if (os === "windows") {
    const supported = arch === "amd64" || arch === "unknown";
    return {
      os,
      arch,
      label: "Windows x64",
      filename: "AutoParts-Store-Agent-Setup-windows-x64.exe",
      url: LEGACY_INSTALLER_URL || WINDOWS_AMD64_URL,
      supported,
      note: supported
        ? "فایل Setup را اجرا کن و اجازه Administrator را تأیید کن؛ Manager و Agent خودکار نصب و Start می‌شوند."
        : "در حال حاضر Installer رسمی Windows برای x64 منتشر می‌شود.",
    };
  }
  if (os === "linux") {
    if (arch === "arm64") {
      return {
        os,
        arch,
        label: "Linux ARM64 (Debian/Ubuntu)",
        filename: "autoparts-store-agent-linux-arm64.deb",
        url: LEGACY_INSTALLER_URL || LINUX_ARM64_URL,
        supported: true,
        note: "فایل DEB را با Software Center باز و Install را تأیید کن؛ سرویس کاربر فعال به‌صورت خودکار Start می‌شود.",
      };
    }
    const supported = arch === "amd64" || arch === "unknown";
    return {
      os,
      arch,
      label: "Linux x64 (Debian/Ubuntu)",
      filename: "autoparts-store-agent-linux-amd64.deb",
      url: LEGACY_INSTALLER_URL || LINUX_AMD64_URL,
      supported,
      note: supported
        ? "فایل DEB را با Software Center باز و Install را تأیید کن؛ سرویس کاربر فعال به‌صورت خودکار Start می‌شود."
        : "این معماری Linux هنوز Installer رسمی ندارد.",
    };
  }
  return {
    os: "unsupported",
    arch,
    label: "سیستم‌عامل پشتیبانی‌نشده",
    filename: "",
    url: "",
    supported: false,
    note: "Store Agent فعلاً برای Windows x64 و Linux Debian/Ubuntu روی amd64/arm64 ارائه می‌شود.",
  };
}

export function detectEdgeInstallerTarget(): EdgeInstallerTarget {
  if (typeof navigator === "undefined") return installerTarget("unsupported", "unknown");
  const nav = navigator as Navigator & { userAgentData?: { platform?: string } };
  const ua = `${navigator.userAgent || ""} ${navigator.platform || ""} ${nav.userAgentData?.platform || ""}`;
  const lower = ua.toLowerCase();
  let os: EdgeInstallerOS = "unsupported";
  if (lower.includes("windows") || lower.includes("win32") || lower.includes("win64")) os = "windows";
  else if (!lower.includes("android") && lower.includes("linux")) os = "linux";

  let arch: EdgeInstallerArch = "unknown";
  if (/aarch64|arm64/.test(lower)) arch = "arm64";
  else if (/x86_64|x86-64|amd64|win64|x64/.test(lower)) arch = "amd64";

  return installerTarget(os, arch);
}

export function selectableInstallerTargets(): EdgeInstallerTarget[] {
  return [installerTarget("windows", "amd64"), installerTarget("linux", "amd64"), installerTarget("linux", "arm64")];
}
