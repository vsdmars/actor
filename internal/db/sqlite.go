package db

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	l "github.com/vsdmars/actor/internal/logger"
	"go.uber.org/zap"
)

// It is easy to get into trouble by accidentally holding on to connections.
// To prevent this:
// Ensure you Scan() every Row object
// Ensure you either Close() or fully-iterate via Next() every Rows object
// Ensure every transaction returns its connection via Commit() or Rollback()
// Note that Rows.Close() can be called multiple times safely,
// so do not fear calling it where it might not be necessary.

// https://www.sqlite.org/pragma.html#pragma_journal_mode
const (
	DELETE = iota
	TRUNCATE
	PERSIST
	MEMORY
	WAL
	OFF
)

// https://www.sqlite.org/sharedcache.html
const (
	SHARED = iota
	PRIVATE
)

const (
	backupDB = iota
	rotateDB
)

const (
	// create directory for sqlite db file due to sqlite sync by directory
	backupDir = "sqlitedb"
	rotateDir = "sqlitedb_rotate"
	dbDSN     = "file:%s?cache=%s&_journal=%s&_sync=OFF"
)

var journalMode = map[int]string{
	DELETE:   "delete",
	TRUNCATE: "truncate",
	PERSIST:  "persist",
	MEMORY:   "memory",
	WAL:      "wal",
	OFF:      "off",
}

var cacheMode = map[int]string{
	SHARED:  "shared",
	PRIVATE: "private",
}

var (
	insertActorStart = `INSERT INTO actor(uuid, name, start_time) VALUES (:uuid, :name, :start_time)`
	updateActorEnd   = `UPDATE actor SET end_time = :end_time WHERE uuid = :uuid`
	insertLog        = `INSERT INTO log(time, message) VALUES (:time, :message)`
)

var (
	ErrDbPathIsAFile = "%s is an existing file"
)

var actor_schema = `
CREATE TABLE if not exists actor(
    uuid text PRIMARY KEY,
    name text,
    start_time text,
    end_time text
);
`
var log_schema = `
CREATE TABLE if not exists log(
    seq INTEGER PRIMARY KEY ASC,
    time text,
    message text
);
`

// Sqlite is the externed sqlite type
type Sqlite struct {
	ctx     context.Context
	name    string
	uuid    string
	journal int
	cache   int
	rotate  int
	db      *sqlx.DB
}

type (
	message struct {
		Msg string `json:"message"`
	}

	actor struct {
		Uuid  string `db:"uuid"`
		Name  string `db:"name"`
		Stime string `db:"start_time"`
		Etime string `db:"end_time"`
	}

	log struct {
		Seq  int    `db:"seq"`
		Time string `db:"time"`
		Msg  []byte `db:"message"`
	}
)

// NewSqlite returns init. Sqlite instance
//
// ctx: context.Context
//
// name: actor name
//
// uuid: actor uuid
//
// jmode: journal mode
//
// cmode: cache mode
//
// rcnt: 0: no rotation, >0: preserve number of records then rotate.
func NewSqlite(
	ctx context.Context, // caller's context
	name string, // actor name
	uuid string, // actor uuid
	jmode int, // journal mode
	cmode int, // cache mode
	rcnt int, // rotate count
) (*Sqlite, error) {

	db, err := initDB(ctx, name, uuid, jmode, cmode, backupDB)
	if err != nil {
		// Log
		l.Logger.Error(
			"error logged",
			zap.String("service", serviceName),
			zap.String("actor", actor.Name()),
			zap.String("uuid", actor.UUID()),
			zap.String("error", e.Error()),
		)
		return nil, err
	}

	if rcnt > 0 {
		go rotate(ctx, name, uuid, db, rcnt)
	}

	return &Sqlite{
		ctx:     ctx,
		name:    name,
		uuid:    uuid,
		journal: jmode,
		cache:   cmode,
		rotate:  rcnt,
		db:      db,
	}, nil
}

