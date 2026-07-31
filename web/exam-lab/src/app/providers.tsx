"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@mui/material/styles";
import CssBaseline from "@mui/material/CssBaseline";
import { useState } from "react";
import theme from "@/theme";

// Providers wires up app-wide client-side context providers. It must be a
// Client Component because QueryClientProvider relies on React context.
export function Providers({ children }: { children: React.ReactNode }) {
  // Create the QueryClient once per browser session via a lazy initializer so
  // it is stable across re-renders and not recreated (which would lose cache).
  const [queryClient] = useState(() => new QueryClient());
  return (
    <ThemeProvider theme={theme}>
      {/* CssBaseline resets browser defaults and paints <body> with the
          theme's background.default/text.primary for the active scheme;
          enableColorScheme lets native UI (scrollbars, form controls) follow
          the same scheme. */}
      <CssBaseline enableColorScheme />
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ThemeProvider>
  );
}
