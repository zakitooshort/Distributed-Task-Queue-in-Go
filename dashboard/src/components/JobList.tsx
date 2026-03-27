import { useEffect, useState } from 'react'
import { Job, JobStatus } from '../types'

interface Props {
  jobs: Job[]
  onRetry: (id: string) => void
  onCancel: (id: string) => void
}

const statusConfig: Record<JobStatus, { bg: string; text: string; border: string; dot: string }> = {
  pending:   { bg: 'bg-amber-900/40',  text: 'text-amber-300',  border: 'border-amber-800/50',  dot: 'bg-amber-400' },
  running:   { bg: 'bg-blue-900/40',   text: 'text-blue-300',   border: 'border-blue-800/50',   dot: 'bg-blue-400' },
  completed: { bg: 'bg-brand-900/40',  text: 'text-brand-300',  border: 'border-brand-800/50',  dot: 'bg-brand-400' },
  failed:    { bg: 'bg-orange-900/40', text: 'text-orange-300', border: 'border-orange-800/50', dot: 'bg-orange-400' },
  dead:      { bg: 'bg-red-900/40',    text: 'text-red-300',    border: 'border-red-800/50',    dot: 'bg-red-400' },
}

const ALL_STATUSES: JobStatus[] = ['pending', 'running', 'completed', 'failed', 'dead']

export function JobList({ jobs, onRetry, onCancel }: Props) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [queueFilter, setQueueFilter] = useState<string>('')
  const [search, setSearch] = useState<string>('')
  const [selected, setSelected] = useState<Job | null>(null)

  // lock body scroll when drawer is open
  useEffect(() => {
    if (selected) {
      document.body.style.overflow = 'hidden'
    } else {
      document.body.style.overflow = ''
    }
    return () => { document.body.style.overflow = '' }
  }, [selected])

  const queues = [...new Set(jobs.map((j) => j.queue))].sort()

  const statusCounts = ALL_STATUSES.reduce<Record<string, number>>((acc, s) => {
    acc[s] = jobs.filter((j) => j.status === s).length
    return acc
  }, {})

  const filtered = jobs.filter((j) => {
    if (statusFilter && j.status !== statusFilter) return false
    if (queueFilter && j.queue !== queueFilter) return false
    if (search && !j.type.includes(search) && !j.id.includes(search)) return false
    return true
  })

  return (
    <div className="bg-surface-raised border border-surface-border rounded-xl overflow-hidden">
      {/* filter bar */}
      <div className="p-4 border-b border-surface-border space-y-3">
        {/* status pills */}
        <div className="flex flex-wrap gap-1.5">
          <PillButton active={statusFilter === ''} onClick={() => setStatusFilter('')}>
            all <span className="opacity-60">{jobs.length}</span>
          </PillButton>
          {ALL_STATUSES.map((s) => (
            <PillButton key={s} active={statusFilter === s} onClick={() => setStatusFilter(s)}>
              {s} <span className="opacity-60">{statusCounts[s]}</span>
            </PillButton>
          ))}
        </div>

        {/* queue + search row */}
        <div className="flex flex-wrap gap-2 items-center">
          {queues.length > 0 && (
            <div className="flex gap-1.5">
              <PillButton active={queueFilter === ''} onClick={() => setQueueFilter('')} small>
                all queues
              </PillButton>
              {queues.map((q) => (
                <PillButton key={q} active={queueFilter === q} onClick={() => setQueueFilter(q)} small>
                  {q}
                </PillButton>
              ))}
            </div>
          )}
          <input
            type="text"
            placeholder="search type or id…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="ml-auto bg-surface-overlay border border-surface-muted rounded-lg px-3 py-1.5 text-sm text-zinc-200 placeholder-zinc-600 focus:outline-none focus:border-brand-600 w-52 transition-colors"
          />
        </div>
      </div>

      {/* table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-overlay text-zinc-500 text-xs font-semibold uppercase tracking-wider">
              <th className="text-left px-4 py-3">type</th>
              <th className="text-left px-4 py-3">queue</th>
              <th className="text-left px-4 py-3">status</th>
              <th className="text-left px-4 py-3">attempts</th>
              <th className="text-left px-4 py-3">created</th>
              <th className="text-left px-4 py-3"></th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 && (
              <tr>
                <td colSpan={6} className="text-center text-zinc-600 py-12 text-sm">
                  no jobs match your filters
                </td>
              </tr>
            )}
            {filtered.map((job) => {
              const sc = statusConfig[job.status]
              const attemptPct = Math.round((job.attempts / job.max_attempts) * 100)
              return (
                <tr
                  key={job.id}
                  className="border-b border-surface-border hover:bg-surface-overlay cursor-pointer transition-colors"
                  onClick={() => setSelected(job)}
                >
                  <td className="px-4 py-3 font-mono text-sm text-zinc-200">{job.type}</td>
                  <td className="px-4 py-3 text-zinc-500 text-xs">{job.queue}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full border ${sc.bg} ${sc.text} ${sc.border}`}>
                      <span className={`w-1.5 h-1.5 rounded-full ${sc.dot}`} />
                      {job.status}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="text-zinc-400 text-xs tabular-nums">{job.attempts}/{job.max_attempts}</span>
                      <div className="w-12 h-1 bg-surface-overlay rounded-full overflow-hidden">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${job.attempts >= job.max_attempts ? 'bg-red-500' : 'bg-brand-600'}`}
                          style={{ width: `${attemptPct}%` }}
                        />
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 text-zinc-600 text-xs">{fmtTime(job.created_at)}</td>
                  <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                    <div className="flex gap-1">
                      {job.status === 'dead' && (
                        <button
                          title="Replay job"
                          onClick={() => onRetry(job.id)}
                          className="w-7 h-7 rounded-lg bg-brand-900/60 hover:bg-brand-800 text-brand-400 flex items-center justify-center transition-colors"
                        >
                          <svg viewBox="0 0 20 20" fill="currentColor" className="w-3.5 h-3.5">
                            <path fillRule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clipRule="evenodd" />
                          </svg>
                        </button>
                      )}
                      {job.status === 'pending' && (
                        <button
                          title="Cancel job"
                          onClick={() => onCancel(job.id)}
                          className="w-7 h-7 rounded-lg bg-red-900/40 hover:bg-red-900/70 text-red-400 flex items-center justify-center transition-colors"
                        >
                          <svg viewBox="0 0 20 20" fill="currentColor" className="w-3.5 h-3.5">
                            <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
                          </svg>
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* side drawer — always mounted so CSS transition works */}
      <JobDrawer
        job={selected}
        onClose={() => setSelected(null)}
        onRetry={onRetry}
        onCancel={onCancel}
      />
    </div>
  )
}

