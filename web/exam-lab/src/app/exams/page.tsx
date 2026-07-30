"use client";

import { useSearchParams } from "next/navigation";

export default function Page() {
  const searchParams = useSearchParams();

  const examid = searchParams.get("examid");

  return <p>examid: {examid}</p>;
}
