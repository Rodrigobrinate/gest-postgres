"use client";

import { createContext, useContext } from "react";
import type { UserRole } from "@/lib/api";

export interface CurrentUser {
  username: string;
  role: UserRole;
}

const CurrentUserContext = createContext<CurrentUser | null>(null);

export const CurrentUserProvider = CurrentUserContext.Provider;

/** null enquanto carrega ou se a checagem de sessão ainda não resolveu. */
export function useCurrentUser() {
  return useContext(CurrentUserContext);
}

export function useIsAdmin() {
  return useCurrentUser()?.role === "admin";
}
