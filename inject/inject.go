package inject

import (
	"os"
	"strconv"

	"github.com/google/wire"
	"github.com/sony/sonyflake"

	"github.com/titpetric/microservice/client"
	"github.com/titpetric/microservice/db"
)

// Sonyflake produces a sonyflake ID generator dependency
func Sonyflake() *sonyflake.Sonyflake {
	var serverID uint16
	if val, err := strconv.ParseInt(os.Getenv("SERVER_ID"), 10, 16); err == nil {
		serverID = uint16(val)
	}
	if serverID > 0 {
		return sonyflake.NewSonyflake(sonyflake.Settings{
			MachineID: func() (uint16, error) {
				return serverID, nil
			},
		})
	}
	return sonyflake.NewSonyflake(sonyflake.Settings{})
}

// Inject is the main ProviderSet for wire
var Inject = wire.NewSet(
	db.Connect,
	Sonyflake,
	client.Inject,
)
