package db

import (
	// sqlite3 driver
	_ "github.com/mattn/go-sqlite3"
	"upmeter/pkg/upmeter/db/context"
)

func Connect(path string) (*context.DbContext, error) {
	dbCtx := context.NewDbContext()
	err := dbCtx.ConnectWithPool(path)
	if err != nil {
		return nil, err
	}
	return dbCtx, nil
}
