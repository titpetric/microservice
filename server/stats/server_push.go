package stats

import (
	"context"
	"errors"
	"time"

	"github.com/titpetric/microservice/internal"
	"github.com/titpetric/microservice/rpc/stats"
)

// Keep returning a single object to avoid allocations
var pushResponseDefault = new(stats.PushResponse)

// Push a record to the incoming log table
func (svc *Server) Push(ctx context.Context, r *stats.PushRequest) (*stats.PushResponse, error) {
	ctx = internal.ContextWithoutCancel(ctx)

	validate := func() error {
		if r.Property == "" {
			return errors.New("missing property")
		}
		if r.Property != "news" {
			return errors.New("invalid property")
		}
		if r.Id < 1 {
			return errors.New("missing id")
		}
		if r.Section < 1 {
			return errors.New("missing section")
		}
		return nil
	}
	if err := validate(); err != nil {
		return nil, err
	}

	var err error
	row := new(Incoming)

	row.ID, err = svc.sonyflake.NextID()
	if err != nil {
		return nil, err
	}

	row.Property = r.Property
	row.PropertySection = r.Section
	row.PropertyID = r.Id
	row.RemoteIP = internal.GetIPFromContext(ctx)
	row.SetStamp(time.Now())

	return pushResponseDefault, svc.flusher.Push(row)
}
