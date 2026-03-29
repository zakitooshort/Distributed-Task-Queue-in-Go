import { useEffect, useState, useCallback } from 'react'
import { Job, Worker, QueueStats, SSEEvent } from './types'
import { QueueStats as QueueStatsComponent } from './components/QueueStats'
import { JobList } from './components/JobList'
import { WorkerStatus } from './components/WorkerStatus'
import { useSSE } from './hooks/useSSE'

type Tab = 'jobs' | 'workers' | 'dead'

export default function App() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [workers, setWorkers] = useState<Worker[]>([])
  const [stats, setStats] = useState<QueueStats | null>(null)
  const [connected, setConnected] = useState(false)
  const [tab, setTab] = useState<Tab>('jobs')

  useEffect(() => {
    fetchJobs()
    fetchWorkers()
    fetchStats()

    // poll workers every 3s so current_job stays live without SSE from worker containers
    const workerPoll = setInterval(fetchWorkers, 3000)
    return () => clearInterval(workerPoll)
  }, [])

  async function fetchJobs() {
    try {
      const res = await fetch('/api/jobs?limit=100')
      const data = await res.json()
      setJobs(data.jobs ?? [])
    } catch {}
  }

  async function fetchWorkers() {
    try {
      const res = await fetch('/api/workers')
      const data = await res.json()
      setWorkers(data.workers ?? [])
    } catch {}
  }

  async function fetchStats() {
    try {
      const res = await fetch('/api/queues')
      const data = await res.json()
      setStats(data)
    } catch {}
  }

  const handleSSEEvent = useCallback((event: SSEEvent) => {
    if (event.type === 'connected') {
      setConnected(true)
      return
    }
    if (event.type === 'queue.stats') {
      setStats(event.data as QueueStats)
      return
    }
    if (event.type === 'worker.heartbeat' || event.type === 'worker.stopped') {
      fetchWorkers()
      return
    }
    const jobEvents = ['job.created', 'job.started', 'job.completed', 'job.failed', 'job.dead']
    if (jobEvents.includes(event.type)) {
      const updatedJob = event.data as Job
      setJobs((prev) => {
        const idx = prev.findIndex((j) => j.id === updatedJob.id)
        if (idx >= 0) {
          const next = [...prev]
          next[idx] = updatedJob
          return next
        }
        return [updatedJob, ...prev]
      })
    }
  }, [])

  useSSE(handleSSEEvent)

  async function handleRetry(id: string) {
    await fetch(`/api/jobs/${id}/retry`, { method: 'POST' })
  }

  async function handleCancel(id: string) {
    await fetch(`/api/jobs/${id}`, { method: 'DELETE' })
  }

  const deadCount = stats?.dead ?? 0
  const deadJobs = jobs.filter((j) => j.status === 'dead')

  const tabs: { id: Tab; label: React.ReactNode }[] = [
    { id: 'jobs', label: <>Jobs <span className="text-zinc-600 font-normal">({jobs.length})</span></> },
    { id: 'workers', label: <>Workers <span className="text-zinc-600 font-normal">({workers.length})</span></> },
    {
      id: 'dead',
      label: (
        <>
          Dead Letter
          {deadCount > 0 && (
            <span className="ml-1.5 px-1.5 py-0.5 rounded-full bg-red-900/60 text-red-400 text-xs font-medium">
              {deadCount}
            </span>
          )}
        </>
      ),
    },
  ]

  return (
    <div className="min-h-screen bg-surface-base flex flex-col">
      {/* header */}
      <header className="h-14 bg-surface-raised border-b border-surface-border flex items-center px-6 flex-shrink-0">
        <div className="flex items-center gap-3 flex-1">
          {/* logo mark */}
          <div className="w-8 h-8 rounded-lg bg-brand-900 border border-brand-800 flex items-center justify-center flex-shrink-0">
            <svg viewBox="0 0 20 20" fill="currentColor" className="w-4 h-4 text-brand-400">
              <path d="M2 3a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H3a1 1 0 01-1-1V3zM2 9a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H3a1 1 0 01-1-1V9zM3 15a1 1 0 000 2h14a1 1 0 000-2H3z" />
            </svg>
          </div>
          <div>
            <div className="text-white font-semibold text-sm leading-tight">TaskQueue</div>
            <div className="text-zinc-500 text-xs leading-tight">distributed job processing</div>
          </div>
        </div>

        {/* demo buttons */}
        <div className="flex items-center gap-2 mr-4">
          <button
            onClick={() => fetch('/api/demo/order', { method: 'POST' })}
            className="px-3 py-1.5 text-xs rounded-lg bg-brand-700 hover:bg-brand-600 text-white font-medium transition-colors"
          >
            Place Order
          </button>
          <button
            onClick={() => fetch('/api/demo/flood', { method: 'POST' })}
            className="px-3 py-1.5 text-xs rounded-lg border border-surface-muted hover:border-zinc-500 text-zinc-400 hover:text-zinc-200 font-medium transition-colors"
          >
            Flood (10)
          </button>
        </div>

        {/* live indicator */}
        <div className="flex items-center gap-2">
          {connected ? (
            <>
              <div className="relative flex items-center justify-center w-4 h-4">
                <span className="absolute inline-flex h-full w-full rounded-full bg-brand-400 opacity-30 motion-safe:animate-ping" />
                <span className="relative w-2 h-2 rounded-full bg-brand-400" />
              </div>
              <span className="text-brand-400 text-xs font-mono font-semibold">LIVE</span>
            </>
          ) : (
            <>
              <span className="w-2 h-2 rounded-full bg-amber-500" />
              <span className="text-amber-400 text-xs font-mono font-semibold">RECONNECTING</span>
            </>
          )}
        </div>
      </header>

      {/* main content */}
      <main className="flex-1 overflow-y-auto">
        <div className="max-w-7xl mx-auto px-6 py-6">
          {/* stats row */}
          <QueueStatsComponent stats={stats} />

          {/* tab nav */}
          <div className="flex gap-6 border-b border-surface-border mb-6">
            {tabs.map((t) => (
              <button
                key={t.id}
                onClick={() => setTab(t.id)}
                className={`pb-3 text-sm font-medium transition-colors border-b-2 -mb-px ${
                  tab === t.id
                    ? 'border-brand-500 text-brand-400'
                    : 'border-transparent text-zinc-500 hover:text-zinc-300'
                }`}
              >
                {t.label}
              </button>
            ))}
          </div>

          {/* content */}
          {tab === 'jobs' && (
            <JobList jobs={jobs} onRetry={handleRetry} onCancel={handleCancel} />
          )}
          {tab === 'workers' && (
            <WorkerStatus workers={workers} />
          )}
          {tab === 'dead' && (
            <JobList jobs={deadJobs} onRetry={handleRetry} onCancel={handleCancel} />
          )}
        </div>
      </main>
    </div>
  )
}
