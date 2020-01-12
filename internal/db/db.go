package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/kisielk/sqlstruct"
	"github.com/rafaelespinoza/standardnotes/internal/errs"

	// initialize driver
	_ "github.com/go-sql-driver/mysql"
)

//Database encapsulates database
type Database struct {
	db *sql.DB
}

//DB returns DB handler
func DB() *sql.DB {
	return database.db
}

func (db Database) begin() (tx *sql.Tx) {
	tx, err := db.db.Begin()
	if err != nil {
		log.Println(err)
		return nil
	}
	return tx
}

func (db Database) prepare(q string) (stmt *sql.Stmt) {
	stmt, err := db.db.Prepare(q)
	if err != nil {
		log.Println(err)
		return nil
	}
	return stmt
}

type dbConnParameters struct{ host, name, pass, port, user string }

var (
	database     Database
	dbConnParams *dbConnParameters
)

func init() {
	dbConnParams = &dbConnParameters{
		host: os.Getenv("DB_HOST"),
		name: os.Getenv("DB_NAME"),
		pass: os.Getenv("DB_PASSWORD"),
		port: os.Getenv("DB_PORT"),
		user: os.Getenv("DB_USER"),
	}
}

// Init opens DB connection
func Init() {
	var err error
	database.db, err = sql.Open(
		"mysql",
		fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s?parseTime=true",
			dbConnParams.user, dbConnParams.pass, dbConnParams.host, dbConnParams.port, dbConnParams.name,
		),
	)

	if err != nil {
		log.Fatal(err)
	}
	if database.db == nil {
		log.Fatal("db nil")
	}
}

// Query is used for inserting or updating db data.
func Query(query string, args ...interface{}) error {
	stmt := database.prepare(query)
	defer stmt.Close()
	tx := database.begin()
	if _, err := tx.Stmt(stmt).Exec(args...); err != nil {
		log.Println("Query error: ", err)
		tx.Rollback()
	}
	err := tx.Commit()
	return err
}

// SelectExists queries for the first row and swallows an ErrNoRows error to
// signal that there are no matching rows. The dest argument should be a pointer
// to a value; the type pointed to by dest should match the query's column type.
func SelectExists(dest interface{}, query string, args ...interface{}) (exists bool, err error) {
	stmt := database.prepare(query)
	defer stmt.Close()
	err = stmt.QueryRow(args...).Scan(dest)
	if err == sql.ErrNoRows {
		err = nil // consider a non-error, means the row does not exist.
		return
	}
	if err == nil {
		exists = true
	}
	return
}

// SelectStruct attempts to select one matching row and fill the result into
// dest. The dest argument should be a pointer to some intended value. If there
// are no rows, then it returns an ErrNoRows error.
func SelectStruct(dest interface{}, query string, args ...interface{}) (err error) {
	var stmt *sql.Stmt
	var rows *sql.Rows
	var numRows int

	defer func() {
		stmt.Close()
		rows.Close()
	}()

	stmt = database.prepare(query)

	if rows, err = stmt.Query(args...); err != nil {
		return
	}
	for rows.Next() {
		// this is a hacky way to limit results to one row. Currently, the
		// sqlstruct library does not let you use the sql.QueryRow function,
		// which would be a better choice here. It likely won't ever support it.
		// See issue #5 in that repo for some details.
		if numRows > 1 {
			return
		}
		if err = sqlstruct.Scan(dest, rows); err != nil {
			return
		}
		numRows++
	}
	if err = rows.Err(); err != nil {
		return
	} else if numRows < 1 {
		err = errNoRows{sql.ErrNoRows}
		return
	}
	return
}

// SelectMany returns multiple results from the DB.
func SelectMany(onRow ScanRow, query string, args ...interface{}) (err error) {
	rows, err := database.db.Query(query, args...)
	if err == sql.ErrNoRows {
		err = errNoRows{err}
		return
	} else if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		if err = onRow(rows); err != nil {
			return
		}
	}
	return
}

// ScanRow is a callback to use when extracting column values from one row of a
// DB query result into an application value.
type ScanRow func(Iterator) error

// Iterator is a query result, such as a sql.Rows.
type Iterator interface {
	Scan(attributes ...interface{}) error
}

type errNoRows struct {
	error
}

func (e errNoRows) NotFound() bool { return true }

var _ errs.NotFound = (*errNoRows)(nil)
