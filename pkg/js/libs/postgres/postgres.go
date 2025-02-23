package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-pg/pg"
	_ "github.com/lib/pq"
	"github.com/praetorian-inc/fingerprintx/pkg/plugins"
	postgres "github.com/praetorian-inc/fingerprintx/pkg/plugins/services/postgresql"
	utils "github.com/projectdiscovery/nuclei/v3/pkg/js/utils"
	"github.com/projectdiscovery/nuclei/v3/pkg/protocols/common/protocolstate"
)

type (
	// PGClient is a client for Postgres database.
	// Internally client uses go-pg/pg driver.
	// @example
	// ```javascript
	// const postgres = require('nuclei/postgres');
	// const client = new postgres.Client();
	// ```
	PGClient struct{}
)

// IsPostgres checks if the given host and port are running Postgres database.
// If connection is successful, it returns true.
// If connection is unsuccessful, it returns false and error.
// @example
// ```javascript
// const postgres = require('nuclei/postgres');
// const isPostgres = postgres.IsPostgres('acme.com', 5432);
// ```
func (c *PGClient) IsPostgres(host string, port int) (bool, error) {
	timeout := 10 * time.Second

	conn, err := protocolstate.Dialer.Dial(context.TODO(), "tcp", fmt.Sprintf("%s:%d", host, port))
	if err != nil {
		return false, err
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	plugin := &postgres.POSTGRESPlugin{}
	service, err := plugin.Run(conn, timeout, plugins.Target{Host: host})
	if err != nil {
		return false, err
	}
	if service == nil {
		return false, nil
	}
	return true, nil
}

// Connect connects to Postgres database using given credentials.
// If connection is successful, it returns true.
// If connection is unsuccessful, it returns false and error.
// The connection is closed after the function returns.
// @example
// ```javascript
// const postgres = require('nuclei/postgres');
// const client = new postgres.Client();
// const connected = client.Connect('acme.com', 5432, 'username', 'password');
// ```
func (c *PGClient) Connect(host string, port int, username, password string) (bool, error) {
	return connect(host, port, username, password, "postgres")
}

// ExecuteQuery connects to Postgres database using given credentials and database name.
// and executes a query on the db.
// If connection is successful, it returns the result of the query.
// @example
// ```javascript
// const postgres = require('nuclei/postgres');
// const client = new postgres.Client();
// const result = client.ExecuteQuery('acme.com', 5432, 'username', 'password', 'dbname', 'select * from users');
// log(to_json(result));
// ```
func (c *PGClient) ExecuteQuery(host string, port int, username, password, dbName, query string) (*utils.SQLResult, error) {
	if !protocolstate.IsHostAllowed(host) {
		// host is not valid according to network policy
		return nil, protocolstate.ErrHostDenied.Msgf(host)
	}

	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	connStr := fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", username, password, target, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	resp, err := utils.UnmarshalSQLRows(rows)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ConnectWithDB connects to Postgres database using given credentials and database name.
// If connection is successful, it returns true.
// If connection is unsuccessful, it returns false and error.
// The connection is closed after the function returns.
// @example
// ```javascript
// const postgres = require('nuclei/postgres');
// const client = new postgres.Client();
// const connected = client.ConnectWithDB('acme.com', 5432, 'username', 'password', 'dbname');
// ```
func (c *PGClient) ConnectWithDB(host string, port int, username, password, dbName string) (bool, error) {
	return connect(host, port, username, password, dbName)
}

func connect(host string, port int, username, password, dbName string) (bool, error) {
	if host == "" || port <= 0 {
		return false, fmt.Errorf("invalid host or port")
	}

	if !protocolstate.IsHostAllowed(host) {
		// host is not valid according to network policy
		return false, protocolstate.ErrHostDenied.Msgf(host)
	}

	target := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	db := pg.Connect(&pg.Options{
		Addr:     target,
		User:     username,
		Password: password,
		Database: dbName,
	})
	_, err := db.Exec("select 1")
	if err != nil {
		switch true {
		case strings.Contains(err.Error(), "connect: connection refused"):
			fallthrough
		case strings.Contains(err.Error(), "no pg_hba.conf entry for host"):
			fallthrough
		case strings.Contains(err.Error(), "network unreachable"):
			fallthrough
		case strings.Contains(err.Error(), "reset"):
			fallthrough
		case strings.Contains(err.Error(), "i/o timeout"):
			return false, err
		}
		return false, nil
	}
	return true, nil
}
