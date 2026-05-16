import React, { useEffect, useState } from 'react';
import { X, Shield, Activity, Brain, FileText, Clock } from 'lucide-react';
import { apiClient } from '../api/client';
import type { InvestigationDetail } from '../types';

interface Props {
  eventId: string;
  onClose: () => void;
}

export const AuditDetailPanel: React.FC<Props> = ({ eventId, onClose }) => {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<InvestigationDetail | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      // eventId is "investigation-123", we need the number
      const id = eventId.split('-')[1];
      if (!id) {
        setError('Invalid investigation ID');
        setLoading(false);
        return;
      }

      try {
        const { data } = await apiClient.get<InvestigationDetail>(`/audit/investigations/${id}`);
        setData(data);
      } catch {
        setError('Failed to fetch investigation details');
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [eventId]);

  return (
    <div className="fixed inset-y-0 right-0 z-50 flex max-w-full pl-10">
      <div className="w-screen max-w-2xl transform bg-white shadow-2xl transition-transform duration-500 ease-in-out sm:duration-700">
        <div className="flex h-full flex-col overflow-y-scroll bg-white shadow-xl">
          <header className="bg-[#1e293b] px-4 py-6 sm:px-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="rounded-lg bg-[#334155] p-2 text-white">
                  <Shield className="h-5 w-5" />
                </div>
                <div>
                  <h2 className="text-lg font-semibold text-white">Audit Investigation Detail</h2>
                  <p className="text-sm text-[#94a3b8]">Reviewing forensics and AI correlation</p>
                </div>
              </div>
              <button onClick={onClose} className="rounded-md text-[#94a3b8] hover:text-white outline-none">
                <X className="h-6 w-6" />
              </button>
            </div>
          </header>

          <div className="relative flex-1 p-6 space-y-8">
            {loading ? (
              <div className="flex items-center justify-center h-64">
                <div className="h-8 w-8 animate-spin rounded-full border-4 border-[#157f7d] border-t-transparent" />
              </div>
            ) : error ? (
              <div className="rounded-lg bg-red-50 p-4 text-red-700 border border-red-200">{error}</div>
            ) : data && (
              <>
                <section>
                  <div className="flex items-center gap-2 mb-4 text-[#1e293b]">
                    <Activity className="h-4 w-4" />
                    <h3 className="font-semibold uppercase tracking-wider text-xs">Subject Information</h3>
                  </div>
                  <div className="grid grid-cols-2 gap-4 rounded-xl bg-[#f8fafc] p-4 border border-[#e2e8f0]">
                    <div>
                      <span className="block text-[10px] uppercase text-[#647084] font-bold">Pod / Namespace</span>
                      <code className="text-[13px] font-medium text-[#0f172a]">{data.namespace}/{data.podName}</code>
                    </div>
                    <div>
                      <span className="block text-[10px] uppercase text-[#647084] font-bold">Trigger Reason</span>
                      <span className="text-[13px] font-medium text-[#dc2626]">{data.reason}</span>
                    </div>
                    <div>
                      <span className="block text-[10px] uppercase text-[#647084] font-bold">Timestamp</span>
                      <div className="flex items-center gap-1.5 text-[13px] text-[#0f172a]">
                        <Clock className="h-3.5 w-3.5 text-[#647084]" />
                        {new Date(data.timestamp).toLocaleString()}
                      </div>
                    </div>
                    <div>
                      <span className="block text-[10px] uppercase text-[#647084] font-bold">AI Confidence</span>
                      <span className="text-[13px] font-bold text-[#15803d]">{data.aiConfidence}%</span>
                    </div>
                  </div>
                </section>

                <section>
                  <div className="flex items-center gap-2 mb-4 text-[#1e293b]">
                    <FileText className="h-4 w-4" />
                    <h3 className="font-semibold uppercase tracking-wider text-xs">Evidence Chain</h3>
                  </div>
                  <div className="space-y-4">
                    <DetailBlock label="Metric Proof" content={data.metricProof} />
                    <DetailBlock label="Event Timeline" content={data.eventTimeline} />
                    <DetailBlock label="Stack Trace" content={data.stackTrace} />
                    <DetailBlock label="Root Cause" content={data.rootCause} isHighlight />
                  </div>
                </section>

                <section>
                  <div className="flex items-center gap-2 mb-4 text-[#1e293b]">
                    <Brain className="h-4 w-4" />
                    <h3 className="font-semibold uppercase tracking-wider text-xs">AI Interaction</h3>
                  </div>
                  <div className="space-y-4">
                    <DetailBlock label="Input Content (Prompt)" content={data.aiPrompt} isCode />
                    <DetailBlock label="Raw AI Response" content={data.aiResponse} isCode />
                  </div>
                </section>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

const DetailBlock = ({ label, content, isCode, isHighlight }: { label: string; content: string; isCode?: boolean; isHighlight?: boolean }) => {
  if (!content || content === "Pending" || content === "None") return null;
  return (
    <div>
      <h4 className="text-[11px] font-bold text-[#647084] uppercase mb-1.5">{label}</h4>
      <div className={`
        rounded-lg p-3 text-[12px] leading-relaxed whitespace-pre-wrap font-mono
        ${isHighlight ? 'bg-[#f0fdf4] text-[#166534] border border-[#bbf7d0]' : 'bg-[#f1f5f9] text-[#334155] border border-[#e2e8f0]'}
        ${isCode ? 'bg-[#0f172a] text-[#f8fafc] border-none overflow-x-auto max-h-64' : ''}
      `}>
        {content}
      </div>
    </div>
  );
};
