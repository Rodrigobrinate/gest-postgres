"use client";

import { useMutation } from "@tanstack/react-query";
import { LogOut } from "lucide-react";
import { Button } from "@/components/ui/button";
import { api } from "@/lib/api";

export function LogoutButton() {
  const logout = useMutation({
    mutationFn: () => api.logout(),
    onSuccess: () => {
      window.location.href = "/login";
    },
  });

  return (
    <Button variant="outline" size="icon" onClick={() => logout.mutate()} title="Sair">
      <LogOut className="size-4" />
    </Button>
  );
}
