import { QueueStats as QueueStatsType } from '../types'

interface Props {
  stats: QueueStatsType | null
}

export function QueueStats({ stats }: Props) {
  const queueDepths = stats?.queues.map((q) => q.depth) ?? []
  const maxDepth = Math.max(...queueDepths, 1)

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-5 gap-4 mb-6">
      {stats?.queues.map((q) => (
        <StatCard
          key={q.queue}
          label={q.queue}
          value={q.depth}
          sublabel="waiting"
          type="queue"
          maxDepth={maxDepth}
        />
      ))}
      <StatCard label="delayed" value={stats?.delayed ?? 0} sublabel="scheduled" type="delayed" />
      <StatCard label="dead" value={stats?.dead ?? 0} sublabel="exhausted" type="dead" />
      <StatCard label="throughput" value={stats?.throughput ?? 0} sublabel="jobs / 60s" type="throughput" />
    </div>
  )
}

function StatCard({
  label,
  value,
  sublabel,
  type,
  maxDepth = 1,
}: {
  label: string
  value: number
  sublabel: string
  type: 'queue' | 'delayed' | 'dead' | 'throughput'
  maxDepth?: number
}) {
  const config = {
    queue: {
      iconBg: 'bg-brand-950',
      iconColor: 'text-brand-400',
      icon: (
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path d="M2 3a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H3a1 1 0 01-1-1V3zM2 9a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H3a1 1 0 01-1-1V9zM3 15a1 1 0 000 2h14a1 1 0 000-2H3z" />
        </svg>
      ),
      barColor: 'bg-brand-600',
      showBar: true,
    },
    delayed: {
      iconBg: 'bg-amber-900/40',
      iconColor: 'text-amber-400',
      icon: (
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clipRule="evenodd" />
        </svg>
      ),
      barColor: 'bg-amber-500',
      showBar: false,
    },
    dead: {
      iconBg: 'bg-red-900/40',
      iconColor: 'text-red-400',
      icon: (
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
        </svg>
      ),
      barColor: 'bg-red-600',
      showBar: false,
    },
    throughput: {
      iconBg: 'bg-brand-950',
      iconColor: 'text-brand-500',
      icon: (
        <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4">
          <path fillRule="evenodd" d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z" clipRule="evenodd" />
        </svg>
      ),
      barColor: 'bg-brand-600',
      showBar: false,
    },
  }

  const c = config[type]
  const barWidth = type === 'queue' ? Math.round((value / maxDepth) * 100) : 0

  return (
    <div className="bg-surface-raised border border-surface-border rounded-xl p-5 hover:border-surface-muted transition-colors">
      <div className="flex items-center justify-between mb-3">
        <span className="text-xs font-medium text-zinc-500 uppercase tracking-wider">{label}</span>
        <div className={`w-7 h-7 rounded-lg ${c.iconBg} flex items-center justify-center ${c.iconColor}`}>
          {c.icon}
        </div>
      </div>
      <div className="text-3xl font-bold tabular-nums text-white">{value}</div>
      <div className="text-xs text-zinc-600 mt-1">{sublabel}</div>
      {c.showBar && (
        <div className="mt-3 h-1 bg-surface-overlay rounded-full overflow-hidden">
          <div
            className={`h-full ${c.barColor} rounded-full transition-all duration-500`}
            style={{ width: `${barWidth}%` }}
          />
        </div>
      )}
    </div>
  )
}
