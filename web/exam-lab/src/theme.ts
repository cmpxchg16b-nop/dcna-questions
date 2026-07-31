'use client';

import { createTheme } from '@mui/material/styles';

// Built-in MUI color-scheme support: declaring both schemes makes the theme
// emit CSS variables for light and dark, and the default colorSchemeSelector
// ('media') applies the dark variables under @media (prefers-color-scheme:
// dark). The whole app therefore follows the OS theme with no JS state and no
// hydration mismatch.
const theme = createTheme({
  colorSchemes: {
    light: true,
    dark: true,
  },
});

export default theme;
