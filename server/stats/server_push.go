package stats

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/titpetric/microservice/rpc/stats"
)

// Push a record to the incoming log table
func (svc *Server) Push(ctx context.Context, r *stats.PushRequest) (*stats.PushResponse, error) {
	var err error
	row := Incoming{}

	row.ID, err = svc.sonyflake.NextID()
	if err != nil {
		return nil, err
	}

	row.Property = r.Property
	row.PropertySection = r.Section
	row.PropertyID = r.Id
	if remoteIP, ok := ctx.Value("ip.address").(string); ok {
		row.RemoteIP = remoteIP
	}
	row.SetStamp(time.Now())

	fields := strings.Join(IncomingFields, ",")
	named := ":" + strings.Join(IncomingFields, ",:")

	query := fmt.Sprintf("insert into %s (%s) values (%s)", IncomingTable, fields, named)
	_, err = svc.db.NamedExecContext(ctx, query, row)
	return nil, err
}
