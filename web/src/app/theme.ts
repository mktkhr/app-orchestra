import { createTheme } from "@mui/material/styles";

/**
 * MUI's default touch targets (IconButton "medium" is 40px) sit below the
 * harness's layout guard, which enforces the WCAG/HIG floor of 44px
 * (harness/quality/browser/layout.spec.ts). Raised here, once, for every
 * button and icon button in the app, including the shell's own bar controls.
 */
export const theme = createTheme({
  colorSchemes: { light: true, dark: true },
  components: {
    MuiButtonBase: {
      styleOverrides: {
        root: {
          minHeight: 44,
        },
      },
    },
    MuiIconButton: {
      styleOverrides: {
        root: {
          minHeight: 44,
          minWidth: 44,
        },
      },
    },
  },
});
