package db

import (

	// sqlite3 driver
	_ "github.com/mattn/go-sqlite3"

	dbcontext "d8.io/upmeter/pkg/db/context"
)

func Connect(path string, opts map[string]string) (*dbcontext.DbContext, error) {
	dbCtx := dbcontext.NewDbContext()
	err := dbCtx.ConnectWithPool(path, opts)
	if err != nil {
		return nil, err
	}
	return dbCtx, nil
}
