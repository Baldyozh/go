package entities

import "time"

type Log struct {
	TimeStamp time.Time
	LogBody   string
}
