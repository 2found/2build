package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/2found/2build/internal/identity"
	"github.com/2found/2build/internal/ticket"
)

// `bbs company` (bs-w5p3c6mq) holds the company-loop numbers that are not a
// ticket: the weekly product scoreboard and founder hours by product × stage
// (SOOT-COMPANY-LOOP §7), plus the decision log read across every project.
// The two series live in ~/.babysit/company/*.jsonl and are written only here
// (the dashboard calls the same functions). Both are append-only; the latest
// row for a key is the current value, so a correction is a new row and the
// earlier number stays on file.

var hourStages = []string{"insight", "product", "marketing", "operation", "soot"}

type scoreRow struct {
	At           string   `json:"at"`
	Week         string   `json:"week"` // Monday, YYYY-MM-DD
	Product      string   `json:"product"`
	Revenue      *float64 `json:"revenue"`
	ActiveUsers  *float64 `json:"active_users"`
	SupportCases *float64 `json:"support_cases"`
	Cost         *float64 `json:"cost"`
	Releases     *float64 `json:"releases"`
	Note         string   `json:"note,omitempty"`
	By           string   `json:"by"`
}

type hoursRow struct {
	At      string  `json:"at"`
	Week    string  `json:"week"`
	Product string  `json:"product"`
	Stage   string  `json:"stage"`
	Hours   float64 `json:"hours"`
	Note    string  `json:"note,omitempty"`
	By      string  `json:"by"`
}

func companyDir(home string) string { return filepath.Join(home, "company") }

// weekOf normalizes any date to the Monday of its ISO week.
func weekOf(date string) (string, error) {
	t := time.Now()
	if date != "" {
		var err error
		if t, err = time.Parse("2006-01-02", date); err != nil {
			return "", fmt.Errorf("invalid date %q (want YYYY-MM-DD)", date)
		}
	}
	wd := (int(t.Weekday()) + 6) % 7 // Monday = 0
	return t.AddDate(0, 0, -wd).Format("2006-01-02"), nil
}

func appendJSONL(home, name string, row interface{}) error {
	if err := os.MkdirAll(companyDir(home), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(companyDir(home), name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(row)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	return err
}

func readJSONL[T any](home, name string) []T {
	var out []T
	f, err := os.Open(filepath.Join(companyDir(home), name))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var r T
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

func scoreRecord(home string, r scoreRow) error {
	if err := ticket.ValidProduct(r.Product); err != nil {
		return err
	}
	if r.Revenue == nil && r.ActiveUsers == nil && r.SupportCases == nil && r.Cost == nil && r.Releases == nil {
		return fmt.Errorf("nothing to record: give at least one of --revenue --active-users --support-cases --cost --releases")
	}
	for _, v := range []*float64{r.Revenue, r.ActiveUsers, r.SupportCases, r.Cost, r.Releases} {
		if v != nil && *v < 0 {
			return fmt.Errorf("scoreboard numbers are not negative")
		}
	}
	r.At = time.Now().UTC().Format(time.RFC3339)
	return appendJSONL(home, "scoreboard.jsonl", r)
}

func hoursRecord(home string, r hoursRow) error {
	if err := ticket.ValidProduct(r.Product); err != nil {
		return err
	}
	if !contains(hourStages, r.Stage) {
		return fmt.Errorf("invalid stage %q (valid: %s)", r.Stage, strings.Join(hourStages, "|"))
	}
	if r.Hours < 0 || r.Hours > 168 {
		return fmt.Errorf("hours must be 0–168 for one week")
	}
	r.At = time.Now().UTC().Format(time.RFC3339)
	return appendJSONL(home, "hours.jsonl", r)
}

// latestScores keeps the newest row per (product, week).
func latestScores(home string) []scoreRow {
	m := map[string]scoreRow{}
	for _, r := range readJSONL[scoreRow](home, "scoreboard.jsonl") {
		m[r.Product+"|"+r.Week] = r // file order is time order
	}
	out := make([]scoreRow, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Week != out[j].Week {
			return out[i].Week > out[j].Week
		}
		return out[i].Product < out[j].Product
	})
	return out
}

// latestHours keeps the newest row per (week, product, stage): `hours log` is
// "this week's hours for this cell", so re-logging corrects rather than adds.
func latestHours(home string) []hoursRow {
	m := map[string]hoursRow{}
	for _, r := range readJSONL[hoursRow](home, "hours.jsonl") {
		m[r.Week+"|"+r.Product+"|"+r.Stage] = r
	}
	out := make([]hoursRow, 0, len(m))
	for _, r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Week != b.Week {
			return a.Week > b.Week
		}
		if a.Product != b.Product {
			return a.Product < b.Product
		}
		return a.Stage < b.Stage
	})
	return out
}

