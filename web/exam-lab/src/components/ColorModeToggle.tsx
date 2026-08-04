"use client";

import { Box, IconButton, Tooltip } from "@mui/material";
import { useColorScheme } from "@mui/material/styles";
import ContrastIcon from "@mui/icons-material/Contrast";
import LightModeIcon from "@mui/icons-material/LightMode";
import DarkModeIcon from "@mui/icons-material/DarkMode";

type Mode = "system" | "light" | "dark";

// Clicking the button advances to the next entry in this cycle.
const NEXT_MODE: Record<Mode, Mode> = {
  system: "light",
  light: "dark",
  dark: "system",
};

const MODE_ICON: Record<Mode, React.ReactNode> = {
  system: <ContrastIcon />,
  light: <LightModeIcon />,
  dark: <DarkModeIcon />,
};

const MODE_LABEL: Record<Mode, string> = {
  system: "System",
  light: "Light",
  dark: "Dark",
};

// Floating corner button that cycles the lightness preference through
// system → light → dark. The choice is persisted by MUI's color-scheme
// manager (localStorage key "mui-mode") and applied to <html> as
// data-mui-color-scheme (see src/theme.ts and app/layout.tsx).
export default function ColorModeToggle() {
  const { mode, setMode } = useColorScheme();

  // mode is undefined during SSR and the hydration render, so the icon is
  // swapped in only after mount; the placeholder keeps the same box size to
  // avoid a mismatch and any layout shift.
  return (
    <Tooltip title={mode ? `Theme: ${MODE_LABEL[mode]}` : ""}>
      <IconButton
        aria-label={
          mode
            ? `Switch color theme, current: ${MODE_LABEL[mode]}`
            : "Switch color theme"
        }
        onClick={() => mode && setMode(NEXT_MODE[mode])}
        sx={{
          position: "fixed",
          top: 8,
          right: 8,
          // One step above theme.zIndex.modal so the toggle stays clickable
          // over modal dialogs and their backdrops (e.g. the login page's
          // always-open Dialog).
          zIndex: (theme) => theme.zIndex.modal + 1,
        }}
      >
        {mode ? MODE_ICON[mode] : <Box sx={{ width: 24, height: 24 }} />}
      </IconButton>
    </Tooltip>
  );
}
