package db

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// ConnectWithRetry uses retry options set in ConnectionOptions{}
func ConnectWithRetry(ctx context.Context, options ConnectionOptions) (db *sqlx.DB, err error) {
	dsn := maskDSN(options.Credentials.DSN)

	connErrCh := make(chan error, 1)
	defer close(connErrCh)

	log.Println("connecting to database", dsn)

	go func() {
		try := 0
		for {
			try++
			if options.Retries <= try {
				err = errors.Errorf("could not connect, dsn=%s, tries=%d", dsn, try)
				break
			}

			db, err = ConnectWithOptions(ctx, options)
			if err != nil {
				log.Printf("can't connect, dsn=%s, err=%s, try=%d", dsn, err, try)

				select {
				case <-ctx.Done():
					break
				case <-time.After(options.RetryDelay):
					continue
				}
			}
			break
		}
		connErrCh <- err
	}()

	select {
	case err = <-connErrCh:
		break
	case <-time.After(options.ConnectTimeout):
		return nil, errors.Errorf("db connect timed out, dsn=%s", dsn)
	case <-ctx.Done():
		return nil, errors.Errorf("db connection cancelled, dsn=%s", dsn)
	}

	return
}
