package stats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/titpetric/microservice/internal"
	"github.com/titpetric/microservice/rpc/stats"
)

// Push a record to the incoming log table
func (svc *Server) Push(ctx context.Context, r *stats.PushRequest) (*stats.PushResponse, error) {
	ctx = internal.ContextWithoutCancel(ctx)

	validate := func() error {
		if r.Property == "" {
			return errors.New("Missing Property")
		}
		if r.Property != "news" {
			return errors.New("Invalid Property")
		}
		if r.Id < 1 {
			return errors.New("Missing ID")
		}
		if r.Section < 1 {
			return errors.New("Missing Section")
		}
		return nil
	}
	if err := validate(); err != nil {
		return nil, err
	}

	var err error
	row := Incoming{}

	row.ID, err = svc.sonyflake.NextID()
	if err != nil {
		return nil, err
	}

	row.Property = r.Property
	row.PropertySection = r.Section
	row.PropertyID = r.Id
	row.RemoteIP = internal.GetIPFromContext(ctx)
	row.SetStamp(time.Now())

	fields := strings.Join(IncomingFields, ",")
	named := ":" + strings.Join(IncomingFields, ",:")

	query := fmt.Sprintf("insert into %s (%s) values (%s)", IncomingTable, fields, named)
	_, err = svc.db.NamedExecContext(ctx, query, row)
	return new(stats.PushResponse), err
}
