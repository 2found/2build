package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/2found/2build/internal/orca"
)

const foremanWaitBatchUsage = `Usage:
  bbs foreman wait-batch --tasks <task,task,...> [--timeout <duration>] [--interval <duration>]

Blocks until every listed Task's current Dispatch is terminal, then prints one
summary. Returns early only when the batch cannot finish without the
coordinator: an unread question/escalation, or a worker that exited without
settling its Dispatch. Defaults: --timeout 9m (fits a 10-minute tool-call cap),
--interval 20s.
`

// The wait runs in this process so the coordinator model is woken once per
// batch rather than once per worker_done: every coordinator wake re-reads its
// whole context, which dominates token cost in a wide fan-out.
var (
	waitBatchSleep = time.Sleep
	waitBatchNow   = time.Now
)

type batchTask struct {
	task     string
	dispatch string
	status   string
	run      string
}

func foremanWaitBatch(args []string) error {
	_, kv, err := foremanFlags(args)
	if err != nil {
		return err
	}
	var tasks []string
	seen := map[string]bool{}
	for _, task := range strings.Split(kv["tasks"], ",") {
		if task = strings.TrimSpace(task); task != "" && !seen[task] {
			seen[task] = true
			tasks = append(tasks, task)
		}
	}
	if len(tasks) == 0 {
		return fmt.Errorf("foreman wait-batch: --tasks is required\n%s", foremanWaitBatchUsage)
	}
	timeout, err := waitBatchDuration(kv, "timeout", 9*time.Minute)
	if err != nil {
		return err
	}
	interval, err := waitBatchDuration(kv, "interval", 20*time.Second)
	if err != nil {
		return err
	}
	// A deadline bounds each Orca call (see Client.run), so one hung probe
	// cannot hold the wait past its timeout.
	ctx, cancel := context.WithTimeout(context.Background(), timeout+time.Minute)
	defer cancel()
	c, err := orca.PreflightContext(ctx)
	if err != nil {
		return fmt.Errorf("foreman wait-batch: %w", err)
	}
	if !c.Orchestration() {
		return fmt.Errorf("foreman wait-batch: %w", orca.ErrNoOrchestration)
	}

	deadline := waitBatchNow().Add(timeout)
	batch := make([]batchTask, len(tasks))
	for i, task := range tasks {
		batch[i].task = task
	}
	for first := true; ; first = false {
		var probeErr error
		settled := 0
		for i := range batch {
			t := &batch[i]
			if dispatchTerminal(t.status) {
				settled++
				continue
			}
			state, err := c.DispatchStateFor(t.task)
			if err != nil {
				probeErr = err
				continue
			}
			if state.ID == "" {
				if first {
					return fmt.Errorf("foreman wait-batch: task %s has no Dispatch; launch it before waiting", t.task)
				}
				continue
			}
			t.dispatch, t.status, t.run = state.ID, state.Status, state.RunID
			if dispatchTerminal(t.status) {
				settled++
			}
		}
		if settled == len(batch) {
			printBatch("settled", batch, nil, probeErr)
			return nil
		}
		var attention []string
		if n, err := c.PendingMessages("question,escalation"); err != nil {
			probeErr = err
		} else if n > 0 {
			attention = append(attention, fmt.Sprintf("messages:%d", n))
		}
		exited, err := exitedUnsettled(c, batch)
		if err != nil {
			probeErr = err
		}
		for _, task := range exited {
			attention = append(attention, "exited:"+task)
		}
		if len(attention) > 0 {
			printBatch("attention", batch, attention, probeErr)
			return nil
		}
		now := waitBatchNow()
		if !now.Before(deadline) {
			printBatch("timeout", batch, nil, probeErr)
			return nil
		}
		waitBatchSleep(min(interval, deadline.Sub(now)))
	}
}

// exitedUnsettled names Tasks whose worker is proven gone while their Dispatch
// is still open; that Dispatch can never settle, so the batch would not end.
func exitedUnsettled(c *orca.Client, batch []batchTask) ([]string, error) {
	runs := map[string]bool{}
	for _, t := range batch {
		if t.run != "" && t.dispatch != "" && !dispatchTerminal(t.status) {
			runs[t.run] = true
		}
	}
	var names []string
	var probeErr error
	for run := range runs {
		exited, err := c.ExitedWorkers(run)
		if err != nil {
			probeErr = err
			continue
		}
		for _, t := range batch {
			if t.run == run && !dispatchTerminal(t.status) && exited[t.dispatch] {
				names = append(names, t.task)
			}
		}
	}
	sort.Strings(names)
	return names, probeErr
}

func printBatch(outcome string, batch []batchTask, attention []string, probeErr error) {
	settled := 0
	for _, t := range batch {
		if dispatchTerminal(t.status) {
			settled++
		}
	}
	fmt.Println("BATCH=" + outcome)
	fmt.Printf("BATCH_SETTLED=%d/%d\n", settled, len(batch))
	for _, t := range batch {
		status := t.status
		if status == "" {
			status = "unknown"
		}
		fmt.Printf("TASK=%s DISPATCH=%s STATUS=%s\n", t.task, t.dispatch, status)
	}
	for _, a := range attention {
		fmt.Println("ATTENTION=" + a)
	}
	if probeErr != nil {
		fmt.Println("PROBE_ERROR=" + probeErr.Error())
	}
}

func waitBatchDuration(kv map[string]string, key string, def time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(kv[key])
	if value == "" {
		return def, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		return 0, errors.New("foreman wait-batch: --" + key + " needs a positive duration such as 9m or 20s")
	}
	return d, nil
}
