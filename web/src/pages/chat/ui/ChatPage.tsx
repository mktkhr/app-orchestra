import Alert from "@mui/material/Alert";
import Box from "@mui/material/Box";
import CircularProgress from "@mui/material/CircularProgress";
import Stack from "@mui/material/Stack";
import Typography from "@mui/material/Typography";
import { useEffect, useState, type JSX } from "react";

import { getHealth } from "@/shared/api/client";

type BackendStatus =
  | { readonly kind: "loading" }
  | { readonly kind: "ok"; readonly status: string }
  | { readonly kind: "error" };

/**
 * The chat screen. There is no conversation yet - this is the scaffold that
 * proves the frontend and the backend are wired together, ahead of
 * docs/plans/orchestration.md Task 1.
 */
export function ChatPage(): JSX.Element {
  const [backend, setBackend] = useState<BackendStatus>({ kind: "loading" });

  useEffect(() => {
    let cancelled = false;

    const checkBackend = async (): Promise<void> => {
      try {
        const health = await getHealth();

        if (!cancelled) {
          setBackend({ kind: "ok", status: health.status });
        }
      } catch {
        if (!cancelled) {
          setBackend({ kind: "error" });
        }
      }
    };

    void checkBackend();

    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Stack spacing={3} sx={{ p: 3 }}>
      <Typography variant="h5" component="h1">
        チャット
      </Typography>
      <Typography variant="body1" color="text.secondary">
        ここに会話が入ります。
      </Typography>
      <Box>
        <BackendStatusView backend={backend} />
      </Box>
    </Stack>
  );
}

function BackendStatusView({ backend }: { readonly backend: BackendStatus }): JSX.Element {
  switch (backend.kind) {
    case "loading":
      return <CircularProgress size={20} aria-label="バックエンドの状態を確認中" />;
    case "ok":
      return <Alert severity="success">バックエンド応答: {backend.status}</Alert>;
    case "error":
      return <Alert severity="error">バックエンドに接続できません。</Alert>;
    default:
      return assertNever(backend);
  }
}

function assertNever(value: never): never {
  throw new Error(`unhandled backend status: ${JSON.stringify(value)}`);
}
