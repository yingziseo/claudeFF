package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"
)

type Proxy struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

func (p Proxy) Validate() error {
	if p.Protocol != "http" && p.Protocol != "https" && p.Protocol != "socks5" {
		return errors.New("代理协议应为 HTTP、HTTPS 或 SOCKS5")
	}
	if p.Port < 1 || p.Port > 65535 {
		return errors.New("代理端口应为 1–65535")
	}
	if p.Host == "" || strings.TrimSpace(p.Host) != p.Host || strings.ContainsAny(p.Host, "/\\@?#\r\n\t ") {
		return errors.New("请填写代理主机，不要包含协议、路径或认证信息")
	}
	if strings.ContainsAny(p.Username+p.Password, "\r\n") || strings.Contains(p.Username, ":") {
		return errors.New("代理认证格式无效")
	}
	if len(p.Host) > 253 || len(p.Username) > 512 || len(p.Password) > 1024 {
		return errors.New("代理信息过长")
	}
	return nil
}
func (p Proxy) Address() string { return net.JoinHostPort(p.Host, strconv.Itoa(p.Port)) }
func (p Proxy) Public() Proxy   { p.Password = ""; return p }

type Profile struct {
	ID        string    `json:"id"`
	Revision  int       `json:"revision"`
	Source    string    `json:"source"`
	Seed      string    `json:"seed"`
	Language  string    `json:"language"`
	Timezone  string    `json:"timezone"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewProfile(source string) Profile {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return Profile{ID: hex.EncodeToString(b[:8]), Seed: strconv.FormatUint(uint64(b[8])<<24|uint64(b[9])<<16|uint64(b[10])<<8|uint64(b[11]), 10), Revision: 1, Source: source, Language: "en-US", Timezone: "UTC", CreatedAt: time.Now().UTC()}
}
func (p Profile) Validate() error {
	if p.Language != "en-US" && p.Language != "zh-CN" && p.Language != "ja-JP" && p.Language != "en-GB" {
		return errors.New("请选择支持的语言")
	}
	if _, e := time.LoadLocation(p.Timezone); e != nil {
		return errors.New("无效时区")
	}
	return nil
}

type Observation struct {
	IP       string    `json:"ip"`
	Country  string    `json:"country"`
	Timezone string    `json:"timezone"`
	Latency  int64     `json:"latency"`
	At       time.Time `json:"at"`
	Revision int       `json:"revision"`
}
type Status struct {
	Phase       string       `json:"phase"`
	Message     string       `json:"message"`
	Observation *Observation `json:"observation,omitempty"`
}
type Snapshot struct {
	Proxy       Proxy    `json:"proxy"`
	HasPassword bool     `json:"hasPassword"`
	Revision    int      `json:"revision"`
	Profile     *Profile `json:"profile,omitempty"`
	Browser     Status   `json:"browser"`
	Code        Status   `json:"code"`
	Network     Status   `json:"network"`
	Kernel      bool     `json:"kernel"`
	Native      bool     `json:"native"`
	DataDir     string   `json:"dataDir"`
}
