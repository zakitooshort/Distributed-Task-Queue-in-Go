import { useState } from 'react'
import { Job, JobStatus } from '../types'

interface Props {
  jobs: Job[]
  onRetry: (id: string) => void
  onCancel: (id: string) => void
}

const statusColors: Record<JobStatus, string> = {
  pending: 'bg-slate-600 text-slate-200',
  running: 'bg-blue-600 text-blue-100',
  completed: 'bg-green-700 text-green-100',
  failed: 'bg-orange-700 text-orange-100',
  dead: 'bg-red-700 text-red-100',
}

export function JobList({ jobs, onRetry, onCancel }: Props) {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [queueFilter, setQueueFilter] = useState<string>('')
  const [selected, setSelected] = useState<Job | null>(null)

  const filtered = jobs.filter((j) => {
    if (statusFilter && j.status !== statusFilter) return false
    if (queueFilter && j.queue !== queueFilter) return false
    return true
  })

  return (
    <div className="bg-slate-800 rounded-lg overflow-hidden">
      <div className="p-4 border-b border-slate-700 flex gap-3 items-center flex-wrap">
        <h2 className="text-lg font-semibold text-white flex-1">jobs</h2>
        <select
          className="bg-slate-700 text-slate-200 text-sm rounded px-2 py-1 border border-slate-600"
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">all statuses</option>
          <option value="pending">pending</option>
          <option value="running">running</option>
          <option value="completed">completed</option>
          <option value="failed">failed</option>
          <option value="dead">dead</option>
        </select>
        <select
          className="bg-slate-700 text-slate-200 text-sm rounded px-2 py-1 border border-slate-600"
          value={queueFilter}
          onChange={(e) => setQueueFilter(e.target.value)}
        >
          <option value="">all queues</option>
          <option value="default">default</option>
          <option value="critical">critical</option>
          <option value="email">email</option>
        </select>
      </div>

      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-slate-400 text-xs uppercase border-b border-slate-700">
              <th className="text-left px-4 py-2">type</th>
              <th className="text-left px-4 py-2">queue</th>
              <th className="text-left px-4 py-2">status</th>
              <th className="text-left px-4 py-2">attempts</th>
              <th className="text-left px-4 py-2">created</th>
              <th className="text-left px-4 py-2">actions</th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 && (
              <tr>
                <td colSpan={6} className="text-center text-slate-500 py-8">
                  no jobs yet
                </td>
              </tr>
            )}
            {filtered.map((job) => (
              <tr
                key={job.id}
                className="border-b border-slate-700 hover:bg-slate-750 cursor-pointer"
                onClick={() => setSelected(job)}
              >
                <td className="px-4 py-2 text-slate-200 font-mono">{job.type}</td>
                <td className="px-4 py-2 text-slate-400">{job.queue}</td>
                <td className="px-4 py-2">
                  <span className={`text-xs px-2 py-0.5 rounded-full ${statusColors[job.status]}`}>
                    {job.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-slate-400">
                  {job.attempts}/{job.max_attempts}
                </td>
                <td className="px-4 py-2 text-slate-500 text-xs">{fmtTime(job.created_at)}</td>
                <td className="px-4 py-2" onClick={(e) => e.stopPropagation()}>
                  {job.status === 'dead' && (
                    <button
                      onClick={() => onRetry(job.id)}
                      className="text-xs px-2 py-1 bg-blue-700 hover:bg-blue-600 text-white rounded mr-1"
                    >
                      replay
                    </button>
                  )}
                  {job.status === 'pending' && (
                    <button
                      onClick={() => onCancel(job.id)}
                      className="text-xs px-2 py-1 bg-slate-600 hover:bg-slate-500 text-white rounded"
                    >
                      cancel
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* job detail modal */}
      {selected && (
        <JobDetail job={selected} onClose={() => setSelected(null)} />
      )}
    </div>
  )
}

function JobDetail({ job, onClose }: { job: Job; onClose: () => void }) {
  return (
    <div
      className="fixed inset-0 bg-black/60 flex items-center justify-center z-50"
      onClick={onClose}
    >
      <div
        className="bg-slate-800 rounded-xl p-6 max-w-lg w-full mx-4 max-h-[80vh] overflow-y-auto"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex justify-between items-start mb-4">
          <h3 className="text-lg font-semibold text-white">{job.type}</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-white">✕</button>
        </div>

        <div className="space-y-3 text-sm">
          <Row label="id" value={job.id} mono />
          <Row label="queue" value={job.queue} />
          <Row label="status" value={job.status} />
          <Row label="priority" value={String(job.priority)} />
          <Row label="attempts" value={`${job.attempts} / ${job.max_attempts}`} />
          <Row label="created" value={fmtTime(job.created_at)} />
          {job.started_at && <Row label="started" value={fmtTime(job.started_at)} />}
          {job.completed_at && <Row label="completed" value={fmtTime(job.completed_at)} />}
          {job.failed_at && <Row label="failed" value={fmtTime(job.failed_at)} />}
          {job.worker_id && <Row label="worker" value={job.worker_id} mono />}
          {job.error && (
            <div>
              <div className="text-slate-400 mb-1">error</div>
              <div className="bg-red-900/40 text-red-300 text-xs font-mono p-2 rounded">{job.error}</div>
            </div>
          )}
          <div>
            <div className="text-slate-400 mb-1">payload</div>
            <pre className="bg-slate-900 text-slate-300 text-xs p-2 rounded overflow-x-auto">
              {JSON.stringify(job.payload, null, 2)}
            </pre>
          </div>
        </div>
      </div>
    </div>
  )
}

function Row({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="flex gap-2">
      <span className="text-slate-400 w-24 shrink-0">{label}</span>
      <span className={`text-slate-200 ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  )
}

function fmtTime(ts: string) {
  if (!ts) return '—'
  const d = new Date(ts)
  return d.toLocaleString()
}