// ─── decision log (CL4.3) ────────────────────────────────────────────────────

type decisionLogRow struct {
	At             string   `json:"at"`
	Project        string   `json:"project"`
	Product        string   `json:"product"`
	Stage          string   `json:"stage"`
	OpportunityRef string   `json:"opportunity_ref"`
	Title          string   `json:"title"`
	Decision       string   `json:"decision"`
	By             string   `json:"by"`
	Note           string   `json:"note"`
	EvidenceRefs   []string `json:"evidence_refs"`
	TargetOutcome  string   `json:"target_outcome"`
	ExpectedDelta  string   `json:"expected_delta"`
	CheckAfterDays *int     `json:"check_after_days"`
	CheckDue       string   `json:"check_due"`
	CheckOverdue   bool     `json:"check_overdue"`
	ActualDelta    *string  `json:"actual_delta"`
	Result         string   `json:"result"`
	Superseded     bool     `json:"superseded"`
}

// decisionLog lists every recorded decision across projects, newest first. A
// decision comes from its `loop_decision` history event; one transcribed by
// `migrate` (no event) comes from the record. Planning fields and the actual
// are read from the record, so a row shows the opportunity as it stands now;
// earlier decisions on the same ticket are kept and marked superseded.
func decisionLog(projectHomes []string, today string) []decisionLogRow {
	var rows []decisionLogRow
	for _, ph := range projectHomes {
		// A read-only report: one unreadable record drops that record, never
		// the project's whole log (the same rule the plan board uses).
		docs, _ := loadProjectDocsSkipping(ph)
		proj := filepath.Base(ph)
		for id, d := range docs {
			if d.Value("loop.decision") == nil {
				continue
			}
			home := filepath.Join(ph, "tickets", id)
			due, _ := ticket.CheckDue(d)
			base := decisionLogRow{
				Project: proj, Product: d.Get("product"), Stage: d.Get("loop.stage"), OpportunityRef: id,
				Title: ticket.Title(home, d), EvidenceRefs: append([]string{}, strList(d.Value("loop.evidence_refs"))...),
				TargetOutcome: d.Get("loop.target_outcome"), ExpectedDelta: d.Get("loop.expected_delta"),
				CheckDue: due, CheckOverdue: ticket.CheckOverdue(d, today), Result: d.Get("loop.actual.result"),
			}
			if n, err := strconv.Atoi(d.Get("loop.check_after_days")); err == nil {
				base.CheckAfterDays = &n
			}
			if a := d.Get("loop.actual.delta"); a != "" {
				base.ActualDelta = &a
			}
			events := decisionEvents(filepath.Join(home, "history.jsonl"))
			if len(events) == 0 {
				r := base
				r.At, r.Decision, r.By, r.Note = d.Get("loop.decision.at"), d.Get("loop.decision.value"), d.Get("loop.decision.by"), d.Get("loop.decision.note")
				rows = append(rows, r)
				continue
			}
			for i, e := range events {
				r := base
				r.At, r.Decision, r.By, r.Note = e.At, e.Decision, e.By, e.Note
				r.Superseded = i < len(events)-1
				if r.Superseded {
					r.ActualDelta, r.Result, r.CheckOverdue = nil, "", false
				}
				rows = append(rows, r)
			}
		}
	}
	// Stable, so two decisions on one ticket in the same second keep history
	// order.
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].At != rows[j].At {
			return rows[i].At > rows[j].At
		}
		if rows[i].OpportunityRef != rows[j].OpportunityRef {
			return rows[i].OpportunityRef < rows[j].OpportunityRef
		}
		return !rows[i].Superseded && rows[j].Superseded
	})
	return rows
}

