'use client';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState } from 'react';

// Providers wires up app-wide client-side context providers. It must be a
// Client Component because QueryClientProvider relies on React context.
export function Providers({ children }: { children: React.ReactNode }) {
  // Create the QueryClient once per browser session via a lazy initializer so
  // it is stable across re-renders and not recreated (which would lose cache).
  const [queryClient] = useState(() => new QueryClient());
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}
