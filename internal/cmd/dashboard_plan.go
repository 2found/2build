package cmd

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/2found/2build/internal/ticket"
)

// Dashboard writes for the planning fields and the company loop
// (bs-w5p3c6mq). Each handler validates through, and writes with, the same
// function the CLI verb uses — the browser is one more caller of the core
// layer, never a second writer. The dashboard is the human's surface, so
// decisions are recorded as the person running it (BBS_PERSON, else $USER).

type fieldsReq struct {
	Title      *string `json:"title"`
	Kind       *string `json:"kind"`
	Product    *string `json:"product"`
	AssignedTo *struct {
		Role   string `json:"role"`
		Person string `json:"person"`
	} `json:"assigned_to"`
	LabelsAdd    []string `json:"labels_add"`
	LabelsRemove []string `json:"labels_remove"`
	Window       *struct {
		Start string `json:"start"`
		Due   string `json:"due"`
	} `json:"window"`
	Loop *struct {
		Stage          *string  `json:"stage"`
		TargetOutcome  *string  `json:"target_outcome"`
		ExpectedDelta  *string  `json:"expected_delta"`
		CheckAfterDays *int     `json:"check_after_days"`
		CheckFrom      *string  `json:"check_from"`
		EvidenceAdd    []string `json:"evidence_add"`
	} `json:"loop"`
}

func (q fieldsReq) writes(doc ticket.Doc) ([]fieldWrite, error) {
	var w []fieldWrite
	one := func(fw fieldWrite, err error) error {
		if err == nil {
			w = append(w, fw)
		}
		return err
	}
	if q.Title != nil {
		if err := one(titleWrite(*q.Title)); err != nil {
			return nil, err
		}
	}
	if q.Kind != nil {
		if err := one(kindWrite(*q.Kind)); err != nil {
			return nil, err
		}
	}
	if q.Product != nil {
		if err := one(productWrite(*q.Product)); err != nil {
			return nil, err
		}
	}
	if q.AssignedTo != nil {
		if err := one(assignedToWrite(q.AssignedTo.Role, q.AssignedTo.Person)); err != nil {
			return nil, err
		}
	}
	if len(q.LabelsAdd)+len(q.LabelsRemove) > 0 {
		l, err := labelsWith(doc, q.LabelsAdd, q.LabelsRemove)
		if err != nil {
			return nil, err
		}
		w = append(w, fieldWrite{"labels", l})
	}
	if q.Window != nil {
		if err := one(windowWrite(q.Window.Start, q.Window.Due)); err != nil {
			return nil, err
		}
	}
	if l := q.Loop; l != nil {
		lw, err := loopWrites(doc, loopSpec{Stage: l.Stage, Target: l.TargetOutcome, Expected: l.ExpectedDelta,
			CheckAfter: l.CheckAfterDays, CheckFrom: l.CheckFrom, Evidence: l.EvidenceAdd})
		if err != nil {
			return nil, err
		}
		w = append(w, lw...)
	}
	return w, nil
}

func (s *dashServer) handleFields(w http.ResponseWriter, r *http.Request) {
	var req fieldsReq
	st, ok := s.ticketReq(w, r, &req)
	if !ok {
		return
	}
	changed, err := fieldsApplyFn(st, req.writes, "dashboard")
	if err != nil {
		code := http.StatusInternalServerError
		var be buildErr
		if errors.As(err, &be) {
			code = http.StatusBadRequest
		}
		writeErr(w, code, err.Error())
		return
	}
	if changed == nil {
		changed = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ticket": st.Env.Ticket, "changed": changed})
}

type decisionReq struct {
	Decision string   `json:"decision"`
	Note     string   `json:"note"`
	Evidence []string `json:"evidence"`
}

func (s *dashServer) handleDecision(w http.ResponseWriter, r *http.Request) {
	var req decisionReq
	st, ok := s.ticketReq(w, r, &req)
	if !ok {
		return
	}
	if err := decideSet(st, req.Decision, req.Note, req.Evidence, personName(), "dashboard"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ticket": st.Env.Ticket, "decision": req.Decision})
}

type actualReq struct {
	Delta    string   `json:"delta"`
	Result   string   `json:"result"`
	Note     string   `json:"note"`
	Evidence []string `json:"evidence"`
}

func (s *dashServer) handleActual(w http.ResponseWriter, r *http.Request) {
	var req actualReq
	st, ok := s.ticketReq(w, r, &req)
	if !ok {
		return
	}
	if err := actualSet(st, req.Delta, req.Result, req.Note, req.Evidence, personName(), "dashboard"); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ticket": st.Env.Ticket, "result": req.Result})
}

func (s *dashServer) handleCompany(w http.ResponseWriter, r *http.Request) {
	weeks := 8
	if v := r.URL.Query().Get("weeks"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 104 {
			writeErr(w, http.StatusBadRequest, "weeks must be 1–104")
			return
		}
		weeks = n
	}
	// The company series live beside projects/ in the state dir the server reads.
	writeJSON(w, http.StatusOK, companySummaryFor(s.stateDir, weeks))
}

type scoreReq struct {
	Product      string   `json:"product"`
	Week         string   `json:"week"`
	Revenue      *float64 `json:"revenue"`
	ActiveUsers  *float64 `json:"active_users"`
	SupportCases *float64 `json:"support_cases"`
	Cost         *float64 `json:"cost"`
	Releases     *float64 `json:"releases"`
	Note         string   `json:"note"`
}

func (s *dashServer) handleScoreboard(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r) {
		return
	}
	var req scoreReq
	if !decode(w, r, &req) {
		return
	}
	week, err := weekOf(req.Week)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row := scoreRow{Week: week, Product: req.Product, Revenue: req.Revenue, ActiveUsers: req.ActiveUsers,
		SupportCases: req.SupportCases, Cost: req.Cost, Releases: req.Releases, Note: req.Note, By: personName()}
	if err := scoreRecord(s.stateDir, row); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"product": row.Product, "week": week})
}

type hoursReq struct {
	Product string  `json:"product"`
	Stage   string  `json:"stage"`
	Hours   float64 `json:"hours"`
	Week    string  `json:"week"`
	Note    string  `json:"note"`
}

func (s *dashServer) handleHours(w http.ResponseWriter, r *http.Request) {
	if !s.guard(w, r) {
		return
	}
	var req hoursReq
	if !decode(w, r, &req) {
		return
	}
	week, err := weekOf(req.Week)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	row := hoursRow{Week: week, Product: req.Product, Stage: req.Stage, Hours: req.Hours, Note: req.Note, By: personName()}
	if err := hoursRecord(s.stateDir, row); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"product": row.Product, "stage": row.Stage, "week": week, "hours": row.Hours})
}
