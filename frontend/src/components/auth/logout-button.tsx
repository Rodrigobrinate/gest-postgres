"use client";

import { useMutation } from "@tanstack/react-query";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";
import { clearAllQueryHistory } from "@/lib/use-query-history";

export function LogoutButton() {
  const logout = useMutation({
    mutationFn: () => api.logout(),
    onSuccess: () => {
      clearAllQueryHistory();
      window.location.href = "/login";
    },
  });

  return (
    <Button variant="outline" size="icon" onClick={() => logout.mutate()} title="Sair">
      <LogOut className="size-4" />
    </Button>
  );
}
