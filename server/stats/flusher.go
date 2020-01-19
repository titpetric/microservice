package stats

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/atomic"
)

// Flusher is a context-driven background data flush job
type Flusher struct {
	context.Context
	finish func()

	enabled *atomic.Bool

	// queueIndex is a key for []queues
	queueIndex *atomic.Uint32
	// queueMask is a masking value for queueIndex -> key
	queueMask uint32
	// queues hold a set of writable queues
	queues []*Queue

	db *sqlx.DB
}

// NewFlusher creates a *Flusher
func NewFlusher(ctx context.Context, db *sqlx.DB) (*Flusher, error) {
	queueSize := 1 << 4
	job := &Flusher{
		db:         db,
		enabled:    atomic.NewBool(true),
		queueIndex: atomic.NewUint32(0),
		queueMask:  uint32(queueSize - 1),
		queues:     NewQueues(queueSize),
	}
	job.Context, job.finish = context.WithCancel(context.Background())
	go job.run(ctx)
	return job, nil
}

// Push spreads queue writes evenly across all queues
func (job *Flusher) Push(item *Incoming) error {
	if job.enabled.Load() {
		index := job.queueIndex.Inc() & job.queueMask
		return job.queues[index].Push(item)
	}
	return errFlusherDisabled
}

func (job *Flusher) run(ctx context.Context) {
	log.Println("Started background job")

	defer job.finish()

	ticker := time.NewTicker(5 * time.Second)

	for {
		select {
		case <-ticker.C:
			job.flush()
			continue
		case <-ctx.Done():
			log.Println("Got cancel")
			job.enabled.Store(false)
			job.flush()
		}
		break
	}

	log.Println("Exiting Run")
}

func (job *Flusher) flush() {
	var err error

	fields := strings.Join(IncomingFields, ",")
	named := ":" + strings.Join(IncomingFields, ",:")
	query := fmt.Sprintf("insert into %s (%s) values (%s)", IncomingTable, fields, named)

	var batchInsertSize int
	log.Println("[flush] begin")
	for k, queue := range job.queues {
		rows := queue.Clear()

		log.Println("[flush] queue", k, "rows", len(rows))

		for len(rows) > 0 {
			batchInsertSize = 1000
			if len(rows) < batchInsertSize {
				batchInsertSize = len(rows)
			}
			if _, err = job.db.NamedExec(query, rows[:batchInsertSize]); err != nil {
				log.Println("Error when flushing data:", err)
			}
			rows = rows[batchInsertSize:]
		}
	}
	log.Println("[flush] done")
}
