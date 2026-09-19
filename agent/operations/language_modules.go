package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"
)

const languageModulesPath = "/etc/panel/language-modules.json"

type LanguageModule struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Status string `json:"status"`
	Source string `json:"source,omitempty"`
}

type languageModuleFile struct {
	Items []LanguageModule `json:"items"`
}

func languageModuleKinds() []string {
	return []string{"perl", "pear", "pecl", "ruby"}
}

func validLanguageModuleKind(kind string) bool {
	switch kind {
	case "perl", "pear", "pecl", "ruby":
		return true
	default:
		return false
	}
}

func ValidateLanguageModuleName(kind, name string) error {
	return validateLanguageModuleName(kind, name)
}

func validateLanguageModuleName(kind, name string) error {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return fmt.Errorf("invalid module name")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, "/\\ \t\n;|&$`'\"<>") {
		return fmt.Errorf("invalid module name")
	}
	switch kind {
	case "perl":
		if !perlModuleNameOK(name) {
			return fmt.Errorf("invalid Perl module name")
		}
	case "pear", "pecl", "ruby":
		if !genericModuleNameOK(name) {
			return fmt.Errorf("invalid module name")
		}
	default:
		return fmt.Errorf("unsupported module kind")
	}
	return nil
}

func perlModuleNameOK(name string) bool {
	if name == "" || !unicode.IsLetter(rune(name[0])) {
		return false
	}
	for i, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		if r == ':' && i > 0 && i < len(name)-1 {
			continue
		}
		return false
	}
	return !strings.Contains(name, ":::")
}

func genericModuleNameOK(name string) bool {
	if name == "" || !unicode.IsLetter(rune(name[0])) {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '+' {
			continue
		}
		return false
	}
	return true
}

func (h *Host) listLanguageModules(kind string) ([]LanguageModule, error) {
	if kind != "" && !validLanguageModuleKind(kind) {
		return nil, fmt.Errorf("unsupported module kind")
	}
	stored := h.loadLanguageModules()
	var out []LanguageModule
	for _, item := range stored {
		if kind != "" && item.Kind != kind {
			continue
		}
		out = append(out, item)
	}
	if out == nil {
		out = []LanguageModule{}
	}
	return out, nil
}

func (h *Host) installLanguageModule(kind, name string) (Result, error) {
	if err := validateLanguageModuleName(kind, name); err != nil {
		return Result{}, err
	}
	name = strings.TrimSpace(name)
	if !h.live() {
		if err := h.rememberLanguageModule(LanguageModule{
			Kind: kind, Name: name, Status: "installed", Source: "staged",
		}); err != nil {
			return Result{}, err
		}
		return Result{
			OK:            true,
			Message:       kind + " module " + name + " staged",
			ObservedState: "staged",
		}, nil
	}
	if err := h.installLanguageModuleLive(kind, name); err != nil {
		return Result{}, err
	}
	if err := h.rememberLanguageModule(LanguageModule{
		Kind: kind, Name: name, Status: "installed", Source: "host",
	}); err != nil {
		return Result{}, err
	}
	return Result{
		OK:            true,
		Message:       kind + " module " + name + " installed",
		ObservedState: "installed",
	}, nil
}

func (h *Host) installLanguageModuleLive(kind, name string) error {
	switch kind {
	case "pecl":
		return h.installPHPExtension(name)
	case "pear":
		return h.installPEARPackage(name)
	case "perl":
		return h.installPerlModule(name)
	case "ruby":
		return h.installRubyGem(name)
	default:
		return fmt.Errorf("unsupported module kind")
	}
}

func (h *Host) installPHPExtension(name string) error {
	var last error
	for _, version := range []string{"8.3", "8.4", "8.5"} {
		pkg := "php" + version + "-" + strings.ToLower(name)
		if err := aptInstall(pkg); err == nil {
			return nil
		} else {
			last = err
		}
	}
	out, err := runFixed("/usr/bin/pecl", "install", name)
	if err != nil {
		if last != nil {
			return fmt.Errorf("apt/pecl: %v; pecl: %s", last, strings.TrimSpace(string(out)))
		}
		return fmt.Errorf("pecl: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *Host) installPEARPackage(name string) error {
	if err := aptInstall("php-pear"); err != nil {
		// pear may already be present; still try the install
		_ = err
	}
	out, err := runFixed("/usr/bin/pear", "install", name)
	if err != nil {
		return fmt.Errorf("pear: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *Host) installPerlModule(name string) error {
	pkg := perlAptPackage(name)
	if err := aptInstall(pkg); err == nil {
		return nil
	}
	out, err := runFixed("/usr/bin/cpan", "-T", name)
	if err != nil {
		return fmt.Errorf("cpan: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (h *Host) installRubyGem(name string) error {
	out, err := runFixed("/usr/bin/gem", "install", "--no-document", name)
	if err != nil {
		return fmt.Errorf("gem: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func aptInstall(pkg string) error {
	out, err := runFixedEnv(
		context.Background(),
		"/usr/bin/apt-get",
		[]string{"DEBIAN_FRONTEND=noninteractive"},
		10*time.Minute,
		nil,
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "Dpkg::Lock::Timeout=120",
		"install", "-y", pkg,
	)
	if err != nil {
		return fmt.Errorf("apt-get: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func perlAptPackage(name string) string {
	s := strings.ToLower(strings.ReplaceAll(name, "::", "-"))
	s = strings.ReplaceAll(s, "_", "-")
	return "lib" + s + "-perl"
}

func (h *Host) loadLanguageModules() []LanguageModule {
	raw, err := h.readManaged(languageModulesPath, 1<<20)
	if err != nil || len(raw) == 0 {
		return nil
	}
	var file languageModuleFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil
	}
	return file.Items
}

func (h *Host) rememberLanguageModule(item LanguageModule) error {
	items := h.loadLanguageModules()
	replaced := false
	for i, existing := range items {
		if existing.Kind == item.Kind && strings.EqualFold(existing.Name, item.Name) {
			items[i] = item
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, item)
	}
	body, err := json.MarshalIndent(languageModuleFile{Items: items}, "", "  ")
	if err != nil {
		return err
	}
	_, err = h.ApplyFile(languageModulesPath, append(body, '\n'), 0o644)
	return err
}
