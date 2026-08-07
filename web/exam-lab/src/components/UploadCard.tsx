"use client";

import {
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  FormControlLabel,
  ListItem,
  Tooltip,
  Typography,
} from "@mui/material";
import DeleteIcon from "@mui/icons-material/Delete";
import DownloadIcon from "@mui/icons-material/Download";
import { formatDistanceToNow } from "date-fns";
import { UserUploadSummary } from "@/api/types";
import { useTranslation } from "react-i18next";
import { dateFnsLocaleFor, localeTagFor } from "@/i18n";

type UploadCardProps = {
  upload: UserUploadSummary;
  onDelete: (upload: UserUploadSummary) => void;
  associated: boolean;
  associateBusy: boolean;
  onAssociateChange: (upload: UserUploadSummary, associate: boolean) => void;
};

// formatBytes renders a byte count in the largest 1024-based unit whose value
// stays >= 1, rounded to one decimal.
function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let value = bytes;
  let unit = "B";
  for (const u of units) {
    if (value < 1024) break;
    value /= 1024;
    unit = u;
  }
  return `${value.toFixed(1)} ${unit}`;
}

// collapsibleButtonSx makes an action button collapse to an icon-only square
// below the sm breakpoint: the label is hidden (see labelSx) and the start
// icon's spacing is neutralized so the icon stays centered.
const collapsibleButtonSx = {
  whiteSpace: "nowrap",
  minWidth: { xs: 0, sm: 64 },
  px: { xs: 1, sm: 2 },
  "& .MuiButton-startIcon": {
    ml: { xs: 0, sm: "-4px" },
    mr: { xs: 0, sm: "8px" },
  },
};

// labelSx hides a button's text label below the sm breakpoint, paired with
// collapsibleButtonSx. The text stays in the DOM at wider widths; the button
// carries an aria-label so it remains accessible when the text is hidden.
const labelSx = { display: { xs: "none", sm: "inline" } };

// One entry in the Uploads list: the file's name, MIME type, size, and when it
// was uploaded, with Download and Delete actions. Delete is reported via
// onDelete so the parent can ask for confirmation first; Download links
// straight to the file endpoint, which serves the bytes with
// Content-Disposition: attachment.
//
// The Associate checkbox reflects whether the upload is bound to its exam
// documents via /api/examassociations. It is fully controlled by the
// `associated` prop (fed from the fetched association list), so a failed
// toggle simply snaps back, and `associateBusy` disables it while the
// create/delete mutation for this upload is in flight. It is only rendered
// for .tar uploads, matching the server, which rejects associations for any
// other file type (see FsBasedAssociationManager.AddAssociation).
export default function UploadCard({
  upload,
  onDelete,
  associated,
  associateBusy,
  onAssociateChange,
}: UploadCardProps) {
  const { t, i18n } = useTranslation();
  const uploadedAt = new Date(upload.last_modified_at);
  return (
    <ListItem disableGutters sx={{ mb: 1 }}>
      <Card sx={{ width: "100%" }}>
        <CardContent>
          <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
            {/* minWidth: 0 lets the text column shrink so the filename clamp
                kicks in instead of pushing the buttons off-card. */}
            <Box sx={{ flexGrow: 1, minWidth: 0 }}>
              <Typography variant="h6" component="div" noWrap>
                {upload.filename}
              </Typography>
              <Box
                sx={{ display: "flex", flexWrap: "wrap", gap: 0.5, my: 0.5 }}
              >
                {upload.mime_type && (
                  <Chip label={upload.mime_type} size="small" />
                )}
                <Chip label={formatBytes(upload.size_bytes)} size="small" />
              </Box>
              <Typography variant="body2" color="textSecondary">
                {t("uploads.uploaded")}{" "}
                <Tooltip
                  title={uploadedAt.toLocaleString(localeTagFor(i18n.language))}
                >
                  <Box component="span">
                    {formatDistanceToNow(uploadedAt, {
                      addSuffix: true,
                      locale: dateFnsLocaleFor(i18n.language),
                    })}
                  </Box>
                </Tooltip>
              </Typography>
              {/* The digest is 64 hex chars; let CSS clamp it with an
                  ellipsis inside the minWidth: 0 column and expose the full
                  value via the tooltip. */}
              <Tooltip title={`SHA-256: ${upload.sha256}`}>
                <Typography
                  variant="body2"
                  color="textSecondary"
                  sx={{
                    fontFamily: "monospace",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  sha256:{upload.sha256}
                </Typography>
              </Tooltip>
              {upload.filename.endsWith(".tar") && (
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={associated}
                      disabled={associateBusy}
                      onChange={(e) =>
                        onAssociateChange(upload, e.target.checked)
                      }
                    />
                  }
                  label={t("uploads.associate")}
                />
              )}
            </Box>
            <Button
              variant="contained"
              sx={collapsibleButtonSx}
              aria-label={t("uploads.download")}
              href={`/api/useruploads/${encodeURIComponent(upload.upload_id)}`}
              startIcon={<DownloadIcon />}
            >
              <Box component="span" sx={labelSx}>
                {t("uploads.download")}
              </Box>
            </Button>
            <Button
              variant="contained"
              color="error"
              sx={collapsibleButtonSx}
              aria-label={t("common.delete")}
              startIcon={<DeleteIcon />}
              onClick={() => onDelete(upload)}
            >
              <Box component="span" sx={labelSx}>
                {t("common.delete")}
              </Box>
            </Button>
          </Box>
        </CardContent>
      </Card>
    </ListItem>
  );
}
