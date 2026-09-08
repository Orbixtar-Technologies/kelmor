package operations

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

func CallUnix(ctx context.Context, sock string, req Request) (any, error) {
	d := net.Dialer{Timeout: 10 * time.Second}
	c, err := d.DialContext(ctx, "unix", sock)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if _, err := c.Write(append(b, '\n')); err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(c)
	sc.Buffer(make([]byte, 0, 64*1024), MaxRPCLine)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("agent closed connection")
	}
	var out struct {
		OK     bool            `json:"ok"`
		Error  string          `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(sc.Bytes(), &out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("%s", out.Error)
	}
	var v any
	_ = json.Unmarshal(out.Result, &v)
	return v, nil
}
