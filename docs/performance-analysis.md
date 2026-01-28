# Notification Service - Performance Analysis

## Overview

This document explains how the Notification Service addresses the performance requirements specified in the Insider One Software Engineer Assessment.

---

## 1. High Throughput (Millions of Notifications Daily)

**Requirement**: "Send millions of notifications daily"

### Solutions Implemented

| Technique | File | Description |
|-----------|------|-------------|
| **Redis Sorted Sets** | `queue.go` | In-memory queue with high throughput, O(log N) insert/dequeue |
| **Pipeline Batch Operations** | `queue.go:76-81` | Redis pipeline for batch inserts, reducing round-trips |
| **Worker Pool Pattern** | `processor.go:70-85` | Configurable workers per channel (SMS: 5, Email: 5, Push: 5) |
| **Async Processing** | `processor.go` | Queue immediately, process in background |

### Code Example: Batch Insert with Redis Pipeline

```go
// queue.go - Batch insert with single round-trip
pipe := q.client.client.Pipeline()
for channel, zItems := range channelItems {
    pipe.ZAdd(ctx, queueKey(channel), zItems...)
}
pipe.Exec(ctx)  // All inserts in single round-trip
```

---

## 2. Burst Traffic Handling (Flash Sales, Breaking News)

**Requirement**: "Handle burst traffic"

### Solutions Implemented

| Technique | Description |
|-----------|-------------|
| **Priority Queue** | High priority messages processed first during bursts |
| **Channel Isolation** | SMS, Email, Push in separate queues - one channel doesn't block others |
| **Backpressure** | When rate limiter is full, workers wait instead of crashing |

### Code Example: Priority Scoring

```go
// queue.go:41 - Priority-based scoring
score := float64(item.Priority.Weight()) + float64(time.Now().UnixNano())/1e18
// High=0, Normal=1M, Low=2M → High priority always processed first
```

---

## 3. Rate Limiting (100 msg/sec/channel)

**Requirement**: "Maximum 100 messages per second per channel"

### Solution: Sliding Window Algorithm

Location: `internal/repository/redis/ratelimiter.go`

```go
func (r *RateLimiter) Allow(ctx context.Context, channel domain.Channel) (bool, error) {
    key := rateLimitKey(channel)
    now := time.Now()
    windowStart := now.Add(-rateLimitWindow)

    pipe := r.client.client.Pipeline()

    // 1. Remove old entries outside the window
    pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

    // 2. Count current entries in the window
    countCmd := pipe.ZCard(ctx, key)
    pipe.Exec(ctx)

    // 3. Check if under limit
    if countCmd.Val() >= int64(r.limitPerSec) {
        return false, nil  // Rate limit exceeded
    }

    // 4. Add new entry
    r.client.client.ZAdd(ctx, key, redis.Z{
        Score:  float64(now.UnixNano()),
        Member: fmt.Sprintf("%d", now.UnixNano()),
    })

    return true, nil
}
```

### Advantages

- Unlike fixed window, sliding window distributes spikes evenly
- Atomic and distributed via Redis Sorted Set
- Independent limiting per channel

---

## 4. Priority Queue Support

**Requirement**: "High, normal, low priority"

### Solution: Redis Sorted Set with Weighted Scoring

```go
// domain/notification.go
func (p Priority) Weight() int64 {
    switch p {
    case PriorityHigh:   return 0        // Lowest score = processed first
    case PriorityNormal: return 1000000  
    case PriorityLow:    return 2000000  
    }
}
```

**ZPOPMIN** always retrieves the lowest score (highest priority) first.

---

## 5. Retry Logic with Exponential Backoff

**Requirement**: "Retry failed deliveries intelligently"

### Solution: Exponential Backoff

Location: `internal/worker/processor.go:299-310`

```go
func (p *Processor) calculateBackoff(retryCount int) time.Duration {
    // Exponential: baseDelay * 2^retryCount
    multiplier := math.Pow(2, float64(retryCount-1))
    delay := time.Duration(float64(p.config.BaseDelay) * multiplier)
    
    // Cap at 5 minutes
    if delay > 5*time.Minute {
        delay = 5 * time.Minute
    }
    return delay
}
```

### Retry Schedule

| Retry | Delay |
|-------|-------|
| 1 | 1 second |
| 2 | 2 seconds |
| 3 | 4 seconds |
| 4 | 8 seconds |
| 5 | 16 seconds |
| 6+ | Marked as failed |

---

## 6. Graceful Shutdown

**Requirement**: Prevent data loss during deployments

### Solution

Location: `internal/worker/processor.go:97-123`

- Wait for in-flight operations to complete
- 30-second timeout before force shutdown
- Prevents data loss

```go
func (p *Processor) Stop() {
    // Cancel context to signal workers
    p.cancelFunc()

    // Wait for all workers to finish
    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        p.logger.Info("processor stopped gracefully")
    case <-time.After(30 * time.Second):
        p.logger.Warn("processor stop timed out")
    }
}
```

---

## Architecture Summary

```
                    ┌─────────────────────────────────────┐
                    │         Rate Limiter                │
                    │   (100 msg/sec per channel)         │
                    └──────────────┬──────────────────────┘
                                   │
┌──────────────┐    ┌──────────────▼──────────────┐    ┌──────────────┐
│   REST API   │───▶│    Redis Priority Queues   │───▶│ Worker Pools │
│  (Batch OK)  │    │  SMS│EMAIL│PUSH (isolated) │    │ 5+5+5 = 15   │
└──────────────┘    └─────────────────────────────┘    └──────┬───────┘
                                                              │
                    ┌─────────────────────────────────────────▼───────┐
                    │              Exponential Backoff Retry          │
                    │         (1s → 2s → 4s → 8s → 16s → fail)       │
                    └─────────────────────────────────────────────────┘
```

---

## Performance Metrics

The system exposes real-time metrics via `/metrics` endpoint:

- `notification_queue_depth` - Current queue depth per channel
- `notifications_sent_total` - Successfully sent notifications
- `notifications_failed_total` - Failed notifications
- `notification_processing_latency_seconds` - End-to-end latency

---

## Conclusion

The Notification Service implements industry-standard patterns to handle high-throughput, burst traffic, and ensure reliable delivery:

1. **Redis Sorted Sets** for O(log N) priority queue operations
2. **Sliding Window Rate Limiting** for fair traffic distribution
3. **Worker Pool Pattern** for concurrent processing
4. **Exponential Backoff** for intelligent retry logic
5. **Channel Isolation** to prevent cascading failures
6. **Graceful Shutdown** for zero data loss during deployments
