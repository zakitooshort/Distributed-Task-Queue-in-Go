export type JobStatus = 'pending' | 'running' | 'completed' | 'failed' | 'dead'

export interface Job {
  id: string
  queue: string
  type: string
  payload: unknown
  status: JobStatus
  priority: number
  attempts: number
  max_attempts: number
  scheduled_at?: string
  started_at?: string
  completed_at?: string
  failed_at?: string
  error?: string
  created_at: string
  worker_id?: string
}

export interface Worker {
  id: string
  status: string
  queues: string
  current_job?: string
  last_seen: string
  started_at: string
}

export interface QueueStat {
  queue: string
  depth: number
}

export interface QueueStats {
  queues: QueueStat[]
  delayed: number
  dead: number
  throughput: number
  timestamp?: string
}

export interface SSEEvent {
  type: string
  data: unknown
}
