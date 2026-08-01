"use client";

import { useSearchParams } from "next/navigation";

export default function Page() {
  const searchParams = useSearchParams();

  const exam_session_id = searchParams.get("exam_session_id");

  return <p>exam_session_id: {exam_session_id}</p>;
}
