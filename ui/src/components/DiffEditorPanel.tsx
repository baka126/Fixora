import React, { useEffect, useState, useRef } from 'react';
import { X, GitCommit, Loader2, Moon, Sun } from 'lucide-react';
import { DiffEditor } from '@monaco-editor/react';
import { apiClient } from '../api/client';

interface FileDiff {
  filePath: string;
  original: string;
  patched: string;
}

interface Props {
  remediationId: number;
  onClose: () => void;
}

export const DiffEditorPanel: React.FC<Props> = ({ remediationId, onClose }) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [diffs, setDiffs] = useState<FileDiff[]>([]);
  const [selectedFileIdx, setSelectedFileIdx] = useState(0);
  const [commitMessage, setCommitMessage] = useState('');
  const [pushing, setPushing] = useState(false);
  const [theme, setTheme] = useState<'light' | 'vs-dark'>('vs-dark');

  // We need to keep track of the modified content.
  // Monaco editor manages its own state, but we need to fetch it when pushing.
  const diffEditorRef = useRef<unknown>(null);

  useEffect(() => {
    const fetchDiff = async () => {
      try {
        const { data } = await apiClient.get<FileDiff[]>(`/remediations/${remediationId}/diff`);
        setDiffs(data || []);
        if (data && data.length > 0) {
          setCommitMessage(`Update ${data[0].filePath}`);
        }
      } catch (err) {
        const errorMsg = err instanceof Error ? err.message : 'Failed to fetch diff details';
        setError(errorMsg);
      } finally {
        setLoading(false);
      }
    };
    fetchDiff();
  }, [remediationId]);

  const handleEditorDidMount = (editor: unknown) => {
    diffEditorRef.current = editor;
  };

  const handlePush = async () => {
    if (!diffs.length || !diffEditorRef.current) return;

    // Get the modified content from the right-hand editor
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const modifiedEditor = (diffEditorRef.current as any).getModifiedEditor();
    const currentContent = modifiedEditor.getValue();
    const currentFile = diffs[selectedFileIdx];

    if (!commitMessage.trim()) {
      setError('Commit message is required');
      return;
    }

    setPushing(true);
    setError(null);
    try {
      await apiClient.post(`/remediations/${remediationId}/commit`, {
        filePath: currentFile.filePath,
        content: currentContent,
        message: commitMessage,
      });
      onClose(); // Close on success
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Failed to push commit';
      setError(errorMsg);
    } finally {
      setPushing(false);
    }
  };

  const currentFile = diffs[selectedFileIdx];

  return (
    <div className="fixed inset-0 z-[100] flex bg-black/50 backdrop-blur-sm transition-opacity">
      <div className="flex h-full w-full flex-col bg-white shadow-2xl m-4 rounded-xl overflow-hidden border border-[#e2e8f0]">
        
        {/* Header */}
        <header className="flex items-center justify-between bg-[#1e293b] px-6 py-4">
          <div className="flex items-center gap-4">
            <div className="rounded-lg bg-[#334155] p-2 text-white">
              <GitCommit className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-lg font-semibold text-white">Review & Edit AI Patch</h2>
              <p className="text-sm text-[#94a3b8]">Remediation #{remediationId}</p>
            </div>
          </div>
          <div className="flex items-center gap-4">
            <button
              onClick={() => setTheme(theme === 'light' ? 'vs-dark' : 'light')}
              className="rounded-md p-2 text-[#94a3b8] hover:bg-[#334155] hover:text-white outline-none transition-colors"
              title="Toggle Theme"
            >
              {theme === 'light' ? <Moon className="h-5 w-5" /> : <Sun className="h-5 w-5" />}
            </button>
            <button onClick={onClose} className="rounded-md p-2 text-[#94a3b8] hover:bg-[#334155] hover:text-white outline-none transition-colors">
              <X className="h-6 w-6" />
            </button>
          </div>
        </header>

        {/* File Selector */}
        {diffs.length > 1 && (
          <div className="flex items-center gap-2 border-b border-[#e2e8f0] bg-[#f8fafc] px-6 py-2">
            <span className="text-sm font-medium text-[#647084]">File:</span>
            <select
              value={selectedFileIdx}
              onChange={(e) => setSelectedFileIdx(Number(e.target.value))}
              className="rounded border border-[#e2e8f0] bg-white px-3 py-1 text-sm outline-none focus:border-[#157f7d]"
            >
              {diffs.map((d, idx) => (
                <option key={d.filePath} value={idx}>{d.filePath}</option>
              ))}
            </select>
          </div>
        )}

        {/* Body */}
        <div className="relative flex-1 bg-[#1e1e1e]">
          {loading ? (
            <div className="flex h-full items-center justify-center bg-white">
              <Loader2 className="h-10 w-10 animate-spin text-[#157f7d]" />
            </div>
          ) : diffs.length === 0 ? (
            <div className="flex h-full items-center justify-center bg-white text-[#647084]">
              No files changed in this remediation.
            </div>
          ) : (
            <DiffEditor
              height="100%"
              language="yaml" // Fixora mostly deals with YAML/JSON
              theme={theme}
              original={currentFile?.original || ''}
              modified={currentFile?.patched || ''}
              onMount={handleEditorDidMount}
              options={{
                renderSideBySide: true,
                readOnly: false, // The modified side is editable by default in Monaco DiffEditor if original is set, wait, actually we might need to set originalEditable to false
                originalEditable: false,
                minimap: { enabled: false },
                wordWrap: 'on',
              }}
            />
          )}
        </div>

        {/* Footer */}
        <footer className="flex items-center justify-between border-t border-[#e2e8f0] bg-[#f8fafc] px-6 py-4">
          <div className="flex-1 max-w-2xl mr-6">
            <input
              type="text"
              placeholder="Commit message (e.g. fix: adjusted resource limits)"
              value={commitMessage}
              onChange={(e) => setCommitMessage(e.target.value)}
              disabled={loading || diffs.length === 0}
              className="w-full rounded-lg border border-[#cbd5e1] px-4 py-2.5 text-sm outline-none focus:border-[#157f7d] focus:ring-1 focus:ring-[#157f7d]"
            />
            {error && <p className="mt-2 text-sm text-red-600">{error}</p>}
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className="rounded-lg border border-[#cbd5e1] bg-white px-5 py-2.5 text-sm font-medium text-[#475569] hover:bg-[#f1f5f9] transition-colors"
            >
              Cancel
            </button>
            <button
              onClick={handlePush}
              disabled={pushing || loading || diffs.length === 0}
              className="flex items-center gap-2 rounded-lg bg-[#157f7d] px-5 py-2.5 text-sm font-medium text-white hover:bg-[#0f6664] disabled:opacity-70 transition-colors"
            >
              {pushing ? <Loader2 className="h-4 w-4 animate-spin" /> : <GitCommit className="h-4 w-4" />}
              Push to PR
            </button>
          </div>
        </footer>

      </div>
    </div>
  );
};
