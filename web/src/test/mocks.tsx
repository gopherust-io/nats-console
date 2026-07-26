import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { type ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { ThemeProvider } from "../lib/theme";

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  });
}

export function TestProviders({
  children,
  initialEntries = ["/systems"],
}: {
  children: ReactNode;
  initialEntries?: string[];
}) {
  const client = createTestQueryClient();
  return (
    <QueryClientProvider client={client}>
      <ThemeProvider>
        <MemoryRouter initialEntries={initialEntries}>{children}</MemoryRouter>
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export function ShellRoutes({ children }: { children: ReactNode }) {
  return (
    <Routes>
      <Route path="/*" element={children} />
    </Routes>
  );
}
