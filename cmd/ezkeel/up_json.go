package main

import (
	"encoding/json"
	"io"
)

// upJSONEmitter writes the NDJSON progress stream for `ezkeel up --json`
// — one JSON object per line on stdout so coding agents can parse
// deploy progress as it happens. Event names map 1:1 onto the real
// pipeline actions in runUp: detect, dockerfile, build, push,
// provision_db, deploy, caddy_route, health.
//
// All methods are nil-receiver safe so call sites inside runUp don't
// need an `if jsonOut` guard around every emission.
type upJSONEmitter struct {
	enc *json.Encoder
}

func newUpJSONEmitter(w io.Writer) *upJSONEmitter {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return &upJSONEmitter{enc: enc}
}

// stepEvent is one progress line:
// {"event":"step","name":"build","status":"start|done|fail","detail":"..."}
type stepEvent struct {
	Event  string `json:"event"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// warningEvent surfaces non-fatal conditions, e.g.
// {"event":"warning","kind":"migrations_not_run","migrator":"prisma","hint":"..."}
type warningEvent struct {
	Event    string `json:"event"`
	Kind     string `json:"kind"`
	Migrator string `json:"migrator"`
	Hint     string `json:"hint"`
}

// resultOKEvent is the final line of a successful deploy.
type resultOKEvent struct {
	Event     string `json:"event"`
	OK        bool   `json:"ok"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Framework string `json:"framework"`
	DB        bool   `json:"db"`
}

// resultFailEvent is the final line of a failed deploy.
type resultFailEvent struct {
	Event string `json:"event"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func (e *upJSONEmitter) step(name, status, detail string) {
	if e == nil {
		return
	}
	_ = e.enc.Encode(stepEvent{Event: "step", Name: name, Status: status, Detail: detail})
}

func (e *upJSONEmitter) warning(kind, migrator, hint string) {
	if e == nil {
		return
	}
	_ = e.enc.Encode(warningEvent{Event: "warning", Kind: kind, Migrator: migrator, Hint: hint})
}

func (e *upJSONEmitter) result(name, url, framework string, db bool) {
	if e == nil {
		return
	}
	_ = e.enc.Encode(resultOKEvent{Event: "result", OK: true, Name: name, URL: url, Framework: framework, DB: db})
}

func (e *upJSONEmitter) fail(errMsg string) {
	if e == nil {
		return
	}
	_ = e.enc.Encode(resultFailEvent{Event: "result", OK: false, Error: errMsg})
}
