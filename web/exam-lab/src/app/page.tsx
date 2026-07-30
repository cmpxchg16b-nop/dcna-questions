'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Box, Button } from '@mui/material';

// The counter value lives on the server (see Go package counter). These helpers
// talk to the /api/counter endpoint; relative URLs resolve against the Go server
// in production (it serves the static export), and against the dev proxy in dev.

async function fetchCounter(): Promise<number> {
  const res = await fetch('/api/counter');
  if (!res.ok) throw new Error(`failed to fetch counter: ${res.status}`);
  const body = await res.json();
  return body.data as number;
}

async function incrementCounter(): Promise<number> {
  const res = await fetch('/api/counter', { method: 'POST' });
  if (!res.ok) throw new Error(`failed to increment counter: ${res.status}`);
  const body = await res.json();
  return body.data as number;
}

export default function Home() {
  const queryClient = useQueryClient();

  const { data: counter = 0, isPending } = useQuery({
    queryKey: ['counter'],
    queryFn: fetchCounter,
  });

  // Optimistically reflect the server-returned value: POST returns the new
  // counter, so we set it directly on the cache instead of refetching.
  const { mutate: addOne, isPending: isIncrementing } = useMutation({
    mutationFn: incrementCounter,
    onSuccess: (value) => {
      queryClient.setQueryData(['counter'], value);
    },
  });

  return (
    <Box>
      <Box>{isPending ? '…' : counter}</Box>
      <Button
        onClick={() => addOne()}
        disabled={isPending || isIncrementing}
      >
        Add One
      </Button>
    </Box>
  );
}
