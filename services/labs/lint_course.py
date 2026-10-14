#!/usr/bin/env python3
# Pre-flight for k8-labs course frontmatter, catching the YAML/schema traps the
# Go loader panics on: field lengths, ": " (mapping) in unquoted scalars, and a
# scalar that leads with a backtick (tokenizer choke). Run: python3 lint_course.py <lab>...
import re, sys, glob
LIMS = {'sub':32,'label':28,'text':40,'caption':64,'revealLabel':32}
SCALAR = ('q','t','explain','title','aha','caption','revealLabel')
bad = 0
for lab in sys.argv[1:]:
    for f in sorted(glob.glob(f'internal/course/content/{lab}/*.md')):
        fm = open(f).read().split('\n---', 2)[0]
        for ln, line in enumerate(fm.splitlines(), 1):
            for key, lim in LIMS.items():
                for m in re.finditer(r"%s:\s*'([^']*)'" % key, line):
                    if len(m.group(1)) > lim:
                        print(f"{f}:{ln} {key} {len(m.group(1))}>{lim}"); bad += 1
            m = re.match(r'\s*(?:- )?(' + '|'.join(SCALAR) + r'):\s+(.*)$', line)
            if m:
                v = m.group(2)
                if v.lstrip().startswith('`'):
                    print(f"{f}:{ln} leads with backtick: {v[:40]}"); bad += 1
                elif ': ' in v and not v.lstrip().startswith(("'", '"', '{', '[')):
                    print(f"{f}:{ln} unquoted ': ' (YAML mapping): {v[:50]}"); bad += 1
print('PREFLIGHT CLEAN' if not bad else f'{bad} preflight issues')
sys.exit(1 if bad else 0)
