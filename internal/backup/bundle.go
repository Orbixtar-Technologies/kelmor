package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"path"
	"strings"
)

type DBDump struct {
	Engine string
	Name   string
	SQL    []byte
}

type MailDump struct {
	Domain string
	Local  string
	TarGz  []byte
}

func PackV2(homeTarGz []byte, dbs []DBDump, mail []MailDump) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := writeTarBytes(tw, "home.tar.gz", homeTarGz); err != nil {
		return nil, err
	}
	for _, d := range dbs {
		if d.Engine == "" || d.Name == "" {
			continue
		}
		name := path.Join("databases", d.Engine, d.Name+".sql")
		if err := writeTarBytes(tw, name, d.SQL); err != nil {
			return nil, err
		}
	}
	for _, m := range mail {
		if m.Domain == "" || m.Local == "" {
			continue
		}
		name := path.Join("mail", m.Domain, m.Local+".tar.gz")
		if err := writeTarBytes(tw, name, m.TarGz); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func SplitV2(raw []byte) (home []byte, dbs []DBDump, mail []MailDump, err error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, nil, nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, nil, nil, nextErr
		}
		name := path.Clean(hdr.Name)
		if strings.HasPrefix(name, "..") || path.IsAbs(name) {
			return nil, nil, nil, fmt.Errorf("archive traversal")
		}
		body, readErr := io.ReadAll(tr)
		if readErr != nil {
			return nil, nil, nil, readErr
		}
		switch {
		case name == "home.tar.gz":
			home = body
		case strings.HasPrefix(name, "databases/") && strings.HasSuffix(name, ".sql"):
			rest := strings.TrimPrefix(name, "databases/")
			engine, file, ok := strings.Cut(rest, "/")
			if !ok {
				continue
			}
			dbs = append(dbs, DBDump{Engine: engine, Name: strings.TrimSuffix(file, ".sql"), SQL: body})
		case strings.HasPrefix(name, "mail/") && strings.HasSuffix(name, ".tar.gz"):
			rest := strings.TrimPrefix(name, "mail/")
			domain, file, ok := strings.Cut(rest, "/")
			if !ok {
				continue
			}
			mail = append(mail, MailDump{Domain: domain, Local: strings.TrimSuffix(file, ".tar.gz"), TarGz: body})
		}
	}
	if len(home) == 0 {
		return nil, nil, nil, fmt.Errorf("bundle missing home.tar.gz")
	}
	return home, dbs, mail, nil
}

func writeTarBytes(tw *tar.Writer, name string, body []byte) error {
	if body == nil {
		body = []byte{}
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o640, Size: int64(len(body))}); err != nil {
		return err
	}
	_, err := tw.Write(body)
	return err
}