function PillButton({
  active,
  onClick,
  children,
  small,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
  small?: boolean
}) {
  return (
    <button
      onClick={onClick}
      className={`${small ? 'px-2.5 py-1 text-xs' : 'px-3 py-1 text-xs'} rounded-full font-medium border transition-colors ${
        active
          ? 'border-brand-600 text-brand-400 bg-brand-950'
          : 'border-surface-muted text-zinc-500 hover:text-zinc-300 hover:border-zinc-500'
      }`}
    >
      {children}
    </button>
  )
}

function JobDrawer({
  job,
  onClose,
  onRetry,
  onCancel,
}: {
  job: Job | null
  onClose: () => void
  onRetry: (id: string) => void
  onCancel: (id: string) => void
}) {
  const isOpen = job !== null

  return (
    <>
      {/* backdrop */}
      <div
        className={`fixed inset-0 bg-black/50 backdrop-blur-sm z-40 transition-opacity duration-200 ${isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
        onClick={onClose}
      />

      {/* drawer panel */}
      <div
        className={`fixed top-0 right-0 h-full w-full max-w-md z-50 bg-surface-raised border-l border-surface-border flex flex-col shadow-2xl transition-transform duration-200 ease-out ${isOpen ? 'translate-x-0' : 'translate-x-full'}`}
      >
        {job && (
          <>
            {/* header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-surface-border flex-shrink-0">
              <div>
                <div className="font-mono text-brand-400 text-sm font-medium">{job.type}</div>
                <div className="text-zinc-500 text-xs mt-0.5 font-mono">{job.id.slice(0, 18)}…</div>
              </div>
              <div className="flex items-center gap-2">
                {(() => {
                  const sc = statusConfig[job.status]
                  return (
                    <span className={`inline-flex items-center gap-1.5 text-xs px-2 py-0.5 rounded-full border ${sc.bg} ${sc.text} ${sc.border}`}>
                      <span className={`w-1.5 h-1.5 rounded-full ${sc.dot}`} />
                      {job.status}
                    </span>
                  )
                })()}
                <button onClick={onClose} className="text-zinc-500 hover:text-zinc-200 transition-colors ml-1">
                  <svg viewBox="0 0 20 20" fill="currentColor" className="w-5 h-5">
                    <path fillRule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clipRule="evenodd" />
                  </svg>
                </button>
              </div>
            </div>

            {/* body */}
            <div className="flex-1 overflow-y-auto px-6 py-5 space-y-6">
              {/* timeline */}
              <Section title="Timeline">
                <TimelineRow icon="create" label="Created" value={fmtTime(job.created_at)} />
                {job.started_at && <TimelineRow icon="play" label="Started" value={fmtTime(job.started_at)} />}
                {job.completed_at && <TimelineRow icon="check" label="Completed" value={fmtTime(job.completed_at)} accent="green" />}
                {job.failed_at && <TimelineRow icon="x" label="Failed" value={fmtTime(job.failed_at)} accent="red" />}
              </Section>

              {/* execution info */}
              <Section title="Execution">
                <InfoRow label="Queue" value={job.queue} />
                <InfoRow label="Priority" value={String(job.priority)} />
                <InfoRow label="Attempts" value={`${job.attempts} / ${job.max_attempts}`} />
                {job.worker_id && <InfoRow label="Worker" value={job.worker_id} mono />}
                {job.scheduled_at && <InfoRow label="Scheduled" value={fmtTime(job.scheduled_at)} />}
              </Section>

              {/* error */}
              {job.error && (
                <Section title="Error">
                  <pre className="bg-red-950/60 border border-red-900/50 text-red-300 text-xs font-mono p-3 rounded-lg overflow-x-auto whitespace-pre-wrap">
                    {job.error}
                  </pre>
                </Section>
              )}

              {/* payload */}
              <Section title="Payload">
                <pre className="bg-surface-overlay border border-surface-border text-zinc-300 text-xs font-mono p-3 rounded-lg overflow-x-auto max-h-56 whitespace-pre-wrap">
                  {JSON.stringify(job.payload, null, 2)}
                </pre>
              </Section>
            </div>

            {/* footer actions */}
            {(job.status === 'dead' || job.status === 'pending') && (
              <div className="px-6 py-4 border-t border-surface-border flex-shrink-0 flex gap-3">
                {job.status === 'dead' && (
                  <button
                    onClick={() => { onRetry(job.id); onClose() }}
                    className="flex-1 py-2 rounded-lg bg-brand-700 hover:bg-brand-600 text-white text-sm font-medium transition-colors"
                  >
                    Replay Job
                  </button>
                )}
                {job.status === 'pending' && (
                  <button
                    onClick={() => { onCancel(job.id); onClose() }}
                    className="flex-1 py-2 rounded-lg border border-red-800 hover:bg-red-900/40 text-red-400 text-sm font-medium transition-colors"
                  >
                    Cancel Job
                  </button>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-xs font-semibold text-zinc-500 uppercase tracking-wider mb-3">{title}</div>
      <div className="space-y-2">{children}</div>
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

function TimelineRow({
  label,
  value,
  accent,
}: {
  icon: string
  label: string
  value: string
  accent?: 'green' | 'red'
}) {
  const color = accent === 'green' ? 'text-brand-400' : accent === 'red' ? 'text-red-400' : 'text-zinc-500'
  return (
    <div className="flex justify-between items-center">
      <span className={`text-sm ${color}`}>{label}</span>
      <span className="text-zinc-400 text-xs">{value}</span>
    </div>
  )
}

function fmtTime(ts: string | undefined) {
  if (!ts) return '—'
  return new Date(ts).toLocaleString()
}