func initDB(
	ctx context.Context,
	name, uuid string,
	jmode, cmode int,
	dbType int) (*sqlx.DB, error) {

	var dbPath string
	currentDir, _ := os.Getwd()

	// Log
	fmt.Printf("CurrentDir: %s\n", currentDir)

	switch dbType {
	case backupDB:
		dbPath = path.Join(currentDir, backupDir)
	case rotateDB:
		dbPath = path.Join(currentDir, rotateDir)
	default:
		dbPath = path.Join(currentDir, backupDir)
	}

	// Log
	fmt.Printf("dbPath: %s\n", dbPath)

	if fi, err := os.Stat(dbPath); err != nil {
		if err := os.Mkdir(dbPath, 0700); err != nil {
			// Log
			fmt.Printf("create directory error: %s\n", err.Error())
			return nil, err
		}
	} else if !fi.IsDir() {
		// dbPath is a file, not directory
		return nil, fmt.Errorf(ErrDbPathIsAFile, dbPath)
	}

	var dbFile string
	gpattern := `%s_%s_*.db`
	rpattern := `%s_%s_(?P<SEQ>\d+).db`

	switch dbType {
	case backupDB:
		dbFile = path.Join(dbPath, fmt.Sprintf("%s_%s.db", name, uuid))
	case rotateDB:
		dbFiles := path.Join(dbPath, fmt.Sprintf(gpattern, name, uuid))
		fmt.Printf("HERE-1 dbFiles: %s\n", dbFiles)

		if m, err := filepath.Glob(dbFiles); err != nil {
			dbFile = path.Join(dbPath, fmt.Sprintf("%s_%s_1.db", name, uuid))
		} else {
			re, err := regexp.Compile(fmt.Sprintf(rpattern, name, uuid))
			if err != nil {
				fmt.Printf("reg compile err: %s\n", err.Error())
				return nil, nil
			}

			maxCnt := 0

			for idx := range m {
				match := re.FindStringSubmatch(m[idx])

				for i, name := range re.SubexpNames() {
					if i != 0 && name == "SEQ" {
						v, _ := strconv.Atoi(match[i])
						maxCnt = max(v, maxCnt)
					}
				}
			}

			dbFile = path.Join(
				dbPath, fmt.Sprintf("%s_%s_%d.db", name, uuid, maxCnt+1))
		}
	default:
		dbFile = path.Join(dbPath, fmt.Sprintf("%s_%s.db", name, uuid))
	}

	// Log
	fmt.Printf("HERE-2 dbFile: %s\n", dbFile)

	dsn := fmt.Sprintf(dbDSN, dbFile, cacheMode[cmode], journalMode[jmode])

	// Log
	fmt.Printf("dsn: %s\n", dsn)

	// Use open instead of MustOpen, which panics if can't open
	// Use open instead of Connect since sqlite is local file
	db, err := sqlx.Open(
		"sqlite3",
		dsn,
	)
	if err != nil {
		return nil, err
	}

	// The connection is returned to the pool before every call's result
	// is returned.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// sqlite Exec can't handle multi-statement, thus makes it different stmnt
	db.MustExecContext(ctx, actor_schema)
	db.MustExecContext(ctx, log_schema)

	return db, nil
}

