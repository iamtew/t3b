package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Report is the result of Check — schema drift vs the embedded example.
type Report struct {
	Path    string
	Missing []string // dotted keys in example but not in the Meat Bag file
	Unknown []string // keys in the file that Config does not know (informational)
	Valid   bool     // false when Validate fails after defaults
	LoadErr string   // Validate / decode message when Valid is false
}

// OK reports whether the file loads cleanly and has every example key.
func (r Report) OK() bool {
	return r.Valid && len(r.Missing) == 0
}

// Check compares path to the embedded example key set and validates the file.
// Omitted keys still get runtime defaults — this is for Meat Bags who want an
// up-to-date editable conf, not for the bot to refuse to start.
func Check(path string) (Report, error) {
	if path == "" {
		path = DefaultPath
	}
	rep := Report{Path: path}

	raw, err := os.ReadFile(path)
	if err != nil {
		return rep, fmt.Errorf("read config %q: %w", path, err)
	}

	exampleKeys, err := keysFromTOML(string(exampleConf))
	if err != nil {
		return rep, fmt.Errorf("parse embedded example: %w", err)
	}
	exampleKeys = leafKeys(exampleKeys)
	userKeys, err := keysFromTOML(string(raw))
	if err != nil {
		return rep, fmt.Errorf("parse config %q: %w", path, err)
	}

	userSet := keySet(userKeys)
	for _, k := range exampleKeys {
		dot := keyDot(k)
		if !userSet[dot] {
			rep.Missing = append(rep.Missing, dot)
		}
	}
	sort.Strings(rep.Missing)

	// Unknown = present in file but not decoded into Config (typos / future junk).
	var cfg Config
	md, err := toml.Decode(string(raw), &cfg)
	if err != nil {
		return rep, fmt.Errorf("parse config %q: %w", path, err)
	}
	for _, k := range md.Undecoded() {
		rep.Unknown = append(rep.Unknown, keyDot(k))
	}
	sort.Strings(rep.Unknown)

	cfg.applyDefaults()
	if err := cfg.Validate(); err != nil {
		rep.Valid = false
		rep.LoadErr = err.Error()
	} else {
		rep.Valid = true
	}
	return rep, nil
}

// MergeMissing appends or inserts keys that Check reports as missing, using
// values from the embedded example. Existing lines and comments are preserved.
// Returns the dotted keys that were added.
func MergeMissing(path string) ([]string, error) {
	if path == "" {
		path = DefaultPath
	}
	rep, err := Check(path)
	if err != nil {
		return nil, err
	}
	if len(rep.Missing) == 0 {
		return nil, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	exampleVals, err := exampleValueMap()
	if err != nil {
		return nil, err
	}

	content := string(raw)
	var added []string

	// Whole missing tables first (cleaner than scattering dotted keys).
	missingTables := wholeMissingTables(rep.Missing, content)
	for _, table := range missingTables {
		block := exampleSectionBlock(table)
		if block == "" {
			continue
		}
		content = strings.TrimRight(content, "\n") + "\n\n# --- added by t3b check config (" +
			time.Now().Format("2006-01-02") + ") ---\n" + strings.TrimSpace(block) + "\n"
		for _, dot := range rep.Missing {
			if dot == table || strings.HasPrefix(dot, table+".") {
				added = append(added, dot)
			}
		}
	}
	addedSet := keySetStrings(added)

	// Remaining missing keys: insert into an existing [table] (or top-level).
	for _, dot := range rep.Missing {
		if addedSet[dot] {
			continue
		}
		table, key := splitDotKey(dot)
		val, ok := lookupExampleValue(exampleVals, table, key)
		if !ok {
			continue
		}
		line := key + " = " + formatTOMLValue(val)
		var err error
		content, err = insertTOMLLine(content, table, line)
		if err != nil {
			return nil, fmt.Errorf("merge %s: %w", dot, err)
		}
		added = append(added, dot)
	}

	if len(added) == 0 {
		return nil, fmt.Errorf("could not merge missing keys: %s", strings.Join(rep.Missing, ", "))
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("write temp config: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return nil, fmt.Errorf("replace config %q: %w", path, err)
	}
	sort.Strings(added)
	return added, nil
}

func keysFromTOML(src string) ([]toml.Key, error) {
	var sink map[string]interface{}
	md, err := toml.Decode(src, &sink)
	if err != nil {
		return nil, err
	}
	return md.Keys(), nil
}

func keyDot(k toml.Key) string {
	return strings.Join([]string(k), ".")
}

// leafKeys drops intermediate table paths (e.g. "automode" when "automode.enabled" exists).
func leafKeys(keys []toml.Key) []toml.Key {
	dots := make([]string, len(keys))
	for i, k := range keys {
		dots[i] = keyDot(k)
	}
	var out []toml.Key
	for i, d := range dots {
		prefix := false
		for _, o := range dots {
			if o != d && strings.HasPrefix(o, d+".") {
				prefix = true
				break
			}
		}
		if !prefix {
			out = append(out, keys[i])
		}
	}
	return out
}

func keySet(keys []toml.Key) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[keyDot(k)] = true
	}
	return m
}

