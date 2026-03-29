import { useState } from 'react'
import { Worker, Job } from '../types'

interface Props {
  workers: Worker[]
}

export function WorkerStatus({ workers }: Props) {
  const [inspecting, setInspecting] = useState<{ worker: Worker; job: Job } | null>(null)

  async function handleCardClick(w: Worker) {
    if (!w.current_job) return
    try {
      const res = await fetch(`/api/jobs/${w.current_job}`)
      if (!res.ok) return
      const job = await res.json()
      setInspecting({ worker: w, job })
    } catch {}
  }

  if (workers.length === 0) {
    return (
      <div className="text-center py-16">
        <div className="w-12 h-12 rounded-xl bg-surface-overlay border border-surface-border flex items-center justify-center mx-auto mb-3">
          <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5 text-zinc-600">
            <path fillRule="evenodd" d="M2 5a2 2 0 012-2h12a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V5zm3.293 1.293a1 1 0 011.414 0l3 3a1 1 0 010 1.414l-3 3a1 1 0 01-1.414-1.414L7.586 10 5.293 7.707a1 1 0 010-1.414zM11 12a1 1 0 100 2h3a1 1 0 100-2h-3z" clipRule="evenodd" />
          </svg>
        </div>
        <div className="text-zinc-400 text-sm font-medium">No workers connected</div>
        <div className="text-zinc-600 text-xs mt-1">
          Run <code className="text-zinc-400 bg-surface-overlay px-1.5 py-0.5 rounded font-mono">go run ./cmd/worker</code>
        </div>
      </div>
    )
  }

  return (
    <>
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {workers.map((w) => {
          const isActive = w.status === 'active'
          const queues = (w.queues || '').split(',').map((q) => q.trim()).filter(Boolean)
          const clickable = isActive && !!w.current_job

          return (
            <div
              key={w.id}
              onClick={() => handleCardClick(w)}
              className={`bg-surface-raised border border-surface-border rounded-xl p-5 relative overflow-hidden transition-colors ${
                clickable ? 'cursor-pointer hover:border-brand-700' : ''
              }`}
            >
              {/* green top accent for active workers */}
              {isActive && (
                <div className="absolute top-0 left-0 right-0 h-0.5 bg-gradient-to-r from-brand-500 to-brand-700" />
              )}

              {/* header */}
              <div className="flex items-start justify-between mb-4">
                <div>
                  <div className="font-mono text-sm text-zinc-200">{w.id.slice(0, 16)}…</div>
                  <div className="text-xs text-zinc-500 mt-0.5">since {fmtRelative(w.started_at)}</div>
                </div>
                {isActive ? (
                  <div className="relative flex items-center justify-center w-8 h-8 flex-shrink-0">
                    <span className="absolute inline-flex h-full w-full rounded-full bg-brand-400 opacity-20 motion-safe:animate-ping" />
                    <span className="relative w-3 h-3 rounded-full bg-brand-400" />
                  </div>
                ) : (
                  <span className="w-2 h-2 rounded-full bg-zinc-600 mt-1 flex-shrink-0" />
                )}
              </div>

              {/* queue pills */}
              <div className="flex flex-wrap gap-1.5 mb-4">
                {queues.map((q) => (
                  <span
                    key={q}
                    className="text-xs px-2 py-0.5 rounded-full bg-brand-950 border border-brand-900 text-brand-400"
                  >
                    {q}
                  </span>
                ))}
              </div>

              {/* current job */}
              {isActive && w.current_job ? (
                <div className="bg-surface-overlay rounded-lg px-3 py-2 border border-surface-border">
                  <div className="text-zinc-500 text-xs mb-0.5">processing — click to inspect</div>
                  <div className="font-mono text-xs text-brand-400 truncate">{w.current_job}</div>
                </div>
              ) : isActive ? (
                <div className="text-xs text-zinc-600 italic">idle — waiting for jobs</div>
              ) : (
                <div className="text-xs text-zinc-600">stopped</div>
              )}

              {/* footer */}
              <div className="flex items-center justify-between mt-4 pt-3 border-t border-surface-border">
                <span className="text-xs text-zinc-600">last seen</span>
                <span className="text-xs text-zinc-400">{fmtRelative(w.last_seen)}</span>
              </div>
            </div>
          )
        })}
      </div>

      {/* job inspect modal */}
      {inspecting && (
        <JobInspectModal
          worker={inspecting.worker}
          job={inspecting.job}
          onClose={() => setInspecting(null)}
        />
      )}
    </>
  )
}

function JobInspectModal({ worker, job, onClose }: { worker: Worker; job: Job; onClose: () => void }) {
  return (
    <div className="fixed inset-0 bg-black/50 backdrop-blur-sm z-50 flex items-center justify-center p-4" onClick={onClose}>
      <div className="bg-surface-raised border border-surface-border rounded-xl w-full max-w-md shadow-2xl" onClick={(e) => e.stopPropagation()}>
        {/* header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-surface-border">
          <div>
            <div className="text-xs text-zinc-500 mb-0.5">worker currently processing</div>
            <div className="font-mono text-sm text-brand-400">{job.type}</div>
          </div>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-200 transition-colors">
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
              <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
            </svg>
          </button>
        </div>

        {/* body */}
        <div className="px-5 py-4 space-y-4">
          <div className="space-y-2">
            <InfoRow label="Job ID" value={job.id} mono />
            <InfoRow label="Queue" value={job.queue} />
            <InfoRow label="Worker" value={worker.id} mono />
            <InfoRow label="Started" value={job.started_at ? new Date(job.started_at).toLocaleString() : '—'} />
            <InfoRow label="Attempt" value={`${job.attempts + 1} / ${job.max_attempts}`} />
          </div>

          <div>
            <div className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-2">Payload</div>
            <pre className="bg-surface-overlay border border-surface-border text-zinc-300 text-xs font-mono p-3 rounded-lg overflow-x-auto max-h-40 whitespace-pre-wrap">
              {JSON.stringify(job.payload, null, 2)}
            </pre>
          </div>
        </div>
      </div>
    </div>
  )
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex justify-between items-start gap-4">
      <span className="text-zinc-500 text-sm shrink-0">{label}</span>
      <span className={`text-zinc-300 text-sm text-right ${mono ? 'font-mono text-xs break-all' : ''}`}>{value}</span>
    </div>
  )
}

function fmtRelative(ts: string) {
  if (!ts) return '—'
  const diff = Date.now() - new Date(ts).getTime()
  const secs = Math.floor(diff / 1000)
  if (secs < 60) return `${secs}s ago`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`
  return new Date(ts).toLocaleTimeString()
}
