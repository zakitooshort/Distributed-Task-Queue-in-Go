# distributed task queue in go

a job queue system built from scratch. you push tasks into it, workers pick them up and run them, failed jobs retry automatically, and a live dashboard shows everything happening in real time.

built this to understand how systems like Sidekiq or BullMQ work under the hood redis for fast queue delivery, postgres as the source of truth, and server-sent events so the dashboard updates without polling.

---

## what it does

- **enqueue jobs** from any service via a simple HTTP API
- **workers** pull jobs off the queue concurrently and process them
- **automatic retries** with exponential backoff, if a job fails it goes back in the queue, and after 3 failures it lands in the dead letter queue
- **scheduled jobs** : delay a job until a future timestamp using a redis sorted set
- **dead letter queue** : failed jobs sit here so you can inspect and replay them
- **live dashboard** : real-time view of queues, workers, and jobs using SSE (no websockets, no polling)
- **e-commerce demo** : built-in order simulator so you can flood the queue with realistic jobs without writing any curl commands

---

## stack

- go (gin, gorm)
- postgres : job records, worker state, history
- redis : queue delivery, delayed job scheduling, dead letter
- react + typescript + tailwind : dashboard
- docker compose : runs everything together

---

## architecture

two separate binaries:

**server** (`cmd/server`) : handles the HTTP API, serves the dashboard, runs the scheduler, and pushes SSE events to connected browsers

**worker** (`cmd/worker`) : pulls jobs from redis, executes them, updates postgres, handles retries

they share postgres and redis but nothing else. you can run multiple workers if you need more throughput, the queue handles it fine.

job lifecycle:

```
pending -> running -> completed
                   -> failed (retried up to max_attempts)
                            -> dead (max retries hit)
```

---

## running it

you need docker and docker compose, that's it.

```bash
git clone https://github.com/ouzai/distributed-task-queue-in-go
cd distributed-task-queue-in-go
docker compose up --build
```

server starts on `http://localhost:8080`

if you want the dashboard with hot reload during development:

```bash
# in one terminal
docker compose up postgres redis server

# in another
cd dashboard && npm install && npm run dev
```

dashboard opens at `http://localhost:5173`

---

## demo

open the dashboard and hit **Place Order** in the top right. it generates a fake e-commerce order and fans out 4 background jobs across different queues:

| job | queue | behavior |
|---|---|---|
| process_payment | critical | 10s, fails 30% of the time |
| send_confirmation_email | email | 10s, always succeeds |
| update_inventory | default | 10s, always succeeds |
| generate_invoice | default | 10s, always succeeds |

the 30% payment failure rate naturally fills the retry flow and dead letter queue so you can see the whole lifecycle without engineering it.

hit **Flood (10)** to send 10 orders at once and watch the queues light up.

---

## API

```
POST   /api/jobs              enqueue a job
GET    /api/jobs              list jobs (filter by status, queue, type)
GET    /api/jobs/:id          get a single job
POST   /api/jobs/:id/retry    replay a dead job
DELETE /api/jobs/:id          cancel a pending job
GET    /api/queues            queue depths + throughput stats
GET    /api/workers           active workers
POST   /api/demo/order        place one fake order (4 jobs)
POST   /api/demo/flood        flood with N orders (default 10, max 50)
GET    /events                SSE stream for the dashboard
```

example : enqueue a custom job:

```bash
curl -X POST http://localhost:8080/api/jobs \
  -H "Content-Type: application/json" \
  -d '{"queue":"default","type":"my_job","payload":{"key":"value"}}'
```

---

## project structure

```
cmd/
  server/    http server + scheduler
  worker/    job processor

internal/
  api/       gin handlers, SSE broadcaster
  queue/     enqueue, dequeue, retry logic
  scheduler/ polls delayed jobs and re-queues them
  store/     postgres (gorm) and redis adapters
  worker/    worker loop, handler registry

dashboard/   react frontend
```

---

## scaling workers

the worker is stateless so you can run as many as you want:

```bash
docker compose up --scale worker=4
```

each worker registers itself in postgres and sends heartbeats every 10 seconds. the dashboard shows all of them live.
