import { useEffect, useRef } from 'react'
import { SSEEvent } from '../types'

type Handler = (event: SSEEvent) => void

// useSSE connects to the /events SSE endpoint and calls the handler for each event
// automatically reconnects if the connection drops
export function useSSE(handler: Handler) {
  const handlerRef = useRef(handler)
  handlerRef.current = handler

  useEffect(() => {
    let es: EventSource
    let retryTimeout: ReturnType<typeof setTimeout>

    function connect() {
      es = new EventSource('/events')

      es.onmessage = (e) => {
        try {
          const event: SSEEvent = JSON.parse(e.data)
          handlerRef.current(event)
        } catch {
          // ignore malformed events
        }
      }

      es.onerror = () => {
        es.close()
        // retry in 3s
        retryTimeout = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      clearTimeout(retryTimeout)
      es?.close()
    }
  }, [])
}
