# grit
A command-line tool to count lines contributed by authors in Git repositories.

## Installation

```bash
go install github.com/natfarleydev/grit@latest
```

## Usage

Count lines changed by authors matching a regex pattern:
```bash
grit count lines --author-regex <pattern> [paths...]
```

View git log:
```bash
grit log [paths...]
```

Examples:
```bash
# Count lines by authors named either Nathanael or Mirabel
grit count lines --author-regex '(Nathanael|Mirabel)' ./

# Count lines from a specific email domain
grit count lines --author-regex '@company\.com$' ./ ../other_repo

# Count lines from multiple repositories
grit count lines --author-regex 'John' ./ ../other_repo
```

The output shows the total lines added and removed by the matching authors for the current day:
```
+141/-38
```

## Requirements
- Go 1.21 or later
