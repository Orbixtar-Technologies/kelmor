package operations

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	mailLogPath   = "/var/log/mail.log"
	mailLogRotate = "/var/log/mail.log.1"
)

type MailDeliveryAttempt struct {
	ID        string `json:"id"`
	QueueID   string `json:"queue_id"`
	Timestamp string `json:"timestamp"`
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Relay     string `json:"relay,omitempty"`
	DSN       string `json:"dsn,omitempty"`
	Direction string `json:"direction"`
}

type MailQueueEntry struct {
	QueueID   string `json:"queue_id"`
	Size      int    `json:"size,omitempty"`
	Sender    string `json:"sender,omitempty"`
	Recipient string `json:"recipient,omitempty"`
	Arrival   string `json:"arrival,omitempty"`
	State     string `json:"state"`
}

type MailDeliveryParams struct {
	Mode  string `json:"mode"`
	Year  int    `json:"year"`
	Month int    `json:"month"`
	Day   int    `json:"day"`
	Query string `json:"query"`
}

type MailDeliveryResult struct {
	Mode    string                `json:"mode"`
	Date    string                `json:"date,omitempty"`
	Query   string                `json:"query,omitempty"`
	Source  string                `json:"source"`
	Partial bool                  `json:"partial"`
	Message string                `json:"message,omitempty"`
	Items   []MailDeliveryAttempt `json:"items"`
	Queue   []MailQueueEntry      `json:"queue,omitempty"`
}

var (
	mailAddrRe  = regexp.MustCompile(`(?i)(to|from)=<([^>]*)>`)
	mailFieldRe = regexp.MustCompile(`(?i)\b(status|dsn|relay)=([^\s,]+)`)
	mailQueueRe = regexp.MustCompile(`^([A-F0-9]+)\*?`)
)

