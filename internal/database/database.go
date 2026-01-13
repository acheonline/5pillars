package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func New(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия БД: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ошибка подключения к БД: %v", err)
	}

	d := &Database{db: db}
	if err := d.init(); err != nil {
		return nil, err
	}

	log.Printf("✅ База данных инициализирована: %s", path)
	return d, nil
}

func (d *Database) init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pillar TEXT NOT NULL,
			description TEXT NOT NULL,
			completed BOOLEAN DEFAULT 0,
			time_utc TEXT NOT NULL,
			date TEXT NOT NULL,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS feelings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT UNIQUE NOT NULL,
			energy_level INTEGER CHECK(energy_level >= 1 AND energy_level <= 10),
			control_level INTEGER CHECK(control_level >= 1 AND control_level <= 10),
			sleep_hours REAL,
			mood TEXT,
			notes TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("ошибка создания таблицы: %v", err)
		}
	}

	d.migrateAddSkippedColumn()

	indexQueries := []string{
		`CREATE INDEX IF NOT EXISTS idx_tasks_date ON tasks(date)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_pillar ON tasks(pillar)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_completed ON tasks(completed)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_skipped ON tasks(skipped)`,
		`CREATE INDEX IF NOT EXISTS idx_tasks_time ON tasks(time_utc)`,
	}

	for _, query := range indexQueries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("ошибка создания индекса: %v", err)
		}
	}

	return nil
}

// migrateAddSkippedColumn добавляет поле skipped если его нет
func (d *Database) migrateAddSkippedColumn() {
	var columnExists bool
	err := d.db.QueryRow(`
		SELECT COUNT(*) > 0 
		FROM pragma_table_info('tasks') 
		WHERE name = 'skipped'
	`).Scan(&columnExists)

	if err != nil {
		log.Printf("Ошибка проверки поля skipped: %v", err)
		return
	}

	if !columnExists {
		_, err := d.db.Exec(`ALTER TABLE tasks ADD COLUMN skipped BOOLEAN DEFAULT 0`)
		if err != nil {
			log.Printf("Ошибка добавления поля skipped: %v", err)
		} else {
			log.Println("✅ Поле 'skipped' добавлено в таблицу tasks")
		}
	}
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) GetDB() *sql.DB {
	return d.db
}
