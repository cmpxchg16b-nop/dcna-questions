"use client";

import NextLink from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { Breadcrumbs, Link as MuiLink, Typography } from "@mui/material";

// A crumb with href renders as a link; without one it is plain text — used
// both for the current page (the last crumb) and for ancestors that have no
// dedicated page to link to yet.
type Crumb = {
  label: string;
  href?: string;
};

// crumbsFor maps the app's known routes onto their breadcrumb trails.
function crumbsFor(pathname: string, examSessionId: string | null): Crumb[] {
  if (pathname === "/") {
    return [{ label: "Home" }];
  }
  if (pathname === "/examsession") {
    return [
      { label: "Home", href: "/" },
      // The ongoing-sessions list lives on Home; there is no dedicated exam
      // sessions page, so this level is intentionally not clickable.
      { label: "Exam Sessions" },
      { label: examSessionId ?? "…" },
    ];
  }
  // Unknown route: fall back to Home plus the raw path.
  return [{ label: "Home", href: "/" }, { label: pathname }];
}

// BreadcrumbNav shows where the current page sits in the hierarchy (e.g.
// "Home > Exam Sessions > <exam_session_id>") so the user can jump back up
// with one click. The trail is derived from the URL, so pages need no wiring.
export default function BreadcrumbNav() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const crumbs = crumbsFor(pathname, searchParams.get("exam_session_id"));

  // A lone segment (e.g. just "Home" on the home page) has no hierarchy to
  // navigate, so the whole bar is hidden.
  if (crumbs.length < 2) return null;

  return (
    <Breadcrumbs aria-label="breadcrumb" sx={{ mb: 2 }}>
      {crumbs.map((crumb, i) => {
        const isLast = i === crumbs.length - 1;
        return !isLast && crumb.href ? (
          <MuiLink
            key={crumb.label}
            component={NextLink}
            href={crumb.href}
            underline="hover"
          >
            {crumb.label}
          </MuiLink>
        ) : (
          // overflowWrap keeps a long exam session id from overflowing
          // narrow viewports.
          <Typography
            key={crumb.label}
            color={isLast ? "text.primary" : "text.secondary"}
            sx={{ overflowWrap: "anywhere" }}
          >
            {crumb.label}
          </Typography>
        );
      })}
    </Breadcrumbs>
  );
}
