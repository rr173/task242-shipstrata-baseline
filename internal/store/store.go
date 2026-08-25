// Package store 提供 SQLite 持久化层（纯 Go 驱动，CGO 无关）。
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store 封装数据库连接与迁移。
type Store struct {
	DB *sql.DB
}

// OpenStore 打开（必要时创建）SQLite 数据库并迁移 schema。
// dsn 启用外键与忙等待，保证并发写入串行化且不丢数据。
func OpenStore(dbPath string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{DB: db}
	if err := s.Migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return s, nil
}

// Close 关闭数据库连接。
func (s *Store) Close() error {
	return s.DB.Close()
}

// BeginTx 开启事务。
func (s *Store) BeginTx() (*sql.Tx, error) {
	return s.DB.Begin()
}
