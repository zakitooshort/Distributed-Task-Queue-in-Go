import { useEffect, useState, useCallback } from 'react'
import { Job, Worker, QueueStats, SSEEvent } from './types'
import { QueueStats as QueueStatsComponent } from './components/QueueStats'
import { JobList } from './components/JobList'
import { WorkerStatus } from './components/WorkerStatus'
import { useSSE } from './hooks/useSSE'

export default function App() {
  const [jobs, setJobs] = useState<Job[]>([])
  const [workers, setWorkers] = useState<Worker[]>([])
  const [stats, setStats] = useState<QueueStats | null>(null)
  const [connected, setConnected] = useState(false)
  const [tab, setTab] = useState<'jobs' | 'workers'>('jobs')

  // load initial data from the API
  useEffect(() => {
    fetchJobs()
    fetchWorkers()
    fetchStats()
  }, [])

  async function fetchJobs() {
    try {
      const res = await fetch('/api/jobs?limit=100')
      const data = await res.json()
      setJobs(data.jobs ?? [])
    } catch {
      // server might not be up yet
    }
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

  // handle incoming SSE events
  const handleSSEEvent = useCallback((event: SSEEvent) => {
    if (event.type === 'connected') {
      setConnected(true)
      return
    }

    if (event.type === 'queue.stats') {
      setStats(event.data as QueueStats)
      return
    }

    if (event.type === 'worker.heartbeat') {
      // refresh the full worker list — simpler than patching in place
      fetchWorkers()
      return
    }

    if (event.type === 'worker.stopped') {
      fetchWorkers()
      return
    }

    // all job.* events — update or insert the job in our local state
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
        // new job — prepend it
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

  return (
    <div className="min-h-screen bg-slate-900 p-4">
      <div className="max-w-6xl mx-auto">
        {/* header */}
        <div className="flex items-center justify-between mb-6">
          <div>
            <h1 className="text-2xl font-bold text-white">task queue</h1>
            <p className="text-slate-400 text-sm">distributed job processing</p>
          </div>
          <div className="flex items-center gap-2">
            <div className={`w-2 h-2 rounded-full ${connected ? 'bg-green-400' : 'bg-red-400'}`} />
            <span className="text-slate-400 text-sm">{connected ? 'live' : 'connecting...'}</span>
          </div>
        </div>

        {/* stats */}
        <QueueStatsComponent stats={stats} />

        {/* tabs */}
        <div className="flex gap-1 mb-4 bg-slate-800 rounded-lg p-1 w-fit">
          {(['jobs', 'workers'] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-1.5 rounded text-sm font-medium transition-colors ${
                tab === t
                  ? 'bg-slate-600 text-white'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              {t}
              {t === 'jobs' && ` (${jobs.length})`}
              {t === 'workers' && ` (${workers.length})`}
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
      </div>
    </div>
  )
}
