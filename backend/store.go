package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Message struct {
	ID         int64  `json:"id"`
	Chat       string `json:"chat"`
	Direction  string `json:"direction"` // in | out
	Text       string `json:"text"`
	MediaURL   string `json:"media_url,omitempty"`
	MediaType  string `json:"media_type,omitempty"` // image | sticker | video | audio | document | template
	Status     string `json:"status"`               // sent | delivered | read | failed | received
	TimeUTC    string `json:"ts"`
	ExternalID string `json:"-"` // fingerprint para dedup de entrantes
	Reference  string `json:"-"` // ref propia para correlacionar estados de CM
}

type Chat struct {
	Phone     string `json:"phone"`
	Name      string `json:"name"` // contacto personalizado, vacío si no hay
	Last      string `json:"last"`
	LastMedia string `json:"last_media"` // para preview en sidebar
	TimeUTC   string `json:"ts"`
	Unread    int    `json:"unread"`
}

type Contact struct {
	Phone string `json:"phone"`
	Name  string `json:"name"`
}

type DB struct{ *sql.DB }

func openDB(path string) *DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS messages(
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			chat       TEXT NOT NULL,
			direction  TEXT NOT NULL,
			text       TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL,
			time_utc   TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS templates(
			name       TEXT PRIMARY KEY,
			namespace  TEXT NOT NULL,
			language   TEXT NOT NULL DEFAULT 'es',
			category   TEXT NOT NULL DEFAULT '',
			body       TEXT NOT NULL DEFAULT '');
		CREATE TABLE IF NOT EXISTS contacts(
			phone      TEXT PRIMARY KEY,
			name       TEXT NOT NULL DEFAULT '');
		CREATE TABLE IF NOT EXISTS sessions(
			token      TEXT PRIMARY KEY,
			created    TEXT NOT NULL);
		CREATE TABLE IF NOT EXISTS chat_reads(
			phone        TEXT PRIMARY KEY,
			last_read_id INTEGER NOT NULL DEFAULT 0)`); err != nil {
		log.Fatal(err)
	}

	// Migraciones: añade columnas/índices nuevos si aún no existen (errores de duplicado ignorados).
	for _, q := range []string{
		`ALTER TABLE messages ADD COLUMN media_url   TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN media_type  TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN external_id TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE messages ADD COLUMN reference   TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE templates ADD COLUMN body       TEXT NOT NULL DEFAULT ''`,
		// índice parcial: varios '' no colisionan, los fingerprints sí son únicos
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_msg_extid ON messages(external_id) WHERE external_id <> ''`,
		`CREATE INDEX IF NOT EXISTS ix_msg_ref ON messages(reference) WHERE reference <> ''`,
	} {
		db.Exec(q) // error ignorado intencionadamente
	}

	return &DB{db}
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// fingerprint genera un id estable por contenido para deduplicar reintentos del webhook.
// ponytail: dedup por contenido; dos mensajes idénticos sin timestamp colapsan (rarísimo, aceptable).
func fingerprint(chat, ts, text, mediaURL string) string {
	h := sha256.Sum256([]byte(chat + "|" + ts + "|" + text + "|" + mediaURL))
	return hex.EncodeToString(h[:16])
}

// ── Messages ──────────────────────────────────────────────────────────────

func (d *DB) save(m *Message) error {
	res, err := d.Exec(
		`INSERT INTO messages(chat,direction,text,media_url,media_type,status,time_utc,external_id,reference)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		m.Chat, m.Direction, m.Text, m.MediaURL, m.MediaType, m.Status, m.TimeUTC, m.ExternalID, m.Reference)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()
	return nil
}

func (d *DB) messageExists(extID string) bool {
	if extID == "" {
		return false
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM messages WHERE external_id=?`, extID).Scan(&n)
	return n > 0
}

// updateStatusByRef actualiza el estado de un saliente por su reference y devuelve su id.
func (d *DB) updateStatusByRef(ref, status string) (int64, bool) {
	if ref == "" {
		return 0, false
	}
	var id int64
	if err := d.QueryRow(`SELECT id FROM messages WHERE reference=?`, ref).Scan(&id); err != nil {
		return 0, false
	}
	d.Exec(`UPDATE messages SET status=? WHERE id=?`, status, id)
	return id, true
}

// history devuelve hasta limit mensajes anteriores a before (0 = los más recientes), en orden cronológico.
func (d *DB) history(chat string, before int64, limit int) ([]Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	rows, err := d.Query(
		`SELECT id,chat,direction,text,media_url,media_type,status,time_utc
		 FROM messages WHERE chat=? AND (?=0 OR id<?) ORDER BY id DESC LIMIT ?`,
		chat, before, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Chat, &m.Direction, &m.Text,
			&m.MediaURL, &m.MediaType, &m.Status, &m.TimeUTC); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	// invierte a orden cronológico (la query es DESC para el LIMIT)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (d *DB) search(q string) ([]Message, error) {
	rows, err := d.Query(
		`SELECT id,chat,direction,text,media_url,media_type,status,time_utc
		 FROM messages WHERE text LIKE ? ESCAPE '\' ORDER BY id DESC LIMIT 50`,
		"%"+likeEscape(q)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Chat, &m.Direction, &m.Text,
			&m.MediaURL, &m.MediaType, &m.Status, &m.TimeUTC); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// likeEscape neutraliza %, _ y \ para que la búsqueda sea literal.
func likeEscape(s string) string {
	r := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' || s[i] == '_' || s[i] == '\\' {
			r = append(r, '\\')
		}
		r = append(r, s[i])
	}
	return string(r)
}

