package mmplugins

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// SQLDatabaseFactory must return an isolated connection for this plugin and
// runtime scope. It must never return the TMS account/mail database. The bridge
// owns the returned pools and releases handles when a plugin is deactivated.
type SQLDatabaseFactory func(pluginID string) (*sql.DB, error)

type sqlConnection struct {
	owner    string
	conn     *sql.Conn
	tx       *sql.Tx
	txID     string
	txCancel context.CancelFunc
}
type sqlStatement struct {
	connID    string
	statement *sql.Stmt
}
type sqlRows struct {
	connID string
	rows   *sql.Rows
	cancel context.CancelFunc
}
type sqlBridge struct {
	mu           sync.Mutex
	factory      SQLDatabaseFactory
	pools        map[string]*sql.DB
	conns        map[string]*sqlConnection
	statements   map[string]sqlStatement
	rows         map[string]sqlRows
	transactions map[string]string
	closed       bool
}

var _ plugin.AppDriver = (*sqlBridge)(nil)
var errSQLHandle = errors.New("invalid or closed plugin SQL handle")

func newSQLBridge(factory SQLDatabaseFactory) *sqlBridge {
	return &sqlBridge{factory: factory, pools: map[string]*sql.DB{}, conns: map[string]*sqlConnection{}, statements: map[string]sqlStatement{}, rows: map[string]sqlRows{}, transactions: map[string]string{}}
}
func sqlTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
func sqlArguments(args []driver.NamedValue) []any {
	out := make([]any, len(args))
	for i, arg := range args {
		out[i] = arg.Value
	}
	return out
}
func sqlResult(result sql.Result, err error) (plugin.ResultContainer, error) {
	if err != nil {
		return plugin.ResultContainer{}, err
	}
	last, le := result.LastInsertId()
	count, ce := result.RowsAffected()
	return plugin.ResultContainer{LastID: last, LastIDError: le, RowsAffected: count, RowsAffectedError: ce}, nil
}

