package stats

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/atomic"
)

// Flusher is a context-driven background data flush job
type Flusher struct {
	context.Context
	finish func()

	enabled *atomic.Bool

	db *sqlx.DB
}

// NewFlusher creates a *Flusher
func NewFlusher(ctx context.Context, db *sqlx.DB) (*Flusher, error) {
	job := &Flusher{
		db:      db,
		enabled: atomic.NewBool(true),
	}
	job.Context, job.finish = context.WithCancel(context.Background())
	go job.run(ctx)
	return job, nil
}

func (job *Flusher) run(ctx context.Context) {
	log.Println("Started background job")

	defer job.finish()

	ticker := time.NewTicker(time.Second)

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
	log.Println("Background flush")
}
