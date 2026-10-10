package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/yingziseo/claudeFF/internal/model"
	_ "modernc.org/sqlite"
)

var ErrProxyFormat = errors.New("旧代理信息无法识别，请重新粘贴识别；浏览器数据已保留")

var ErrSecretUnavailable = errors.New("无法读取代理密码，请允许系统秘密存储访问或重新填写密码")

type Protector interface {
	Protect([]byte) ([]byte, error)
	Unprotect([]byte) ([]byte, error)
}
type Store struct {
	mu        sync.Mutex
	db        *sql.DB
	protector Protector
}

func Open(dir string, p Protector) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	db, e := sql.Open("sqlite", filepath.Join(dir, "workspace.db"))
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	var version int
	if e = db.QueryRow("PRAGMA user_version").Scan(&version); e != nil {
		db.Close()
		return nil, e
	}
	if version > 1 {
		db.Close()
		return nil, errors.New("数据库来自更新版本，请使用对应客户端；未覆盖数据")
	}

	if _, e = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA secure_delete=ON;
 CREATE TABLE IF NOT EXISTS settings (id INTEGER PRIMARY KEY CHECK(id=1), proxy BLOB NOT NULL, secret BLOB, revision INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS profile (id INTEGER PRIMARY KEY CHECK(id=1), value BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS runtime_history(name TEXT PRIMARY KEY,value BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS license_session(id INTEGER PRIMARY KEY CHECK(id=1),secret BLOB NOT NULL);
 PRAGMA user_version=1;`); e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db: db, protector: p}, nil
}
func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Proxy() (model.Proxy, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw, enc []byte
	var rev int
	e := s.db.QueryRow("SELECT proxy,secret,revision FROM settings WHERE id=1").Scan(&raw, &enc, &rev)
	if errors.Is(e, sql.ErrNoRows) {
		return model.Proxy{Protocol: "http"}, 0, nil
	}
	if e != nil {
		return model.Proxy{}, 0, e
	}
	var p model.Proxy
	if e = json.Unmarshal(raw, &p); e != nil {
		return p, rev, e
	}
	// Repair V2 rows where provider text was mistakenly stored as a hostname.
	// Never return that legacy string in a public snapshot, even if migration fails.
	if strings.Contains(p.Host, ":") && net.ParseIP(p.Host) == nil {
		normalized, err := model.ParseProxyText(p.Host, p.Protocol)
		if err != nil {
			return model.Proxy{Protocol: "http", Port: 8080}, rev, ErrProxyFormat
		}
		if normalized.Username == "" {
			normalized.Username = p.Username
		}
		if normalized.Password != "" {
			enc, err = s.protector.Protect([]byte(normalized.Password))
			if err != nil {
				return normalized.Public(), rev, ErrSecretUnavailable
			}
		}
		public, _ := json.Marshal(normalized.Public())
		if _, err = s.db.Exec("UPDATE settings SET proxy=?,secret=?,revision=revision+1 WHERE id=1", public, enc); err != nil {
			return normalized.Public(), rev, errors.New("旧代理格式修复失败，请重试")
		}
		rev++
		p = normalized
		// Finish the credential repair without retaining old WAL frames.
		_, _ = s.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	}
	if len(enc) > 0 {
		b, e := s.protector.Unprotect(enc)
		if e != nil {
			return p, rev, ErrSecretUnavailable
		}
		p.Password = string(b)
	}
	return p, rev, nil
}
func (s *Store) SaveProxy(p model.Proxy) (int, error) {
	if e := p.Validate(); e != nil {
		return 0, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var enc []byte
	var e error
	if p.Password != "" {
		enc, e = s.protector.Protect([]byte(p.Password))
		if e != nil {
			return 0, e
		}
	}
	raw, e := json.Marshal(p.Public())
	if e != nil {
		return 0, e
	}
	var rev int
	if e = s.db.QueryRow(`INSERT INTO settings(id,proxy,secret,revision) VALUES(1,?,?,1)
 ON CONFLICT(id) DO UPDATE SET proxy=excluded.proxy,secret=excluded.secret,revision=settings.revision+1 RETURNING revision`, raw, enc).Scan(&rev); e != nil {
		return 0, e
	}
	return rev, nil
}
func (s *Store) Profile() (*model.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw []byte
	e := s.db.QueryRow("SELECT value FROM profile WHERE id=1").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var p model.Profile
	e = json.Unmarshal(raw, &p)
	return &p, e
}
func (s *Store) EnsureProfile(source string) (*model.Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := model.NewProfile(source)
	raw, e := json.Marshal(p)
	if e != nil {
		return nil, e
	}
	if _, e = s.db.Exec("INSERT OR IGNORE INTO profile(id,value) VALUES(1,?)", raw); e != nil {
		return nil, e
	}
	e = s.db.QueryRow("SELECT value FROM profile WHERE id=1").Scan(&raw)
	if e != nil {
		return nil, e
	}
	e = json.Unmarshal(raw, &p)
	return &p, e
}
func (s *Store) SaveProfile(p model.Profile) error {
	if e := p.Validate(); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, e := json.Marshal(p)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("UPDATE profile SET value=? WHERE id=1", raw)
	return e
}

func (s *Store) LicenseToken() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw []byte
	e := s.db.QueryRow("SELECT secret FROM license_session WHERE id=1").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return "", nil
	}
	if e != nil {
		return "", e
	}
	b, e := s.protector.Unprotect(raw)
	return string(b), e
}
func (s *Store) SaveLicenseToken(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if token == "" {
		_, e := s.db.Exec("DELETE FROM license_session WHERE id=1")
		return e
	}
	sealed, e := s.protector.Protect([]byte(token))
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO license_session(id,secret) VALUES(1,?) ON CONFLICT(id) DO UPDATE SET secret=excluded.secret", sealed)
	return e
}
func (s *Store) Record(name string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, e := json.Marshal(value)
	if e != nil {
		return e
	}
	_, e = s.db.Exec("INSERT INTO runtime_history(name,value) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET value=excluded.value", name, raw)
	return e
}
func (s *Store) PreviousObservation() (*model.Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var raw []byte
	e := s.db.QueryRow("SELECT value FROM runtime_history WHERE name='network'").Scan(&raw)
	if errors.Is(e, sql.ErrNoRows) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var o model.Observation
	e = json.Unmarshal(raw, &o)
	return &o, e
}
