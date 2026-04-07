import { useState, useCallback } from "react";
import { api } from "../api/client";
import type { UploadProgress } from "../models/types";

interface UploadState {
  uploading: boolean;
  progress: UploadProgress | null;
  error: string | null;
}

export function useUpload() {
  const [state, setState] = useState<UploadState>({
    uploading: false,
    progress: null,
    error: null,
  });

  const upload = useCallback(async (file: File): Promise<{ mediaId: number } | null> => {
    setState({ uploading: true, progress: { loaded: 0, total: file.size, percent: 0 }, error: null });
    try {
      const result = await api.uploadWithProgress(file, (p) =>
        setState((prev) => ({ ...prev, progress: p }))
      );
      setState({ uploading: false, progress: null, error: null });
      return { mediaId: result.mediaId };
    } catch (err) {
      setState({
        uploading: false,
        progress: null,
        error: err instanceof Error ? err.message : "Upload failed",
      });
      return null;
    }
  }, []);

  const reset = useCallback(() => {
    setState({ uploading: false, progress: null, error: null });
  }, []);

  return { ...state, upload, reset };
}
