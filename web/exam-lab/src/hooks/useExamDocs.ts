import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ExamDocs, ExamDocsLine, LabelFilter } from "@/api/types";

// /api/examdocs streams NDJSON: one JSON object per line, either
// {"Data":{...excerpt...}} or {"Err":"..."}. The body is buffered (excerpts are
// small) and successful Data items are collected; Err lines are skipped so that
// a single failing source doesn't blank out the whole list.
async function fetchExamDocs(labelFilter?: LabelFilter): Promise<ExamDocs> {
  const res = await fetch(examDocsURL(labelFilter));
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

// examDocsURL builds the listing URL for a label filter. A non-empty filter
// goes to /api/examdocs/bylabel with each key repeated per accepted value
// (OR within a key, AND across keys); an empty or absent filter uses the
// unfiltered /api/examdocs endpoint.
function examDocsURL(labelFilter?: LabelFilter): string {
  const keys = Object.keys(labelFilter ?? {});
  if (keys.length === 0) {
    return "/api/examdocs";
  }
  const params = new URLSearchParams();
  for (const key of keys) {
    for (const value of labelFilter![key]) {
      params.append(key, value);
    }
  }
  return `/api/examdocs/bylabel?${params}`;
}

// useExamDocs fetches and caches the list of exam documents under the
// "examdocs" query key. `generation` is appended to the key, so bumping it
// mounts a fresh query and refetches the list — the mechanism by which the
// parent page lets one section refresh another (prefix-based invalidations
// such as invalidateQueries({queryKey: ["examdocs"]}) still match). The
// optional label filter narrows the listing server-side via
// /api/examdocs/bylabel and is part of the query key, so different filters
// cache independently. `data` is always a defined array (empty while the
// first request is pending). `isPending` is true only for the very first
// fetch: on generation bumps keepPreviousData serves the previous list while
// the new query fetches, so callers keep rendering the list instead of
// collapsing to the loading placeholder (which would shrink the page and yank
// the scroll position).
export function useExamDocs(
  generation: number,
  labelFilter?: LabelFilter,
): {
  data: ExamDocs;
  isPending: boolean;
} {
  const { data = [], isPending } = useQuery({
    queryKey: ["examdocs", generation, labelFilter ?? null],
    queryFn: () => fetchExamDocs(labelFilter),
    placeholderData: keepPreviousData,
  });
  return { data, isPending };
}