func keySetStrings(keys []string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func splitDotKey(dot string) (table, key string) {
	i := strings.LastIndex(dot, ".")
	if i < 0 {
		return "", dot
	}
	return dot[:i], dot[i+1:]
}

// wholeMissingTables returns top-level table names where every example key under
// that table is missing and the file has no [table] header yet.
func wholeMissingTables(missing []string, content string) []string {
	byTable := map[string]int{}
	for _, dot := range missing {
		table, _ := splitDotKey(dot)
		if table == "" || strings.Contains(table, ".") {
			continue // skip top-level and nested arrays-of-tables for now
		}
		byTable[table]++
	}
	exampleKeys, _ := keysFromTOML(string(exampleConf))
	exampleKeys = leafKeys(exampleKeys)
	exampleCount := map[string]int{}
	for _, k := range exampleKeys {
		if len(k) >= 2 {
			exampleCount[k[0]]++
		}
	}
	var out []string
	for table, n := range byTable {
		if n < exampleCount[table] {
			continue
		}
		if hasTableHeader(content, table) {
			continue
		}
		out = append(out, table)
	}
	sort.Strings(out)
	return out
}

func hasTableHeader(content, table string) bool {
	want := "[" + table + "]"
	for _, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// exampleSectionBlock returns the [table] … block from the embedded example
// (through the line before the next [section]), including a short lead comment.
func exampleSectionBlock(table string) string {
	lines := strings.Split(string(exampleConf), "\n")
	want := "[" + table + "]"
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == want {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	// Include immediately preceding comment / blank lines (doc for the table).
	from := start
	for from > 0 {
		prev := strings.TrimSpace(lines[from-1])
		if prev == "" || strings.HasPrefix(prev, "#") {
			from--
			continue
		}
		break
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") && !strings.HasPrefix(trim, "[[") {
			end = i
			break
		}
	}
	return strings.Join(lines[from:end], "\n")
}

func exampleValueMap() (map[string]interface{}, error) {
	var m map[string]interface{}
	if _, err := toml.Decode(string(exampleConf), &m); err != nil {
		return nil, err
	}
	return m, nil
}

func lookupExampleValue(m map[string]interface{}, table, key string) (interface{}, bool) {
	if table == "" {
		v, ok := m[key]
		return v, ok
	}
	parts := strings.Split(table, ".")
	cur := m
	for _, p := range parts {
		next, ok := cur[p]
		if !ok {
			return nil, false
		}
		nm, ok := next.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur = nm
	}
	v, ok := cur[key]
	return v, ok
}

func formatTOMLValue(v interface{}) string {
	switch x := v.(type) {
	case bool:
		return strconv.FormatBool(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case string:
		return strconv.Quote(x)
	case []interface{}:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = formatTOMLValue(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return strconv.Quote(fmt.Sprint(v))
	}
}

// insertTOMLLine inserts line into [table] (or top-level when table is empty).
func insertTOMLLine(content, table, line string) (string, error) {
	lines := strings.Split(content, "\n")
	if table == "" {
		// Before first [table] header.
		idx := len(lines)
		for i, l := range lines {
			trim := strings.TrimSpace(l)
			if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
				idx = i
				break
			}
		}
		return insertAt(lines, idx, line), nil
	}

	want := "[" + table + "]"
	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == want {
			start = i
			break
		}
	}
	if start < 0 {
		return "", fmt.Errorf("table [%s] not found", table)
	}
	// Insert after existing keys in the table (before next [section] or EOF).
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trim := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") && !strings.HasPrefix(trim, "[[") {
			end = i
			break
		}
	}
	// Prefer inserting after the last non-empty, non-comment line in the table.
	insertAtIdx := end
	for i := end - 1; i > start; i-- {
		trim := strings.TrimSpace(lines[i])
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "#") {
			continue
		}
		insertAtIdx = i + 1
		break
	}
	if insertAtIdx == end && end > start+1 {
		// Table was only comments — put after header.
		insertAtIdx = start + 1
	}
	if insertAtIdx == end && end == start+1 {
		insertAtIdx = start + 1
	}
	return insertAt(lines, insertAtIdx, line), nil
}

func insertAt(lines []string, idx int, line string) string {
	if idx < 0 {
		idx = 0
	}
	if idx > len(lines) {
		idx = len(lines)
	}
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:idx]...)
	out = append(out, line)
	out = append(out, lines[idx:]...)
	return strings.Join(out, "\n")
}
