package clickhouse

import (
	"database/sql"
	"fmt"
	"log-processor/internal/domain/entities"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type DBConfig struct {
	Address  string
	Port     int
	Login    string
	Password string
}
type LogDBClient interface {
	Init(config DBConfig) driver.Conn
	Close()
	InsertBatch(clickhouseLogs []entities.ClickHouseLogRecord) error
	Select() []entities.ClickHouseLogRecord
}
type Client struct {
	conn sql.DB
}

func New(config DBConfig) *Client {
	conn := clickhouse.OpenDB(&clickhouse.Options{Addr: []string{fmt.Sprintf("%s:%d", config.Address, config.Port)}, Auth: clickhouse.Auth{Username: config.Login, Password: config.Password}, Protocol: clickhouse.HTTP})

	var client = Client{}
	client.conn = *conn
	fmt.Println(client.conn.Ping())
	return &client
}

func (c Client) Close() {
	c.conn.Close()
}

func (c Client) InsertBatch(clickhouseLogs []entities.ClickHouseLogRecord) error {

}

func (c Client) Select() []entities.ClickHouseLogRecord {

	panic("implement me")
}
