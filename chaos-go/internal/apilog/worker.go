package apilog

import (
	"log/slog"
	"time"
)

// logCh 异步落库缓冲通道（采集端 middleware → 持久化端 flushLoop）。
var logCh = make(chan *ApiLog, 2000)

// Start 启动后台消费者，由启动流程显式调用（不在 init 里起 goroutine）。
func Start() {
	go flushLoop()
}

// flushLoop 后台消费者：累计 100 条或每 2s 批量落库一次。
func flushLoop() {
	const batchSize = 100
	batch := make([]*ApiLog, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := persistBatch(batch); err != nil {
			slog.Error("api 日志批量落库失败", "err", err)
		}
		batch = batch[:0]
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case rec := <-logCh:
			batch = append(batch, rec)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
