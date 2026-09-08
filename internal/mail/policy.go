package mail

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type LimitRow struct {
	Address string
	Account string
	Limit   int
}

func ParseSendLimits(body string) []LimitRow {
	var out []LimitRow
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}
		out = append(out, LimitRow{Address: strings.ToLower(fields[0]), Account: fields[1], Limit: n})
	}
	return out
}

func LookupLimit(rows []LimitRow, sender, sasl string) (LimitRow, bool) {
	sender = strings.ToLower(strings.TrimSpace(sender))
	sasl = strings.ToLower(strings.TrimSpace(sasl))
	for _, r := range rows {
		if r.Address == sender || r.Address == sasl {
			return r, true
		}
	}
	return LimitRow{}, false
}

// Decide is the Postfix access policy action for one message.
func Decide(limitsPath, countsDir, sender, sasl string, now time.Time) (string, error) {
	body, err := os.ReadFile(limitsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "DUNNO", nil
		}
		return "DUNNO", err
	}
	row, ok := LookupLimit(ParseSendLimits(string(body)), sender, sasl)
	if !ok || row.Limit <= 0 {
		return "DUNNO", nil
	}
	used, err := bumpCount(countsDir, row.Account, now)
	if err != nil {
		return "DUNNO", err
	}
	if used > row.Limit {
		return "REJECT 5.7.1 Daily message limit exceeded", nil
	}
	return "DUNNO", nil
}

func bumpCount(dir, account string, now time.Time) (int, error) {
	if err := os.MkdirAll(filepath.Join(dir, now.UTC().Format("20060102")), 0o775); err != nil {
		return 0, err
	}
	path := filepath.Join(dir, now.UTC().Format("20060102"), account)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o664)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return 0, err
	}
	defer func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }()
	n := 0
	sc := bufio.NewScanner(f)
	if sc.Scan() {
		n, _ = strconv.Atoi(strings.TrimSpace(sc.Text()))
	}
	n++
	if _, err := f.Seek(0, 0); err != nil {
		return 0, err
	}
	if err := f.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := fmt.Fprintf(f, "%d\n", n); err != nil {
		return 0, err
	}
	return n, nil
}

func PolicyResponse(action string) string {
	if action == "" {
		action = "DUNNO"
	}
	return "action=" + action + "\n\n"
}