// The SDK binds ConnWithPluginID to the calling plugin. Bare Conn never grants
// access; all subsequent handles are unguessable capabilities from that bind.
func (b *sqlBridge) Conn(bool) (string, error) { return "", errSQLHandle }
func (b *sqlBridge) ConnWithPluginID(_ bool, id string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed || !pluginID.MatchString(id) {
		return "", errSQLHandle
	}
	count := 0
	for _, conn := range b.conns {
		if conn.owner == id {
			count++
		}
	}
	if count >= 32 {
		return "", fmt.Errorf("plugin SQL connection limit reached")
	}
	db := b.pools[id]
	if db == nil {
		var err error
		db, err = b.factory(id)
		if err != nil {
			return "", err
		}
		if db == nil {
			return "", errors.New("plugin SQL database unavailable")
		}
		b.pools[id] = db
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		return "", err
	}
	key := uuid.NewString()
	b.conns[key] = &sqlConnection{owner: id, conn: conn}
	return key, nil
}
func (b *sqlBridge) ConnPing(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	conn := b.conns[id]
	if conn == nil {
		return errSQLHandle
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	return conn.conn.PingContext(ctx)
}
func (b *sqlBridge) closeConn(id string) error {
	conn := b.conns[id]
	if conn == nil {
		return errSQLHandle
	}
	for key, row := range b.rows {
		if row.connID == id {
			_ = row.rows.Close()
			row.cancel()
			delete(b.rows, key)
		}
	}
	for key, stmt := range b.statements {
		if stmt.connID == id {
			_ = stmt.statement.Close()
			delete(b.statements, key)
		}
	}
	if conn.tx != nil {
		_ = conn.tx.Rollback()
		conn.txCancel()
		delete(b.transactions, conn.txID)
	}
	delete(b.conns, id)
	return conn.conn.Close()
}
func (b *sqlBridge) ConnClose(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closeConn(id)
}
func (b *sqlBridge) ShutdownConns(owner string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, c := range b.conns {
		if c.owner == owner {
			_ = b.closeConn(id)
		}
	}
}
func (b *sqlBridge) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for id := range b.conns {
		_ = b.closeConn(id)
	}
	for _, db := range b.pools {
		_ = db.Close()
	}
}
func (b *sqlBridge) addRows(connID string, rows *sql.Rows, cancel context.CancelFunc, err error) (string, error) {
	if err != nil {
		cancel()
		return "", err
	}
	if len(b.rows) >= 256 {
		_ = rows.Close()
		cancel()
		return "", errors.New("plugin SQL rows limit reached")
	}
	id := uuid.NewString()
	b.rows[id] = sqlRows{connID, rows, cancel}
	return id, nil
}
func (b *sqlBridge) ConnQuery(id, q string, args []driver.NamedValue) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.conns[id]
	if c == nil {
		return "", errSQLHandle
	}
	// Keep the deadline alive across RPC row reads; close releases it.
	ctx, cancel := sqlTimeout()
	if c.tx != nil {
		rows, e := c.tx.QueryContext(ctx, q, sqlArguments(args)...)
		return b.addRows(id, rows, cancel, e)
	}
	rows, e := c.conn.QueryContext(ctx, q, sqlArguments(args)...)
	return b.addRows(id, rows, cancel, e)
}
func (b *sqlBridge) ConnExec(id, q string, args []driver.NamedValue) (plugin.ResultContainer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.conns[id]
	if c == nil {
		return plugin.ResultContainer{}, errSQLHandle
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	if c.tx != nil {
		return sqlResult(c.tx.ExecContext(ctx, q, sqlArguments(args)...))
	}
	return sqlResult(c.conn.ExecContext(ctx, q, sqlArguments(args)...))
}
func (b *sqlBridge) Tx(id string, opts driver.TxOptions) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.conns[id]
	if c == nil || c.tx != nil {
		return "", errSQLHandle
	}
	ctx, cancel := sqlTimeout()
	tx, e := c.conn.BeginTx(ctx, &sql.TxOptions{Isolation: sql.IsolationLevel(opts.Isolation), ReadOnly: opts.ReadOnly})
	if e != nil {
		cancel()
		return "", e
	}
	key := uuid.NewString()
	c.tx = tx
	c.txCancel = cancel
	c.txID = key
	b.transactions[key] = id
	return key, nil
}
func (b *sqlBridge) finishTx(id string, commit bool) error {
	connID, ok := b.transactions[id]
	if !ok {
		return errSQLHandle
	}
	c := b.conns[connID]
	if c == nil || c.tx == nil {
		return errSQLHandle
	}
	var e error
	if commit {
		e = c.tx.Commit()
	} else {
		e = c.tx.Rollback()
	}
	c.txCancel()
	c.txCancel = nil
	c.tx = nil
	c.txID = ""
	delete(b.transactions, id)
	return e
}
func (b *sqlBridge) TxCommit(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finishTx(id, true)
}
func (b *sqlBridge) TxRollback(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.finishTx(id, false)
}
func (b *sqlBridge) Stmt(connID, q string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.conns[connID]
	if c == nil {
		return "", errSQLHandle
	}
	if len(b.statements) >= 256 {
		return "", errors.New("plugin SQL statement limit reached")
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	var stmt *sql.Stmt
	var err error
	if c.tx != nil {
		stmt, err = c.tx.PrepareContext(ctx, q)
	} else {
		stmt, err = c.conn.PrepareContext(ctx, q)
	}
	if err != nil {
		return "", err
	}
	key := uuid.NewString()
	b.statements[key] = sqlStatement{connID, stmt}
	return key, nil
}
func (b *sqlBridge) StmtClose(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.statements[id]
	if !ok {
		return errSQLHandle
	}
	delete(b.statements, id)
	return s.statement.Close()
}
func (b *sqlBridge) StmtNumInput(string) int { return -1 }
func (b *sqlBridge) StmtQuery(id string, args []driver.NamedValue) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.statements[id]
	if !ok {
		return "", errSQLHandle
	}
	ctx, cancel := sqlTimeout()
	rows, e := s.statement.QueryContext(ctx, sqlArguments(args)...)
	return b.addRows(s.connID, rows, cancel, e)
}
func (b *sqlBridge) StmtExec(id string, args []driver.NamedValue) (plugin.ResultContainer, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.statements[id]
	if !ok {
		return plugin.ResultContainer{}, errSQLHandle
	}
	ctx, cancel := sqlTimeout()
	defer cancel()
	return sqlResult(s.statement.ExecContext(ctx, sqlArguments(args)...))
}
func (b *sqlBridge) RowsColumns(id string) []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return nil
	}
	cols, _ := row.rows.Columns()
	return cols
}
func (b *sqlBridge) RowsClose(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return errSQLHandle
	}
	delete(b.rows, id)
	err := row.rows.Close()
	row.cancel()
	return err
}
func (b *sqlBridge) RowsNext(id string, dest []driver.Value) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return errSQLHandle
	}
	if !row.rows.Next() {
		if e := row.rows.Err(); e != nil {
			return e
		}
		return io.EOF
	}
	values := make([]any, len(dest))
	for i := range dest {
		values[i] = &dest[i]
	}
	return row.rows.Scan(values...)
}
func (b *sqlBridge) RowsHasNextResultSet(string) bool { return false }
func (b *sqlBridge) RowsNextResultSet(string) error   { return io.EOF }
func (b *sqlBridge) RowsColumnTypeDatabaseTypeName(id string, index int) string {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return ""
	}
	types, e := row.rows.ColumnTypes()
	if e != nil || index < 0 || index >= len(types) {
		return ""
	}
	return types[index].DatabaseTypeName()
}
func (b *sqlBridge) RowsColumnTypePrecisionScale(id string, index int) (int64, int64, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	row, ok := b.rows[id]
	if !ok {
		return 0, 0, false
	}
	types, e := row.rows.ColumnTypes()
	if e != nil || index < 0 || index >= len(types) {
		return 0, 0, false
	}
	return types[index].DecimalSize()
}
