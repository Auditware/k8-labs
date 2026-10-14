package course

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// services/labs/schema/step.schema.json is declared to be the single authority
// for the course format. Without this test it would be documentation pretending
// to be enforcement: the Go validate() in parse.go restates the same rules, and
// nothing would notice when the two drift apart.
//
// So: validate every shipped course FILE against the schema, and separately
// prove the two validators agree by feeding known-bad documents to both.
const schemaPath = "../../../labs/schema/step.schema.json"

func loadSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(schemaPath)
	if err != nil {
		t.Fatalf("open schema: %v", err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("step.schema.json", doc); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	s, err := c.Compile("step.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return s
}

// schemaDoc turns a course file into the object the schema describes: the
// frontmatter fields, plus the Markdown body as `body`. Deliberately built from
// the raw file rather than from a parsed Step - a Step has already been through
// Go's own validation, so validating it would only prove Go agrees with itself.
func schemaDoc(t *testing.T, data []byte) map[string]any {
	t.Helper()
	front, body, err := splitFrontmatter(data)
	if err != nil {
		t.Fatalf("split frontmatter: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(front, &doc); err != nil {
		t.Fatalf("parse frontmatter: %v", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	if b := strings.TrimSpace(string(body)); b != "" {
		doc["body"] = b
	}
	// Round-trip through JSON so the value types match what the validator
	// expects (YAML gives int where JSON Schema reasons about number).
	blob, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(blob, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func TestShippedContentMatchesSchema(t *testing.T) {
	schema := loadSchema(t)

	root := "content"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read content dir: %v", err)
	}
	checked := 0
	for _, mod := range entries {
		if !mod.IsDir() {
			continue
		}
		dir := filepath.Join(root, mod.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, f := range files {
			if !strings.HasSuffix(f.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, f.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if err := schema.Validate(schemaDoc(t, data)); err != nil {
				t.Errorf("%s does not match the schema:\n%v", path, err)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no course files were checked - the walk found nothing")
	}
	t.Logf("validated %d course files against the schema", checked)
}

// The schema and Go's parse-time validation must not drift. For each bad
// document below, BOTH must reject it - if only one does, the other has a hole.
func TestSchemaAndParserAgreeOnRejection(t *testing.T) {
	schema := loadSchema(t)

	bad := map[string]string{
		"unknown top-level field": `---
kind: concept
ey: A
eyt: B
title: T
aha: A
revealLable: typo
---
Body.
`,
		"tag and chip together": `---
kind: concept
ey: A
eyt: B
tag: x
chip:
  label: y
  tone: accent
title: T
aha: A
---
Body.
`,
		"bad chip tone": `---
kind: concept
ey: A
eyt: B
chip:
  label: y
  tone: purple
title: T
aha: A
---
Body.
`,
		"concept missing aha": `---
kind: concept
ey: A
eyt: B
title: T
---
Body.
`,
		"quiz with one option": `---
kind: quiz
ey: A
eyt: B
q: Q?
opts:
  - t: only one
    correct: true
explain: E
---
`,
		"unknown shape type": `---
kind: concept
ey: A
eyt: B
title: T
aha: A
diagram:
  shapes:
    - {type: blob, x: 10, y: 10}
---
Body.
`,
		"unknown shape field": `---
kind: concept
ey: A
eyt: B
title: T
aha: A
diagram:
  shapes:
    - {type: label, x: 10, y: 40, text: hi, colour: red}
---
Body.
`,
		"flag missing body": `---
kind: flag
ey: A
eyt: B
title: T
---
`,
		"flag carrying quiz fields": `---
kind: flag
ey: A
eyt: B
title: T
q: Q?
opts:
  - t: one
    correct: true
  - t: two
explain: E
---
Body.
`,
		"unknown kind": `---
kind: capstone
ey: A
eyt: B
title: T
---
Body.
`,
	}

	for name, src := range bad {
		t.Run(name, func(t *testing.T) {
			if err := schema.Validate(schemaDoc(t, []byte(src))); err == nil {
				t.Error("schema accepted it; the schema has a hole")
			}
			if _, err := Parse("01-x.md", []byte(src)); err == nil {
				t.Error("parser accepted it; parse.go has a hole")
			}
		})
	}
}

// And the healthy shape must pass both, or the checks above are only proving
// that everything fails.
func TestSchemaAcceptsAValidStep(t *testing.T) {
	schema := loadSchema(t)
	good := `---
kind: concept
ey: CONCEPT 1
eyt: Thing
chip:
  label: KEY
  tone: accent
title: A plain claim
aha: An analogy
revealLabel: the takeaway
diagram:
  caption: a caption
  shapes:
    - {type: frame, x: 40, y: 60, w: 400, h: 100, label: cluster}
    - {type: box, x: 60, y: 80, w: 150, h: 60, label: thing, sub: a subtitle}
    - {type: wire, from: {x: 210, y: 110}, to: {x: 260, y: 110}, tone: accent}
    - {type: label, x: 400, y: 200, text: a note, anchor: middle}
    - {type: seal, cx: 500, cy: 110, r: 24, broken: true}
---
Body with a ` + "`term`" + `.
`
	if err := schema.Validate(schemaDoc(t, []byte(good))); err != nil {
		t.Errorf("schema rejected a valid step:\n%v", err)
	}
	if _, err := Parse("01-x.md", []byte(good)); err != nil {
		t.Errorf("parser rejected a valid step: %v", err)
	}
}

// A flag capstone - title + body, no answer field, optional reveal - must pass
// both validators. The expected flag is never in the content, so there is
// nothing here for a schema field to carry.
func TestSchemaAcceptsAFlagStep(t *testing.T) {
	schema := loadSchema(t)
	good := `---
kind: flag
ey: CAPTURE
eyt: The flag
title: You read a secret you were never meant to reach
aha: Submitting it is the proof the boundary was only ever a suggestion.
revealLabel: why this counts
---
Paste the ` + "`k8labs{...}`" + ` value you read out of the Secret.
`
	if err := schema.Validate(schemaDoc(t, []byte(good))); err != nil {
		t.Errorf("schema rejected a valid flag step:\n%v", err)
	}
	if _, err := Parse("13-flag.md", []byte(good)); err != nil {
		t.Errorf("parser rejected a valid flag step: %v", err)
	}
}
