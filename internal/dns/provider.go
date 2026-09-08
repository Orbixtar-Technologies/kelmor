package dns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Record struct {
	Name     string
	Type     string
	Content  string
	TTL      int
	Priority int
}

type Provider interface {
	CreateZone(ctx context.Context, name string) error
	DeleteZone(ctx context.Context, name string) error
	ListRecords(ctx context.Context, zone string) ([]Record, error)
	UpsertRecord(ctx context.Context, zone string, rec Record) error
	DeleteRecord(ctx context.Context, zone string, rec Record) error
	EnableDNSSEC(ctx context.Context, zone string) error
	DisableDNSSEC(ctx context.Context, zone string) error
	GetDSRecords(ctx context.Context, zone string) ([]Record, error)
}

// PowerDNS is the V1 local authoritative provider. The HTTP API is loopback-only.
type PowerDNS struct {
	BaseURL string
	APIKey  string
}

func (p *PowerDNS) CreateZone(ctx context.Context, name string) error {
	return p.roundTrip(ctx, "POST", "/api/v1/servers/localhost/zones", map[string]any{
		"name": name + ".", "kind": "Native", "nameservers": []string{},
	})
}

func (p *PowerDNS) DeleteZone(ctx context.Context, name string) error {
	return p.roundTrip(ctx, "DELETE", "/api/v1/servers/localhost/zones/"+name+".", nil)
}

func (p *PowerDNS) ListRecords(ctx context.Context, zone string) ([]Record, error) {
	return nil, nil
}
func (p *PowerDNS) UpsertRecord(ctx context.Context, zone string, rec Record) error {
	return p.roundTrip(ctx, "PATCH", "/api/v1/servers/localhost/zones/"+zone+".", map[string]any{
		"rrsets": []map[string]any{{
			"name": rec.Name, "type": rec.Type, "ttl": rec.TTL, "changetype": "REPLACE",
			"records": []map[string]any{{"content": rec.Content, "disabled": false}},
		}},
	})
}
func (p *PowerDNS) DeleteRecord(ctx context.Context, zone string, rec Record) error {
	return p.roundTrip(ctx, "PATCH", "/api/v1/servers/localhost/zones/"+zone+".", map[string]any{
		"rrsets": []map[string]any{{"name": rec.Name, "type": rec.Type, "changetype": "DELETE"}},
	})
}
func (p *PowerDNS) EnableDNSSEC(ctx context.Context, zone string) error {
	return p.roundTrip(ctx, "PUT", "/api/v1/servers/localhost/zones/"+zone+"./cryptokeys", map[string]any{"active": true})
}
func (p *PowerDNS) DisableDNSSEC(ctx context.Context, zone string) error {
	return nil
}
func (p *PowerDNS) GetDSRecords(ctx context.Context, zone string) ([]Record, error) {
	return nil, nil
}

func (p *PowerDNS) roundTrip(ctx context.Context, method, path string, body any) error {
	if p.BaseURL == "" {
		return nil
	}
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.BaseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	c := &http.Client{Timeout: 10 * time.Second}
	res, err := c.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("powerdns %s: %s", res.Status, string(b))
	}
	return nil
}

var _ Provider = (*PowerDNS)(nil)