func max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func rotate(
	ctx context.Context,
	name, uuid string,
	db *sqlx.DB,
	rcnt int) {

	// c := time.Tick(5 * time.Second)

	selectSql := `SELECT seq, time, message FROM log ORDER BY seq LIMIT ? ;`
	deleteSql := `DELETE FROM log WHERE seq <= ? ;`
	selectCnt := `SELECT COUNT(*) FROM log ;`

	// var result []log

	runner := func() {
		var rowCnt int

		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			// Log
			fmt.Printf("BeginTxx error: %s\n", err.Error())
			return
		}

		cntRow := tx.QueryRowxContext(ctx, selectCnt)
		cntRow.Scan(&rowCnt)
		fmt.Printf("rowCnt: %v\n", rowCnt)

		if rowCnt > rcnt {
			fmt.Printf("rowCnt: %v, rcnt: %v \n", rowCnt, rcnt)

			rdb, err := initDB(ctx, name, uuid, DELETE, PRIVATE, rotateDB)
			if err != nil {
				// Log
				fmt.Println("rotate db create failure")
				tx.Rollback()
				return
			}
			defer rdb.Close()

			fmt.Println("FU-1")
			rows, err := tx.QueryxContext(ctx, selectSql, rcnt)
			fmt.Println("FU-2")
			if err != nil {
				// Log
				fmt.Printf("QueryxContext %s error: %s", selectSql, err.Error())
				tx.Rollback()
				return
			}
			defer rows.Close()
			fmt.Println("FU-3")

			lastSeq := 0

			for rows.Next() {
				var l log
				if err := rows.StructScan(&l); err != nil {
					fmt.Printf("StructScan error: %s\n", err.Error())
					tx.Rollback()
					return
				}

				fmt.Printf("seq: %v, time: %v msg: %v\n", l.Seq, l.Time, l.Msg)

				_, err := rdb.NamedExecContext(
					ctx,
					insertLog,
					l,
				)
				if err != nil {
					// log
					fmt.Printf("rotate insert err: %s\n", err.Error())
					tx.Rollback()
					return
				}
				lastSeq = l.Seq
			}

			fmt.Printf("LastSeq: %d\n", lastSeq)

			// delete
			_, err = tx.ExecContext(
				ctx,
				deleteSql,
				lastSeq,
			)
			if err != nil {
				// log
				fmt.Printf("rotate delete err: %s\n", err.Error())
				tx.Rollback()
				return
			}
			fmt.Printf("ShitLastSeq: %d\n", lastSeq)
		}

		if err := tx.Commit(); err != nil {
			// Log
			fmt.Printf("Commit error: %s", err.Error())
			return
		}
	}

	runner()

	// for {
	// select {
	// case <-ctx.Done():
	// return
	// case <-c:
	// runner()

	// }
	// }
}

func (s *Sqlite) Insert(msg string) error {
	b, err := json.Marshal(message{Msg: msg})
	if err != nil {
		// log
		fmt.Printf("InsertLog marshal err: %s\n", err.Error())
		return err
	}

	r, err := s.db.NamedExecContext(
		s.ctx,
		insertLog,
		log{
			Time: time.Now().Format(time.RFC3339),
			Msg:  b,
		},
	)
	if err != nil {
		// log
		fmt.Printf("insert err: %s\n", err.Error())
		return nil
	}

	_ = r
	return nil
}

func (s *Sqlite) Start(startTime time.Time) error {
	// use RFC3339 time format
	r, err := s.db.NamedExecContext(
		s.ctx,
		insertActorStart,
		actor{
			Uuid:  s.uuid,
			Name:  s.name,
			Stime: startTime.Format(time.RFC3339),
		},
	)
	if err != nil {
		// Log
		fmt.Printf("insertActorStart err: %s\n", err.Error())
		return err
	}

	// Log result
	_ = r
	return nil
}

func (s *Sqlite) Stop(endTime time.Time) error {
	// updateActorEnd   = `UPDATE actor SET end_time = :end_time WHERE uuid = :uuid`
	// use RFC3339 time format
	r, err := s.db.NamedExecContext(
		s.ctx,
		updateActorEnd,
		actor{
			Uuid:  s.uuid,
			Etime: endTime.Format(time.RFC3339),
		},
	)
	if err != nil {
		// log
		fmt.Printf("updateActorEnd err: %s\n", err.Error())
		return nil
	}

	// log
	_ = r
	return nil
}

func main_old() {
	fmt.Println("starts")
	ctx, cancel := context.WithCancel(context.Background())

	db, _ := initDB(ctx, "actorX", "123456", DELETE, SHARED, backupDB)
	rotate(ctx, "actorX", "123456", db, 30)

	_ = cancel
	s, err := NewSqlite(
		ctx,
		"actorX",
		"123456",
		DELETE,
		SHARED,
		100,
	)
	if err != nil {
		fmt.Printf("error: %s\n", err.Error())
	}

	s.Start(time.Now())

	for {
		s.Insert("hi there~")
		time.Sleep(2 * time.Second)
	}

	s.Stop(time.Now())
	_ = cancel
	<-ctx.Done()
	fmt.Println("ends")
}
