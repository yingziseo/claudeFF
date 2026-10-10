package licensing

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/scrypt"
	_ "modernc.org/sqlite"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Server struct {
	DB        *sql.DB
	AdminHash [32]byte
	Now       func() time.Time
}

func Open(path, admin string) (*Server, error) {
	if len(admin) < 32 {
		return nil, errors.New("管理员令牌至少 32 字符，必须通过环境配置")
	}
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	_, e = db.Exec(`PRAGMA journal_mode=WAL;PRAGMA busy_timeout=5000;
 CREATE TABLE IF NOT EXISTS cards(id INTEGER PRIMARY KEY,hash TEXT UNIQUE NOT NULL,label TEXT NOT NULL,created TEXT NOT NULL,revoked INTEGER NOT NULL DEFAULT 0,user_id INTEGER);
 CREATE TABLE IF NOT EXISTS users(id INTEGER PRIMARY KEY,username TEXT UNIQUE NOT NULL,salt BLOB NOT NULL,password BLOB NOT NULL,created TEXT NOT NULL,last_seen TEXT,version TEXT,browser_running INTEGER NOT NULL DEFAULT 0,code_running INTEGER NOT NULL DEFAULT 0);
 CREATE TABLE IF NOT EXISTS sessions(hash TEXT PRIMARY KEY,user_id INTEGER NOT NULL,expires TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS sessions_user ON sessions(user_id);
 CREATE TABLE IF NOT EXISTS audit(id INTEGER PRIMARY KEY,action TEXT NOT NULL,subject INTEGER NOT NULL,created TEXT NOT NULL);`)
	if e != nil {
		db.Close()
		return nil, e
	}
	return &Server{DB: db, AdminHash: sha256.Sum256([]byte(admin)), Now: time.Now}, nil
}
func hash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func random(n int) string {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)
}
func (s *Server) Handler() http.Handler { return http.HandlerFunc(s.serve) }
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
func fail(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, "请求格式无效")
		return false
	}
	return true
}
func validUser(name string) bool {
	if len([]rune(name)) < 2 || len([]rune(name)) > 32 {
		return false
	}
	for _, c := range name {
		if !unicode.IsLetter(c) && !unicode.IsDigit(c) && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/v1/admin/") {
		got := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
		if subtle.ConstantTimeCompare(got[:], s.AdminHash[:]) != 1 {
			fail(w, 401, "管理员认证失败")
			return
		}
		s.admin(w, r)
		return
	}
	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/register":
		s.register(w, r)
	case r.Method == "POST" && r.URL.Path == "/v1/login":
		s.login(w, r)
	case r.Method == "POST" && r.URL.Path == "/v1/heartbeat":
		s.heartbeat(w, r)
	case r.Method == "POST" && r.URL.Path == "/v1/logout":
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.DB.Exec("DELETE FROM sessions WHERE hash=?", hash(token))
		write(w, 200, map[string]bool{"ok": true})
	default:
		fail(w, 404, "接口不存在")
	}
}
func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	var body struct{ Username, Password, Card string }
	if !decode(w, r, &body) {
		return
	}
	if !validUser(body.Username) || len(body.Password) < 8 || len(body.Password) > 128 {
		fail(w, 400, "用户名需 2–32 位，密码需 8–128 字节")
		return
	}
	salt := []byte(random(24))
	derived, e := scrypt.Key([]byte(body.Password), salt, 32768, 8, 1, 32)
	if e != nil {
		fail(w, 500, "密码处理失败")
		return
	}
	tx, e := s.DB.Begin()
	if e != nil {
		fail(w, 503, "服务暂不可用")
		return
	}
	defer tx.Rollback()
	var cardID int
	var user sql.NullInt64
	var revoked int
	if e = tx.QueryRow("SELECT id,user_id,revoked FROM cards WHERE hash=?", hash(strings.TrimSpace(body.Card))).Scan(&cardID, &user, &revoked); e != nil || revoked != 0 || user.Valid {
		fail(w, 400, "卡密不存在、已使用或已撤销")
		return
	}
	result, e := tx.Exec("INSERT INTO users(username,salt,password,created) VALUES(?,?,?,?)", body.Username, salt, derived, s.Now().UTC().Format(time.RFC3339))
	if e != nil {
		fail(w, 400, "用户名已存在")
		return
	}
	id, _ := result.LastInsertId()
	result, e = tx.Exec("UPDATE cards SET user_id=? WHERE id=? AND user_id IS NULL AND revoked=0", id, cardID)
	if e != nil {
		fail(w, 409, "激活冲突，请重试")
		return
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		fail(w, 409, "卡密已激活")
		return
	}
	if e = tx.Commit(); e != nil {
		fail(w, 503, "激活未完成")
		return
	}
	s.newSession(w, id, body.Username)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var body struct{ Username, Password string }
	if !decode(w, r, &body) {
		return
	}
	if len(body.Password) > 128 {
		fail(w, 401, "用户名或密码错误")
		return
	}
	var id int
	var salt, stored []byte
	e := s.DB.QueryRow("SELECT id,salt,password FROM users WHERE username=?", body.Username).Scan(&id, &salt, &stored)
	if e != nil {
		salt = []byte("fixed-dummy-login-salt")
		stored = make([]byte, 32)
	}
	candidate, err := scrypt.Key([]byte(body.Password), salt, 32768, 8, 1, 32)
	if e != nil || err != nil || subtle.ConstantTimeCompare(candidate, stored) != 1 {
		fail(w, 401, "用户名或密码错误")
		return
	}
	var revoked int
	if e = s.DB.QueryRow("SELECT revoked FROM cards WHERE user_id=?", id).Scan(&revoked); e != nil || revoked != 0 {
		fail(w, 403, "授权已撤销")
		return
	}
	s.newSession(w, int64(id), body.Username)
}
func (s *Server) newSession(w http.ResponseWriter, id int64, username string) {
	token := random(32)
	expires := s.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	s.DB.Exec("DELETE FROM sessions WHERE expires<?", s.Now().UTC().Format(time.RFC3339))
	if _, e := s.DB.Exec("INSERT INTO sessions(hash,user_id,expires) VALUES(?,?,?)", hash(token), id, expires); e != nil {
		fail(w, 503, "会话创建失败")
		return
	}
	write(w, 200, map[string]any{"token": token, "username": username, "permanent": true})
}
func (s *Server) session(r *http.Request) (int, string, error) {
	var id int
	var username, expires string
	var revoked int
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		return 0, "", errors.New("not authorized")
	}
	e := s.DB.QueryRow("SELECT u.id,u.username,s.expires,c.revoked FROM sessions s JOIN users u ON u.id=s.user_id JOIN cards c ON c.user_id=u.id WHERE s.hash=?", hash(token)).Scan(&id, &username, &expires, &revoked)
	if e != nil || revoked != 0 || expires < s.Now().UTC().Format(time.RFC3339) {
		return 0, "", errors.New("授权无效或会话过期")
	}
	return id, username, nil
}
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request) {
	id, name, e := s.session(r)
	if e != nil {
		fail(w, 403, "授权已撤销或会话过期，请重新登录")
		return
	}
	var data struct {
		Version string `json:"version"`
		Browser bool   `json:"browser"`
		Code    bool   `json:"code"`
	}
	if !decode(w, r, &data) {
		return
	}
	if len(data.Version) > 32 {
		fail(w, 400, "版本字段过长")
		return
	}
	now := s.Now().UTC().Format(time.RFC3339)
	if _, e = s.DB.Exec("UPDATE users SET last_seen=?,version=?,browser_running=?,code_running=? WHERE id=?", now, data.Version, data.Browser, data.Code, id); e != nil {
		fail(w, 503, "状态更新失败")
		return
	}
	write(w, 200, map[string]any{"authorized": true, "username": name, "serverTime": now, "permanent": true})
}
func (s *Server) admin(w http.ResponseWriter, r *http.Request) {
	offset := 0
	var e error
	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, e = strconv.Atoi(raw)
		if e != nil || offset < 0 || offset > 10000000 {
			fail(w, 400, "分页参数无效")
			return
		}
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 120 {
		fail(w, 400, "查询条件过长")
		return
	}
	pattern := "%" + query + "%"

	switch {
	case r.Method == "POST" && r.URL.Path == "/v1/admin/cards":
		var p struct{ Label string }
		if !decode(w, r, &p) {
			return
		}
		if len(p.Label) > 120 {
			fail(w, 400, "备注过长")
			return
		}
		key := "CFF-" + random(20)
		result, e := s.DB.Exec("INSERT INTO cards(hash,label,created) VALUES(?,?,?)", hash(key), p.Label, s.Now().UTC().Format(time.RFC3339))
		if e != nil {
			fail(w, 503, "生成失败")
			return
		}
		id, _ := result.LastInsertId()
		s.DB.Exec("INSERT INTO audit(action,subject,created) VALUES(?,?,?)", "card.create", id, s.Now().UTC().Format(time.RFC3339))
		write(w, 201, map[string]any{"id": id, "card": key, "label": p.Label})
	case r.Method == "GET" && r.URL.Path == "/v1/admin/cards":
		rows, e := s.DB.Query("SELECT c.id,c.label,c.created,c.revoked,COALESCE(u.username,'') FROM cards c LEFT JOIN users u ON c.user_id=u.id WHERE c.label LIKE ? OR COALESCE(u.username,'') LIKE ? ORDER BY c.id DESC LIMIT 100 OFFSET ?", pattern, pattern, offset)
		if e != nil {
			fail(w, 503, "查询失败")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, revoked int
			var label, created, user string
			if rows.Scan(&id, &label, &created, &revoked, &user) != nil {
				fail(w, 500, "查询失败")
				return
			}
			items = append(items, map[string]any{"id": id, "label": label, "created": created, "revoked": revoked == 1, "username": user})
		}
		write(w, 200, items)
	case r.Method == "POST" && r.URL.Path == "/v1/admin/revoke":
		var p struct{ ID int }
		if !decode(w, r, &p) {
			return
		}
		result, e := s.DB.Exec("UPDATE cards SET revoked=1 WHERE id=?", p.ID)
		if e != nil {
			fail(w, 503, "撤销失败")
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			fail(w, 404, "卡密不存在")
			return
		}
		s.DB.Exec("INSERT INTO audit(action,subject,created) VALUES(?,?,?)", "card.revoke", p.ID, s.Now().UTC().Format(time.RFC3339))
		write(w, 200, map[string]bool{"ok": true})
	case r.Method == "GET" && r.URL.Path == "/v1/admin/users":
		rows, e := s.DB.Query("SELECT u.id,u.username,u.created,COALESCE(u.last_seen,''),COALESCE(u.version,''),u.browser_running,u.code_running,c.revoked,c.id FROM users u JOIN cards c ON c.user_id=u.id WHERE u.username LIKE ? ORDER BY u.id DESC LIMIT 100 OFFSET ?", pattern, offset)
		if e != nil {
			fail(w, 503, "查询失败")
			return
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, b, c, revoked, card int
			var name, created, last, version string
			if rows.Scan(&id, &name, &created, &last, &version, &b, &c, &revoked, &card) != nil {
				fail(w, 500, "查询失败")
				return
			}
			seen, _ := time.Parse(time.RFC3339, last)
			online := !seen.IsZero() && s.Now().Sub(seen) < 3*time.Minute
			items = append(items, map[string]any{"id": id, "username": name, "created": created, "lastSeen": last, "version": version, "browser": b == 1, "code": c == 1, "revoked": revoked == 1, "cardId": card, "online": online})
		}
		write(w, 200, items)
	default:
		fail(w, 404, "管理员接口不存在")
	}
}
