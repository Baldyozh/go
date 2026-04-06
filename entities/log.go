package entities

import "time"

type Log struct {
	TimeStamp time.Time
	PartnerId string
	LogBody   string
}
