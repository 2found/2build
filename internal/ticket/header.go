package ticket

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Header is what `migrate` reads from a requirement.md written before the
// planning fields existed: the H1 and the `- **Label:** value` bullets that
// sit directly under it (the kiem-lai convention, in English and Vietnamese).
// Bullets further down the body ("Owns", "Acceptance" …) are content, not
// header, and are never read.
type Header struct {
	H1      string
	Bullets map[string]string // canonical label -> raw value
}

// headerLabels maps every label seen in real stores to one canonical name.
var headerLabels = map[string]string{
	"loại": "kind", "kind": "kind",
	"product": "product", "sản phẩm": "product",
	"assignee":       "assignee",
	"size":           "size",
	"target outcome": "target", "expected delta": "expected", "check after days": "check_after",
	"blocked by": "blocked_by",
	"quyết định": "decision", "decision": "decision",
	"bằng chứng": "evidence", "evidence": "evidence",
	"key":             "key",
	"khung thời gian": "window", "window": "window",
	"milestone": "milestone", "stage": "stage",
}

var bulletRe = regexp.MustCompile(`^-\s+\*\*([^*]+?):\*\*\s*(.*)$`)

// ReadHeader parses the header block of a requirement file. A missing file is
// an empty header.
func ReadHeader(path string) Header {
	h := Header{Bullets: map[string]string{}}
	f, err := os.Open(path)
	if err != nil {
		return h
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	inHeader := false
	for i := 0; i < 60 && sc.Scan(); i++ {
		line := strings.TrimRight(sc.Text(), " \t")
		if !inHeader {
			if m := h1Re.FindStringSubmatch(line); m != nil {
				h.H1, inHeader = m[1], true
			}
			continue
		}
		if line == "" {
			if len(h.Bullets) > 0 {
				break // the block ended
			}
			continue
		}
		m := bulletRe.FindStringSubmatch(line)
		if m == nil {
			break
		}
		if k, ok := headerLabels[strings.ToLower(strings.TrimSpace(m[1]))]; ok {
			if _, dup := h.Bullets[k]; !dup {
				h.Bullets[k] = strings.TrimSpace(m[2])
			}
		}
	}
	return h
}

var (
	ticketIDRe   = regexp.MustCompile(`\bbs-[a-z0-9]{8}\b`)
	leadIntRe    = regexp.MustCompile(`^\s*(\d+)`)
	isoDateRe    = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
	titlePrefRe  = regexp.MustCompile(`^(MILESTONE|PHASE\s+(\d+))\s*[·:—-]\s*`)
	titleRangeRe = regexp.MustCompile(`\s*\(([^()]*\d[^()]*[–-][^()]*\d[^()]*)\)\s*$`)
	dayRangeRe   = regexp.MustCompile(`^(\d{1,2})/(\d{1,2})(?:/(\d{4}))?\s*[–-]\s*(\d{1,2})/(\d{1,2})/(\d{4})$`)
	monRangeRe   = regexp.MustCompile(`^(\d{1,2})/(\d{4})\s*[–-]\s*(\d{1,2})/(\d{4})$`)
	refRe        = regexp.MustCompile("https?://[^\\s,;)`]+|\\bbs-[a-z0-9]{8}\\b")
)

// FirstWord is the leading word of a bullet value, lowercased ("human (founder)"
// -> "human", "go — founder 2026-11-02" -> "go").
func FirstWord(v string) string {
	f := strings.FieldsFunc(strings.ToLower(v), func(r rune) bool {
		return r == ' ' || r == '(' || r == ',' || r == ';' || r == '—' || r == '-' || r == ':' || r == '`'
	})
	if len(f) == 0 {
		return ""
	}
	return f[0]
}

// LeadingInt reads "90 (đọc kết quả 01/02/2027)" as 90.
func LeadingInt(v string) (int, bool) {
	m := leadIntRe.FindStringSubmatch(v)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// IDsIn lists every bs-<8> id in a value, in order, deduped.
func IDsIn(v string) []string { return dedupe(ticketIDRe.FindAllString(v, -1)) }

// Refs lists URLs and ticket ids in an evidence value.
func Refs(v string) []string { return dedupe(refRe.FindAllString(v, -1)) }

// ISODate is the first YYYY-MM-DD in a value.
func ISODate(v string) string {
	if m := isoDateRe.FindStringSubmatch(v); m != nil && ValidDate(m[1]) == nil {
		return m[1]
	}
	return ""
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ParseWindow reads the two range shapes the store uses: "02/12 – 31/12/2026"
// (day/month, the start's year implied by the end's) and "11/2026 – 01/2027"
// (whole months). ok=false for anything else — a window is never guessed.
func ParseWindow(v string) (start, due string, ok bool) {
	v = strings.TrimSpace(v)
	if m := dayRangeRe.FindStringSubmatch(v); m != nil {
		ey, _ := strconv.Atoi(m[6])
		em, _ := strconv.Atoi(m[5])
		ed, _ := strconv.Atoi(m[4])
		sm, _ := strconv.Atoi(m[2])
		sd, _ := strconv.Atoi(m[1])
		sy := ey
		if m[3] != "" {
			sy, _ = strconv.Atoi(m[3])
		} else if sm > em {
			sy = ey - 1
		}
		s, e := ymd(sy, sm, sd), ymd(ey, em, ed)
		if s == "" || e == "" || e < s {
			return "", "", false
		}
		return s, e, true
	}
	if m := monRangeRe.FindStringSubmatch(v); m != nil {
		sm, _ := strconv.Atoi(m[1])
		sy, _ := strconv.Atoi(m[2])
		em, _ := strconv.Atoi(m[3])
		ey, _ := strconv.Atoi(m[4])
		if sm < 1 || sm > 12 || em < 1 || em > 12 {
			return "", "", false
		}
		s := ymd(sy, sm, 1)
		e := time.Date(ey, time.Month(em)+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		if e < s {
			return "", "", false
		}
		return s, e, true
	}
	return "", "", false
}

func ymd(y, m, d int) string {
	s := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	if ValidDate(s) != nil {
		return ""
	}
	return s
}

// CleanTitle turns an H1 into a title: "MILESTONE · X" -> "X", "PHASE 2 · X"
// -> "Phase 2 · X", and a trailing date-range parenthetical moves to the
// window (returned, when it parses). Anything else is kept verbatim.
func CleanTitle(h1 string) (title, start, due string) {
	t := strings.TrimSpace(h1)
	if m := titlePrefRe.FindStringSubmatch(t); m != nil {
		rest := t[len(m[0]):]
		if m[2] != "" {
			t = "Phase " + m[2] + " · " + rest
		} else {
			t = rest
		}
	}
	if m := titleRangeRe.FindStringSubmatchIndex(t); m != nil {
		if s, d, ok := ParseWindow(t[m[2]:m[3]]); ok {
			start, due = s, d
			t = strings.TrimSpace(t[:m[0]])
		}
	}
	for len(t) > 300 { // titleWrite's bound, cut on a rune boundary
		r := []rune(t)
		t = string(r[:len(r)-1])
	}
	return t, start, due
}
