package course

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Course content is embedded, not read from disk at runtime: the backend runs
// from a distroless image with no repo checkout, and embedding is what makes
// "content can never be newer than the renderer" true - they are the same
// artifact. Same mechanism the module's manifest bundle already uses.
//
//go:embed content
var contentFS embed.FS

// courses is built once at init. A malformed course file panics the process at
// startup rather than degrading to a blank lesson pane for a student: content
// is compiled in, so a break here is a build-time defect that CI must catch,
// and failing loudly at boot is how it gets caught if CI ever misses it.
var courses = mustLoadAll()

// Get returns a module's course. ok is false for a module with no course
// authored yet, which the API turns into an empty lesson rather than an error -
// a lab without a lesson is incomplete, not broken.
func Get(moduleSlug string) (Course, bool) {
	c, ok := courses[moduleSlug]
	return c, ok
}

// Slugs lists every module that has a course, sorted. Used by the lint and its
// tests to walk all content without hardcoding a list that drifts.
func Slugs() []string {
	out := make([]string, 0, len(courses))
	for slug := range courses {
		out = append(out, slug)
	}
	sort.Strings(out)
	return out
}

func mustLoadAll() map[string]Course {
	loaded, err := LoadAll(contentFS, "content")
	if err != nil {
		panic("course content failed to load: " + err.Error())
	}
	return loaded
}

// LoadAll walks <root>/<module-slug>/*.md. Exported and taking an fs.FS so
// tests can feed it a fixture tree rather than the embedded one.
func LoadAll(fsys fs.FS, root string) (map[string]Course, error) {
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return nil, err
	}
	out := map[string]Course{}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := LoadCourse(fsys, root+"/"+e.Name(), e.Name())
		if err != nil {
			return nil, err
		}
		out[e.Name()] = c
	}
	return out, nil
}

// LoadCourse reads one module's steps and returns them in file order.
func LoadCourse(fsys fs.FS, dir, moduleSlug string) (Course, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return Course{}, err
	}

	var steps []Step
	seenIndex := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := fs.ReadFile(fsys, dir+"/"+e.Name())
		if err != nil {
			return Course{}, err
		}
		step, err := Parse(e.Name(), data)
		if err != nil {
			return Course{}, err
		}
		// Duplicate order prefixes make the sort below unstable, so two steps
		// could swap places between builds - a lesson that teaches a term after
		// it was used, intermittently.
		if prev, dup := seenIndex[step.Index]; dup {
			return Course{}, fmt.Errorf("%s/%s: order prefix %02d already used by %s", dir, e.Name(), step.Index, prev)
		}
		seenIndex[step.Index] = e.Name()
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return Course{}, fmt.Errorf("%s: no course steps found", dir)
	}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Index < steps[j].Index })
	return Course{ModuleSlug: moduleSlug, Steps: steps}, nil
}
