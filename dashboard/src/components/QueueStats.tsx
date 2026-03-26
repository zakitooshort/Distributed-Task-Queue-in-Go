import { QueueStats as QueueStatsType } from '../types'

interface Props {
  stats: QueueStatsType | null
}

export function QueueStats({ stats }: Props) {
  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
      {stats?.queues.map((q) => (
        <StatCard key={q.queue} label={q.queue} value={q.depth} sublabel="waiting" color="blue" />
      ))}
      <StatCard label="delayed" value={stats?.delayed ?? 0} sublabel="scheduled" color="yellow" />
      <StatCard label="dead" value={stats?.dead ?? 0} sublabel="failed" color="red" />
      <StatCard label="throughput" value={stats?.throughput ?? 0} sublabel="jobs/60s" color="green" />
    </div>
  )
}

function StatCard({
  label,
  value,
  sublabel,
  color,
}: {
  label: string
  value: number
  sublabel: string
  color: 'blue' | 'yellow' | 'red' | 'green'
}) {
  const colors = {
    blue: 'border-blue-500 text-blue-400',
    yellow: 'border-yellow-500 text-yellow-400',
    red: 'border-red-500 text-red-400',
    green: 'border-green-500 text-green-400',
  }

  return (
    <div className={`bg-slate-800 rounded-lg p-4 border-l-4 ${colors[color]}`}>
      <div className="text-2xl font-bold text-white">{value}</div>
      <div className={`text-sm font-medium ${colors[color]}`}>{label}</div>
      <div className="text-xs text-slate-400">{sublabel}</div>
    </div>
  )
}
