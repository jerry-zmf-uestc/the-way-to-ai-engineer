# 示例： json

## Goal

Learn basic JSON encoding and decoding with `encoding/json`.

## Concepts

- struct tags such as `json:"name"` and `omitempty`
- `json.Marshal` and `json.Unmarshal`
- missing fields keep Go zero values
- default decoding ignores unknown fields
- `Decoder.DisallowUnknownFields` enables stricter input validation

## Run

```bash
go -C labs run ./part1-golang/chapter02/json
go -C labs test ./part1-golang/chapter02/json
```

## Code Map

- [main.go](main.go): shows JSON tags, encode/decode behavior, and strict
  decoding in one bounded file.
- [main_test.go](main_test.go): verifies both permissive and strict decoding
  paths.

## Project Use

JSON is the default data format for many APIs, config files, and CLI pipelines.

## Related Exercises

- [Chapter 2 · Struct and JSON Exercises](../../../../content/book/part1-golang/chapter02.md#struct-json-exercises): return to the chapter page for the
  executable practice tasks that extend this topic with step-by-step
  guidance and acceptance checks.

## Continue

Return to [Chapter 2 · Struct and JSON](../../../../content/book/part1-golang/chapter02.md#struct-json), then continue with
[project-layout](../../chapter11/project-layout) to see how these basic building
blocks start to turn into maintainable projects.