type decisionEvent struct {
	Event, At, Decision, By, Note string
}

func decisionEvents(path string) []decisionEvent {
	var out []decisionEvent
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		if !strings.Contains(sc.Text(), `"loop_decision"`) {
			continue
		}
		var e struct {
			Event, At, Decision, By string
			Note                    *string
		}
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Event == "loop_decision" {
			de := decisionEvent{Event: e.Event, At: e.At, Decision: e.Decision, By: e.By}
			if e.Note != nil {
				de.Note = *e.Note
			}
			out = append(out, de)
		}
	}
	return out
}

func allProjectHomes(home string) []string {
	root := filepath.Join(home, "projects")
	ents, _ := os.ReadDir(root)
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, filepath.Join(root, e.Name()))
		}
	}
	return out
}

// ─── summary (`show`, and the dashboard's Company view) ──────────────────────

type companySummary struct {
	Today      string           `json:"today"`
	Weeks      []string         `json:"weeks"` // newest first
	Scoreboard []scoreRow       `json:"scoreboard"`
	Hours      []hoursRow       `json:"hours"`
	Stages     []string         `json:"stages"`
	Decisions  []decisionLogRow `json:"decisions"`
	HitRate    struct {
		Go      int `json:"go"`
		Checked int `json:"checked"`
		Hit     int `json:"hit"`
		Partial int `json:"partial"`
		Miss    int `json:"miss"`
		Overdue int `json:"overdue"`
	} `json:"hit_rate"`
}

func companySummaryFor(home string, weeks int) companySummary {
	today := ticket.Today()
	s := companySummary{Today: today, Stages: hourStages}
	w, _ := weekOf(today)
	t, _ := time.Parse("2006-01-02", w)
	keep := map[string]bool{}
	for i := 0; i < weeks; i++ {
		wk := t.AddDate(0, 0, -7*i).Format("2006-01-02")
		s.Weeks = append(s.Weeks, wk)
		keep[wk] = true
	}
	s.Scoreboard, s.Hours = []scoreRow{}, []hoursRow{}
	for _, r := range latestScores(home) {
		if keep[r.Week] {
			s.Scoreboard = append(s.Scoreboard, r)
		}
	}
	for _, r := range latestHours(home) {
		if keep[r.Week] {
			s.Hours = append(s.Hours, r)
		}
	}
	s.Decisions = decisionLog(allProjectHomes(home), today)
	for _, d := range s.Decisions {
		if d.Superseded || d.Decision != "go" {
			continue
		}
		s.HitRate.Go++
		switch d.Result {
		case "hit":
			s.HitRate.Hit++
		case "partial":
			s.HitRate.Partial++
		case "miss":
			s.HitRate.Miss++
		}
		if d.CheckOverdue {
			s.HitRate.Overdue++
		}
	}
	s.HitRate.Checked = s.HitRate.Hit + s.HitRate.Partial + s.HitRate.Miss
	return s
}

// ─── argv ────────────────────────────────────────────────────────────────────

func newCompanyCmd() *cobra.Command {
	return &cobra.Command{
		Use:                "company",
		Short:              "company loop: weekly scoreboard, founder hours, decision log",
		DisableFlagParsing: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return runCompany(args)
		},
	}
}

