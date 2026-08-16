"use client";

import Keycloak from "keycloak-js";

let instance: Keycloak | null = null;
let initPromise: Promise<boolean> | null = null;

export function getKeycloak(): Keycloak {
  if (typeof window === "undefined") {
    throw new Error("Keycloak is only available in the browser");
  }
  if (!instance) {
    instance = new Keycloak({
      url: process.env.NEXT_PUBLIC_KEYCLOAK_URL || "http://localhost:8081",
      realm: process.env.NEXT_PUBLIC_KEYCLOAK_REALM || "autoparts",
      clientId: process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID || "autoparts-web",
    });
  }
  return instance;
}

export function initKeycloak(): Promise<boolean> {
  if (!initPromise) {
    initPromise = getKeycloak().init({
      onLoad: "check-sso",
      pkceMethod: "S256",
      checkLoginIframe: false,
    });
  }
  return initPromise!;
}
