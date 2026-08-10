"use client";

import { Typography } from "@mui/material";
import { useTranslation } from "react-i18next";

// Deployments on these hostnames carry a sponsor acknowledgement at the
// bottom of the home page, naming this sponsor.
const SPONSORS: Record<string, string> = {
  "exam.edu.dn42": "nedifinita (AS4242420454)",
  "testcenter.edu.dn42": "nedifinita (AS4242420454)",
};

// Renders a centered "… is sponsored by …" line for sponsored deployments,
// nothing anywhere else. Reading window during render is safe: I18nProvider
// gates the whole tree behind client-side mounting, so this component never
// renders on the server or during hydration.
export default function SponsorAcknowledgement() {
  const { t } = useTranslation();
  const hostname = window.location.hostname;
  const sponsor = SPONSORS[hostname];

  if (!sponsor) return null;

  return (
    <Typography
      component="footer"
      variant="body2"
      color="text.secondary"
      align="center"
      sx={{ mt: 4, mb: 2 }}
    >
      {t("sponsor.acknowledgement", { hostname, sponsor })}
    </Typography>
  );
}