const companyUsage = `bbs company — the company-loop numbers that are not a ticket.

  scoreboard record --product P [--week YYYY-MM-DD] [--revenue N] [--active-users N]
                    [--support-cases N] [--cost N] [--releases N] [--note M]
        one product's week (any date in the week; default this week). Latest row wins.
  hours log --product P --stage insight|product|marketing|operation|soot --hours N
                    [--week YYYY-MM-DD] [--note M]
        founder hours for one product × stage cell of a week. Re-logging corrects it.
  log [--product P] [--since YYYY-MM-DD] [--csv|--json]
        the decision log (CL4.3) across every project, newest first
  show [--weeks N] [--json]
        scoreboard, hours by product × stage, hit rate, overdue checks

Files: ~/.babysit/company/{scoreboard,hours}.jsonl (append-only).
`

func runCompany(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Print(companyUsage)
		return nil
	}
	fail := func(err error) error {
		fmt.Fprintln(os.Stderr, "company:", err)
		os.Exit(2)
		return nil
	}
	sub := strings.Join(args[:min(2, len(args))], " ")
	switch {
	case sub == "scoreboard record":
		r, err := parseScore(args[2:])
		if err != nil {
			return fail(err)
		}
		if err := scoreRecord(identity.BabysitHome(), r); err != nil {
			return fail(err)
		}
		fmt.Printf("scoreboard: %s week of %s recorded\n", r.Product, r.Week)
	case sub == "hours log":
		r, err := parseHours(args[2:])
		if err != nil {
			return fail(err)
		}
		if err := hoursRecord(identity.BabysitHome(), r); err != nil {
			return fail(err)
		}
		fmt.Printf("hours: %s × %s week of %s = %g\n", r.Product, r.Stage, r.Week, r.Hours)
	case args[0] == "log":
		return runCompanyLog(args[1:])
	case args[0] == "show":
		weeks, asJSON := 8, false
		for i := 1; i < len(args); i++ {
			switch args[i] {
			case "--json":
				asJSON = true
			case "--weeks":
				n, err := strconv.Atoi(valueOf(args, i, "--weeks"))
				if err != nil || n < 1 || n > 104 {
					return fail(fmt.Errorf("--weeks must be 1–104"))
				}
				weeks, i = n, i+1
			default:
				return fail(fmt.Errorf("unknown argument %s", args[i]))
			}
		}
		s := companySummaryFor(identity.BabysitHome(), weeks)
		if asJSON {
			b, _ := json.MarshalIndent(s, "", "  ")
			fmt.Println(string(b))
			return nil
		}
		printSummary(s)
	default:
		fmt.Fprint(os.Stderr, companyUsage)
		os.Exit(2)
	}
	return nil
}

func parseNum(v, flag string) (*float64, error) {
	n, err := strconv.ParseFloat(strings.ReplaceAll(v, ",", ""), 64)
	if err != nil {
		return nil, fmt.Errorf("%s: %q is not a number", flag, v)
	}
	return &n, nil
}

func parseScore(args []string) (scoreRow, error) {
	r := scoreRow{By: personName()}
	week := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		nums := map[string]**float64{"--revenue": &r.Revenue, "--active-users": &r.ActiveUsers,
			"--support-cases": &r.SupportCases, "--cost": &r.Cost, "--releases": &r.Releases}
		switch {
		case nums[a] != nil:
			n, err := parseNum(valueOf(args, i, a), a)
			if err != nil {
				return r, err
			}
			*nums[a], i = n, i+1
		case a == "--product":
			r.Product, i = valueOf(args, i, a), i+1
		case a == "--week":
			week, i = valueOf(args, i, a), i+1
		case a == "--note":
			r.Note, i = valueOf(args, i, a), i+1
		default:
			return r, fmt.Errorf("unknown argument %s", a)
		}
	}
	var err error
	r.Week, err = weekOf(week)
	return r, err
}

func parseHours(args []string) (hoursRow, error) {
	r := hoursRow{By: personName()}
	week, set := "", false
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--product":
			r.Product, i = valueOf(args, i, a), i+1
		case "--stage":
			r.Stage, i = valueOf(args, i, a), i+1
		case "--hours":
			n, err := parseNum(valueOf(args, i, a), a)
			if err != nil {
				return r, err
			}
			r.Hours, i, set = *n, i+1, true
		case "--week":
			week, i = valueOf(args, i, a), i+1
		case "--note":
			r.Note, i = valueOf(args, i, a), i+1
		default:
			return r, fmt.Errorf("unknown argument %s", a)
		}
	}
	if !set {
		return r, fmt.Errorf("--hours is required")
	}
	var err error
	r.Week, err = weekOf(week)
	return r, err
}

