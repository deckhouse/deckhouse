package context

import "database/sql"

// FIXME (e.shevchenko) Exec(  ctx context.Context, query string, args ...interface{}) (*sql.Rows,  error)
// FIXME (e.shevchenko) Query( ctx context.Context, query string, args ...interface{}) (sql.Result, error)

type StmtRunner interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}
