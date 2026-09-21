package operations

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

const sampleMailLog = `Sep 19 04:12:00 host postfix/qmgr[11]: ABC123: from=<shop@shop.test>, size=512, nrcpt=1 (queue active)
Sep 19 04:12:01 host postfix/smtp[12]: ABC123: to=<user@example.com>, relay=mx.example.com[203.0.113.10]:25, delay=0.2, delays=0.1/0/0/0.1, dsn=2.0.0, status=sent (250 2.0.0 OK)
Sep 19 04:15:00 host postfix/smtp[13]: DEF456: to=<late@example.com>, relay=none, delay=30, dsn=4.4.1, status=deferred (connect timed out)
Sep 18 09:00:00 host postfix/smtp[14]: OLD789: to=<old@example.com>, relay=mx.example.com[203.0.113.10]:25, dsn=2.0.0, status=sent (250 OK)
Sep 19 04:16:00 host postfix/smtpd[15]: NOQUEUE: reject: RCPT from unknown[198.51.100.8]: 550 5.1.1 <ghost@shop.test>: Recipient address rejected
`

const sampleQueue = `-Queue ID-  --Size-- ----Arrival Time---- -Sender/Recipient-------
ABC999*      1024 Sat Sep 19 04:20:01  sender@shop.test
                                         dest@example.com
`

func TestParsePostfixMailLogFiltersByDateAndRecipient(t *testing.T) {
	items := parsePostfixMailLog(sampleMailLog, 2026, 9, 19, "user@example.com")
	if len(items) != 1 {
		t.Fatalf("items=%d %#v", len(items), items)
	}
	if items[0].Status != "delivered" || items[0].Recipient != "user@example.com" {
		t.Fatalf("attempt %#v", items[0])
	}
	if items[0].Sender != "shop@shop.test" || items[0].DSN != "2.0.0" {
		t.Fatalf("sender/dsn %#v", items[0])
	}
}

func TestParsePostfixMailLogKeepsDeferredAndRejected(t *testing.T) {
	items := parsePostfixMailLog(sampleMailLog, 2026, 9, 19, "")
	var sawDeferred, sawRejected bool
	for _, item := range items {
		if item.Status == "deferred" && item.Recipient == "late@example.com" {
			sawDeferred = true
		}
		if item.Status == "rejected" {
			sawRejected = true
		}
		if item.Recipient == "old@example.com" {
			t.Fatalf("included other day: %#v", item)
		}
	}
	if !sawDeferred || !sawRejected {
		t.Fatalf("missing statuses %#v", items)
	}
}

func TestParsePostqueueExtractsRecipient(t *testing.T) {
	items := parsePostqueue(sampleQueue)
	if len(items) != 1 {
		t.Fatalf("queue %#v", items)
	}
	if items[0].QueueID != "ABC999" || items[0].Recipient != "dest@example.com" {
		t.Fatalf("entry %#v", items[0])
	}
}

func TestParsePostqueueJSONExtractsRecipients(t *testing.T) {
	body := `{"queue_name":"deferred","queue_id":"4CBL5S0N0QZ","arrival_time":1474400354,"message_size":451,"sender":"sender@shop.test","recipients":[{"address":"dest@example.com"}]}`
	items := parsePostqueue(body)
	if len(items) != 1 {
		t.Fatalf("json queue %#v", items)
	}
	if items[0].QueueID != "4CBL5S0N0QZ" || items[0].Recipient != "dest@example.com" {
		t.Fatalf("entry %#v", items[0])
	}
	if items[0].Sender != "sender@shop.test" || items[0].Size != 451 || items[0].State != "deferred" {
		t.Fatalf("fields %#v", items[0])
	}
}

func TestParsePostqueueEmptyIsEmpty(t *testing.T) {
	if items := parsePostqueue("Mail queue is empty\n"); len(items) != 0 {
		t.Fatalf("text empty %#v", items)
	}
	if items := parsePostqueue(""); items == nil || len(items) != 0 {
		t.Fatalf("json empty %#v", items)
	}
}

func TestReadMailDeliveryUsesSandboxLog(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "var/log/mail.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format("Jan _2 15:04:05")
	logBody := sampleMailLog + today + " host postfix/smtp[16]: DEF456: to=<late@example.com>, relay=none, delay=30, dsn=4.4.1, status=deferred (connect timed out)\n"
	if err := os.WriteFile(path, []byte(logBody), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &Host{Root: root}
	report, err := host.readMailDelivery(MailDeliveryParams{Mode: "report", Year: 2026, Month: 9, Day: 19})
	if err != nil {
		t.Fatal(err)
	}
	if report.Date != "2026-09-19" || len(report.Items) == 0 {
		t.Fatalf("report %#v", report)
	}
	track, err := host.readMailDelivery(MailDeliveryParams{Mode: "track", Query: "late@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(track.Items) != 1 || track.Items[0].Status != "deferred" {
		t.Fatalf("track %#v", track)
	}
}

func TestValidateMailReportDateRejectsImpossibleDay(t *testing.T) {
	if err := validateMailReportDate(2026, 2, 31); err == nil {
		t.Fatal("expected invalid date")
	}
	if err := validateMailReportDate(2026, 9, 19); err != nil {
		t.Fatal(err)
	}
}
