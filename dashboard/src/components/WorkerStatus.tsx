import { Worker } from '../types'

interface Props {
  workers: Worker[]
}

export function WorkerStatus({ workers }: Props) {
  if (workers.length === 0) {
    return (
      <div className="bg-slate-800 rounded-lg p-6 text-center text-slate-500 text-sm">
        no workers connected — run <code className="text-slate-300">go run ./cmd/worker</code>
      </div>
    )
  }

  return (
    <div className="bg-slate-800 rounded-lg overflow-hidden">
      <div className="p-4 border-b border-slate-700">
        <h2 className="text-lg font-semibold text-white">workers</h2>
      </div>
      <table className="w-full text-sm">
        <thead>
          <tr className="text-slate-400 text-xs uppercase border-b border-slate-700">
            <th className="text-left px-4 py-2">id</th>
            <th className="text-left px-4 py-2">status</th>
            <th className="text-left px-4 py-2">queues</th>
            <th className="text-left px-4 py-2">current job</th>
            <th className="text-left px-4 py-2">last seen</th>
          </tr>
        </thead>
        <tbody>
          {workers.map((w) => {
            const isActive = w.status === 'active'
            return (
              <tr key={w.id} className="border-b border-slate-700">
                <td className="px-4 py-2 font-mono text-xs text-slate-300">{w.id}</td>
                <td className="px-4 py-2">
                  <span
                    className={`text-xs px-2 py-0.5 rounded-full ${
                      isActive ? 'bg-green-700 text-green-100' : 'bg-slate-600 text-slate-300'
                    }`}
                  >
                    {w.status}
                  </span>
                </td>
                <td className="px-4 py-2 text-slate-400 text-xs">{w.queues}</td>
                <td className="px-4 py-2 text-slate-500 font-mono text-xs">
                  {w.current_job ? w.current_job.slice(0, 8) + '...' : '—'}
                </td>
                <td className="px-4 py-2 text-slate-500 text-xs">{fmtRelative(w.last_seen)}</td>
              </tr>
            )
          })}
        </tbody>
      </table>
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
