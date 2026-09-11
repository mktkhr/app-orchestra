import CssBaseline from "@mui/material/CssBaseline";
import { ThemeProvider } from "@mui/material/styles";
import type { JSX } from "react";

import { theme } from "./theme";
import { Shell } from "./ui/Shell";

/**
 * The application: the theme, and the shell that lives under it.
 *
 * The previous DashboardLayout-based shell was replaced with a hand-built
 * MUI shell: that dependency's peer range topped out at @mui/material ^7,
 * this repository is on ^9, and no MUI 8/9 compatible release of it exists.
 */
export function App(): JSX.Element {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Shell />
    </ThemeProvider>
  );
}
