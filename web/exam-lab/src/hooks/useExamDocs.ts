import { useQuery } from "@tanstack/react-query";
import { ExamDocs, ExamDocsLine } from "@/api/types";

// /api/examdocs streams NDJSON: one JSON object per line, either
// {"Data":{...excerpt...}} or {"Err":"..."}. The body is buffered (excerpts are
// small) and successful Data items are collected; Err lines are skipped so that
// a single failing source doesn't blank out the whole list.
async function fetchExamDocs(): Promise<ExamDocs> {
  const res = await fetch("/api/examdocs");
  if (!res.ok) throw new Error(`failed to fetch exams: ${res.status}`);
  const text = await res.text();
  const exams: ExamDocs = [];
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed) continue;
    const parsed = JSON.parse(trimmed) as ExamDocsLine;
    if (parsed.Data) {
      exams.push(parsed.Data);
    }
  }
  return exams;
}

// useExamDocs fetches and caches the list of exam documents under the
// "examdocs" query key. `generation` is appended to the key, so bumping it
// mounts a fresh query and refetches the list — the mechanism by which the
// parent page lets one section refresh another (prefix-based invalidations
// such as invalidateQueries({queryKey: ["examdocs"]}) still match). `data` is
// always a defined array (empty while the first request is pending), and
// `isPending` is true during the initial fetch so callers can show a loading
// placeholder.
export function useExamDocs(generation: number): {
  data: ExamDocs;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["examdocs", generation],
    queryFn: fetchExamDocs,
  });
  return { data, isPending };
}