func (d *DB) listChats() ([]Chat, error) {
	rows, err := d.Query(`
		SELECT m.chat,
		       COALESCE(c.name,'') AS name,
		       m.text,
		       m.media_type,
		       m.time_utc,
		       (SELECT COUNT(*) FROM messages x
		          WHERE x.chat=m.chat AND x.direction='in'
		            AND x.id > COALESCE(r.last_read_id,0)) AS unread
		FROM messages m
		LEFT JOIN contacts   c ON c.phone = m.chat
		LEFT JOIN chat_reads r ON r.phone = m.chat
		WHERE m.id IN (SELECT MAX(id) FROM messages GROUP BY chat)
		ORDER BY m.time_utc DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Chat{}
	for rows.Next() {
		var c Chat
		if err := rows.Scan(&c.Phone, &c.Name, &c.Last, &c.LastMedia, &c.TimeUTC, &c.Unread); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (d *DB) markRead(phone string) error {
	_, err := d.Exec(
		`INSERT INTO chat_reads(phone,last_read_id)
		 VALUES(?, (SELECT COALESCE(MAX(id),0) FROM messages WHERE chat=?))
		 ON CONFLICT(phone) DO UPDATE SET last_read_id=excluded.last_read_id`,
		phone, phone)
	return err
}

// ── Contacts ──────────────────────────────────────────────────────────────

func (d *DB) upsertContact(phone, name string) error {
	_, err := d.Exec(
		`INSERT INTO contacts(phone,name) VALUES(?,?)
		 ON CONFLICT(phone) DO UPDATE SET name=excluded.name`,
		phone, name)
	return err
}

func (d *DB) getContact(phone string) (Contact, error) {
	var c Contact
	err := d.QueryRow(`SELECT phone,name FROM contacts WHERE phone=?`, phone).
		Scan(&c.Phone, &c.Name)
	if err == sql.ErrNoRows {
		return Contact{Phone: phone}, nil
	}
	return c, err
}

// ── Templates ─────────────────────────────────────────────────────────────

type DBTemplate struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Language  string `json:"language"`
	Category  string `json:"category"`
	Body      string `json:"body"`
}

func (d *DB) listTemplates() ([]DBTemplate, error) {
	rows, err := d.Query(`SELECT name,namespace,language,category,body FROM templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DBTemplate{}
	for rows.Next() {
		var t DBTemplate
		if err := rows.Scan(&t.Name, &t.Namespace, &t.Language, &t.Category, &t.Body); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) saveTemplate(t DBTemplate) error {
	_, err := d.Exec(
		`INSERT INTO templates(name,namespace,language,category,body) VALUES(?,?,?,?,?)
		 ON CONFLICT(name) DO UPDATE SET namespace=excluded.namespace,
		 language=excluded.language, category=excluded.category, body=excluded.body`,
		t.Name, t.Namespace, t.Language, t.Category, t.Body)
	return err
}

func (d *DB) deleteTemplate(name string) error {
	_, err := d.Exec(`DELETE FROM templates WHERE name=?`, name)
	return err
}

// ── Sessions (auth) ───────────────────────────────────────────────────────

func (d *DB) newSession(token string) error {
	_, err := d.Exec(`INSERT INTO sessions(token,created) VALUES(?,?)`, token, nowUTC())
	return err
}

func (d *DB) sessionValid(token string) bool {
	if token == "" {
		return false
	}
	var n int
	d.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token=?`, token).Scan(&n)
	return n > 0
}

func (d *DB) deleteSession(token string) {
	d.Exec(`DELETE FROM sessions WHERE token=?`, token)
}

// ── Backup ────────────────────────────────────────────────────────────────

// backupLoop hace una copia diaria con VACUUM INTO y conserva los últimos 7 días.
// ponytail: backup local en el mismo volumen; añade envío a S3/offsite si necesitas recuperación ante desastre.
func (d *DB) backupLoop(dir string) {
	for {
		name := filepath.Join(dir, "backup-"+time.Now().UTC().Format("2006-01-02")+".db")
		if _, err := d.Exec(`VACUUM INTO ?`, name); err != nil {
			slog.Error("backup falló", "err", err)
		} else {
			d.pruneBackups(dir, 7)
		}
		time.Sleep(24 * time.Hour)
	}
}

func (d *DB) pruneBackups(dir string, keepDays int) {
	files, _ := filepath.Glob(filepath.Join(dir, "backup-*.db"))
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	for _, f := range files {
		base := filepath.Base(f) // backup-2006-01-02.db
		if len(base) < 17 {
			continue
		}
		t, err := time.Parse("2006-01-02", base[7:17])
		if err == nil && t.Before(cutoff) {
			os.Remove(f)
		}
	}
}