func runCompanyLog(args []string) error {
	product, since, format := "", "", "text"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; a {
		case "--product":
			product, i = valueOf(args, i, a), i+1
		case "--since":
			since, i = valueOf(args, i, a), i+1
		case "--csv":
			format = "csv"
		case "--json":
			format = "json"
		default:
			fmt.Fprintf(os.Stderr, "company log: unknown argument %s\n", a)
			os.Exit(2)
		}
	}
	var rows []decisionLogRow
	for _, r := range decisionLog(allProjectHomes(identity.BabysitHome()), ticket.Today()) {
		if (product == "" || r.Product == product) && (since == "" || r.At >= since) {
			rows = append(rows, r)
		}
	}
	switch format {
	case "json":
		if rows == nil {
			rows = []decisionLogRow{}
		}
		b, _ := json.MarshalIndent(rows, "", "  ")
		fmt.Println(string(b))
	case "csv":
		w := csv.NewWriter(os.Stdout)
		_ = w.Write([]string{"at", "stage", "opportunity_ref", "decision", "note", "evidence_refs", "target_outcome",
			"expected_delta", "check_after_days", "actual_delta", "result", "product", "project", "title", "by", "superseded"})
		for _, r := range rows {
			days, actual := "", ""
			if r.CheckAfterDays != nil {
				days = strconv.Itoa(*r.CheckAfterDays)
			}
			if r.ActualDelta != nil {
				actual = *r.ActualDelta
			}
			_ = w.Write([]string{r.At, r.Stage, r.OpportunityRef, r.Decision, r.Note, strings.Join(r.EvidenceRefs, " "),
				r.TargetOutcome, r.ExpectedDelta, days, actual, r.Result, r.Product, r.Project, r.Title, r.By,
				strconv.FormatBool(r.Superseded)})
		}
		w.Flush()
	default:
		for _, r := range rows {
			if r.Superseded {
				continue
			}
			state := r.Result
			if r.CheckOverdue {
				state = "CHECK DUE " + r.CheckDue
			} else if state == "" && r.CheckDue != "" {
				state = "check " + r.CheckDue
			}
			fmt.Printf("%-10s %-5s %-8s %-12s %-22s %s\n", firstN(r.At, 10), r.Decision, r.Product, r.OpportunityRef, state, r.Title)
		}
	}
	return nil
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func printSummary(s companySummary) {
	fmt.Printf("Company loop — %s\n\nScoreboard (latest per product × week)\n", s.Today)
	if len(s.Scoreboard) == 0 {
		fmt.Println("  none recorded — bbs company scoreboard record --product P --revenue N …")
	}
	f := func(v *float64) string {
		if v == nil {
			return "-"
		}
		return strconv.FormatFloat(*v, 'f', -1, 64)
	}
	for _, r := range s.Scoreboard {
		fmt.Printf("  %s  %-8s revenue %-12s active %-8s support %-5s cost %-10s releases %s\n",
			r.Week, r.Product, f(r.Revenue), f(r.ActiveUsers), f(r.SupportCases), f(r.Cost), f(r.Releases))
	}
	fmt.Println("\nFounder hours (product × stage)")
	if len(s.Hours) == 0 {
		fmt.Println("  none logged — bbs company hours log --product P --stage S --hours N")
	}
	for _, r := range s.Hours {
		fmt.Printf("  %s  %-8s %-10s %g\n", r.Week, r.Product, r.Stage, r.Hours)
	}
	h := s.HitRate
	fmt.Printf("\nDecisions: %d go, %d checked (%d hit, %d partial, %d miss), %d checks overdue\n",
		h.Go, h.Checked, h.Hit, h.Partial, h.Miss, h.Overdue)
}
