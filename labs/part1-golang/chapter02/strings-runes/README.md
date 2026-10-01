# 示例： strings runes

## Goal

Learn that a Go string is a read-only byte sequence, while `range` decodes UTF-8 into runes.

## Concepts

- `len` returns byte length
- `utf8.RuneCountInString` counts decoded runes
- `range` over a string yields runes
- `strings.Fields`, `strings.Join`, and `strings.ReplaceAll`

## Run

```bash
go -C labs run ./part1-golang/chapter02/strings-runes
go -C labs test ./part1-golang/chapter02/strings-runes
```

## Code Map

- [main.go](main.go): contrasts byte length, rune iteration, and word-level
  string helpers in one bounded example.
- [main_test.go](main_test.go): keeps the Unicode-sensitive behavior explicit.

## Project Use

Text processing code should be explicit about whether it works with bytes, runes, or normalized words.

## Related Exercises

- [Chapter 2 · Strings and runes Exercises](../../../../book/part1-golang/chapter02.md#strings-runes-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 2 · Strings and runes](../../../../book/part1-golang/chapter02.md#strings-runes), then continue with
[arrays-slices](../arrays-slices) to see how grouped data builds on these
basic types.