func parsePostfixMailLog(body string, year, month, day int, query string) []MailDeliveryAttempt {
	senders := map[string]string{}
	var items []MailDeliveryAttempt
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || !strings.Contains(line, "postfix/") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		stamp, ok := parseSyslogTime(fields[0], fields[1], fields[2], year)
		if !ok {
			continue
		}
		if month > 0 && day > 0 {
			if int(stamp.Month()) != month || stamp.Day() != day {
				continue
			}
		}
		queueID, rest := splitQueueToken(fields)
		if queueID == "" && !strings.Contains(line, "NOQUEUE") {
			continue
		}
		if queueID == "NOQUEUE" {
			queueID = "NOQUEUE"
		}
		from, to := mailAddresses(line)
		if from != "" && queueID != "" && queueID != "NOQUEUE" {
			senders[queueID] = from
		}
		if from == "" {
			from = senders[queueID]
		}
		status, dsn, relay, message := mailAttemptFields(line, rest)
		if status == "" && to == "" && from == "" {
			continue
		}
		if status == "" {
			if to != "" {
				status = "accepted"
			} else {
				status = "queued"
			}
		}
		item := MailDeliveryAttempt{
			ID:        mailAttemptID(queueID, to, stamp, status),
			QueueID:   queueID,
			Timestamp: stamp.UTC().Format(time.RFC3339),
			Sender:    from,
			Recipient: to,
			Status:    status,
			Message:   message,
			Relay:     relay,
			DSN:       dsn,
			Direction: mailDirection(from, to, status),
		}
		if needle != "" && !mailAttemptMatches(item, needle) {
			continue
		}
		if item.Recipient == "" && item.Sender == "" && item.Status == "queued" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func parsePostqueue(body string) []MailQueueEntry {
	var items []MailQueueEntry
	var current *MailQueueEntry
	flush := func() {
		if current == nil {
			return
		}
		items = append(items, *current)
		current = nil
	}
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimRight(raw, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "-Queue ID-") || strings.HasPrefix(trim, "--") {
			continue
		}
		if strings.HasPrefix(trim, "Mail queue is empty") {
			return nil
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && mailQueueRe.MatchString(fields[0]) {
			flush()
			id := strings.TrimSuffix(fields[0], "*")
			size, _ := strconv.Atoi(fields[1])
			arrival := ""
			sender := ""
			if len(fields) >= 7 {
				arrival = strings.Join(fields[2:len(fields)-1], " ")
				sender = fields[len(fields)-1]
			} else if len(fields) >= 3 {
				sender = fields[len(fields)-1]
			}
			current = &MailQueueEntry{
				QueueID: id,
				Size:    size,
				Sender:  sender,
				Arrival: arrival,
				State:   "queued",
			}
			continue
		}
		if current != nil && strings.HasPrefix(line, " ") && trim != "" && !strings.HasPrefix(trim, "(") {
			if current.Recipient == "" {
				current.Recipient = strings.Trim(trim, "<>")
			}
		}
	}
	flush()
	return items
}

func (h *Host) readMailDelivery(p MailDeliveryParams) (MailDeliveryResult, error) {
	mode := strings.ToLower(strings.TrimSpace(p.Mode))
	if mode != "track" {
		mode = "report"
	}
	if mode == "report" {
		if err := validateMailReportDate(p.Year, p.Month, p.Day); err != nil {
			return MailDeliveryResult{}, err
		}
	}
	query := strings.TrimSpace(p.Query)
	year, month, day := p.Year, p.Month, p.Day
	if mode == "track" && (year == 0 || month == 0 || day == 0) {
		now := time.Now()
		year, month, day = now.Year(), int(now.Month()), now.Day()
		if query == "" {
			month, day = 0, 0
		}
	}
	var items []MailDeliveryAttempt
	var sources []string
	for _, name := range []string{mailLogPath, mailLogRotate} {
		path, err := h.resolve(name)
		if err != nil {
			continue
		}
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		sources = append(sources, name)
		items = append(items, parsePostfixMailLog(string(body), year, month, day, query)...)
	}
	result := MailDeliveryResult{
		Mode:   mode,
		Query:  query,
		Source: strings.Join(sources, ", "),
		Items:  items,
		Queue:  []MailQueueEntry{},
	}
	if mode == "report" {
		result.Date = fmt.Sprintf("%04d-%02d-%02d", p.Year, p.Month, p.Day)
	}
	if mode == "track" {
		queue, qsrc, qerr := h.readMailQueue()
		if qerr == nil {
			result.Queue = filterMailQueue(queue, query)
			if qsrc != "" {
				sources = append(sources, qsrc)
				result.Source = strings.Join(sources, ", ")
			}
		}
	}
	if result.Source == "" {
		result.Partial = true
		result.Message = "Kelmor reads Postfix syslog at /var/log/mail.log. This host has no mail log yet, or the Agent sandbox has no copy of that file."
	} else if !h.live() && mode == "track" && len(result.Queue) == 0 {
		result.Partial = true
		if result.Message == "" {
			result.Message = "Live queue tracking needs a privileged Agent (postqueue). Recent log lines are shown when /var/log/mail.log is present."
		}
	}
	if result.Items == nil {
		result.Items = []MailDeliveryAttempt{}
	}
	return result, nil
}

func (h *Host) readMailQueue() ([]MailQueueEntry, string, error) {
	if !h.live() {
		return nil, "", nil
	}
	out, err := runFixed("/usr/sbin/postqueue", "-p")
	if err != nil {
		return nil, "", err
	}
	return parsePostqueue(string(out)), "postqueue", nil
}

func validateMailReportDate(year, month, day int) error {
	if year < 1970 || year > 2100 {
		return fmt.Errorf("year must be between 1970 and 2100")
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("month must be between 1 and 12")
	}
	if day < 1 || day > 31 {
		return fmt.Errorf("day must be between 1 and 31")
	}
	stamp := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if stamp.Year() != year || int(stamp.Month()) != month || stamp.Day() != day {
		return fmt.Errorf("invalid calendar date")
	}
	return nil
}

func parseSyslogTime(month, day, clock string, year int) (time.Time, bool) {
	if year <= 0 {
		year = time.Now().Year()
	}
	raw := month + " " + day + " " + clock
	stamp, err := time.ParseInLocation("Jan _2 15:04:05", raw, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return time.Date(year, stamp.Month(), stamp.Day(), stamp.Hour(), stamp.Minute(), stamp.Second(), 0, time.UTC), true
}

func splitQueueToken(fields []string) (string, string) {
	for _, field := range fields {
		token := strings.TrimSuffix(field, ":")
		if token == "NOQUEUE" {
			return "NOQUEUE", strings.Join(fields, " ")
		}
		if mailQueueRe.MatchString(token) && strings.HasSuffix(field, ":") {
			return strings.TrimSuffix(token, "*"), strings.Join(fields, " ")
		}
	}
	return "", strings.Join(fields, " ")
}

func mailAddresses(line string) (from, to string) {
	for _, match := range mailAddrRe.FindAllStringSubmatch(line, -1) {
		if len(match) < 3 {
			continue
		}
		kind := strings.ToLower(match[1])
		addr := strings.TrimSpace(match[2])
		if kind == "from" {
			from = addr
		}
		if kind == "to" {
			to = addr
		}
	}
	return from, to
}

func mailAttemptFields(line, rest string) (status, dsn, relay, message string) {
	for _, match := range mailFieldRe.FindAllStringSubmatch(line, -1) {
		if len(match) < 3 {
			continue
		}
		switch strings.ToLower(match[1]) {
		case "status":
			status = normalizeMailStatus(match[2])
		case "dsn":
			dsn = match[2]
		case "relay":
			relay = match[2]
		}
	}
	if status == "" && (strings.Contains(line, "reject:") || strings.Contains(line, "NOQUEUE")) {
		status = "rejected"
	}
	if idx := strings.Index(rest, "("); idx >= 0 {
		message = strings.Trim(rest[idx:], "()")
	}
	if message == "" {
		message = summarizeMailLine(line)
	}
	return status, dsn, relay, message
}

func normalizeMailStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sent":
		return "delivered"
	case "deferred":
		return "deferred"
	case "bounced":
		return "bounced"
	case "expired":
		return "expired"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func mailDirection(from, to, status string) string {
	if status == "rejected" && to == "" {
		return "inbound"
	}
	if from == "" && to != "" {
		return "inbound"
	}
	if from != "" && to != "" {
		return "outbound"
	}
	return "unknown"
}

func mailAttemptMatches(item MailDeliveryAttempt, needle string) bool {
	blob := strings.ToLower(strings.Join([]string{
		item.QueueID, item.Sender, item.Recipient, item.Status, item.Message, item.Relay,
	}, " "))
	return strings.Contains(blob, needle)
}

func filterMailQueue(items []MailQueueEntry, query string) []MailQueueEntry {
	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		if items == nil {
			return []MailQueueEntry{}
		}
		return items
	}
	out := make([]MailQueueEntry, 0, len(items))
	for _, item := range items {
		blob := strings.ToLower(item.QueueID + " " + item.Sender + " " + item.Recipient)
		if strings.Contains(blob, needle) {
			out = append(out, item)
		}
	}
	return out
}

func mailAttemptID(queueID, recipient string, stamp time.Time, status string) string {
	return fmt.Sprintf("%s:%s:%d:%s", queueID, recipient, stamp.Unix(), status)
}

func summarizeMailLine(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return line
	}
	return strings.Join(fields[4:], " ")
}
