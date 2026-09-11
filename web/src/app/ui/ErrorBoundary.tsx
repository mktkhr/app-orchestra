import Alert from "@mui/material/Alert";
import AlertTitle from "@mui/material/AlertTitle";
import Box from "@mui/material/Box";
import { Component, type ErrorInfo, type JSX, type ReactNode } from "react";

interface ErrorBoundaryProps {
  readonly children: ReactNode;
}

interface ErrorBoundaryState {
  readonly message: string | null;
}

/**
 * Catches a throw from anywhere below it and shows what happened.
 *
 * Without one, React unmounts the whole tree when a render throws and the
 * page goes blank - no message, no hint of which component failed, nothing
 * to report. That is how a `crypto.randomUUID is not a function` over plain
 * HTTP looked from the outside: a white screen.
 *
 * A boundary cannot recover the state that was lost, so it does not try to;
 * it names the failure and leaves reloading to the person.
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  public constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { message: null };
  }

  public static getDerivedStateFromError(error: unknown): ErrorBoundaryState {
    return { message: error instanceof Error ? error.message : String(error) };
  }

  public override componentDidCatch(error: Error, info: ErrorInfo): void {
    // The browser's own report is the one with the stack in it; this exists
    // so the boundary is a place to add reporting later, not to duplicate
    // what the console already shows. `console` is forbidden here (oxlint).
    void error;
    void info;
  }

  public override render(): JSX.Element | ReactNode {
    const { message } = this.state;

    if (message === null) {
      return this.props.children;
    }

    return (
      <Box sx={{ p: 3 }}>
        <Alert severity="error">
          <AlertTitle>画面の描画に失敗しました</AlertTitle>
          {message}
        </Alert>
      </Box>
    );
  }
}
