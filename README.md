# jqgo: variations on experimental Go clones of jq

The main jqgo program is a path-aware, partial-parsing JSON
implementation.

The "control" program uses Go's builtin (non-path-aware) JSON parsing
library with a path implementation on top.

And the ultimate control is `jq` itself.

# Usage:

```bash
$ go build
$ cat testdata/everything.json
{"a": "1", "x": [2,3, [99, {"aa": 12}]], "d": {"a": 2, "b": {"c": 1}, "x": {}}, "c": true, "b": 2}
$ cat testdata/everything.json| ./jqgo 'x.0'
2
```

# Limitations

TONS!

1. Can't output a JSON object or array currently, end of path must be a scalar
1. Can't filter/map/etc
1. Can't fetch multiple items

# Benchmarks

## Cat

| Command                                                  |    Mean [ms] | Min [ms] | Max [ms] |    Relative |
|:---------------------------------------------------------|-------------:|---------:|---------:|------------:|
| `cat large-file.json \| ./jqgo '.created_at'`            |  205.7 ± 0.6 |    204.9 |    207.4 |        1.00 |
| `cat large-file.json \| ./control/control '.created_at'` | 337.6 ± 20.8 |    319.2 |    380.7 | 1.64 ± 0.10 |
| `cat large-file.json \| jq '.created_at'`                |  456.6 ± 1.0 |    455.4 |    458.6 | 2.22 ± 0.01 |

## Gunzip

| Command                                                           |    Mean [ms] | Min [ms] | Max [ms] |    Relative |
|:------------------------------------------------------------------|-------------:|---------:|---------:|------------:|
| `gunzip -c large-file.json.gz \| ./jqgo '.created_at'`            | 215.6 ± 26.1 |    203.7 |    277.1 |        1.00 |
| `gunzip -c large-file.json.gz \| ./control/control '.created_at'` |  326.2 ± 4.8 |    319.0 |    336.2 | 1.51 ± 0.18 |
| `gunzip -c large-file.json.gz \| jq '.created_at'`                |  456.0 ± 1.4 |    454.2 |    459.1 | 2.12 ± 0.26 |
