"use client";

import { useState } from "react";
import type { ReactNode } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Autocomplete,
  Badge,
  Box,
  Collapse,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import FilterListIcon from "@mui/icons-material/FilterList";
import { useTranslation } from "react-i18next";

// SEPARATORS trigger tokenization: the draft up to the last separator becomes
// chips, the remainder keeps being edited. "&" is the query-string separator,
// "," and ";" are conveniences.
const SEPARATORS = /[,;&]/;

// normalizePair canonicalizes one chip: trims around key and value and drops
// a trailing "=" with an empty value, so near-identical typings serialize
// (and dedupe) identically.
const normalizePair = (pair: string): string => {
  const eq = pair.indexOf("=");
  if (eq < 0) return pair.trim();
  const key = pair.slice(0, eq).trim();
  const value = pair.slice(eq + 1).trim();
  return value === "" ? key : `${key}=${value}`;
};

// pairsToQuery serializes chips into the page URL's query string, one
// repeated parameter per chip (OR within a key, AND across keys).
const pairsToQuery = (pairs: string[]): string => {
  const params = new URLSearchParams();
  for (const pair of pairs) {
    const eq = pair.indexOf("=");
    if (eq < 0) {
      params.append(pair, "");
    } else {
      params.append(pair.slice(0, eq), pair.slice(eq + 1));
    }
  }
  return params.toString();
};

// queryToPairs expands the page URL's query string into chips, one decoded
// "key=value" per parameter.
const queryToPairs = (query: string): string[] => {
  const pairs: string[] = [];
  for (const [key, value] of new URLSearchParams(query)) {
    pairs.push(value === "" ? key : `${key}=${value}`);
  }
  return pairs;
};

type ExamLabelFilterProps = {
  // The section's one-line description, rendered to the toggle's left so the
  // closed state is a single slim row.
  children: ReactNode;
};

// ExamLabelFilter is the collapsed filter affordance of the Exams section: a
// small icon toggle (badged while a filter is active) that unfolds a tags
// input on demand, so it never dominates the section when unused.
//
// The tags input holds one "key=value" chip per query parameter; typing a
// separator (",", ";", "&"), pressing Enter, or leaving the field converts
// the draft into chips. The page URL's query string is the single source of
// truth — page.tsx forwards it to /api/examdocs/bylabel — so every chip
// commit is a router.replace (no history entries, no debounce: chip edits are
// already discrete). External URL changes (back/forward navigation, a shared
// link) resync the chips by adjusting state during render, so the user's own
// in-flight draft is never clobbered by the replace it triggered.
export default function ExamLabelFilter({ children }: ExamLabelFilterProps) {
  const { t } = useTranslation();
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlQuery = searchParams.toString();
  const active = urlQuery !== "";
  const [pairs, setPairs] = useState<string[]>(() => queryToPairs(urlQuery));
  const [draft, setDraft] = useState("");
  // The panel starts open when the page was loaded with a filter in the URL,
  // so the user sees why the listing is narrowed.
  const [open, setOpen] = useState(active);

  // Resync on external navigation (back/forward, a shared link) by adjusting
  // state during render: adopt the URL's chips only when the current chips
  // don't already serialize to it.
  const [prevUrlQuery, setPrevUrlQuery] = useState(urlQuery);
  if (prevUrlQuery !== urlQuery) {
    setPrevUrlQuery(urlQuery);
    if (pairsToQuery(pairs) !== urlQuery) {
      setPairs(queryToPairs(urlQuery));
      setDraft("");
    }
  }

  // commit replaces the URL's query with the serialization of nextPairs.
  const commit = (nextPairs: string[]) => {
    const normalized = [...new Set(nextPairs.map(normalizePair))].filter(
      (p) => p !== "",
    );
    setPairs(normalized);
    const query = pairsToQuery(normalized);
    if (query !== urlQuery) {
      router.replace(query ? `/?${query}` : "/", { scroll: false });
    }
  };

  return (
    <>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
        <Box sx={{ flexGrow: 1, minWidth: 0 }}>{children}</Box>
        <Tooltip title={t("exams.filter.label")}>
          <Badge variant="dot" color="primary" invisible={!active}>
            <IconButton
              aria-label={t("exams.filter.label")}
              color={open ? "primary" : "default"}
              size="small"
              onClick={() => setOpen((o) => !o)}
            >
              <FilterListIcon fontSize="small" />
            </IconButton>
          </Badge>
        </Tooltip>
      </Box>
      <Collapse in={open}>
        <Box sx={{ mb: 2 }}>
          <Autocomplete
            multiple
            freeSolo
            // A pure tokenizer: no suggestion list, no popup, no dropdown
            // arrow. autoSelect converts the draft into a chip on blur;
            // Enter does so natively.
            options={[]}
            open={false}
            forcePopupIcon={false}
            autoSelect
            size="small"
            value={pairs}
            inputValue={draft}
            onInputChange={(_e, raw, reason) => {
              // "reset" follows an Enter-committed chip; anything else
              // without a separator is plain typing.
              if (reason === "reset") {
                setDraft("");
                return;
              }
              if (!SEPARATORS.test(raw)) {
                setDraft(raw);
                return;
              }
              // Everything before the last separator is complete pairs; the
              // remainder keeps being edited.
              const segments = raw.split(/[,;&]+/);
              const complete = segments
                .slice(0, -1)
                .map(normalizePair)
                .filter((p) => p !== "");
              if (complete.length > 0) {
                commit([...pairs, ...complete]);
              }
              setDraft(segments[segments.length - 1]);
            }}
            onChange={(_e, newPairs) => commit(newPairs)}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={
                  pairs.length === 0 ? "label1=a&label1=b&label2=c" : ""
                }
                slotProps={{
                  ...params.slotProps,
                  htmlInput: {
                    ...params.slotProps.htmlInput,
                    "aria-label": t("exams.filter.label"),
                  },
                }}
              />
            )}
          />
          <Typography variant="caption" color="text.secondary">
            {t("exams.filter.helper")}
          </Typography>
        </Box>
      </Collapse>
    </>
  );
}
